package hostapi

import (
	"strings"
	"testing"
	"time"
)

func TestListsSnapshotsWithTruePrefix(t *testing.T) {
	server := serveOnce(t, 200, `{"snapshots":[
		{"id":"1","name":"agent--setup-aaa","status":"ready"},
		{"id":"2","name":"other--setup-bbb","status":"ready"},
		{"id":"3","name":"agent--setup-ccc","status":"building"}]}`)

	found, err := testClient(t, server.URL).ListSnapshotsWithPrefix(t.Context(), "agent--setup-")

	if err != nil || len(found) != 2 {
		t.Fatalf("found = %+v, %v", found, err)
	}
	if line := server.Only(t).Line(); !strings.HasPrefix(line, "GET /api/v2/sandboxes/snapshots?") {
		t.Fatalf("request = %s", line)
	}
}

func TestFindReadySnapshotByNameWantsAnExactReadyMatch(t *testing.T) {
	body := `{"snapshots":[{"id":"1","name":"agent--setup-a","status":"building"},{"id":"2","name":"agent--setup-ab","status":"ready"}]}`
	found, err := testClient(t, serveOnce(t, 200, body).URL).FindReadySnapshotByName(t.Context(), "agent--setup-a")
	if err != nil || found != nil {
		t.Fatalf("found = %+v, %v; want none", found, err)
	}
	found, err = testClient(t, serveOnce(t, 200, body).URL).FindReadySnapshotByName(t.Context(), "agent--setup-ab")
	if err != nil || found == nil || found.ID != "2" {
		t.Fatalf("found = %+v, %v; want snapshot 2", found, err)
	}
}

func TestCaptureSnapshotPostsAndWaits(t *testing.T) {
	server := serveSequence(t,
		reply{status: 200, body: `{"id":"snap-1","name":"agent--setup-abc","status":"building"}`},
		reply{status: 200, body: `{"id":"snap-1","name":"agent--setup-abc","status":"ready","tags":["v1"]}`},
	)

	snapshot, err := testClient(t, server.URL).CaptureSnapshot(t.Context(), "builder", "agent--setup-abc", 10*time.Second)

	if err != nil || snapshot.Status != "ready" || strings.Join(snapshot.Tags, ",") != "v1" {
		t.Fatalf("snapshot = %+v, %v", snapshot, err)
	}
	requests := server.Requests()
	if requests[0].Line() != "POST /api/v2/sandboxes/boxes/builder/snapshot" || string(requests[0].Body) != `{"name":"agent--setup-abc"}` {
		t.Fatalf("capture = %s %s", requests[0].Line(), requests[0].Body)
	}
	if requests[1].Line() != "GET /api/v2/sandboxes/snapshots/snap-1" {
		t.Fatalf("poll = %s", requests[1].Line())
	}
}

func TestCreateSnapshotFromDockerImageNamesItsRegistry(t *testing.T) {
	server := serveSequence(t,
		reply{status: 200, body: `{"id":"snap-1","status":"building"}`},
		reply{status: 200, body: `{"id":"snap-1","status":"failed"}`},
	)

	_, err := testClient(t, server.URL).CreateSnapshotFromDockerImage(t.Context(), "agent--setup-abc", "ghcr.io/acme/agent:1", 1<<30, time.Minute, "reg-1")

	if err == nil || err.Error() != "snapshot snap-1 failed to become ready" {
		t.Fatalf("err = %v", err)
	}
	create := server.Requests()[0]
	if create.Line() != "POST /api/v2/sandboxes/snapshots" ||
		!jsonEqual(t, create.JSON(t), `{"name":"agent--setup-abc","docker_image":"ghcr.io/acme/agent:1","fs_capacity_bytes":1073741824,"registry_id":"reg-1"}`) {
		t.Fatalf("create = %s %s", create.Line(), create.Body)
	}
}

func TestWaitForSnapshotGivesUpAtTheDeadline(t *testing.T) {
	server := serveOnce(t, 200, `{"id":"snap-1","status":"building"}`)
	_, err := testClient(t, server.URL).WaitForSnapshot(t.Context(), "snap-1", 0)
	if err == nil || err.Error() != "snapshot snap-1 did not become ready within 0s" {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteSnapshotTreatsConflictAsInUse(t *testing.T) {
	server := serveOnce(t, 409, `{"error":"Conflict","message":"snapshot is in use by one or more sandboxes; release them before deleting"}`)
	outcome, err := testClient(t, server.URL).DeleteSnapshot(t.Context(), "stale-id")
	if err != nil || outcome != SnapshotDeleteInUse {
		t.Fatalf("outcome = %v, %v", outcome, err)
	}
	if line := server.Only(t).Line(); line != "DELETE /api/v2/sandboxes/snapshots/stale-id" {
		t.Fatalf("request = %s", line)
	}
}

func TestDeleteSnapshotTreatsNotFoundAsMissing(t *testing.T) {
	outcome, err := testClient(t, serveOnce(t, 404, "").URL).DeleteSnapshot(t.Context(), "gone")
	if err != nil || outcome != SnapshotDeleteMissing {
		t.Fatalf("outcome = %v, %v", outcome, err)
	}
	outcome, err = testClient(t, serveOnce(t, 204, "").URL).DeleteSnapshot(t.Context(), "snap")
	if err != nil || outcome != SnapshotDeleteDeleted {
		t.Fatalf("outcome = %v, %v", outcome, err)
	}
}

func TestURLEncodingPathEscapesAllButUnreservedBytes(t *testing.T) {
	if got := urlencodingPath("a.b_c-D9/é ~"); got != "a.b_c-D9%2F%C3%A9%20%7E" {
		t.Fatalf("urlencodingPath = %q", got)
	}
}
