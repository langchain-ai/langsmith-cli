package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Diagnostics contain fixed text, never user JSON or upstream response bodies.
// The CLI entry point prints Error(), so JSON formatting remains scoped here.
type dashboardDiagnostic struct {
	code, message, next string
}

func (e dashboardDiagnostic) Error() string {
	if GetFormat() != "json" {
		return e.message + "\nNext: " + e.next
	}
	body, _ := json.Marshal(map[string]any{"error": map[string]any{
		"code": e.code, "message": e.message, "next_steps": []string{e.next},
	}})
	return string(body)
}

func dashboardUUID(value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return dashboardDiagnostic{"invalid_resource_id", "Resource ID must be a UUID.", "List the resource and pass its id, not its display name."}
	}
	return nil
}

func sameDashboardID(a, b string) bool {
	left, err := uuid.Parse(a)
	if err != nil {
		return false
	}
	right, err := uuid.Parse(b)
	return err == nil && left == right
}

func dashboardWorkspaceID() *string {
	id := GetWorkspaceID()
	if id == "" {
		return nil
	}
	return &id
}

type dashboardPage struct {
	Returned   int    `json:"returned"`
	HasMore    *bool  `json:"has_more"`
	NextOffset *int64 `json:"next_offset"`
}

// A full offset page does not prove another item exists.
func describeDashboardPage(count int, limit, offset int64) dashboardPage {
	page := dashboardPage{Returned: count}
	if int64(count) < limit {
		more := false
		page.HasMore = &more
	} else {
		next := offset + int64(count)
		page.NextOffset = &next
	}
	return page
}

func dashboardReadNextStep(args ...string) string {
	// IDs are validated before reaching this helper. Avoid embedding credentials
	// or mutable profile names; remind the caller to retain connection context.
	return fmt.Sprintf("Run langsmith %s in the same profile, endpoint, and workspace.", strings.Join(args, " "))
}
