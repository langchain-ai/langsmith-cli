## This template: annotation-queue review

A 3-pane queue-review UI: item list (RUN + THREAD), type-specific viewer
(run IO or thread chat), and feedback rubric. It picks a queue via
`GET /api/v1/annotation-queues` and drives everything else through
`window.langsmith.call` — see `src/api.ts`.

Membership comes from `GET .../items` (cursor pages + `.../items/count`).
Hydrate RUN with `GET /v2/runs/{id}` and THREAD with
`POST /v1/trajectory` (`format: "messages"`). Feedback uses `run_id` or
`feedback_thread_id` depending on `item_type`.

This is just a starting point, not a spec. Change the layout, the review
flow, or the whole concept — rip out anything here and build whatever app
you actually want.
