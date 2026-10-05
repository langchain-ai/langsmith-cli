# Chart recipes

Source: [Monitor projects with dashboards](https://docs.langchain.com/langsmith/dashboards), reviewed September 18, 2026. Product docs describe behavior, not exact JSON contracts. The restrictions below also reflect the current UI form and renderer. Confirm deployment support.

## Compatibility and scope

| Intent | Modern chart type | Constraints |
| --- | --- | --- |
| Trends / percentile comparison | `line` | Same-unit metrics OR one grouped metric |
| Input/output or volume comparison | `bar` | Stacked bars; same-unit metrics OR one grouped metric |
| One headline value | `kpi` | One metric, no grouping |
| Ranked categories | `top-k` | One metric, one grouping attribute |
| Composition | `pie` | Donut; one metric, one grouping attribute |
| Inspect runs/experiment rows | `table` | Current UI supports count only, one series; not latency/cost rankings |
| Explain scope | `text` | Markdown, no metric series |

Grouping and multiple metrics are mutually exclusive; only one group-by attribute is allowed. Feedback-label grouping requires run count, not score aggregation. Ratios have grouping restrictions: verify the exact combination. Legacy custom dashboards offer line/bar only.

Input versus output tokens belongs in two-series bars/lines, not a two-series donut. A token-share donut can use one token-sum metric grouped by model. For model latency choose grouped lines or ranked bars, not a table.

`run_filter` selects individual spans; `trace_filter` selects by root properties; `tree_filter` selects whole traces if any span matches. To count journeys through a decision node, combine its tree filter with a root-run filter so each trace is counted once. Query-path support varies: never silently drop a tree filter when translating to analytics.

Multiple projects pool runs; group by project to compare them. Choose projects OR one dataset, not both. Current dataset charts are tables with no tracing filters, grouping, or tracing time range. Metadata/tags do not propagate between parents and children.

Use actual resolved UUIDs in place of `PROJECT_ID` and `DASHBOARD_ID`. JSON flags accept inline JSON, a file path, or `@file.json`. Shell single quotes preserve JSON double quotes; files are easier for larger layouts.

## Data chart and query

```bash
langsmith dashboard chart preview --config chart.json --query window.json
langsmith dashboard create --title "Service health"
langsmith dashboard chart create --dashboard-id DASHBOARD_ID --config @chart.json --dry-run
langsmith dashboard chart create --dashboard-id DASHBOARD_ID --config @chart.json
langsmith dashboard get DASHBOARD_ID
langsmith dashboard get DASHBOARD_ID --query @window.json
```

`chart.json` — count LLM invocations grouped by observed model metadata:

```json
{
  "title": "LLM calls by model",
  "chart_type": "bar",
  "series": [{
    "name": "LLM calls",
    "metric_definition": {"type": "count"},
    "filter_definition": {"source_type": "tracing_project", "project_ids": ["PROJECT_ID"], "run_filter": "eq(run_type, \"llm\")"},
    "group_by_definitions": [{"attribute": "metadata", "path": "ls_model_name"}]
  }]
}
```

`window.json` — replace with the desired bounded UTC window:

```json
{
  "start_time": "2026-09-17T00:00:00Z",
  "end_time": "2026-09-18T00:00:00Z",
  "stride": {"hours": 1}
}
```

Default `get` requests definitions with `omit_data: true`; explicit queries retrieve data. Use realistic bucket sizes: minute buckets over months are not a sensible first query.

Modern rendering needs these definition fields. Legacy-only metric/filters may save and preview successfully while modern UI charts remain blank. Read [verification.md](verification.md) before saving; it includes the renderer's analytics request and saved-data checks.

## Useful adaptations (legacy metric names)

| Question | Series metric / scope | Grouping or caveat |
| --- | --- | --- |
| Requests over time | `run_count`; `eq(is_root, true)` | Roots, not every span |
| Tool invocations | `run_count`; `eq(run_type, "tool")` | `group_by: {"attribute":"name","max_groups":20}` |
| Model spend | `total_cost`; LLM spans | Model metadata verified from traces |
| User spend | `total_cost`; roots | Metadata path `user_id` only if observed; disclose unassigned users |
| Evaluation scores | `feedback_score_avg`; selected run population | Supply `feedback_key` in each series |
| Evaluation coverage | `feedback`; same population/key | Read score count and error count, not just averages |

`max_groups` limits returned groups; top groups are not all users/models. A per-user cost breakdown is not an average cost per unique user.

Feedback values may be objects containing statistics rather than scalars. Inspect the response: score charts use the average and volume charts use the count. Preserve missing values and errors. Thread feedback requires deduplication/scope verification before reporting unique-thread rates.

For modern metrics use, for example, `{"type":"sum","field":"total_cost"}`, `{"type":"sum","field":"prompt_tokens"}`, or `{"type":"percentile","field":"latency_seconds","params":{"p":0.99}}`. Feedback average uses `{"type":"avg","field":"feedback_score","params":{"feedback_key":"VERIFIED_KEY"}}`. Verify feedback support on the renderer path, not only the legacy preview. If retaining both legacy and modern definitions, keep their populations and metrics consistent.

Pair feedback scores with coverage/error counts. Never average bucket percentiles into an overall percentile or average bucket averages without weights. Ratios need explicit numerator, denominator, filters, and zero-denominator behavior. High input-token volume is a reason to inspect context/caching, not proof of equivalent billed spend.

Useful compact starting set: root latency p50/p99; tool volume with a separate failure/latency breakdown; LLM spend by observed model; and verified quality scores with coverage. Add business-journey counts only when the trace structure contains the required decisions. Do not automatically add every recipe or duplicate prebuilt charts.

## Text and layout

Text blocks use `{"chart_type":"text","markdown":"## Scope\nRoot-level traffic; LLM-span model costs."}` with `chart create --dashboard-id`. Do not include data-chart title or series fields. Edit existing text with `{"markdown":"..."}`; do not convert a data chart to text through update.

Dashboard updates accept title, description, index, and layout. Chart updates accept writable chart fields; preserve series IDs when replacing series. Read responses also include read-only fields: do not send the entire response back as a configuration.

Reads may expand grouped series into display IDs such as `UUID:group`. These are not writable UUIDs: obtain stored IDs from an unexpanded definition or verified original create response rather than replaying display series.

For responsive layouts, `layout.version` is `1`; `layout.breakpoints.sm.rows` and `.md.rows` each contain rows with `height_units` and `items` (`chart_id`, `width_units`). Use the same unique chart IDs at each breakpoint. Width totals are 60 for sm and 120 for md, with minimum item width 20. Typical heights are 8 for charts and 4 for short text blocks. Read the existing layout first and preserve unrelated charts. Layout acceptance is checked by the service; preview does not render it.

Product docs also describe prebuilt cloning and project-default dashboard linking. Do not claim these have dedicated CLI commands unless installed help confirms them.
