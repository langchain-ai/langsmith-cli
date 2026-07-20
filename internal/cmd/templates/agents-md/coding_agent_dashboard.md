## This app: coding-agent dashboard

Headline stats for runs whose metadata `ls_agent_purpose == "coding"` in one
project — turns, threads, cost, tokens, error rate, latency — plus a bounded
recent-runs table for browsing. It picks its own project via
`GET /api/v1/sessions` (`src/components/ProjectBar.tsx`, one guided step)
because everything below is project-scoped. `src/api.ts` wraps every call.

**This app used to compute every number by pulling up to 100 raw runs per
run-type and aggregating client-side.** That silently undercounted on any
project with real weekly volume (see git history if you want the gory
details) — every "total" was actually "total across the 100 most recent
matches, older runs uncounted," with no visual indication of the cap. It has
since been rebuilt to get headline numbers from LangSmith's stats endpoints
instead, which aggregate server-side over the *entire* matching window with
no row limit. Keep that split when extending this app: a number the user
might trust as "the total" belongs in a stats call, never in something
computed from the bounded recent-runs sample.

## Stats endpoints (`src/api.ts` → `fetchProjectStats`)

Two independent calls, both filtered to `ls_agent_purpose == "coding"`:

- **`POST /api/v1/runs/stats`** — `{ session: [projectId], filter, start_time, is_root: true, select: [...] }`.
  Exact, whole-window aggregate over root runs (turns): `run_count`,
  `error_rate`, `latency_p50`, `latency_p99`, `latency_avg`, `total_tokens`,
  `prompt_tokens`, `completion_tokens`, `total_cost`, `prompt_cost`,
  `completion_cost`. Every field is optional in the response — a field this
  app didn't ask for, or one the backend couldn't compute, is simply absent.
  Field names mirror `schemas.RunStats` in smith-backend; if a tile in
  `OverviewPanel.tsx` unexpectedly shows "—", verify the field name against a
  live response before assuming the stat doesn't exist.
- **`POST /api/v1/runs/group/stats`** — `{ session_id: projectId, group_by: "conversation", filter, start_time }`.
  Same shape as above plus `group_count`, the **distinct thread count** — the
  only place thread count exists as a real server-side aggregate. There is no
  arbitrary-metadata-key grouped-count endpoint confirmed reachable ad-hoc
  (checked as of this rewrite); don't assume one exists for a future
  "breakdown by model" chart without re-verifying against smith-backend.

The two calls run **one at a time** (with a 250ms gap), not in parallel — a
lesson carried over from this app's raw-run-query days: several requests
fired together was enough on its own to trip a workspace's rate limit. Both
go through `callWithRetry` (5 attempts, exponential backoff capped at 6s).
`window.langsmith.call` drops the HTTP status of a failure before it reaches
the app — a 429 looks like any other error — so retry blindly rather than
string-matching "rate limit" out of a message.

If one call is still failing once its retry budget is exhausted, it does
**not** fail the other — `fetchProjectStats` catches it, records the scope
in `ProjectStats.failedScopes`, and returns `null` for that half only.
`App.tsx` renders whichever stats did load and shows a banner naming the
ones that didn't, instead of blanking the whole page over one bad call.

## Recent-runs table (`src/api.ts` → `fetchRecentRuns`)

One bounded `POST /api/v1/runs/query` call (`is_root: true`, same coding
filter, `limit: 100`, cursor ignored) powers `RunsTable.tsx` — a sample for
browsing recent activity, explicitly labeled as such in the UI. **Never wire
a headline stat off this data** — that's exactly the bug this app used to
have. If you need an accurate new aggregate, add it to the `select` list on
one of the two stats calls above instead of pulling more raw rows.

## Where the data lives on a run

- Custom metadata: `run.extra.metadata`.
- **Token & cost totals roll up onto the root run** (`total_tokens`,
  `total_cost`). Read economics from roots only — child totals are already
  included, so summing them double-counts.
- A non-null `run.error` counts as a failed run.

## Verified metadata keys still used here

- `thread_id`, `repository_name` — read through `src/lib/normalize.ts`
  (`threadOf`, `repoOf`), which falls back to `"unknown"` when absent. Other
  keys this app no longer reads (`ls_integration`, `ls_model_name`,
  `ls_tool_name`, `ls_subagent_type`, `git_branch`, user/contributor fields,
  …) are still present on real coding-agent traces if you're adding a panel
  that needs them — see git history for the old `normalize.ts` accessors.

## What this app renders

- `src/api.ts` — `fetchProjects`, `fetchProjectStats`, `fetchRecentRuns`
- `src/components/ProjectBar.tsx` — the one guided step (project picker)
- `src/components/OverviewPanel.tsx` — the stats-hero tile grid
- `src/components/RunsTable.tsx` — the bounded recent-runs table
- `src/lib/normalize.ts` / `format.ts` — metadata reads, display formatting
- `src/components/primitives.tsx` — `Section`, `StatTile` (optional status
  `tone`), `Empty`

To add a new headline stat: extend the `select` list on the relevant stats
call in `api.ts` and add a tile in `OverviewPanel.tsx` — don't reach for the
recent-runs sample. To add a new browsable column: extend `RunsTable.tsx`
and, if needed, the `select` list on `fetchRecentRuns` and an accessor in
`normalize.ts`.
