package compile

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// `schedules/` declarations turned into LangGraph cron config.
//
// Schedule modules are authored as code, but deploy-time cron provisioning
// needs JSON, so only plain-data declarations are accepted
// ([introspect.ReadScheduleConfigs]).

// ScheduleCrons are cron entries keyed by their file-derived schedule name.
type ScheduleCrons map[string]map[string]any

// scheduleReadContext is what the compiler derives before reading schedules.
type scheduleReadContext struct {
	assistantID string
	// channels are the file-derived names of the channels the project
	// declares.
	channels []string
}

// readScheduleCrons reads every direct child of `schedules/` for the project's
// language and turns its named `schedule` export into a LangGraph `crons`
// entry.
//
// assistantID is written into each cron as `graph` so deploy can map it to the
// LangGraph assistant id (matching `langgraph.json` `graphs` keys).
func readScheduleCrons(ctx context.Context, p *project.Project, context scheduleReadContext) (ScheduleCrons, error) {
	crons := ScheduleCrons{}
	declarations, err := introspect.ReadScheduleConfigs(ctx, p)
	if err != nil {
		return nil, err
	}
	for _, declaration := range declarations {
		name, err := introspect.EntryStemName(declaration.Path, "schedule")
		if err != nil {
			return nil, err
		}
		if _, seen := crons[name]; seen {
			return nil, fmt.Errorf("duplicate schedule name '%s' from schedules/", name)
		}
		cron, err := scheduleCron(name, declaration.Config, context.assistantID, context.channels)
		if err != nil {
			return nil, err
		}
		crons[name] = cron
	}
	return crons, nil
}

func scheduleCron(name string, object map[string]any, assistantID string, channels []string) (map[string]any, error) {
	cron, err := stringField(object, "cron", name)
	if err != nil {
		return nil, err
	}
	if err := validateCron(cron, name); err != nil {
		return nil, err
	}

	prompt, hasPrompt := object["prompt"]
	input, hasInput := object["input"]
	switch {
	case hasPrompt && !hasInput:
		text, ok := prompt.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("schedule '%s' prompt must be a non-empty string", name)
		}
		input = map[string]any{"messages": []any{map[string]any{"role": "user", "content": text}}}
	case hasInput && !hasPrompt:
	default:
		return nil, fmt.Errorf("schedule '%s' must define exactly one of `prompt` or `input`", name)
	}

	entry := map[string]any{
		"graph":    assistantID,
		"schedule": cron,
		"input":    input,
		"metadata": map[string]any{
			"mda_managed":    true,
			"mda_managed_by": "deploy",
			"mda_schedule":   name,
		},
	}
	scheduleContext := map[string]any{}
	if raw, ok := object["context"]; ok {
		authored, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("schedule '%s' context must be an object", name)
		}
		for key, value := range authored {
			scheduleContext[key] = value
		}
	}
	for key := range scheduleContext {
		if key == "channel" || strings.HasPrefix(key, "mda_") {
			return nil, fmt.Errorf("schedule '%s' context keys `channel` and `mda_*` are reserved", name)
		}
	}
	mode, threadID, err := threadMode(object, name)
	if err != nil {
		return nil, err
	}
	schedule := map[string]any{"name": name}
	deliverTo, hasDeliverTo := object["deliverTo"]
	if !hasDeliverTo {
		deliverTo, hasDeliverTo = object["deliver_to"]
	}
	if hasDeliverTo {
		schedule["deliver_to"] = deliverTo
	}
	opensThread := mode == threadEphemeral && hasDeliverTo && startsConversation(deliverTo, channels)
	if opensThread {
		schedule["new_thread"] = true
	}
	scheduleContext["mda_schedule"] = schedule
	entry["context"] = scheduleContext

	if raw, ok := object["timezone"]; ok {
		timezone, ok := raw.(string)
		if !ok || strings.TrimSpace(timezone) == "" {
			return nil, fmt.Errorf("schedule '%s' timezone must be a non-empty string", name)
		}
		entry["timezone"] = timezone
	}

	switch mode {
	case threadEphemeral:
		completed := "delete"
		if opensThread {
			completed = "keep"
		}
		entry["on_run_completed"] = completed
	case threadPersistent:
		entry["thread_id"] = threadID
	}
	return entry, nil
}

// startsConversation reports whether a run's `deliverTo` starts a conversation
// in a declared channel, whose replies continue the run's thread. Other
// deliveries keep no thread.
func startsConversation(deliverTo any, channels []string) bool {
	target, ok := deliverTo.(map[string]any)
	if !ok {
		return false
	}
	channel, ok := target["channel"].(string)
	declared := ok && slices.Contains(channels, channel)
	autoPost, hasAutoPost := target["autoPost"]
	if !hasAutoPost {
		autoPost = target["auto_post"]
	}
	posts := autoPost != false
	to, _ := target["to"].(map[string]any)
	conversation := to != nil && to["type"] == "provider_conversation"
	return declared && posts && conversation
}

func stringField(object map[string]any, key, name string) (string, error) {
	value, ok := object[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("schedule '%s' must declare a non-empty `%s`", name, key)
	}
	return value, nil
}

func validateCron(cron, name string) error {
	if len(strings.Fields(cron)) != 5 {
		return fmt.Errorf("schedule '%s' cron must be a standard 5-field expression", name)
	}
	return nil
}

type scheduleThreadMode int

const (
	threadEphemeral scheduleThreadMode = iota
	threadPersistent
)

// threadMode reads the schedule's `thread`, and the id of a persistent one.
func threadMode(object map[string]any, name string) (scheduleThreadMode, string, error) {
	raw, ok := object["thread"]
	if !ok {
		return threadEphemeral, "", nil
	}
	thread, ok := raw.(map[string]any)
	if !ok {
		return 0, "", fmt.Errorf("schedule '%s' thread must be an object", name)
	}
	mode, ok := thread["mode"].(string)
	if !ok {
		mode = "ephemeral"
	}
	switch mode {
	case "ephemeral":
		return threadEphemeral, "", nil
	case "persistent":
		id, ok := thread["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			return 0, "", fmt.Errorf("schedule '%s' persistent thread requires an id", name)
		}
		return threadPersistent, id, nil
	}
	return 0, "", fmt.Errorf("schedule '%s' has unsupported thread mode '%s'", name, mode)
}
