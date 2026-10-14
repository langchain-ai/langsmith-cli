package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	langsmith "github.com/langchain-ai/langsmith-go"
)

// listRuns runs the shared list query. --ai-search uses the v2 event stream;
// every other list stays on queryRunsAuto.
func listRuns(ctx context.Context, c *client.Client, ff *FilterFlags, isRoot bool, includeIO, includeFeedback bool, sessionID string) ([]langsmith.RunSchema, error) {
	params := BuildRunQueryParams(ff, isRoot, ff.Limit)
	if criterion := strings.TrimSpace(ff.AISearch); criterion != "" {
		requireV2Feature(ctx, c, "AI search")
		return queryRunsAISearch(ctx, c, toV2Params(params, buildRunSelectV2(includeIO, includeFeedback)), sessionID, criterion, ff.Limit, ff.MinTokens)
	}
	if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
		params.Select = langsmith.F(sel)
	}
	return queryRunsAuto(ctx, c, params, buildRunSelectV2(includeIO, includeFeedback), sessionID, ff.Limit, ff.MinTokens)
}

var errSSEEnd = errors.New("sse end")

func queryRunsAISearch(ctx context.Context, c *client.Client, params langsmith.RunQueryV2Params, sessionID, criterion string, limit, minTokens int) ([]langsmith.RunSchema, error) {
	if sessionID != "" {
		params.ProjectIDs = langsmith.F([]string{sessionID})
	}
	if limit <= 0 {
		limit = 1
	}

	var all []langsmith.RunSchema
	for len(all) < limit {
		body, err := withAISearch(params, criterion)
		if err != nil {
			return nil, err
		}
		_, raw, err := c.PostEventStream(ctx, "/api/v2/runs/query", body)
		if err != nil {
			return nil, fmt.Errorf("querying runs (ai search): %w", err)
		}
		doc, err := collectSSEPatches(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("querying runs (ai search): %w", err)
		}
		encoded, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items      []langsmith.Run `json:"items"`
			NextCursor *string         `json:"next_cursor"`
		}
		if err := json.Unmarshal(encoded, &page); err != nil {
			return nil, fmt.Errorf("decoding ai search page: %w", err)
		}
		for _, run := range page.Items {
			if len(all) >= limit {
				break
			}
			schema := runV2ToSchema(run)
			if minTokens > 0 && schema.TotalTokens < int64(minTokens) {
				continue
			}
			all = append(all, schema)
		}
		if page.NextCursor == nil || *page.NextCursor == "" || len(page.Items) == 0 {
			break
		}
		params.Cursor = langsmith.F(*page.NextCursor)
	}
	return all, nil
}

func withAISearch(params langsmith.RunQueryV2Params, criterion string) (map[string]any, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var body map[string]any
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
	}
	if body == nil {
		body = map[string]any{}
	}
	body["ai_search"] = criterion
	return body, nil
}

func unescapePointerToken(token string) string {
	token = strings.ReplaceAll(token, "~1", "/")
	return strings.ReplaceAll(token, "~0", "~")
}

func setPointer(doc any, path string, value any) (any, error) {
	if path == "" {
		return value, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("patch path %q", path)
	}
	parts := strings.Split(path[1:], "/")
	for i, part := range parts {
		parts[i] = unescapePointerToken(part)
	}
	return writePointer(doc, parts, value)
}

func writePointer(node any, parts []string, value any) (any, error) {
	key := parts[0]
	last := len(parts) == 1
	switch cur := node.(type) {
	case map[string]any:
		if last {
			cur[key] = value
			return cur, nil
		}
		child, ok := cur[key]
		if !ok || child == nil {
			if parts[1] == "-" || indexToken(parts[1]) {
				child = []any{}
			} else {
				child = map[string]any{}
			}
		}
		next, err := writePointer(child, parts[1:], value)
		if err != nil {
			return nil, err
		}
		cur[key] = next
		return cur, nil
	case []any:
		if key == "-" {
			if !last {
				return nil, fmt.Errorf("patch path continues after -")
			}
			return append(cur, value), nil
		}
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 {
			return nil, fmt.Errorf("patch index %q", key)
		}
		if last {
			if idx == len(cur) {
				return append(cur, value), nil
			}
			if idx > len(cur) {
				return nil, fmt.Errorf("patch index %d past end", idx)
			}
			cur[idx] = value
			return cur, nil
		}
		if idx >= len(cur) {
			return nil, fmt.Errorf("patch index %d past end", idx)
		}
		next, err := writePointer(cur[idx], parts[1:], value)
		if err != nil {
			return nil, err
		}
		cur[idx] = next
		return cur, nil
	default:
		return nil, fmt.Errorf("cannot apply patch to %T", node)
	}
}

func indexToken(token string) bool {
	if token == "-" {
		return true
	}
	_, err := strconv.Atoi(token)
	return err == nil
}

func applyPatchOps(doc any, payload []byte) (any, error) {
	var ops []struct {
		Op    string          `json:"op"`
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(payload, &ops); err != nil {
		return nil, fmt.Errorf("reading patch event: %w", err)
	}
	for _, op := range ops {
		if op.Op != "add" && op.Op != "replace" {
			return nil, fmt.Errorf("unsupported patch op %q", op.Op)
		}
		var value any
		if len(op.Value) > 0 {
			dec := json.NewDecoder(bytes.NewReader(op.Value))
			dec.UseNumber()
			if err := dec.Decode(&value); err != nil {
				return nil, fmt.Errorf("reading patch value: %w", err)
			}
		}
		next, err := setPointer(doc, op.Path, value)
		if err != nil {
			return nil, err
		}
		doc = next
	}
	return doc, nil
}

func collectSSEPatches(r io.Reader) (any, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)

	var (
		event string
		data  string
		doc   any
	)
	flush := func() error {
		kind := event
		payload := data
		event, data = "", ""
		switch kind {
		case "end":
			return errSSEEnd
		case "error":
			msg := strings.TrimSpace(payload)
			if msg == "" {
				msg = "query stream failed"
			}
			return errors.New(msg)
		case "data":
			if strings.TrimSpace(payload) == "" {
				return nil
			}
			next, err := applyPatchOps(doc, []byte(payload))
			if err != nil {
				return err
			}
			doc = next
			return nil
		default:
			return nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				if errors.Is(err, errSSEEnd) {
					return docOrEmpty(doc), nil
				}
				return nil, err
			}
			continue
		}
		if rest, ok := strings.CutPrefix(line, "event:"); ok {
			event = strings.TrimSpace(rest)
			continue
		}
		if rest, ok := strings.CutPrefix(line, "data:"); ok {
			rest = strings.TrimPrefix(rest, " ")
			if data != "" {
				data += "\n"
			}
			data += rest
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if event != "" || data != "" {
		if err := flush(); err != nil && !errors.Is(err, errSSEEnd) {
			return nil, err
		}
	}
	return docOrEmpty(doc), nil
}

func docOrEmpty(doc any) any {
	if doc == nil {
		return map[string]any{"items": []any{}}
	}
	return doc
}
