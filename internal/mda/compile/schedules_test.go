package compile

import (
	"encoding/json"
	"reflect"
	"testing"
)

func cron(t *testing.T, config map[string]any) map[string]any {
	t.Helper()
	entry, err := scheduleCron("daily", config, "agent", []string{"slack"})
	if err != nil {
		t.Fatalf("schedule should compile: %v", err)
	}
	return entry
}

func cronError(t *testing.T, config map[string]any) string {
	t.Helper()
	_, err := scheduleCron("daily", config, "agent", nil)
	if err == nil {
		t.Fatal("schedule should be rejected")
	}
	return err.Error()
}

func scheduleContext(t *testing.T, entry map[string]any) map[string]any {
	t.Helper()
	return entry["context"].(map[string]any)["mda_schedule"].(map[string]any)
}

func TestPromptBecomesUserMessageOnEphemeralThread(t *testing.T) {
	entry := cron(t, map[string]any{"cron": "0 9 * * *", "prompt": "Summarize"})

	want := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Summarize"}}}
	if !reflect.DeepEqual(entry["input"], want) {
		t.Fatalf("input: %#v", entry["input"])
	}
	if entry["on_run_completed"] != "delete" {
		t.Fatalf("on_run_completed: %v", entry["on_run_completed"])
	}
	if _, ok := entry["thread_id"]; ok {
		t.Fatalf("thread_id: %v", entry["thread_id"])
	}
}

func TestInputPassesThroughAndPersistentThreadSetsThreadID(t *testing.T) {
	entry := cron(t, map[string]any{
		"cron":   "0 9 * * *",
		"input":  map[string]any{"topic": "news"},
		"thread": map[string]any{"mode": "persistent", "id": "daily-thread"},
	})

	if !reflect.DeepEqual(entry["input"], map[string]any{"topic": "news"}) || entry["thread_id"] != "daily-thread" {
		t.Fatalf("entry: %#v", entry)
	}
	if _, ok := entry["on_run_completed"]; ok {
		t.Fatalf("on_run_completed: %v", entry["on_run_completed"])
	}
	if _, ok := scheduleContext(t, entry)["new_thread"]; ok {
		t.Fatalf("new_thread: %#v", entry)
	}
}

func TestDeliverToLandsInScheduleContextForBothSpellings(t *testing.T) {
	for _, key := range []string{"deliverTo", "deliver_to"} {
		entry := cron(t, map[string]any{
			"cron": "0 9 * * *", "prompt": "Summarize", key: map[string]any{"channel": "slack"},
		})
		want := map[string]any{"name": "daily", "deliver_to": map[string]any{"channel": "slack"}}
		if got := scheduleContext(t, entry); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %#v", key, got)
		}
	}
}

func TestConversationDeliverToKeepsItsEphemeralThread(t *testing.T) {
	entry := cron(t, map[string]any{
		"cron": "0 9 * * *", "prompt": "Summarize",
		"deliverTo": map[string]any{
			"channel": "slack",
			"to":      map[string]any{"type": "provider_conversation", "conversationId": "C1"},
		},
	})
	if entry["on_run_completed"] != "keep" || scheduleContext(t, entry)["new_thread"] != true {
		t.Fatalf("entry: %#v", entry)
	}

	for _, deliverTo := range []map[string]any{
		{"channel": "slack", "to": map[string]any{"type": "provider_thread", "conversationId": "C1", "threadId": "1.2"}},
		{"channel": "other", "to": map[string]any{"type": "provider_conversation", "conversationId": "C1"}},
		{"channel": "slack", "to": map[string]any{"type": "provider_conversation", "conversationId": "C1"}, "autoPost": false},
	} {
		entry := cron(t, map[string]any{"cron": "0 9 * * *", "prompt": "Summarize", "deliverTo": deliverTo})
		if entry["on_run_completed"] != "delete" {
			t.Fatalf("%v: on_run_completed %v", deliverTo, entry["on_run_completed"])
		}
		if _, ok := scheduleContext(t, entry)["new_thread"]; ok {
			t.Fatalf("%v: new_thread", deliverTo)
		}
	}
}

func TestRejectsBothOrNeitherPromptAndInput(t *testing.T) {
	for _, config := range []map[string]any{
		{"cron": "0 9 * * *"},
		{"cron": "0 9 * * *", "prompt": "Summarize", "input": map[string]any{}},
	} {
		if got := cronError(t, config); got != "schedule 'daily' must define exactly one of `prompt` or `input`" {
			t.Fatalf("%v: %s", config, got)
		}
	}
}

func TestRejectsInvalidScheduleFields(t *testing.T) {
	for _, tc := range []struct {
		config map[string]any
		want   string
	}{
		{map[string]any{"cron": "0 9 * * *", "prompt": "  "}, "schedule 'daily' prompt must be a non-empty string"},
		{map[string]any{"cron": "0 9 * *", "prompt": "Summarize"}, "schedule 'daily' cron must be a standard 5-field expression"},
		{map[string]any{"cron": "0 9 * * *", "prompt": "Summarize", "context": map[string]any{"channel": "x"}},
			"schedule 'daily' context keys `channel` and `mda_*` are reserved"},
		{map[string]any{"cron": "0 9 * * *", "prompt": "Summarize", "context": map[string]any{"mda_x": 1.0}},
			"schedule 'daily' context keys `channel` and `mda_*` are reserved"},
		{map[string]any{"cron": "0 9 * * *", "prompt": "Summarize", "thread": map[string]any{"mode": "persistent"}},
			"schedule 'daily' persistent thread requires an id"},
		{map[string]any{"cron": "0 9 * * *", "prompt": "Summarize", "thread": map[string]any{"mode": "forever"}},
			"schedule 'daily' has unsupported thread mode 'forever'"},
	} {
		if got := cronError(t, tc.config); got != tc.want {
			t.Fatalf("%v: got %q, want %q", tc.config, got, tc.want)
		}
	}
}

func TestCompileMaterializesTypescriptSchedulesAsCrons(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts":           tsAgent,
		"schedules/daily.md": "Write the daily digest.\n",
		"schedules/daily-digest.ts": "import { defineSchedule } from \"managed-deepagents\";\n" +
			"import prompt from \"./daily.md\" with { type: \"text\" };\n" +
			"export const schedule = defineSchedule({\n" +
			"  cron: \"0 8 * * 1-5\",\n  timezone: \"America/Los_Angeles\",\n  prompt,\n});\n",
	})

	_, err := f.deploy(t)
	requireErrorContains(t, err, "schedules require a root identity")

	writeFile(t, f.root(), "identity.ts", tsIdentity)
	f.reload(t)
	summary := mustCompile(t, f.deploy)

	if _, ok := readJSON(t, f.read(t, "langgraph.json"))["crons"]; ok {
		t.Fatal("langgraph.json declares crons")
	}
	want := map[string]any{
		"graph":    "agent",
		"schedule": "0 8 * * 1-5",
		"timezone": "America/Los_Angeles",
		"input": map[string]any{"messages": []any{
			map[string]any{"role": "user", "content": "Write the daily digest.\n"},
		}},
		"metadata": map[string]any{
			"mda_managed": true, "mda_managed_by": "deploy", "mda_schedule": "daily-digest",
		},
		"context":          map[string]any{"mda_schedule": map[string]any{"name": "daily-digest"}},
		"on_run_completed": "delete",
	}
	if got := summary.ScheduleCrons["daily-digest"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestCompilePreservesTypescriptScheduleDeliverTo(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts":    tsAgent,
		"identity.ts": tsIdentity,
		"schedules/monday-greeting.ts": "import { defineSchedule } from \"managed-deepagents\";\n" +
			"const BOTCAVE_CHANNEL_ID = \"C0123456789\";\n" +
			"export const schedule = defineSchedule({\n" +
			"  cron: \"0 9 * * 1\",\n  timezone: \"America/Los_Angeles\",\n  prompt: \"Say hello\",\n" +
			"  deliverTo: {\n    channel: \"slack\",\n" +
			"    to: { type: \"provider_conversation\", conversationId: BOTCAVE_CHANNEL_ID },\n" +
			"    autoPost: true,\n  },\n" +
			"  context: { team: \"eng\", dryRun: true },\n});\n",
	})

	summary := mustCompile(t, f.deploy)

	want := map[string]any{
		"mda_schedule": map[string]any{
			"name": "monday-greeting",
			"deliver_to": map[string]any{
				"channel":  "slack",
				"to":       map[string]any{"type": "provider_conversation", "conversationId": "C0123456789"},
				"autoPost": true,
			},
		},
		"team":   "eng",
		"dryRun": true,
	}
	cron := summary.ScheduleCrons["monday-greeting"]
	if !reflect.DeepEqual(cron["context"], want) {
		t.Fatalf("context: %#v", cron["context"])
	}
	if _, ok := cron["config"]; ok {
		t.Fatalf("config: %#v", cron)
	}

	g := typescriptProject(t, map[string]string{
		"agent.ts":    tsAgent,
		"identity.ts": tsIdentity,
		"schedules/monday-greeting.ts": "import { defineSchedule } from \"managed-deepagents\";\n" +
			"export const schedule = defineSchedule({\n  cron: \"0 9 * * 1\",\n  prompt: \"Say hello\",\n" +
			"  context: { channel: \"slack\" },\n});\n",
	})
	_, err := g.deploy(t)
	requireErrorContains(t, err, "schedule 'monday-greeting' context keys `channel` and `mda_*` are reserved")
}

func TestCompileMaterializesPythonSchedulesAsCrons(t *testing.T) {
	files := map[string]string{
		"agent.py": pyAgent,
		"schedules/nightly.py": "from managed_deepagents import define_schedule\n" +
			"schedule = define_schedule(\n    cron=\"30 2 * * *\",\n" +
			"    input={\"messages\": [{\"role\": \"user\", \"content\": \"Sweep stale tickets.\"}]},\n" +
			"    thread={\"mode\": \"persistent\", \"id\": \"nightly-thread\"},\n)\n",
	}
	_, err := pythonProject(t, files).deploy(t)
	requireErrorContains(t, err, "schedules require a root identity")

	files["identity.py"] = pyIdentity
	f := pythonProject(t, files)
	summary := mustCompile(t, f.deploy)

	if _, ok := readJSON(t, f.read(t, "langgraph.json"))["crons"]; ok {
		t.Fatal("langgraph.json declares crons")
	}
	cron := summary.ScheduleCrons["nightly"]
	want := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Sweep stale tickets."}}}
	if cron["graph"] != "agent" || cron["schedule"] != "30 2 * * *" || cron["thread_id"] != "nightly-thread" ||
		!reflect.DeepEqual(cron["input"], want) {
		t.Fatalf("cron: %#v", cron)
	}
	if _, ok := cron["on_run_completed"]; ok {
		t.Fatalf("on_run_completed: %#v", cron)
	}
}

func TestCompilePreservesPythonScheduleDeliverTo(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py":    pyAgent,
		"identity.py": pyIdentity,
		"schedules/monday_greeting.py": "from managed_deepagents import define_schedule\n" +
			"schedule = define_schedule(\n    cron=\"0 9 * * 1\",\n    timezone=\"America/Los_Angeles\",\n" +
			"    prompt=\"Say hello\",\n    deliver_to={\n        \"channel\": \"slack\",\n" +
			"        \"to\": {\"type\": \"provider_conversation\", \"conversation_id\": \"C0123456789\"},\n" +
			"        \"auto_post\": True,\n    },\n    context={\"team\": \"eng\", \"limit\": 3},\n)\n",
	})

	summary := mustCompile(t, f.deploy)

	want := map[string]any{
		"mda_schedule": map[string]any{
			"name": "monday_greeting",
			"deliver_to": map[string]any{
				"channel":   "slack",
				"to":        map[string]any{"type": "provider_conversation", "conversation_id": "C0123456789"},
				"auto_post": true,
			},
		},
		"team":  "eng",
		"limit": json.Number("3"),
	}
	if got := summary.ScheduleCrons["monday_greeting"]["context"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("context: %#v", got)
	}
}

func TestCompileRejectsDuplicateScheduleNames(t *testing.T) {
	schedule := "import { defineSchedule } from \"managed-deepagents\";\n" +
		"export const schedule = defineSchedule({ cron: \"0 9 * * *\", prompt: \"Hi\" });\n"
	f := typescriptProject(t, map[string]string{
		"agent.ts":            tsAgent,
		"identity.ts":         tsIdentity,
		"schedules/daily.ts":  schedule,
		"schedules/daily.mts": schedule,
	})

	_, err := f.deploy(t)
	requireErrorContains(t, err, "duplicate schedule name 'daily' from schedules/")
}
