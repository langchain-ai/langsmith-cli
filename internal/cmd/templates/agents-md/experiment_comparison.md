## This app: experiment-comparison dashboard

Compares evaluation experiments for one dataset. It picks its own dataset and
experiments (`src/components/Pickers.tsx`) and fetches everything via
`window.langsmith.call`; `src/api.ts` wraps every call below.

LangSmith's built-in comparison view colors regressions for **feedback scores
only**. This template also colors **latency, cost, and token** regressions vs
the chosen baseline — all computed client-side in `src/lib/delta.ts` from the
same response. No SSE/delta endpoint is used. Delta colors use `--green-*` /
`--red-*` tokens (`text-success-primary` / `text-error-primary`), legible in
both themes.

## Endpoints this app uses

- `GET /api/v1/datasets` — list datasets (top-level array of `{ id, name }`)
- `GET /api/v1/sessions?reference_dataset=<id>&reference_free=false` — list the
  experiments (tracer sessions) for a dataset (array of `{ id, name }`)
- `POST /api/v1/datasets/{dataset_id}/runs` — per-example rows joined across
  experiments; body `{ session_ids: [...], limit, offset }`
- `GET /api/v1/feedback-configs?key=<k>` — `is_lower_score_better` per feedback
  key, so score deltas honor direction

## Reading the comparison response

`POST /api/v1/datasets/{id}/runs` returns an array of examples, each
`{ id, name, inputs, outputs, runs: [...] }`. Every run belongs to one
experiment via `run.session_id`, and carries:

- `outputs` / `outputs_preview`
- `start_time`, `end_time` — latency is `end_time - start_time` (computed here;
  there is no latency field)
- `total_tokens`, `total_cost` (cost is a numeric string)
- `error`
- `feedback_stats` — an **untyped nested map** `{ [key]: { avg, n, ... } }`.
  Read `feedback_stats[key].avg`, coerce to a number, and guard missing keys
  (see `scoreFor` in `src/lib/delta.ts`) — a missing score must yield null, not
  NaN, or it will miscolor the delta.

## Aggregates

Per-experiment aggregates (avg latency, total cost, avg tokens, avg score per
key) are derived client-side over the fetched examples — no separate
`feedback_stats`/session-stats call. The example list is capped (default 25);
the UI notes when it shows only the first N.

## What this app already does

Read-only by design: it visualizes and aggregates, and compares a baseline
against any number of comparison experiments. It never creates or deletes
datasets/experiments — add that yourself if you want it.

- `src/api.ts` — the four calls above
- `src/components/Pickers.tsx` — dataset → baseline → comparison selection
- `src/lib/delta.ts` — safe score/latency/cost accessors + baseline verdict/colors
- `src/lib/metrics.ts` — shared metric descriptors, formats, series letters/colors
- `src/components/SummaryPanel.tsx` — per-experiment aggregates, colored vs baseline
- `src/components/Scorecard.tsx` — per-comparison win/loss/tie counts vs baseline
- `src/components/ScatterPlot.tsx` — baseline vs comparison per metric (first four series)
- `src/components/ExampleTable.tsx` — per-example outputs and a sortable, selectable metric
- `src/App.tsx` — fetches, derives aggregates and feedback keys, wires state

Series colors come from `--series-1..4` in `index.css` (validated CVD-safe,
stepped separately for the dark surface) so charts theme without branching.
