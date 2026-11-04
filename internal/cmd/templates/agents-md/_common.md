# AGENTS.md — building a LangSmith custom app

This app runs in a sandboxed iframe with no network access of its own —
every LangSmith API call goes through `window.langsmith.call`. `src/entry.tsx`
exports `render(data, root, metadata)`; keep that shape, the sandbox depends
on it. `data` is normally `{}`; call `window.langsmith.setData(patch)` if you
need to push a mutation out for the host to persist.

## Macaw is required for every UI change

Polly, LangSmith chat, and any other coding agent must use the installed
LangChain Macaw Design System for **all new and edited UI**, including chat
interfaces. This applies to every template and every later edit; keep this
requirement when changing the app's layout or purpose.

- Keep React and React DOM at 19.2 or newer within Macaw's supported major range;
  the published chart legend uses `useEffectEvent`.
- Use `@langchain/macaw-components` for controls, typography, cards, feedback,
  loading states, icons, and charts. Compose existing components rather than
  recreating them with styled native controls or another component library.
- Read `node_modules/@langchain/macaw-components/docs/DESIGN.md` and the relevant
  sections of `STYLES.md` before editing. Discover the installed API with
  `npm run macaw -- search "<capability>"` and
  `npm run macaw -- inspect <Component> --json`. Do not guess component props.
- Use the semantic tokens from `@langchain/macaw-tokens` through Macaw's Tailwind
  preset. Do not copy token definitions, invent a palette, use raw color values,
  or override component-owned styles to recreate another design.
- When no component covers a feature (for example a data table or scatter plot),
  compose semantic HTML with Macaw components and tokens. Keep custom code scoped
  to the missing behavior and use Macaw's `utils/chartColors` for chart colors.
- For chat, use `Textarea`, `Button`/`IconButton`, `ThinkingState` for generation,
  `Spinner` for fetching or saving, and `Banner`/`ErrorState` for failures.
- Preserve `src/index.css`'s Macaw setup (layer order, stylesheet import before
  Tailwind, Macaw source scan, and Tailwind preset) and `HostTheme` in
  `src/entry.tsx`. The stylesheet already includes the token CSS;
  do not import it twice. CSS and fonts must remain inlined in `dist/bundle.js`
  because the sandbox cannot load separate assets or use a CDN.
- Run `npm run typecheck` and `npm run build`, then inspect the affected UI in
  light and dark mode, including loading, empty, error, disabled, and narrow states.

## Don't run `langsmith apps push`

Publishing is the developer's call, not yours. `langsmith apps push` uploads
to their LangSmith workspace, where everyone with access sees the result — so
leave it to them even when the work looks finished.

Use `langsmith apps dev` to check your changes locally. When you're done, say
the app is ready and let the developer push it once they're satisfied.

## context.md — the handoff file

Keep a `context.md` at the app root and treat it as this app's memory. On a
fresh `langsmith apps pull` it's the first thing to read; before you hand the
app back to the developer to push, it's the last thing to update. It rides
along in the source archive automatically — it's just a root file — so what
you write there is where the next developer's agent starts instead of from
scratch.

Update it as you work, not as a write-up at the end:

- What the app does, and who it's for.
- The key files and how they fit together — where data is fetched, where it's rendered.
- Decisions and why, especially the ones the code doesn't explain on its own.
- Dead ends already ruled out, so nobody burns a day re-trying them.
- Anything non-obvious about the data or API you hit: which project or dataset,
  filters that matter, fields that are usually empty, endpoints that were slower
  or shaped differently than the docs suggest.

It's shared, not secret — no API keys, tokens, or customer data. Nothing
enforces this; it only pays off if you keep it current.

## Calling the LangSmith API

```ts
const projects = await window.langsmith.call('GET /api/v1/sessions', {
  params: { limit: '20' },
});
```

`operation` is `"<METHOD> <path>"` — use the full path including its prefix
(`/api/v1/...` for Python-hosted endpoints, `/v1/platform/...` and `/v2/...`
for Go-hosted ones). `args` carries `params` (query string) and/or `body`
(JSON). This is a generic passthrough, not a curated allowlist — anything
your API key can already do works; a permission error is a real limit of the
key. Full reference: https://docs.langchain.com/langsmith/smith-api-ref. Base URL:
`https://api.smith.langchain.com` (or your self-hosted instance's URL).

While `langsmith apps dev` is running, the app's failed API calls (with status
codes and error messages) and uncaught errors stream to that terminal — read it
to debug without opening browser devtools. Add `--verbose` to also see every
successful call and all `console.*` output, or `--quiet` to silence it.

## Services outside LangSmith

To reach Google, Slack or your own API, call a LangSmith Tool Gateway kit
(beta): a set of MCP tools set up on the **Tools** page. Calls run as the
person viewing the app, with their connected accounts, so credentials never
reach app code. Your own API joins by putting an MCP server in front of it and
adding that server on the Tools page. A kit's connection instructions on the
Tools page include its MCP URL; the kit id is the path segment before `/mcp`.

```ts
const reply = await window.langsmith.call(`POST /v1/managed-tools/kits/${kitId}/mcp`, {
  body: {
    jsonrpc: '2.0',
    id: 1,
    method: 'tools/call',
    params: { name: toolName, arguments: { query } },
  },
});
if (reply.error || reply.result?.isError) {
  // Show the tool's error to the viewer.
}
```

- `method: 'tools/list'` (no `params`) returns the tools the viewer can call,
  with their names and input schemas. By default, tools from accounts the
  viewer hasn't connected are left out, and the missing connections appear in
  `result._meta["com.langchain.langsmith/missingCredentials"]`; the list can be
  empty. A kit that requires all connections rejects the list until they're
  all connected.
- When the viewer hasn't connected an account a tool needs, the call rejects
  with an error whose message contains "Authorize the linked servers, then
  retry the MCP request". Ask them to connect it on the Tools page.
- A call times out after 10 seconds, and a timed-out tool call may still have
  run. Never retry a tool call automatically.
- Under `langsmith apps dev`, calls use the CLI's configured credentials. Use a
  personal API key or an OAuth login for tools connected to your account; a
  workspace or service-account key doesn't identify a user.

## Rate limits & pagination

Every call counts against a rate limit; going over returns **HTTP 429**. Retry
with exponential backoff + jitter, and don't fan out parallel queries to go
faster — that just trips the limit sooner. Query endpoints
(`POST /v2/runs/query`, `POST /v2/threads/query`) carry the tightest,
time-window-based limits, so query deliberately:

- **Set `min_start_time`.** Omitting it counts as a >7-day "large window" and drops
  you from 10 req/10s to 3 req/10s. Keep windows ≤ 7 days where you can.
- **Split long windows.** The query time range (`max_start_time` − `min_start_time`)
  is capped at **401 days**; a longer span is rejected with a 400. Walk it in
  chunks (ideally ≤ 7 days) instead of one huge request.
- **Paginate.** Pass `page_size`, then feed the returned `next_cursor` back as
  `cursor` for the next page (`runs/query` returns `{ items, next_cursor }`).
  Don't pull everything at once.
- **Use `selects`** to fetch only the fields you render — smaller, faster responses.
- **Avoid full-text `search(...)` filters and selecting `child_run_ids`** — both
  drop you into a stricter tier (as low as 1 req/10s). Prefer `eq()` / `has()`.
- **Prefer `runs/stats` over paging** for any headline number (counts, rates,
  sums) — one call instead of walking every page.

Other caps worth knowing: ~2,000 req/min per key overall (writes to `/runs` and
`/feedback` allow 5,000/min), and 25,000 runs per trace. Full reference:
https://docs.langchain.com/langsmith/usage-and-billing and
https://docs.langchain.com/langsmith/export-traces#rate-limits

## Downloading files

The sandbox can't download files on its own, so `<a download>` won't work. Build
the file in the app and hand it to `window.langsmith.download()`:

```ts
await window.langsmith.download({
  filename: 'runs.json',
  content: JSON.stringify(runs, null, 2),
  mimeType: 'application/json',
});
```

`content` is a string, `Blob`, `ArrayBuffer`, or typed array, so any format the
app can build works: CSV (quote fields containing commas, quotes, or newlines),
JSON, XLSX, PDF, images, ZIP. Files are capped at 50 MB, and executable types
such as `.exe`, `.sh`, or `.dmg` are refused. LangSmith asks the viewer to
confirm each download, and the call waits for their answer; `langsmith apps dev`
saves straight away. The promise resolves `{ status: 'started' }` once the
browser has the file and rejects with the reason otherwise.

## Showing a run's attachments

`window.langsmith.call` resolves `image/*`, `audio/*`, `video/*`,
`application/pdf`, and `application/octet-stream` responses to a `Blob`; other
responses are parsed as JSON. Add `ATTACHMENTS` to `selects` on
`GET /v2/runs/{run_id}` or `POST /v2/runs/query`, and the run's `attachments`
maps each attachment name to a signed download URL (v1 runs carry it at
`s3_urls["attachment.<name>"].presigned_url`). `call` takes a path, so strip the
URL's origin and keep its query:

```tsx
function useAttachmentSrc(signedUrl: string | undefined) {
  const [src, setSrc] = useState<string>();
  useEffect(() => {
    if (!signedUrl) return;
    let objectUrl: string | undefined;
    let cancelled = false;
    const path = signedUrl.replace(/^https?:\/\/[^/]+/, '');
    window.langsmith
      .call(`GET ${path}`)
      .then((blob) => {
        if (cancelled || !(blob instanceof Blob)) return;
        objectUrl = URL.createObjectURL(blob);
        setSrc(objectUrl);
      })
      .catch((err) => console.error(err));
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
      setSrc(undefined);
    };
  }, [signedUrl]);
  return src;
}
```

- Show it with `<img src={src} alt="…">`. The hook revokes the object URL when
  the image goes away; do the same for any object URL you create.
- Signed URLs expire. Fetch the run again for a fresh one rather than storing it.
- The sandbox displays only images. Audio, video, and PDFs can't play or embed
  there, so offer them through `window.langsmith.download({ filename, content: blob })`.

## Theme

`metadata.mode` is `"dark"` | `"light"`. `HostTheme` synchronizes Macaw's
`AppThemeProvider` on every host update without remounting the app. Keep
`storageKey={null}`: the sandbox has no localStorage, and LangSmith owns the
preference. Use semantic tokens so both themes work without branching.

## Filter DSL for metadata equality

Query endpoints take a `filter` string. Metadata equality is **two paired
clauses**, not `eq(metadata.key, ...)`:

```
and(eq(metadata_key, "ls_agent_purpose"), eq(metadata_value, "coding"))
```

Combine with `and(...)` / `or(...)`; other examples: `has(tags, "prod")`,
`eq(name, "ChatOpenAI")`, `eq(is_root, true)`.

## Opening links

`window.langsmith.openUrl(url)` navigates LangSmith to a URL — pass an absolute
URL or a path. Pass `{ newTab: true }` as a second argument to open it in a new
tab instead. An `https://` URL outside LangSmith, such as a Jira issue or a
GitHub pull request, opens in a new tab after LangSmith shows the viewer the
URL and they confirm (`langsmith apps dev` opens it without asking). Call
`openUrl` from a click handler. The app can't navigate on its own, so
`<a href>` and `window.open` won't work; the host resolves the URL and rejects
anything else, including `http://` URLs outside LangSmith.

For a run or trace, ask the API for the URL rather than building one:

```ts
const { url } = await window.langsmith.call(`GET /api/v2/runs/${runId}/url`, {
  params: { project_id: projectId, trace_id: traceId },
});
window.langsmith.openUrl(url);
```

Both params are required. This resolves host and workspace server-side, so it
stays correct on self-hosted and EU.

Build everything else from `metadata.host` and `metadata.workspaceId` — never
hardcode `smith.langchain.com`:

| Target | Path |
| --- | --- |
| Project | `/o/{workspaceId}/projects/p/{projectId}` |
| Dataset | `/o/{workspaceId}/datasets/{datasetId}` |
| Example | `/o/{workspaceId}/datasets/{datasetId}/e/{exampleId}` |
| Experiment | `/o/{workspaceId}/datasets/{datasetId}/compare?selectedSessions={sessionId}` |
| Annotation queue | `/o/{workspaceId}/annotation-queues/{queueId}` |

The `/o/` segment is the workspace ID despite reading like an organization ID.
Experiments have no standalone route — they're a compare view on their dataset.

## Deep links

Give an app with more than one view deep links, so a link, a refresh, or Back
and Forward return to the view the viewer was on. LangSmith keeps the app's
route in the page's `?appRoute=` parameter and passes it to `render()` as
`metadata.route`; the app changes it with `window.langsmith.setRoute(route)`.
The route's format is up to the app, such as `/runs/123?tab=feedback`;
LangSmith never parses it.

Deep links work only where `metadata.route` is a string and `setRoute`
exists. Older hosts and CLIs send no route, older editor previews have no
`setRoute`, and annotation queue views ignore it. This hook falls back to
local navigation in all of those:

```ts
// src/lib/useAppRoute.ts
import { useEffect, useState } from 'react';

export function useAppRoute(hostRoute: string | undefined) {
  const [route, setLocalRoute] = useState(hostRoute ?? '/');
  // LangSmith's route wins when it sends one, including on Back and Forward.
  useEffect(() => {
    if (hostRoute !== undefined) setLocalRoute(hostRoute);
  }, [hostRoute]);

  function go(next: string, options?: { replace?: boolean }) {
    const target = next === '' ? '/' : next;
    // LangSmith ignores routes over 2,048 bytes, so don't show one it won't keep.
    if (new TextEncoder().encode(target).byteLength > 2048) return;
    setLocalRoute(target);
    window.langsmith.setRoute?.(target, options);
  }

  return { route, go };
}
```

```tsx
const { route, go } = useAppRoute(metadata?.route);
const runId = /^\/runs\/([0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12})$/.exec(route)?.[1];
return runId ? (
  <RunDetail runId={runId} />
) : (
  <RunList onOpen={(id) => go(`/runs/${id}`, { replace: false })} />
);
```

- Call `go` only from event handlers, never while rendering or from an effect
  that watches the route. For a route the app doesn't recognize, show the home
  view or a not-found state instead of redirecting.
- Pass `{ replace: false }` to open a new page, such as a list to a detail
  view, so Back returns to the list. Filters and search use the default, which
  replaces the current entry.
- Treat the route as untrusted input: anyone can craft a link. It only chooses
  what to show; opening a link never writes data, sends feedback, or calls a
  tool. Validate ids from the route before they go into an API path.
- Never put secrets in the route. It shows in the URL, the browser history, and
  every link someone shares.
- `setRoute('')` and `setRoute('/')` clear the route. A route over 2,048 UTF-8
  bytes is ignored, never truncated.

`langsmith apps dev` keeps the route in its own `?appRoute=`, so you can check
links, refresh, and Back and Forward locally.

<!-- TEMPLATE-SPECIFIC -->

## More of the LangSmith API (starting points, not exhaustive)

**Runs**
- `POST /v2/runs/query` — query runs (body: `project_ids`, `filter`, `is_root`, `run_type`, `min_start_time`, `max_start_time`, `page_size`, `cursor`, `selects` — UPPER_SNAKE field names, e.g. `ID`, `NAME`, `TOTAL_COST`); returns `{ items, next_cursor }`
- `GET /v2/runs/{run_id}` — fetch one full run (`project_id` + `selects` control which fields come back)
- `POST /api/v1/runs/stats` — server-side aggregates over a filtered set of runs, no row limit (counts, error rate, latency percentiles, token/cost sums) — prefer this over paging through `runs/query` for any headline number; no v2 equivalent
- `POST /api/v1/runs/group/stats` — same, grouped (e.g. `group_by: "conversation"` for a distinct thread count); no v2 equivalent
- `POST /api/v1/runs` / `PATCH /api/v1/runs/{run_id}` — create / update a run

**Projects (tracing sessions)**
- `GET /api/v1/sessions` — list projects
- `GET /api/v1/sessions/{session_id}` — get a project

**Datasets & experiments**
- `GET /api/v1/datasets` — list datasets
- `POST /v2/datasets/{dataset_id}/experiment-runs` — per-example rows across experiments (body: `experiment_ids`, `page_size`, `cursor`, `selects`); returns `{ items, next_cursor }`
- `POST /v1/platform/datasets/{dataset_id}/examples` — create examples

**Feedback**
- `POST /api/v1/feedback` — create feedback (`run_id` for RUN items, or `feedback_thread_id` for THREAD items)
- `GET /api/v1/feedback?run={run_id}` — list feedback for a run
- `GET /api/v1/feedback?feedback_thread_id={thread_id}` — list feedback for a thread
- `GET /api/v1/feedback-configs?key={key}` — a feedback key's type / direction

**Annotation queues**
- `GET /api/v1/annotation-queues` — list queues
- `GET /api/v1/platform/annotation-queues/{queue_id}/items` — list membership stubs (`status`, `page_size`, `cursor`); returns `{ items, next_cursor }`. Items are metadata-only (`id`, `item_type` RUN|THREAD, `run_id`/`thread_id`, `project_id`, …) — hydrate payloads separately. Use `/platform/` (smith-go); plain `/api/v1/annotation-queues/.../items` 404s on SaaS.
- `GET /api/v1/platform/annotation-queues/{queue_id}/items/count?status=` — section totals (`needs_my_review`, `needs_others_review`, `archived`)
- `POST /api/v1/platform/annotation-queues/items/{item_id}/status` — mark reviewer complete (`{ status: "completed" }`)
- UI "Completed" maps to API status `archived`

**Threads**
- `POST /v2/threads/query` — query threads
- `POST /v1/trajectory` — thread chat messages as JSON (`{ project_id, thread_id, format: "messages" }` → `{ messages, next_cursor }`). Prefer this over `GET /v2/threads/{id}/messages` (SSE-only; JSON bridge cannot stream it) and over `/traces` when you want human/AI text
- `GET /v2/threads/{thread_id}/traces` — thread turn list / IO stubs (not normalized chat)

If you need something not listed, check the docs — almost everything in the
LangSmith API is reachable this way.
