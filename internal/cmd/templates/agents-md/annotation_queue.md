## This template: annotation-queue review

A 3-pane queue-review UI: run list, inputs/outputs viewer, feedback rubric.
It picks a queue via `GET /api/v1/annotation-queues` and drives everything
else through `window.langsmith.call` — see `src/api.ts`.

This is just a starting point, not a spec. Change the layout, the review
flow, or the whole concept — rip out anything here and build whatever app
you actually want.
