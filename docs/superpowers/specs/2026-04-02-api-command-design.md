# `langsmith api` Command Design Spec

## Goal

Add a `langsmith api` command that lets developers and coding agents discover LangSmith API endpoints (via the OpenAPI spec) and make authenticated HTTP requests — similar to `gh api`, `vercel api`, and `az rest`.

## Motivation

Many CLI tools (gh, az, vercel, glab, stripe, heroku) provide a raw API request command that injects auth headers automatically. This is especially valuable for:
- **Coding agents** that need to discover and call endpoints programmatically
- **Developers** exploring the API without leaving the terminal
- **Scripts** that need authenticated API access without managing headers manually

## Command Structure

Three modes under `langsmith api`:

```
langsmith api ls [--tag TAG] [--search QUERY] [--refresh]   # List endpoints
langsmith api info METHOD /path                              # Endpoint details
langsmith api METHOD path [--body JSON] [flags]              # Make requests
```

`ls` and `info` are cobra subcommands. The request mode is handled by the parent `api` command when the first arg matches an HTTP method (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS).

All three respect the global `--format json|pretty` flag. JSON is default (agent-first).

---

## `langsmith api ls` — Endpoint Listing

Lists all endpoints from the cached OpenAPI spec.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--tag` | `-t` | (none) | Filter endpoints by tag |
| `--search` | `-s` | (none) | Case-insensitive substring match on path, summary, or tag |
| `--refresh` | | false | Force re-fetch of the cached OpenAPI spec |

### Output (JSON, default)

```json
[
  {"method": "GET", "path": "/api/v1/sessions", "summary": "Read Tracer Sessions", "tag": "tracer-sessions"},
  {"method": "POST", "path": "/api/v1/sessions", "summary": "Create Tracer Session", "tag": "tracer-sessions"}
]
```

### Output (pretty)

```
METHOD  PATH                          TAG               SUMMARY
GET     /api/v1/sessions              tracer-sessions   Read Tracer Sessions
POST    /api/v1/sessions              tracer-sessions   Create Tracer Session
DELETE  /api/v1/sessions/{session_id} tracer-sessions   Delete Tracer Session
(461 endpoints)
```

---

## `langsmith api info` — Endpoint Detail

Shows full detail for a specific endpoint: parameters, request body schema, response schema.

### Usage

```
langsmith api info METHOD /path
langsmith api info GET /api/v1/sessions
langsmith api info GET sessions          # shorthand path works too
```

### Output (JSON, default)

```json
{
  "method": "GET",
  "path": "/api/v1/sessions",
  "summary": "Read Tracer Sessions",
  "tag": "tracer-sessions",
  "parameters": [
    {"name": "limit", "in": "query", "type": "integer", "required": false, "description": "Max results to return"},
    {"name": "name", "in": "query", "type": "string", "required": false, "description": "Filter by name"}
  ],
  "request_body": null,
  "response_schema": {
    "type": "array",
    "items": {"$ref": "TracerSession"}
  }
}
```

For endpoints with a request body:

```json
{
  "method": "POST",
  "path": "/api/v1/runs/query",
  "summary": "Query Runs",
  "tag": "run",
  "parameters": [],
  "request_body": {
    "content_type": "application/json",
    "required": true,
    "schema": {
      "type": "object",
      "properties": {
        "session_id": {"type": "string", "description": "..."},
        "limit": {"type": "integer", "default": 20}
      },
      "required": ["session_id"]
    }
  },
  "response_schema": {
    "type": "object",
    "properties": {
      "runs": {"type": "array", "items": {"$ref": "RunSchema"}},
      "cursors": {"type": "object"}
    }
  }
}
```

### Schema Resolution

`$ref` references are resolved one level deep — properties are inlined, but nested refs stay as references to avoid dumping the entire 562-schema tree.

---

## `langsmith api METHOD path` — Making Requests

Makes authenticated HTTP requests to the LangSmith API.

### Usage

```bash
langsmith api GET sessions?limit=5
langsmith api POST runs/query --body '{"session_id":"abc","limit":10}'
langsmith api POST datasets --body @create-dataset.json
echo '{"name":"test"}' | langsmith api POST sessions --body @-
langsmith api GET sessions --include
langsmith api GET sessions -H "Accept:text/csv"
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--body` | | (none) | JSON request body. `@file.json` for file, `@-` for stdin. |
| `--header` | `-H` | (none) | Additional headers as `Key:Value` (repeatable) |
| `--include` | `-i` | false | Print response status line and headers before body |

### Path Resolution

| Input | Resolved URL |
|-------|-------------|
| `sessions` | `{base}/api/v1/sessions` |
| `/api/v1/sessions` | `{base}/api/v1/sessions` |
| `https://custom.host/path` | `https://custom.host/path` |
| `sessions?limit=5` | `{base}/api/v1/sessions?limit=5` |

Where `{base}` is the API URL from `--api-url`, `LANGSMITH_ENDPOINT`, or the default `https://api.smith.langchain.com`.

### Auth Injection (automatic)

- `x-api-key` header from `--api-key` flag or `LANGSMITH_API_KEY` env var
- `x-tenant-id` header from `LANGSMITH_WORKSPACE_ID` env var (if set)
- `Content-Type: application/json` when body is provided

### Output

- Response body pretty-printed as indented JSON (or raw if not valid JSON)
- With `--include`: status line + headers printed first, blank line, then body
- Exit code 0 for 2xx/3xx, exit code 1 for 4xx/5xx (body still printed so agents get error details)

### No Auto-Detection

The HTTP method is always the first positional arg. No auto-detection from body presence. This keeps it explicit and unambiguous, which is better for agents.

---

## OpenAPI Spec Caching

### Cache Location

`~/.langsmith/cache/openapi-<hash>.json`

Where `<hash>` is a short hash of the API base URL, so self-hosted instances get their own cache file.

### Cache Format

```json
{
  "fetched_at": "2026-04-02T12:00:00Z",
  "api_url": "https://api.smith.langchain.com",
  "spec": { ... raw OpenAPI JSON ... }
}
```

### TTL

- 24-hour TTL — if cache is older than 24h, automatically re-fetch
- `--refresh` flag on `ls` and `info` bypasses TTL and forces re-fetch
- Spec is fetched from `{api_url}/openapi.json`

---

## File Structure

```
internal/cmd/api/
├── api.go           # Parent command + NewCmd() exported, request handling
├── ls.go            # langsmith api ls
├── info.go          # langsmith api info
├── request.go       # langsmith api METHOD path (request execution)
├── spec.go          # OpenAPI spec fetch/cache/parse
├── api_test.go      # Tests for ls, info, request
└── spec_test.go     # Tests for spec fetch/cache/parse
```

Additionally:
- `internal/client/client.go` — add `RawDo` method returning raw status, headers, body
- `internal/client/client_test.go` — tests for `RawDo`
- `internal/cmd/root.go` — register `api.NewCmd()`
- `internal/cmd/root_test.go` — add `"api"` to expected subcommands

### Package boundary

`internal/cmd/api/` is a sub-package of cmd (new pattern for this repo, but justified by file count). The `gh` CLI uses the same pattern (`pkg/cmd/api/`). Root registration:

```go
import "github.com/langchain-ai/langsmith-cli/internal/cmd/api"
rootCmd.AddCommand(api.NewCmd())
```

The api package needs access to the global flags (api-key, api-url, format). These will be resolved via the cobra command's persistent flags from the root, or passed in as parameters to `NewCmd()`.

### No New Dependencies

- `net/http` for fetching spec and making requests
- `encoding/json` for parsing
- `crypto/sha256` for cache key hashing
- `os` for file caching
- All stdlib. No OpenAPI parsing library needed — we extract only the fields we use.

---

## Error Handling

- Missing API key: `{"error": "LANGSMITH_API_KEY not set"}` to stderr, exit 1
- Spec fetch failure: `{"error": "fetching OpenAPI spec: <detail>"}` to stderr, exit 1
- Invalid `--body` JSON: `{"error": "invalid JSON body: <detail>"}` to stderr, exit 1
- Invalid `@file` path: `{"error": "opening input file: <detail>"}` to stderr, exit 1
- Invalid header format: `{"error": "invalid header format \"...\": expected Key:Value"}` to stderr, exit 1
- Endpoint not found in spec (for `info`): `{"error": "endpoint not found: GET /api/v1/foo"}` to stderr, exit 1
- HTTP 4xx/5xx: body printed to stdout, exit code 1
