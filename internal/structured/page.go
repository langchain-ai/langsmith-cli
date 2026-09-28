package structured

import (
	"fmt"
	"io"
	"strconv"
)

// Page is the output envelope for list commands. A command returns one page
// plus what a caller needs to continue; it never silently fetches the rest.
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// pageInfo lets renderers read continuation state without knowing T.
type pageInfo interface {
	continuation() (string, bool)
}

func (p Page[T]) continuation() (string, bool) {
	if p.NextCursor == nil {
		return "", p.HasMore
	}
	return *p.NextCursor, p.HasMore
}

// NewCursorPage builds a page from an opaque server cursor.
func NewCursorPage[T any](items []T, next string) Page[T] {
	p := Page[T]{Items: nonNil(items)}
	if next != "" {
		p.NextCursor = &next
		p.HasMore = true
	}
	return p
}

// NewOffsetPage builds a page for offset pagination. The next offset is
// returned as the cursor, so every list command continues with --cursor.
// total is nil when the API does not report it; a full page then means more.
func NewOffsetPage[T any](items []T, offset, limit int64, total *int64) Page[T] {
	p := Page[T]{Items: nonNil(items)}
	next := offset + int64(len(items))
	if total != nil {
		p.HasMore = next < *total
	} else {
		p.HasMore = limit > 0 && int64(len(items)) >= limit
	}
	if p.HasMore {
		cursor := strconv.FormatInt(next, 10)
		p.NextCursor = &cursor
	}
	return p
}

// ParseOffsetCursor reads a cursor produced by NewOffsetPage.
func ParseOffsetCursor(cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || offset < 0 {
		return 0, NewError(CategoryInvalidInput, fmt.Sprintf("invalid --cursor %q: pass next_cursor from a previous page", cursor))
	}
	return offset, nil
}

func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// PageTable renders a Page's items as a table and says how to continue.
type PageTable struct {
	Title   string
	Columns []Column
}

func (t PageTable) RenderText(w io.Writer, model any) error {
	if err := (Table{Title: t.Title, Rows: ".Items", Columns: t.Columns}).RenderText(w, model); err != nil {
		return err
	}
	page, ok := model.(pageInfo)
	if !ok {
		return nil
	}
	if cursor, more := page.continuation(); more && cursor != "" {
		fmt.Fprintf(w, "\nMore results: rerun with --cursor %s\n", cursor)
	}
	return nil
}
