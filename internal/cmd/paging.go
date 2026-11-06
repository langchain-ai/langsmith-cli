package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

// cursorFlagUsage describes --cursor on list commands that page with a server
// cursor.
const cursorFlagUsage = `Resume after the last item of a previous call by passing its cursors.next. Passing --cursor (even "" for the first page) adds cursors.next to the JSON output`

// addCursorFlag registers --cursor on a cursor-paged list command.
func addCursorFlag(cmd *cobra.Command, cursor *string) {
	cmd.Flags().StringVar(cursor, "cursor", "", cursorFlagUsage)
}

// pageFetcher fetches one page of at most pageSize items starting at cursor
// ("" for the first page) and returns them with the server's next cursor, or
// "" on the last page.
type pageFetcher[T any] func(cursor string, pageSize int) (items []T, next string, err error)

// collectPages walks cursor pages from start until it has limit items or the
// server has no more. Each request asks for at most the items still needed, so
// the walk stops on a page boundary and the returned cursor resumes right after
// the last item returned. keep, when non-nil, drops items client-side; dropped
// items still count as consumed, so the cursor never replays them.
//
// A server that ignores the page size (v1 runs fetched by trace or ID return
// every match with no next cursor) can return more than is needed. The walk
// then stops mid-page and returns a CLI skip cursor (see skipCursor) that
// refetches that page and skips what was already returned.
func collectPages[T any](start string, limit, maxPageSize int, fetch pageFetcher[T], keep func(T) bool) ([]T, string, error) {
	var out []T
	cursor, skip := parseSkipCursor(start)
	for limit <= 0 || len(out) < limit {
		pageSize := maxPageSize
		if limit > 0 {
			pageSize = min(pageSize, limit-len(out)+skip)
		}
		items, next, err := fetch(cursor, pageSize)
		if err != nil {
			return nil, "", err
		}
		for i := skip; i < len(items); i++ {
			if limit > 0 && len(out) >= limit {
				return out, skipCursor(cursor, i), nil
			}
			if keep == nil || keep(items[i]) {
				out = append(out, items[i])
			}
		}
		skip = 0
		// A repeated cursor would loop forever; treat it as the end.
		if next == "" || next == cursor {
			next = ""
		}
		cursor = next
		if next == "" {
			break
		}
	}
	return out, cursor, nil
}

// skipCursorPrefix marks a cursor the CLI built to resume inside a page whose
// server cursor would skip items. Server cursors never start with it.
const skipCursorPrefix = "cli-skip:"

// skipCursor resumes at item skip of the page fetched with server cursor
// cursor ("" for the first page).
func skipCursor(cursor string, skip int) string {
	return skipCursorPrefix + strconv.Itoa(skip) + ":" + cursor
}

// parseSkipCursor splits a cursor from skipCursor into the server cursor and
// the number of items to skip. Any other cursor is returned unchanged with no
// skip.
func parseSkipCursor(c string) (string, int) {
	rest, ok := strings.CutPrefix(c, skipCursorPrefix)
	if !ok {
		return c, 0
	}
	n, cursor, ok := strings.Cut(rest, ":")
	skip, err := strconv.Atoi(n)
	if !ok || err != nil || skip < 0 {
		return c, 0
	}
	return cursor, skip
}

// cursorsField is the cursors object list commands add to their JSON output,
// matching `trace messages` and `thread messages`: next is set only when
// another page exists.
func cursorsField(next string) map[string]any {
	cursors := map[string]any{}
	if next != "" {
		cursors["next"] = next
	}
	return cursors
}

// printNextCursorHint tells a person reading pretty output how to fetch the
// next page. It writes to stderr so the table on stdout stays clean.
func printNextCursorHint(next string) {
	if next == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "\nMore results: rerun with --cursor %s\n", next)
}

// hubPageQuery is one request of a hub list (repos, commits). Cursor and
// Offset are never both set.
type hubPageQuery struct {
	Cursor   string
	Offset   int
	PageSize int
}

// collectHubPages walks a hub list that returns next_cursor in its response
// body. The SDK models these lists as offset pages and leaves next_cursor out,
// so list returns the raw body alongside the items. The first page is fetched
// at startOffset (or start, when set) and later pages follow next_cursor.
// Servers that predate cursor paging send no next_cursor: the walk then
// continues by offset and returns no cursor, and a non-empty start fails
// rather than silently restarting from the first page.
func collectHubPages[T any](start string, startOffset, limit int, list func(hubPageQuery) ([]T, string, error)) ([]T, string, error) {
	offset := startOffset
	legacy := false
	fetch := func(cursor string, pageSize int) ([]T, string, error) {
		q := hubPageQuery{PageSize: pageSize}
		switch {
		case legacy || cursor == "":
			q.Offset = offset
		default:
			q.Cursor = cursor
		}
		items, raw, err := list(q)
		if err != nil {
			return nil, "", err
		}
		offset += len(items)
		next := gjson.Get(raw, "next_cursor")
		if next.Exists() && !legacy {
			return items, next.String(), nil
		}
		if q.Cursor != "" {
			return nil, "", errors.New("this LangSmith server does not support --cursor on this command; upgrade it, or page with --limit")
		}
		legacy = true
		if len(items) < pageSize {
			return items, "", nil
		}
		// Not a server cursor: it only keeps collectPages walking, and is
		// dropped below.
		return items, strconv.Itoa(offset), nil
	}
	items, next, err := collectPages(start, limit, maxHubPageSize, fetch, nil)
	if legacy {
		next = ""
	}
	return items, next, err
}

// maxHubPageSize is the largest page the hub repo and commit lists accept.
const maxHubPageSize = 100
