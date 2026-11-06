package cmd

import (
	"slices"
	"strconv"
	"testing"
)

// fakeCursorServer serves items 0..n-1 newest first, honouring page size; its
// cursor is the index of the next item.
func fakeCursorServer(n int, sizes *[]int) pageFetcher[int] {
	return func(cursor string, pageSize int) ([]int, string, error) {
		*sizes = append(*sizes, pageSize)
		start := 0
		if cursor != "" {
			start, _ = strconv.Atoi(cursor)
		}
		end := min(start+pageSize, n)
		items := make([]int, 0, end-start)
		for i := start; i < end; i++ {
			items = append(items, i)
		}
		if end == n {
			return items, "", nil
		}
		return items, strconv.Itoa(end), nil
	}
}

// fakeUnpagedServer serves items 0..n-1 in one response with no next cursor,
// ignoring page size, like v1 runs fetched by trace.
func fakeUnpagedServer(n int, sizes *[]int) pageFetcher[int] {
	return func(cursor string, pageSize int) ([]int, string, error) {
		*sizes = append(*sizes, pageSize)
		items := make([]int, n)
		for i := range items {
			items[i] = i
		}
		return items, "", nil
	}
}

func TestCollectPages(t *testing.T) {
	tests := []struct {
		name      string
		n, limit  int
		unpaged   bool
		start     string
		keep      func(int) bool
		wantItems []int
		wantNext  string
		wantSizes []int
	}{
		{
			name: "stops on the page boundary at limit", n: 10, limit: 7,
			wantItems: []int{0, 1, 2, 3, 4, 5, 6}, wantNext: "7", wantSizes: []int{3, 3, 1},
		},
		{
			name: "resumes from a cursor", n: 10, limit: 7, start: "7",
			wantItems: []int{7, 8, 9}, wantSizes: []int{3},
		},
		{
			name: "no limit reads every page", n: 5,
			wantItems: []int{0, 1, 2, 3, 4}, wantSizes: []int{3, 3},
		},
		{
			name: "dropped items are consumed, not replayed", n: 10, limit: 2,
			keep:      func(i int) bool { return i%2 == 0 },
			wantItems: []int{0, 2}, wantNext: "3", wantSizes: []int{2, 1},
		},
		{
			name: "oversized page resumes inside it", n: 8, limit: 3, unpaged: true,
			wantItems: []int{0, 1, 2}, wantNext: "cli-skip:3:", wantSizes: []int{3},
		},
		{
			name: "resumes inside an oversized page", n: 8, limit: 3, unpaged: true, start: "cli-skip:3:",
			wantItems: []int{3, 4, 5}, wantNext: "cli-skip:6:", wantSizes: []int{3},
		},
		{
			name: "last part of an oversized page", n: 8, limit: 3, unpaged: true, start: "cli-skip:6:",
			wantItems: []int{6, 7}, wantSizes: []int{3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sizes []int
			fetch := fakeCursorServer(tt.n, &sizes)
			if tt.unpaged {
				fetch = fakeUnpagedServer(tt.n, &sizes)
			}
			items, next, err := collectPages(tt.start, tt.limit, 3, fetch, tt.keep)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(items, tt.wantItems) || next != tt.wantNext {
				t.Errorf("got %v next %q, want %v next %q", items, next, tt.wantItems, tt.wantNext)
			}
			if !slices.Equal(sizes, tt.wantSizes) {
				t.Errorf("page sizes = %v, want %v", sizes, tt.wantSizes)
			}
		})
	}
}

func TestCollectPages_StopsOnRepeatedCursor(t *testing.T) {
	calls := 0
	fetch := func(cursor string, pageSize int) ([]int, string, error) {
		calls++
		return []int{calls}, "same", nil
	}
	items, next, err := collectPages("same", 10, 5, fetch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || next != "" || !slices.Equal(items, []int{1}) {
		t.Errorf("calls %d, items %v, next %q; want one call and no cursor", calls, items, next)
	}
}

// Servers that predate cursor paging send no next_cursor; the walk must keep
// paging by offset instead of stopping after the first page.
func TestCollectHubPages_FallsBackToOffsetOnOlderServers(t *testing.T) {
	var queries []hubPageQuery
	legacy := func(q hubPageQuery) ([]int, string, error) {
		queries = append(queries, q)
		var items []int
		for i := q.Offset; i < min(q.Offset+q.PageSize, 250); i++ {
			items = append(items, i)
		}
		return items, `{"total":250}`, nil
	}
	items, next, err := collectHubPages("", 0, 220, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 220 || items[219] != 219 || next != "" {
		t.Errorf("got %d items (last %d), next %q; want 0..219 and no cursor", len(items), items[len(items)-1], next)
	}
	want := []hubPageQuery{{PageSize: 100}, {Offset: 100, PageSize: 100}, {Offset: 200, PageSize: 20}}
	if !slices.Equal(queries, want) {
		t.Errorf("queries = %+v, want %+v", queries, want)
	}

	if _, _, err := collectHubPages("abc", 0, 10, legacy); err == nil {
		t.Error("want an error when --cursor reaches a server without cursor paging")
	}
}

// Newer servers return next_cursor even on an offset request, so only the
// first page uses the offset.
func TestCollectHubPages_FollowsNextCursor(t *testing.T) {
	var queries []hubPageQuery
	list := func(q hubPageQuery) ([]int, string, error) {
		queries = append(queries, q)
		if q.Cursor == "" {
			return []int{1, 2}, `{"next_cursor":"c2"}`, nil
		}
		return []int{3}, `{"next_cursor":null}`, nil
	}
	items, next, err := collectHubPages("", 5, 3, list)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(items, []int{1, 2, 3}) || next != "" {
		t.Errorf("got %v next %q", items, next)
	}
	want := []hubPageQuery{{Offset: 5, PageSize: 3}, {Cursor: "c2", PageSize: 1}}
	if !slices.Equal(queries, want) {
		t.Errorf("queries = %+v, want %+v", queries, want)
	}
}
