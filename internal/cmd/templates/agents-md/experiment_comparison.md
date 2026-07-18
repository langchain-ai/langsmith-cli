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

One shared metric — the "Focus metric" dropdown in `src/App.tsx` — drives
every metric-scoped section (Scoreboard, Regression scorecard, Delta
distribution, Baseline vs comparison, and the per-example sort). It's a
single control row above those sections, not repeated per chart.

- `src/api.ts` — the four calls above
- `src/components/Pickers.tsx` — dataset → baseline → comparison selection
- `src/lib/delta.ts` — safe score/latency/cost accessors, `improvementDelta`
  (signed, direction-aware), and baseline verdict/colors built on top of it
- `src/lib/metrics.ts` — shared metric descriptors, formats, series
  letters/colors, `aggregateValue` (reads a `RunMetric`'s value off an
  `Aggregate`), and `histogram` (fixed-bin bucketing for the delta
  distribution)
- `src/components/primitives.tsx` — the small chart/layout building blocks
  every section below is made of: `Section`, `StatTile` (with an optional
  colored delta pill), `BarList` (grouped mini bar chart), `StackedBar`
  (win/tie/loss proportion bars), `Legend`, `Empty`
- `src/components/Scoreboard.tsx` — hero stat tiles: the focus metric's value
  per experiment, with a colored delta vs baseline
- `src/components/SummaryPanel.tsx` — every metric as a mini bar chart across
  experiments (falls back to plain stat tiles when only the baseline is
  picked, so it's never a one-bar chart)
- `src/components/Scorecard.tsx` — per-comparison win/tie/loss proportions vs
  baseline, one stacked bar per metric
- `src/components/DeltaHistogram.tsx` — per comparison, the distribution of
  each example's signed improvement on the focus metric (green bars = wins,
  red = regressions)
- `src/components/ScatterPlot.tsx` — baseline vs comparison per metric (first four series)
- `src/components/ExampleTable.tsx` — per-example outputs and a sortable, selectable metric
- `src/App.tsx` — fetches, derives aggregates and feedback keys, wires state;
  holds the previous render (dimmed) while a new selection loads instead of
  flashing back to a bare loading screen

Series colors come from `--series-1..4` in `index.css` (validated CVD-safe,
stepped separately for the dark surface) so charts theme without branching.
Win/loss/tie states use the semantic `--bg-success-strong` / `--border-strong`
/ `--bg-error-strong` tokens instead, since those encode a status (good/bad),
not a series.
