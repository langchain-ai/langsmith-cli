package introspect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// The source-reading half of `compile/schedules.rs` (and of
// `compile/static_extract.rs`, which only schedules and sandboxes used): the
// config each `schedules/` module exports. Turning a config into a LangGraph
// cron, and the duplicate-name check, stay in `compile`.

// ScheduleDeclaration is the config one `schedules/` module exports.
type ScheduleDeclaration struct {
	// Path is the module, relative to the project root.
	Path string
	// Config is what `defineSchedule(...)` / `define_schedule(...)` was given,
	// as plain JSON data (`cron`, `prompt` or `input`, `thread`, `deliverTo` /
	// `deliver_to`, `context`, `timezone`), numbers as [json.Number].
	Config map[string]any
}

// ReadScheduleConfigs reads the named `schedule` export of every direct child
// of `schedules/` for the project's language, in [LanguageDirEntries] order.
//
// Deploy-time cron provisioning needs JSON, so a schedule config must be plain
// data: functions, class instances and other values JSON cannot carry are
// rejected, as the Rust CLI rejected anything but literals.
func ReadScheduleConfigs(ctx context.Context, p *project.Project) ([]ScheduleDeclaration, error) {
	entries := LanguageDirEntries(p, "schedules")
	if len(entries) == 0 {
		return nil, nil
	}
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return nil, err
	}
	if len(doc.Schedules) != len(entries) {
		return nil, fmt.Errorf("schedules/: %w", errNoHelperValue)
	}
	declarations := make([]ScheduleDeclaration, 0, len(entries))
	for i, rel := range entries {
		fields, err := doc.Schedules[i].get()
		if err != nil {
			return nil, err
		}
		config, err := scheduleConfig(fields, rel, p.Language)
		if err != nil {
			return nil, err
		}
		declarations = append(declarations, ScheduleDeclaration{Path: rel, Config: config})
	}
	return declarations, nil
}

func scheduleConfig(fields scheduleFields, rel string, language project.Language) (map[string]any, error) {
	if !fields.Declared {
		return nil, fmt.Errorf("%s must export a named `schedule` declaration", rel)
	}
	if !fields.IsSchedule {
		return nil, fmt.Errorf("%s must export `schedule = %s(...)`",
			rel, language.Ident("defineSchedule", "define_schedule"))
	}
	if fields.Error != nil {
		if language == project.Python {
			return nil, fmt.Errorf("%s schedule is not static: %s", rel, *fields.Error)
		}
		return nil, fmt.Errorf("schedule %s must be statically serializable: %s", rel, *fields.Error)
	}
	// Numbers stay [json.Number], so an integer past 2^53 reaches the cron
	// payload as written.
	decoder := json.NewDecoder(bytes.NewReader(fields.Config))
	decoder.UseNumber()
	var config map[string]any
	if err := decoder.Decode(&config); err != nil || config == nil {
		return nil, fmt.Errorf("%s must declare a module-level `schedule` value", rel)
	}
	return config, nil
}
