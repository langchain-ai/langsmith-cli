package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
)

// UserFlaggedIssueKey is the feedback key used to mark a trace as flagged for
// issues-agent review.
const UserFlaggedIssueKey = "langsmith_user_flagged_issue"

// FlaggedTrace is a single user-flagged trace with its optional review
// comment.
type FlaggedTrace struct {
	TraceID string
	Comment string
}

// FetchFlaggedTraces paginates /api/v1/feedback for the user-flagged issue
// feedback key on the given session and returns the set of flagged traces.
// Failures are logged to stderr and surfaced as partial results — callers
// get whatever was successfully fetched before the error.
//
// If lastNMinutes > 0, only flagged traces created within the window are
// returned. Otherwise all flagged traces for the session are returned.
func FetchFlaggedTraces(ctx context.Context, c *client.Client, sessionID string, lastNMinutes int) []FlaggedTrace {
	var out []FlaggedTrace
	seen := map[string]bool{}
	offset := 0
	const limit = 100

	minCreatedQS := ""
	if lastNMinutes > 0 {
		minCreated := time.Now().UTC().Add(-time.Duration(lastNMinutes) * time.Minute)
		minCreatedQS = "&min_created_at=" + url.QueryEscape(minCreated.Format(time.RFC3339))
	}

	for {
		path := fmt.Sprintf(
			"/api/v1/feedback?key=%s&session=%s%s&limit=%d&offset=%d",
			url.QueryEscape(UserFlaggedIssueKey),
			url.QueryEscape(sessionID),
			minCreatedQS,
			limit,
			offset,
		)
		var page []map[string]any
		if err := c.RawGet(ctx, path, &page); err != nil {
			fmt.Fprintf(os.Stderr, "warning: fetching flagged traces failed: %v\n", err)
			return out
		}
		if len(page) == 0 {
			break
		}
		for _, fb := range page {
			tid, _ := fb["trace_id"].(string)
			if tid == "" {
				tid, _ = fb["run_id"].(string)
			}
			if tid == "" || seen[tid] {
				continue
			}
			seen[tid] = true
			comment, _ := fb["comment"].(string)
			out = append(out, FlaggedTrace{TraceID: tid, Comment: comment})
		}
		if len(page) < limit {
			break
		}
		offset += limit
	}
	return out
}
