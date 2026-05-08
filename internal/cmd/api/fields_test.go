package api

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseFields_RawFieldsAreStrings(t *testing.T) {
	got, err := parseFields([]string{"limit=10", "draft=false", "empty="}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{"limit": "10", "draft": "false", "empty": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseFields_TypedFields(t *testing.T) {
	got, err := parseFields(nil, []string{"limit=10", "draft=false", "enabled=true", "name=test", "nil=null"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{"limit": 10, "draft": false, "enabled": true, "name": "test", "nil": nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseFields_NestedFields(t *testing.T) {
	got, err := parseFields(
		[]string{"metadata[source]=cli"},
		[]string{"config[limit]=5", "config[enabled]=true"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{
		"metadata": map[string]any{"source": "cli"},
		"config":   map[string]any{"limit": 5, "enabled": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseFields_ArrayFields(t *testing.T) {
	got, err := parseFields(
		[]string{"tags[]=alpha", "tags[]=beta"},
		[]string{"examples[][id]=1", "examples[][name]=first", "examples[][id]=2"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{
		"tags": []any{"alpha", "beta"},
		"examples": []any{
			map[string]any{"id": 1, "name": "first"},
			map[string]any{"id": 2},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseFields_TypedValueFromFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "field-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("hello from file"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := parseFields(nil, []string{"body=@" + f.Name()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["body"] != "hello from file" {
		t.Fatalf("got %#v", got["body"])
	}
}

func TestParseFields_Errors(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   string
	}{
		{"missing equals", []string{"name"}, "requires a value"},
		{"empty key", []string{"=value"}, "invalid field key"},
		{"override", []string{"name=x", "name=y"}, "unexpected override"},
		{"object conflict", []string{"config=x", "config[name]=y"}, "expected object"},
		{"array conflict", []string{"items=x", "items[]=y"}, "expected array"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFields(tt.fields, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %q, want containing %q", err.Error(), tt.want)
			}
		})
	}
}
