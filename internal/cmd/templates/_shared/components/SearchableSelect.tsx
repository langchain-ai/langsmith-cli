// Canonical single source. apps_init.go copies this into a scaffolded app's
// src/ at generation time, and only if the template imports it — there are no
// standing per-template copies and no sync script.
//
// A thin wrapper over the design system's <Typeahead> that adds what a
// workspace-scoped picker needs and the component itself deliberately leaves
// out: server-side search (LangSmith list endpoints filter by name, so
// filtering client-side would only ever see the first page) and paging as the
// list is scrolled.
import { useCallback, useEffect, useRef, useState } from 'react';

import { Spinner } from '@/components/langsmith/design-system/components/Spinner';
import { Text } from '@/components/langsmith/design-system/components/Text';
import { Typeahead } from '@/components/langsmith/design-system/components/Typeahead';

const PAGE_SIZE = 25;
const SEARCH_DEBOUNCE_MS = 250;

export interface SearchableSelectItem {
  id: string;
  name: string;
}

interface Props<T extends SearchableSelectItem> {
  id?: string;
  value: string;
  onSelect: (item: T) => void;
  /** Fetches one page of results for the given search term. Called with
   * offset 0 the first time the control is focused and whenever the search
   * term changes, and with an increasing offset as the list is scrolled. A
   * page shorter than PAGE_SIZE (25) is treated as the last page. */
  fetchPage: (search: string, offset: number, limit: number) => Promise<T[]>;
  placeholder: string;
  searchPlaceholder?: string;
  emptyLabel?: string;
  disabled?: boolean;
  className?: string;
}

export function SearchableSelect<T extends SearchableSelectItem>({
  id,
  value,
  onSelect,
  fetchPage,
  placeholder,
  searchPlaceholder,
  emptyLabel = 'No results',
  disabled,
  className,
}: Props<T>) {
  const [items, setItems] = useState<T[]>([]);
  const [search, setSearch] = useState('');
  const [selectedItem, setSelectedItem] = useState<T | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  // Nothing is fetched until the control is first focused — a page of apps
  // full of pickers shouldn't spend its rate limit before anyone clicks.
  const [activated, setActivated] = useState(false);
  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // Guards against a slow, stale page landing after a newer search.
  const requestIdRef = useRef(0);

  // (Re)load the first page on activation and on every (debounced) search.
  useEffect(() => {
    if (!activated) return;
    const timer = setTimeout(() => {
      const myRequest = ++requestIdRef.current;
      setLoading(true);
      fetchPage(search, 0, PAGE_SIZE)
        .then((page) => {
          if (requestIdRef.current !== myRequest) return;
          setItems(page);
          setHasMore(page.length === PAGE_SIZE);
        })
        .catch((e) => console.error('Failed to load options', e))
        .finally(() => {
          if (requestIdRef.current === myRequest) setLoading(false);
        });
    }, search ? SEARCH_DEBOUNCE_MS : 0);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fetchPage identity isn't expected to change per render
  }, [activated, search]);

  const loadMore = useCallback(() => {
    if (loadingMore || loading || !hasMore) return;
    const myRequest = requestIdRef.current;
    setLoadingMore(true);
    fetchPage(search, items.length, PAGE_SIZE)
      .then((page) => {
        if (requestIdRef.current !== myRequest) return;
        setItems((prev) => {
          const seen = new Set(prev.map((i) => i.id));
          return [...prev, ...page.filter((i) => !seen.has(i.id))];
        });
        setHasMore(page.length === PAGE_SIZE);
      })
      .catch((e) => console.error('Failed to load more options', e))
      .finally(() => setLoadingMore(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasMore, items.length, loading, loadingMore, search]);

  // Paging is driven by the footer scrolling into view inside the open list.
  useEffect(() => {
    const target = sentinelRef.current;
    if (!target || !hasMore) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) loadMore();
      },
      { rootMargin: '100px' }
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [hasMore, loadMore]);

  // selectedItem is a cache for the closed-state label — if the parent resets
  // `value` out from under us (e.g. a downstream selection clearing when an
  // upstream one changes), only trust the cache while it still matches.
  //
  // With a `value` we have no object for yet (an app rendered with a
  // preselected id, before anything has been fetched) fall back to showing the
  // raw id, so the control never reads as empty when something is selected.
  const resolvedItem: T | string | undefined =
    (selectedItem?.id === value ? selectedItem : items.find((i) => i.id === value)) ??
    (value || undefined);

  return (
    <Typeahead<T>
      inputId={id}
      className={className}
      options={items}
      value={resolvedItem}
      disabled={disabled}
      placeholder={resolvedItem ? placeholder : (searchPlaceholder ?? placeholder)}
      emptyText={loading ? 'Loading…' : emptyLabel}
      getOptionLabel={(item) => (typeof item === 'string' ? item : item.name)}
      getOptionValue={(item) => (typeof item === 'string' ? item : item.id)}
      displaySelectedValueWhenInputEmpty
      clearOnBlur
      disableClearable
      onFocus={() => setActivated(true)}
      onInputChange={setSearch}
      onChange={(next) => {
        if (!next || typeof next === 'string') return;
        setSelectedItem(next);
        onSelect(next);
      }}
      emptyState={
        loading ? (
          <span className="flex items-center gap-space-2">
            <Spinner size="xs" />
            <Text variant="sm" color="secondary">
              Loading…
            </Text>
          </span>
        ) : undefined
      }
      listFooter={
        hasMore ? (
          <div ref={sentinelRef} className="flex items-center justify-center py-space-2">
            {loadingMore && <Spinner size="xs" />}
          </div>
        ) : null
      }
    />
  );
}
