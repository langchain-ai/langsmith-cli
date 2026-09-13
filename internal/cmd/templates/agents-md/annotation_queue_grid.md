## This template: annotation-queue review, as a spreadsheet grid

Reviews an annotation queue as a spreadsheet grid — rows are queue items
(RUN or THREAD), columns are rubric keys, editable inline. See `src/api.ts`
and `src/components/DataGrid.tsx`.

List via `GET .../items` + `.../items/count`. Expand a RUN to hydrate IO
(`GET /v2/runs/{id}`); expand a THREAD for chat via
`POST /v1/trajectory` (`format: "messages"`). Cell feedback uses `run_id` or
`feedback_thread_id` from `item_type`.

This is just a starting point, not a spec. Change the grid, the review flow,
or the whole concept — rip out anything here and build whatever app you
actually want.
