package cmd

import (
	"encoding/json"
	"testing"
)

func TestModelSerializationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, node string
		valid      bool
	}{
		{"secret", `{"lc":1,"type":"secret","id":["OPENAI_API_KEY"]}`, true},
		{"constructor", `{"lc":1,"type":"constructor","id":["model"],"kwargs":{}}`, true},
		{"unsupported", `{"lc":1,"type":"not_implemented","id":["model"]}`, false},
		{"version", `{"lc":2,"type":"secret","id":["key"]}`, false},
		{"secret value", `{"lc":1,"type":"secret","id":["key"],"value":"hidden"}`, false},
		{"missing kwargs", `{"lc":1,"type":"constructor","id":["model"]}`, false},
		{"nested invalid", `{"lc":1,"type":"constructor","id":["model"],"kwargs":{"nested":[{"lc":1,"type":"secret","id":[]}]}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var node any
			if err := json.Unmarshal([]byte(tc.node), &node); err != nil {
				t.Fatal(err)
			}
			if got := validModelSerialization(node, 0); got != tc.valid {
				t.Fatalf("valid=%v", got)
			}
		})
	}
	var deep any = "leaf"
	for i := 0; i < 66; i++ {
		deep = []any{deep}
	}
	if validModelSerialization(deep, 0) {
		t.Fatal("accepted excessive nesting")
	}
}
