---
name: langsmith-dashboard
description: Create, inspect, or improve LangSmith dashboards using the CLI, choosing metrics and charts from observed trace structure and feedback coverage.
---

# Evidence-first dashboards

Use the installed CLI's `dashboard --help` and subcommand help to confirm availability. If dashboard commands are missing, report the version mismatch; do not silently install or switch accounts.

The [dashboard guide](https://docs.langchain.com/langsmith/dashboards) distinguishes prebuilt project dashboards, modern custom dashboards (Cloud US), and legacy dashboards (Cloud EU/APAC and self-hosted). Confirm the actual deployment; API acceptance alone does not establish rendering support. Prefer prebuilt monitoring when it answers the question; custom charts should add a useful comparison or business signal.

## Discover before designing

1. Resolve the intended workspace and project to IDs. Preserve the user's profile and defaults; use explicit workspace/project scope for reproducible operations.
2. Agree on the question and time window: service health, tool usage, model cost, user cost, or evaluation quality. For an exploratory request, propose a small dashboard based on evidence rather than asking for every chart setting.
3. Inspect a bounded sample of recent roots and representative child runs using `trace` and `run` help. Check run types, parent relationships, timestamps, token/cost availability, and metadata keys. Inspect one representative trace tree before deciding aggregation scope. Do not dump customer inputs or full metadata when keys suffice.
4. Check candidate grouping fields across multiple traces. A populated `user_id` is not proof of an authenticated person; identify guests, missing values, and identity semantics. Model metadata may be absent or provider-specific. Do not substitute run names for model identities without evidence.
5. Inspect feedback keys, counts, errors, and source/scope. Separate online, offline, manual, and thread feedback where observable; otherwise label uncertainty. A small sample discovers structure, not population-wide coverage.

With little context, ask only for a missing target or objective that materially changes the result. Otherwise state assumptions, start with the last seven days and a small trace sample, and propose 3–6 complementary charts. Use an equal-length preceding window for trend comparisons. Do not fabricate unavailable business signals.

Metadata and tags do not propagate between roots and children. Verify grouping keys on the exact run population being charted. Prefer missing/present counts over exposing user identifiers.

## Design and validate

Read [chart-recipes.md](references/chart-recipes.md) for chart types, JSON examples, filters, and layout guidance.

- Use root runs for end-to-end traffic, latency, and rolled-up total cost. Use LLM spans for model breakdowns and tool spans for tool calls. Never sum root cost together with descendants.
- Label units, aggregation, time window, and population. A tool span counts an invocation, not necessarily a successful action. Cost per user needs verified user metadata and a stated treatment of missing identities.
- Use the chart compatibility rules in the recipes, not general visualization intuition: tables are not arbitrary metric tables, and a pie is not a collection of unrelated metrics.
- Before writing, describe each chart as question → population/filter → metric/unit → grouping → visualization → decision it supports. Avoid decorative charts, unrelated units on one axis, and duplicated metrics. Put service health first, diagnostic breakdowns second, and quality/business outcomes last.
- Preview each data query with a bounded window. Empty data is not zero; feedback errors with no scores are not successful zero scores. Preview validates data querying, not rendering or the complete save payload. Dry-run checks local structure only; the service still validates metrics and deployment-specific fields.

## Save and verify

Create or modify only dashboards within the user's request. Do not create persistent resources for a read-only question. No model inference is needed to create charts.

Read an existing dashboard before updating it. Preserve chart/series IDs and unrelated content: supplied arrays replace their existing values. Use dry-run before consequential changes, then apply the reviewed payload. Do not automatically replay an uncertain write; reconcile with list/get first.

Read [verification.md](references/verification.md) before saving or repairing charts. Read back saved definitions and execute the renderer's query path with those definitions and the UI's time range: legacy preview alone cannot validate modern charts. Check actual numeric data, not just HTTP success or field presence. Visually verify when browser access is authorized; otherwise explicitly report rendering as unverified. Never bypass denied browser access.

Deliver the dashboard link, chart purposes, a few observations with time-window and coverage caveats, and the exact verification level. Retain resources unless deletion was requested. Successfully saved charts with failed renderer queries are not finished.
