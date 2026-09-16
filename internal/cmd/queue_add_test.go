package cmd

import (
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestQueueAdditionVerifiesIdentity(t *testing.T) {
	for _, mode := range []string{"run", "thread"} {
		t.Run(mode, func(t *testing.T) {
			source := queueAddItem{RunID: "11111111-1111-4111-8111-111111111111"}
			item := langsmith.AnnotationQueueItemNewResponseItem{ID: "queue-item", QueueID: "22222222-2222-4222-8222-222222222222", ProjectID: deleteTestProjectID, RunID: "11111111-1111-4111-8111-111111111111", ItemType: "RUN"}
			if mode == "thread" {
				source = queueAddItem{ThreadID: "conversation"}
				item.RunID = ""
				item.ThreadID = "conversation"
				item.ItemType = "THREAD"
			}
			valid := &langsmith.AnnotationQueueItemNewResponse{Items: []langsmith.AnnotationQueueItemNewResponseItem{item}}
			if !queueAdditionMatches(valid, "22222222-2222-4222-8222-222222222222", deleteTestProjectID, source) {
				t.Fatal("valid identity rejected")
			}
			for _, field := range []string{"id", "queue", "project", "source", "type"} {
				bad := item
				switch field {
				case "id":
					bad.ID = ""
				case "queue":
					bad.QueueID = "other"
				case "project":
					bad.ProjectID = "other"
				case "source":
					bad.RunID = "other"
					bad.ThreadID = "other"
				case "type":
					bad.ItemType = "unknown"
				}
				if queueAdditionMatches(&langsmith.AnnotationQueueItemNewResponse{Items: []langsmith.AnnotationQueueItemNewResponseItem{bad}}, "22222222-2222-4222-8222-222222222222", deleteTestProjectID, source) {
					t.Fatalf("accepted incorrect %s", field)
				}
			}
			if queueAdditionMatches(nil, "22222222-2222-4222-8222-222222222222", deleteTestProjectID, source) {
				t.Fatal("nil accepted")
			}
			if queueAdditionMatches(&langsmith.AnnotationQueueItemNewResponse{}, "22222222-2222-4222-8222-222222222222", deleteTestProjectID, source) {
				t.Fatal("empty accepted")
			}
		})
	}
}
