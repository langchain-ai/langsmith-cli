package hostapi

import (
	"fmt"
	"strings"
	"testing"
)

func TestManagedCronsSearchPayloadDoesNotFilterByAssistantID(t *testing.T) {
	payload := managedCronsSearchPayload(100)
	if _, ok := payload["assistant_id"]; ok || payload["offset"] != 100 {
		t.Fatalf("payload = %v", payload)
	}
}

func cronPage(offset, count int) string {
	items := make([]string, 0, count)
	for id := offset; id < offset+count; id++ {
		items = append(items, fmt.Sprintf(`{"cron_id":"%d","metadata":{"mda_managed":true}}`, id))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func TestSearchCollectsAllPagesBeforeReturningRecords(t *testing.T) {
	server := serveSequence(t, reply{status: 200, body: cronPage(0, 100)}, reply{status: 200, body: cronPage(100, 1)})

	result, err := testClient(t, "http://unused.invalid").SearchManagedCrons(t.Context(), server.URL+"/")
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 101 || field(result[100], "cron_id") != "100" {
		t.Fatalf("got %d crons, last %v", len(result), result[len(result)-1])
	}
	for i, request := range server.Requests() {
		if request.Line() != "POST /runs/crons/search" || request.Header.Get("X-Auth-Scheme") != "langsmith" || request.Header.Get("X-Api-Key") != "key" {
			t.Errorf("request %d = %s, headers %v", i, request.Line(), request.Header)
		}
		if offset := field(request.JSON(t), "offset"); offset != float64(i*100) {
			t.Errorf("request %d offset = %v", i, offset)
		}
	}
}

func TestSearchReadsAnItemsEnvelope(t *testing.T) {
	server := serveOnce(t, 200, `{"items":[{"cron_id":"a"}]}`)
	result, err := testClient(t, "http://unused.invalid").SearchManagedCrons(t.Context(), server.URL)
	if err != nil || len(result) != 1 {
		t.Fatalf("result = %v, %v", result, err)
	}
	_, err = testClient(t, "http://unused.invalid").SearchManagedCrons(t.Context(), serveOnce(t, 200, `{"crons":[]}`).URL)
	if err == nil || err.Error() != "unexpected cron search response shape" {
		t.Fatalf("err = %v", err)
	}
}

func TestCronWritesTargetTheAgentServer(t *testing.T) {
	server := serveSequence(t,
		reply{status: 200, body: `{"cron_id":"c1"}`},
		reply{status: 200, body: `{"cron_id":"c2"}`},
		reply{status: 204},
	)
	c := testClient(t, "http://unused.invalid")

	created, err := c.CreateCron(t.Context(), server.URL, map[string]any{"schedule": "0 * * * *"})
	if err != nil || created["cron_id"] != "c1" {
		t.Fatalf("CreateCron = %v, %v", created, err)
	}
	if _, err := c.CreateThreadCron(t.Context(), server.URL, "thread-1", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteCron(t.Context(), server.URL, "c1"); err != nil {
		t.Fatal(err)
	}

	var lines []string
	for _, request := range server.Requests() {
		lines = append(lines, request.Line())
	}
	if strings.Join(lines, "\n") != "POST /runs/crons\nPOST /threads/thread-1/runs/crons\nDELETE /runs/crons/c1" {
		t.Fatalf("requests = %v", lines)
	}
}
