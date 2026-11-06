package introspect

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadsTypescriptScheduleConfigs(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"schedules/daily.ts": "import { defineSchedule } from \"managed-deepagents\";\n" +
			"const PROMPT = \"Summarize\";\n" +
			"export const schedule = defineSchedule({\n" +
			"  cron: \"0 9 * * *\", prompt: PROMPT, timezone: undefined,\n" +
			"  deliverTo: { channel: \"slack\", to: { type: \"provider_conversation\", conversationId: \"C1\" } },\n" +
			"  context: { team: \"support\", priority: 1 },\n" +
			"});\n",
		"schedules/notes.md": "not a schedule\n",
	})

	declarations, err := ReadScheduleConfigs(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	want := []ScheduleDeclaration{{
		Path: filepath.Join("schedules", "daily.ts"),
		Config: map[string]any{
			"cron":   "0 9 * * *",
			"prompt": "Summarize",
			"deliverTo": map[string]any{
				"channel": "slack",
				"to":      map[string]any{"type": "provider_conversation", "conversationId": "C1"},
			},
			"context": map[string]any{"team": "support", "priority": json.Number("1")},
		},
	}}
	if !reflect.DeepEqual(declarations, want) {
		t.Fatalf("got %#v", declarations)
	}
}

func TestReadsPythonScheduleConfigs(t *testing.T) {
	p := pythonProject(t, map[string]string{
		"agent.py": pyAgent,
		"schedules/weekly.py": "from managed_deepagents import define_schedule\n" +
			"schedule = define_schedule(cron='0 9 * * 1', input={'topic': 'news', 'since': 2**63 + 1}, " +
			"thread={'mode': 'persistent', 'id': 'weekly'}, deliver_to={'channel': 'slack'})\n",
	})

	declarations, err := ReadScheduleConfigs(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	want := []ScheduleDeclaration{{
		Path: filepath.Join("schedules", "weekly.py"),
		Config: map[string]any{
			"cron":       "0 9 * * 1",
			"input":      map[string]any{"topic": "news", "since": json.Number("9223372036854775809")},
			"thread":     map[string]any{"mode": "persistent", "id": "weekly"},
			"deliver_to": map[string]any{"channel": "slack"},
		},
	}}
	if !reflect.DeepEqual(declarations, want) {
		t.Fatalf("got %#v", declarations)
	}
	// The cron payload carries the integer as written.
	payload, err := json.Marshal(declarations[0].Config["input"])
	if err != nil {
		t.Fatal(err)
	}
	if got := string(payload); got != `{"since":9223372036854775809,"topic":"news"}` {
		t.Fatalf("payload: %s", got)
	}
}

func TestScheduleConfigsMustBePlainData(t *testing.T) {
	ts := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"schedules/daily.ts": "import { defineSchedule } from \"managed-deepagents\";\n" +
			"export const schedule = defineSchedule({ cron: \"0 9 * * *\", prompt: \"x\", context: { at: new Date(0) } });\n",
	})
	_, err := ReadScheduleConfigs(t.Context(), ts)
	requireErrorPrefix(t, err, "schedule schedules/daily.ts must be statically serializable: "+
		"property `context` is not static: property `at` is not static: an instance of Date is not plain JSON data")

	py := pythonProject(t, map[string]string{
		"agent.py": pyAgent,
		"schedules/daily.py": "from managed_deepagents import define_schedule\n" +
			"schedule = define_schedule(cron='0 9 * * *', input={'make': lambda: 1})\n",
	})
	_, err = ReadScheduleConfigs(t.Context(), py)
	requireErrorPrefix(t, err, "schedules/daily.py schedule is not static: "+
		"property `input` is not static: property `make` is not static: a function is not plain JSON data")
}

func TestScheduleModulesMustExportASchedule(t *testing.T) {
	missing := typescriptProject(t, map[string]string{"agent.ts": tsAgent, "schedules/daily.ts": "export const other = 1;\n"})
	_, err := ReadScheduleConfigs(t.Context(), missing)
	requireErrorPrefix(t, err, "schedules/daily.ts must export a named `schedule` declaration")

	other := pythonProject(t, map[string]string{"agent.py": pyAgent, "schedules/daily.py": "schedule = {'cron': '0 9 * * *'}\n"})
	_, err = ReadScheduleConfigs(t.Context(), other)
	requireErrorPrefix(t, err, "schedules/daily.py must export `schedule = define_schedule(...)`")
}
