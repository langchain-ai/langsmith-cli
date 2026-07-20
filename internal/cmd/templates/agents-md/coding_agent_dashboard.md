## This template: coding-agent dashboard

Headline stats (turns, threads, cost, tokens, errors, latency) plus a
recent-runs table, for one project's coding-agent traces
(`ls_agent_purpose == "coding"`). Stats come from LangSmith's `runs/stats` /
`runs/group/stats` endpoints rather than sampling raw runs — see `src/api.ts`.

This is just a starting point, not a spec. Change the metrics, the layout, or
the whole concept — rip out anything here and build whatever app you actually
want.
