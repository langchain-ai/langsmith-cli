package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestCLIErrorProcessHelper(t *testing.T) {
	if os.Getenv("LANGSMITH_CLI_ERROR_TEST_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"langsmith"}, os.Args[i+1:]...)
			main()
			return
		}
	}
	t.Fatal("missing command arguments")
}

func TestCLIErrorStreams(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code string
	}{
		{"unknown before flags", []string{"does-not-exist", "--format=json"}, "command_failed"},
		{"unknown after flags", []string{"--format", "json", "does-not-exist"}, "command_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			process := exec.Command(os.Args[0], append([]string{"-test.run=^TestCLIErrorProcessHelper$", "--"}, tc.args...)...)
			process.Env = append(os.Environ(), "LANGSMITH_CLI_ERROR_TEST_HELPER=1")
			var stdout, stderr bytes.Buffer
			process.Stdout = &stdout
			process.Stderr = &stderr
			err := process.Run()
			if err == nil || process.ProcessState.ExitCode() != 1 {
				t.Fatalf("expected exit 1: %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout contains diagnostic: %s", stdout.String())
			}
			var result commandErrorEnvelope
			if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
				t.Fatalf("stderr not one JSON document: %s", stderr.String())
			}
			if result.Error.Code != tc.code {
				t.Fatalf("code=%s expected=%s", result.Error.Code, tc.code)
			}
		})
	}
}
