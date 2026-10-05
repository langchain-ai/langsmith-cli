# Verify queries, saved charts, and rendering

Use explicit workspace/profile context where needed; do not change defaults silently. Replace placeholder UUIDs and dates with the resolved project and intended bounded window. No model inference is required.

## Before saving

1. Check chart-type constraints in [chart-recipes.md](chart-recipes.md). State question, population, units, grouping, and the decision the chart supports.
2. Run `dashboard chart create --dashboard-id DASHBOARD_ID --config @chart.json --dry-run`. This checks local structure and known visualization constraints, not server semantics. For updates, supply chart_type and series together: omitted fields are not read or checked, and dry-run reports `validation_scope: supplied_fields_only`.
3. Run `dashboard chart preview --config @chart.json --query @window.json`. This tests the charts query path, not every modern renderer. If modern-only fields fail on legacy preview, report the limitation and test analytics; do not remove required modern definitions just to make preview pass.
4. Test the modern renderer path using the same metric, filters, groups, and dates. Ranked bar/donut/KPI use snapshots; line/bar use time-series intervals. Tables follow run/experiment reads and must be verified separately, not through a numeric analytics test.

For the model-call chart in the recipes, `analytics.json` is:

```json
{
  "metric": {"type": "count"},
  "group_by": [{"attribute": "metadata", "path": "ls_model_name"}],
  "filters": {
    "project_ids": ["PROJECT_ID"],
    "run_filter": "eq(run_type, \"llm\")"
  },
  "time_range": {
    "start_time": "2026-09-11T18:00:00Z",
    "end_time": "2026-09-18T18:00:00Z",
    "time_interval": "PT6H"
  }
}
```

```bash
langsmith api /api/v2/runs/analytics -X POST --body @analytics.json
```

For snapshots omit `time_interval`; ranked bars can set `sort_by: "value_desc"`. The charts API uses `stride: {"hours":6}` at the top level of its query instead; do not interchange request shapes. Preserve trace filters when translating. Check tree-filter support for the particular path; do not claim a query is equivalent when a filter is missing.

Inspect `items[].type`. Snapshot `data` is a number/null; time-series `data` contains timestamp/value points. Count valid numeric values, missing values, and returned groups. Inspect truncation/unmatched groups and confirm expected magnitude. HTTP 200, an array, or a nonempty definition is not proof of usable data. Empty data may be legitimate; establish whether the time range and selected population actually have matching runs.

## After saving

```bash
langsmith dashboard chart create --dashboard-id DASHBOARD_ID --config @chart.json
langsmith dashboard get DASHBOARD_ID
langsmith dashboard get DASHBOARD_ID --query @window.json
```

Reconstruct the renderer query from the **saved** series, not only the input file. Verify metric/filter/group definitions, project IDs, chart type and layout survived. Rerun and inspect numeric data. Query dates do not set a saved UI date-range default.

When authorized browser access is available, open the chart at the same dates and verify axes, labels, legends, units, groups, loading/errors and layout. Do not bypass a browser denial. If browser access is unavailable, say: “Saved definitions and analytics verified; visual rendering unverified.” Request a screenshot for visual issues.

If a chart is blank or invalid, check modern definition fields and visualization constraints first, then query errors, scope, dates, grouping coverage and data sparsity. Do not automatically create another chart or blame the time range. Preserve existing IDs when repairing authorized resources.

Handoff should distinguish local validation, preview, saved analytics and visual rendering. Include a dashboard link plus a few observations tied to the tested window; never label an unverified render fully working.
