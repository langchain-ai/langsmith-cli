// Canonical single source. apps_init.go copies this into a scaffolded app's
// src/ at generation time, and only if the template imports it — there are no
// standing per-template copies and no sync script. Deliberately
// self-contained (own cn()/chevron, no external icon package) so it drops
// into any template unmodified regardless of what other dependencies that
// template happens to have.
import { useEffect, useRef, useState } from 'react';

function cn(...classes: (string | false | null | undefined)[]): string {
  return classes.filter(Boolean).join(' ');
}

const PAGE_SIZE = 25;

function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className={className}>
      <path d="m6 9 6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export interface SearchableSelectItem {
  id: string;
  name: string;
}

interface Props<T extends SearchableSelectItem> {
  id?: string;
  value: string;
  onSelect: (item: T) => void;
  /** Fetches one page of results for the given search term. Called with
   * offset 0 whenever the dropdown opens or the search term changes, and
   * with an increasing offset as the user scrolls the list. A page shorter
   * than PAGE_SIZE (25) is treated as the last page. */
  fetchPage: (search: string, offset: number, limit: number) => Promise<T[]>;
  placeholder: string;
  searchPlaceholder?: string;
  emptyLabel?: string;
  disabled?: boolean;
  className?: string;
}

// A native <select> can't hold a search box or paginate its own options, so
// this is a small custom combobox: closed-state button + an open popover
// with a search input on top and an infinite-scrolling list below. Used
// anywhere an app picks one item from a workspace-scoped list (queues,
// projects, datasets, ...) — see the other templates' copies of this file.
export function SearchableSelect<T extends SearchableSelectItem>({
  id,
  value,
  onSelect,
  fetchPage,
  placeholder,
  searchPlaceholder = 'Search by name…',
  emptyLabel = 'No results',
  disabled,
  className,
}: Props<T>) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [items, setItems] = useState<T[]>([]);
  const [selectedItem, setSelectedItem] = useState<T | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const listRef = useRef<HTMLDivElement | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  // Guards against a slow, stale page landing after a newer search/open.
  const requestIdRef = useRef(0);

  // (Re)load the first page whenever the dropdown opens or the search term
  // changes while it's open.
  useEffect(() => {
    if (!open) return;
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fetchPage identity isn't expected to change per render
  }, [open, search]);

  function loadMore() {
    if (loadingMore || loading || !hasMore) return;
    const myRequest = requestIdRef.current;
    const offset = items.length;
    setLoadingMore(true);
    fetchPage(search, offset, PAGE_SIZE)
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
  }

  // Infinite scroll within the open list.
  useEffect(() => {
    if (!open || !hasMore) return;
    const root = listRef.current;
    const target = sentinelRef.current;
    if (!root || !target) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) loadMore();
      },
      { root, rootMargin: '100px' }
    );
    observer.observe(target);
    return () => observer.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, hasMore, items.length]);

  // Close on outside click / Escape; focus the search box on open.
  useEffect(() => {
    if (!open) return;
    function handlePointerDown(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false);
    }
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    searchInputRef.current?.focus();
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  function handleToggle() {
    if (disabled) return;
    setOpen((prev) => {
      const next = !prev;
      if (next) setSearch('');
      return next;
    });
  }

  function handleSelect(item: T) {
    setSelectedItem(item);
    onSelect(item);
    setOpen(false);
  }

  // selectedItem is a cache for the closed-button label — if the parent
  // resets `value` out from under us (e.g. a downstream selection clearing
  // when an upstream one changes), only trust the cache while it still
  // matches the current value, so the label doesn't show stale text.
  const resolvedItem = selectedItem?.id === value ? selectedItem : null;
  const showPlaceholder = !resolvedItem && !value;
  const label = resolvedItem?.name ?? (value || placeholder);

  return (
    <div ref={containerRef} className={cn('relative min-w-0 max-w-[420px] flex-1', className)}>
      <button
        id={id}
        type="button"
        disabled={disabled}
        onClick={handleToggle}
        className="flex w-full items-center justify-between gap-2 rounded-md border border-secondary bg-primary px-3 py-1.5 text-left text-sm text-primary focus:border-brand focus:outline-none disabled:opacity-60"
      >
        <span className={cn('min-w-0 truncate', showPlaceholder && 'text-tertiary')}>{label}</span>
        <ChevronDownIcon className="h-4 w-4 shrink-0 text-tertiary" />
      </button>

      {open && (
        <div className="absolute left-0 right-0 top-full z-20 mt-1 overflow-hidden rounded-md border border-secondary bg-primary shadow-lg">
          <div className="border-b border-secondary p-1.5">
            <input
              ref={searchInputRef}
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={searchPlaceholder}
              className="w-full rounded border border-secondary bg-primary px-2 py-1 text-sm text-primary focus:border-brand focus:outline-none"
            />
          </div>
          <div ref={listRef} className="max-h-[260px] overflow-auto py-1">
            {loading ? (
              <div className="px-3 py-4 text-center text-sm text-tertiary">Loading…</div>
            ) : items.length === 0 ? (
              <div className="px-3 py-4 text-center text-sm text-tertiary">{emptyLabel}</div>
            ) : (
              <>
                {items.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => handleSelect(item)}
                    className={cn(
                      'block w-full truncate px-3 py-1.5 text-left text-sm hover:bg-surface-level-1-hover',
                      item.id === value ? 'bg-selected text-primary' : 'text-secondary'
                    )}
                  >
                    {item.name}
                  </button>
                ))}
                {hasMore && (
                  <div ref={sentinelRef} className="px-3 py-2 text-center text-xs text-tertiary">
                    {loadingMore ? 'Loading more…' : ''}
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
