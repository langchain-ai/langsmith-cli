# AGENTS.md — building a LangSmith custom app

This app runs in a sandboxed iframe with no network access of its own —
every LangSmith API call goes through `window.langsmith.call`. `src/entry.tsx`
exports `render(data, root, metadata)`; keep that shape, the sandbox depends
on it. `data` is normally `{}`; call `window.langsmith.setData(patch)` if you
need to push a mutation out for the host to persist.

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

## Theme

`metadata.mode` is `"dark"` | `"light"`. The sandbox sets `html.dark` from it
before every render, so Tailwind/token-based UIs theme for free with no
branching. Only branch on it yourself if you're using inline styles — and
re-check it every render, since it can change without a remount.

## Design system

This app is built on LangSmith's design system — the same components and tokens
the LangSmith product uses, published as a shadcn registry. Part of it is
already vendored into `src/components/langsmith/design-system/`: the theme, the
Tailwind preset, and every component this template renders. It's ordinary source
you can read and edit.

**A component before a raw element.** Check what's already in
`src/components/langsmith/design-system/components/` before writing a `<button>`,
`<input>`, `<span>` of text, or a chip. If what you need isn't there yet, add it
rather than hand-rolling it:

```bash
npx shadcn list @langsmith                       # everything available
npx shadcn add @langsmith/dialog @langsmith/tabs # source + npm deps
```

`components.json` already registers the `@langsmith` namespace, so `registry add`
is not needed, and `add` installs a component's npm dependencies along with its
source — always prefer it to copying files in by hand. Do **not** run
`shadcn init`: it installs a competing preset and breaks the build. (If
`smith.langchain.com` is unreachable, use `$LANGSMITH_ENDPOINT` as the base.)

Easy-to-miss mappings, by capability rather than name:

| You want | Use |
| --- | --- |
| Any text | `Text` (`as="span"` inside buttons/links) |
| Searchable single/multi-select, free-solo input | `Typeahead` |
| Plain dropdown of known options | `Select` |
| Icon-only action | `IconButton` (its `label` becomes the tooltip) |
| Icon with optional tooltip | `Icon` |
| Status chip, count, tag | `Badge` |
| Inline alert | `Banner`; richer error payloads → `ErrorMessage` |
| Loading | `Spinner`, `Skeleton`, `LinearProgress`, `CircularProgress` |
| Overlay / sheet | `Dialog`, `Pane` |
| View switching | `Tabs`, `GroupedTabs` |
| Keyboard shortcut chips | `Kbd` / `KbdGroup` |

Icons come from `@langchain/untitled-ui-icons` — not heroicons or lucide.

`<Tooltip>` is Radix-based and needs a provider above it; the scaffold already
mounts one in `entry.tsx` (`DesignSystemProvider`). Keep it — `IconButton` and
`Icon label=…` render tooltips, so removing it crashes them at runtime.

### Tokens, not literals

Never write a hex/rgb color, a primitive var (`--neutral-100`), a primitive
utility (`bg-neutral-100`), or a hardcoded `z-10`. The preset defines semantic
Tailwind classes for everything:

| Situation | Token |
| --- | --- |
| Page background | `bg-surface-level-1` |
| Card, panel, sidebar | `bg-surface-level-2` |
| Nested container / deepest fill | `bg-surface-level-3` / `-4` |
| Popover, tooltip, dropdown | `bg-elevated` |
| Selected row | `bg-selected` |
| Brand fill (non-Button) | `bg-brand`; tinted `bg-brand-subtle` |
| Intent fills | `bg-success` / `bg-error` / `bg-warning` (+ `-subtle`, `-strong`) |
| Body / label / helper / faint text | `text-primary` / `-secondary` / `-tertiary` / `-quaternary` |
| Placeholder, disabled text | `text-placeholder`, `text-disabled` |
| Standalone icon | `text-icon-primary` (…`-tertiary`, `-error`, `-success`) |
| Borders | `border-default`, `border-subtle`, `border-focus`, `border-error` |
| Run/system status | `text-status-green/orange/yellow/red` |
| Chart series | `var(--chart-categorical-line-1…8)` (fills: `…-fill-1…8`) |

Pair intent color with an icon or text — never color alone.

Spacing is the 4-point `space-*` scale (`gap-space-2`, `px-space-4`,
`p-space-5` for card padding): 1=4px, 2=8px, 3=12px, 4=16px, 5=24px, 6=32px,
7=40px, 8=48px, 9=64px. Radius `rounded-xs…xl`/`rounded-full`, elevation
`shadow-sm/md/lg`, motion `duration-fast/normal/slow/slower`. Prefer flex +
`gap` over margins, and honor `prefers-reduced-motion` (`motion-safe:`).

Do **not** add a `theme.extend` for any of these in `tailwind.config.js`: a
local key of the same name overrides the preset's, which silently restyles every
design-system component.

### Refreshing the vendored copy

The theme and components are checked in, so they can fall behind the registry:

```bash
npx shadcn add --overwrite @langsmith/theme
```

`--overwrite` is required because both files already exist. Afterwards, check
`tailwind.langsmith.cjs` for any newly `require`d package and add it to
`devDependencies` — an undeclared one fails the build with `MODULE_NOT_FOUND`.
Diff the refresh before keeping it, so a token rename doesn't silently restyle
the app.

### Before you call it done

Run `npm run typecheck` and `npm run build`, and look at the result in
`langsmith apps dev` — in both light and dark, and in the loading, empty, and
error states, not just the happy path. Don't claim visual correctness from types
alone.

## Filter DSL for metadata equality

Query endpoints take a `filter` string. Metadata equality is **two paired
clauses**, not `eq(metadata.key, ...)`:

```
and(eq(metadata_key, "ls_agent_purpose"), eq(metadata_value, "coding"))
```

Combine with `and(...)` / `or(...)`; other examples: `has(tags, "prod")`,
`eq(name, "ChatOpenAI")`, `eq(is_root, true)`.

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
