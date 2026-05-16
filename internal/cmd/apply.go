package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// `langsmith apply -f path/...` reads YAML manifests describing LangSmith
// resources and reconciles them with the server. Modeled after `kubectl
// apply` — same verbs (apply, diff/dry-run, prune) and the same one-or-many
// resource shape using `---` separators.
//
// V1 supports `kind: Alert` only. The Manifest struct is intentionally
// generic so new kinds can be added without touching the apply flow itself.

// Manifest is the top-level YAML envelope. Modeled after Kubernetes' GVK +
// metadata + spec layout to leave room for additional resource kinds.
type Manifest struct {
	APIVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind"       json:"kind"`
	Metadata   ManifestMeta `yaml:"metadata"   json:"metadata"`
	Spec       yaml.Node    `yaml:"spec"       json:"-"`
	// SourceFile is set by the parser for diagnostics; not part of the wire format.
	SourceFile string `yaml:"-" json:"-"`
}

type ManifestMeta struct {
	Name    string `yaml:"name"    json:"name"`
	Project string `yaml:"project" json:"project"`
}

// AlertSpec is the `spec:` payload for `kind: Alert`. Field names match the
// LangSmith alerts API (smith-go/alerts/types.go::AlertRuleBase) so the
// manifest reads as a thin envelope around the wire format.
type AlertSpec struct {
	Description            string            `yaml:"description"`
	Type                   string            `yaml:"type"`
	Attribute              string            `yaml:"attribute"`
	Aggregation            string            `yaml:"aggregation"`
	WindowMinutes          int               `yaml:"window_minutes"`
	Operator               string            `yaml:"operator"`
	Threshold              *float64          `yaml:"threshold,omitempty"`
	ThresholdWindowMinutes *int              `yaml:"threshold_window_minutes,omitempty"`
	ThresholdMultiplier    *float64          `yaml:"threshold_multiplier,omitempty"`
	Filter                 *string           `yaml:"filter,omitempty"`
	DenominatorFilter      *string           `yaml:"denominator_filter,omitempty"`
	Actions                []AlertActionSpec `yaml:"actions"`
}

// AlertActionSpec carries the nested action config as a mapping in YAML for
// readability; we JSON-encode it at apply time to match the wire format
// (which expects a JSON string).
type AlertActionSpec struct {
	Target string         `yaml:"target"`
	Config map[string]any `yaml:"config"`
}

// Plan entry — one resource's intended action.
type planAction string

const (
	planCreate planAction = "create"
	planUpdate planAction = "update"
	planDelete planAction = "delete"
	planNoop   planAction = "noop"
)

type planEntry struct {
	Action  planAction
	Kind    string
	Name    string
	Project string
	Reason  string
	// Internal: populated for create/update so apply doesn't re-render.
	desired *AlertWriteRequest
	// Internal: populated for update/delete so we know the API ID.
	existingID string
}

func newApplyCmd() *cobra.Command {
	var (
		paths   []string
		dryRun  bool
		prune   bool
		yes     bool
		outFile string
	)
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply LangSmith resource manifests from YAML",
		Long: `Apply LangSmith resource manifests from YAML.

Reads one or more YAML files (or directories of *.yaml files), diffs each
resource against the server, and creates/updates/deletes as needed.

Supported kinds:
  Alert    Alert rule on a tracing project (kind: Alert)

YAML format follows the Kubernetes-style envelope:

  apiVersion: langsmith.langchain.com/v1
  kind: Alert
  metadata:
    name: my-alert
    project: my-project
  spec:
    type: threshold
    attribute: error_count
    aggregation: sum
    window_minutes: 15
    operator: gte
    threshold: 10
    actions:
      - target: webhook
        config:
          url: https://...
          project_name: my-project
          headers: {Content-Type: application/json}
          body: {text: ...}

Multiple documents per file are supported via '---' separators.

Examples:
  langsmith apply -f alerts/
  langsmith apply -f a.yaml -f b.yaml --dry-run
  langsmith apply -f alerts/ --prune --yes`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(paths) == 0 {
				ExitErrorf("at least one --file/-f is required")
			}
			ctx := context.Background()
			c := MustGetClient()

			manifests, err := loadManifests(paths)
			if err != nil {
				ExitErrorf("loading manifests: %v", err)
			}
			if len(manifests) == 0 {
				ExitErrorf("no manifests found under: %s", strings.Join(paths, ", "))
			}

			plan, err := buildApplyPlan(ctx, c, manifests, prune)
			if err != nil {
				ExitErrorf("planning: %v", err)
			}
			printPlan(plan, outFile)

			if dryRun || !planHasChanges(plan) {
				return
			}
			if !yes && !confirmApply() {
				fmt.Println("aborted")
				return
			}
			if err := executePlan(ctx, c, plan); err != nil {
				ExitErrorf("applying: %v", err)
			}
		},
	}
	cmd.Flags().StringSliceVarP(&paths, "file", "f", nil, "Manifest file or directory (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the plan but don't make changes")
	cmd.Flags().BoolVar(&prune, "prune", false, "Delete resources on the server that don't appear in the manifests (scoped to the manifests' projects)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt before applying")
	cmd.Flags().StringVarP(&outFile, "output", "o", "", "Write plan to a file (JSON)")
	return cmd
}

// --- Manifest parsing. ---

func loadManifests(paths []string) ([]Manifest, error) {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", p, err)
		}
		if info.IsDir() {
			matches, err := filepath.Glob(filepath.Join(p, "*.yaml"))
			if err != nil {
				return nil, fmt.Errorf("glob %s: %w", p, err)
			}
			matchesYml, err := filepath.Glob(filepath.Join(p, "*.yml"))
			if err != nil {
				return nil, fmt.Errorf("glob %s: %w", p, err)
			}
			files = append(files, matches...)
			files = append(files, matchesYml...)
		} else {
			files = append(files, p)
		}
	}
	sort.Strings(files)

	var out []Manifest
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var m Manifest
			if err := decoder.Decode(&m); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("parse %s: %w", f, err)
			}
			if m.Kind == "" && m.APIVersion == "" && m.Metadata.Name == "" {
				// Empty document (e.g. just `---` at end of file) — skip.
				continue
			}
			m.SourceFile = f
			out = append(out, m)
		}
	}
	return out, nil
}

// --- Plan building. ---

func buildApplyPlan(ctx context.Context, c *client.Client, manifests []Manifest, prune bool) ([]planEntry, error) {
	// Group desired manifests by project so we can fetch existing state once
	// per project and (optionally) prune within that scope.
	type desired struct {
		manifest Manifest
		write    AlertWriteRequest
	}
	byProject := map[string][]desired{}
	for _, m := range manifests {
		if m.Kind != "Alert" {
			return nil, fmt.Errorf("%s: kind %q is not supported (only \"Alert\")", m.SourceFile, m.Kind)
		}
		if m.Metadata.Name == "" {
			return nil, fmt.Errorf("%s: metadata.name is required", m.SourceFile)
		}
		if m.Metadata.Project == "" {
			return nil, fmt.Errorf("%s: metadata.project is required", m.SourceFile)
		}
		write, err := manifestToWriteRequest(m)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", m.SourceFile, err)
		}
		byProject[m.Metadata.Project] = append(byProject[m.Metadata.Project], desired{m, write})
	}

	var plan []planEntry
	for project, items := range byProject {
		sessionID, err := c.ResolveSessionID(ctx, project)
		if err != nil {
			return nil, fmt.Errorf("project %q: %w", project, err)
		}
		existing, err := alertList(ctx, c, sessionID)
		if err != nil {
			return nil, fmt.Errorf("listing existing alerts in %q: %w", project, err)
		}
		existingByName := map[string]AlertResponse{}
		for _, e := range existing {
			existingByName[e.Rule.Name] = e
		}
		desiredByName := map[string]bool{}
		for _, d := range items {
			desiredByName[d.manifest.Metadata.Name] = true
			match, ok := existingByName[d.manifest.Metadata.Name]
			write := d.write
			if !ok {
				plan = append(plan, planEntry{
					Action:  planCreate,
					Kind:    "Alert",
					Name:    d.manifest.Metadata.Name,
					Project: project,
					desired: &write,
				})
				continue
			}
			reason := diffAlert(write, match)
			if reason == "" {
				plan = append(plan, planEntry{
					Action:     planNoop,
					Kind:       "Alert",
					Name:       d.manifest.Metadata.Name,
					Project:    project,
					existingID: match.Rule.ID,
				})
				continue
			}
			plan = append(plan, planEntry{
				Action:     planUpdate,
				Kind:       "Alert",
				Name:       d.manifest.Metadata.Name,
				Project:    project,
				Reason:     reason,
				desired:    &write,
				existingID: match.Rule.ID,
			})
		}
		if prune {
			for name, e := range existingByName {
				if desiredByName[name] {
					continue
				}
				plan = append(plan, planEntry{
					Action:     planDelete,
					Kind:       "Alert",
					Name:       name,
					Project:    project,
					existingID: e.Rule.ID,
				})
			}
		}
	}
	// Stable order: project, kind, name.
	sort.SliceStable(plan, func(i, j int) bool {
		if plan[i].Project != plan[j].Project {
			return plan[i].Project < plan[j].Project
		}
		if plan[i].Kind != plan[j].Kind {
			return plan[i].Kind < plan[j].Kind
		}
		return plan[i].Name < plan[j].Name
	})
	return plan, nil
}

func manifestToWriteRequest(m Manifest) (AlertWriteRequest, error) {
	var spec AlertSpec
	if err := m.Spec.Decode(&spec); err != nil {
		return AlertWriteRequest{}, fmt.Errorf("decoding spec: %w", err)
	}
	if err := validateAlertSpec(spec); err != nil {
		return AlertWriteRequest{}, err
	}
	rule := AlertRule{
		Name:                   m.Metadata.Name,
		Description:            spec.Description,
		Type:                   spec.Type,
		Attribute:              spec.Attribute,
		Aggregation:            spec.Aggregation,
		WindowMinutes:          spec.WindowMinutes,
		Operator:               spec.Operator,
		Threshold:              spec.Threshold,
		ThresholdWindowMinutes: spec.ThresholdWindowMinutes,
		ThresholdMultiplier:    spec.ThresholdMultiplier,
		Filter:                 spec.Filter,
		DenominatorFilter:      spec.DenominatorFilter,
	}
	var actions []AlertAction
	for i, a := range spec.Actions {
		configJSON, err := json.Marshal(a.Config)
		if err != nil {
			return AlertWriteRequest{}, fmt.Errorf("actions[%d].config: %w", i, err)
		}
		actions = append(actions, AlertAction{Target: a.Target, Config: string(configJSON)})
	}
	return AlertWriteRequest{Rule: rule, Actions: actions}, nil
}

func validateAlertSpec(s AlertSpec) error {
	if !oneOf([]string{"threshold", "change"}, s.Type) {
		return fmt.Errorf("type must be 'threshold' or 'change' (got %q)", s.Type)
	}
	if !oneOf([]string{"latency", "error_count", "feedback_score", "run_latency", "run_count", "total_cost"}, s.Attribute) {
		return fmt.Errorf("attribute is invalid: %q", s.Attribute)
	}
	if !oneOf([]string{"avg", "sum", "pct"}, s.Aggregation) {
		return fmt.Errorf("aggregation must be 'avg', 'sum', or 'pct' (got %q)", s.Aggregation)
	}
	if !oneOf([]string{"gte", "lte", "gt", "lt"}, s.Operator) {
		return fmt.Errorf("operator must be 'gte', 'lte', 'gt', or 'lt' (got %q)", s.Operator)
	}
	if s.WindowMinutes < 1 || s.WindowMinutes > 60 {
		return fmt.Errorf("window_minutes must be 1..60 (got %d)", s.WindowMinutes)
	}
	switch s.Type {
	case "threshold":
		if s.Threshold == nil {
			return fmt.Errorf("threshold is required when type=threshold")
		}
	case "change":
		if s.ThresholdWindowMinutes == nil || s.ThresholdMultiplier == nil {
			return fmt.Errorf("threshold_window_minutes and threshold_multiplier are required when type=change")
		}
	}
	if len(s.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	for i, a := range s.Actions {
		if !oneOf([]string{"webhook", "pagerduty", "dynatrace"}, a.Target) {
			return fmt.Errorf("actions[%d].target must be 'webhook', 'pagerduty', or 'dynatrace' (got %q)", i, a.Target)
		}
	}
	return nil
}

// diffAlert returns a human-readable summary of differences between desired
// and existing, or "" if equal.
func diffAlert(desired AlertWriteRequest, existing AlertResponse) string {
	var diffs []string

	desRule := desired.Rule
	exRule := existing.Rule
	if desRule.Description != exRule.Description {
		diffs = append(diffs, fmt.Sprintf("description: %q → %q", exRule.Description, desRule.Description))
	}
	if desRule.Type != exRule.Type {
		diffs = append(diffs, fmt.Sprintf("type: %q → %q", exRule.Type, desRule.Type))
	}
	if desRule.Attribute != exRule.Attribute {
		diffs = append(diffs, fmt.Sprintf("attribute: %q → %q", exRule.Attribute, desRule.Attribute))
	}
	if desRule.Aggregation != exRule.Aggregation {
		diffs = append(diffs, fmt.Sprintf("aggregation: %q → %q", exRule.Aggregation, desRule.Aggregation))
	}
	if desRule.WindowMinutes != exRule.WindowMinutes {
		diffs = append(diffs, fmt.Sprintf("window_minutes: %d → %d", exRule.WindowMinutes, desRule.WindowMinutes))
	}
	if desRule.Operator != exRule.Operator {
		diffs = append(diffs, fmt.Sprintf("operator: %q → %q", exRule.Operator, desRule.Operator))
	}
	if !floatPtrEq(desRule.Threshold, exRule.Threshold) {
		diffs = append(diffs, fmt.Sprintf("threshold: %s → %s", floatPtrStr(exRule.Threshold), floatPtrStr(desRule.Threshold)))
	}
	if !intPtrEq(desRule.ThresholdWindowMinutes, exRule.ThresholdWindowMinutes) {
		diffs = append(diffs, "threshold_window_minutes changed")
	}
	if !floatPtrEq(desRule.ThresholdMultiplier, exRule.ThresholdMultiplier) {
		diffs = append(diffs, "threshold_multiplier changed")
	}
	if !stringPtrEq(desRule.Filter, exRule.Filter) {
		diffs = append(diffs, "filter changed")
	}
	if !stringPtrEq(desRule.DenominatorFilter, exRule.DenominatorFilter) {
		diffs = append(diffs, "denominator_filter changed")
	}

	if !actionsEqual(desired.Actions, existing.Actions) {
		diffs = append(diffs, fmt.Sprintf("actions changed (%d → %d)", len(existing.Actions), len(desired.Actions)))
	}

	return strings.Join(diffs, "; ")
}

func actionsEqual(desired, existing []AlertAction) bool {
	if len(desired) != len(existing) {
		return false
	}
	// Order-insensitive by target. Two webhooks with different configs are
	// inherently order-sensitive but we don't have a stable identity beyond
	// target — treat any per-target config drift as a change.
	desByTarget := map[string][]map[string]any{}
	for _, a := range desired {
		desByTarget[a.Target] = append(desByTarget[a.Target], parseActionConfig(a.Config))
	}
	exByTarget := map[string][]map[string]any{}
	for _, a := range existing {
		exByTarget[a.Target] = append(exByTarget[a.Target], parseActionConfig(a.Config))
	}
	if len(desByTarget) != len(exByTarget) {
		return false
	}
	for target, desConfigs := range desByTarget {
		exConfigs, ok := exByTarget[target]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(desConfigs, exConfigs) {
			return false
		}
	}
	return true
}

// --- Plan output + apply. ---

func planHasChanges(plan []planEntry) bool {
	for _, p := range plan {
		if p.Action != planNoop {
			return true
		}
	}
	return false
}

func printPlan(plan []planEntry, outFile string) {
	if outFile != "" {
		clean := make([]map[string]string, 0, len(plan))
		for _, p := range plan {
			entry := map[string]string{
				"action":  string(p.Action),
				"kind":    p.Kind,
				"name":    p.Name,
				"project": p.Project,
			}
			if p.Reason != "" {
				entry["reason"] = p.Reason
			}
			clean = append(clean, entry)
		}
		output.OutputJSON(clean, outFile)
		return
	}
	counts := map[planAction]int{}
	for _, p := range plan {
		counts[p.Action]++
	}
	fmt.Printf("Plan: %d to create, %d to update, %d to delete, %d unchanged.\n\n",
		counts[planCreate], counts[planUpdate], counts[planDelete], counts[planNoop])
	for _, p := range plan {
		switch p.Action {
		case planCreate:
			fmt.Printf("  + create  %s/%s in project %s\n", p.Kind, p.Name, p.Project)
		case planUpdate:
			fmt.Printf("  ~ update  %s/%s in project %s   (%s)\n", p.Kind, p.Name, p.Project, p.Reason)
		case planDelete:
			fmt.Printf("  - delete  %s/%s in project %s   (id=%s)\n", p.Kind, p.Name, p.Project, p.existingID)
		case planNoop:
			fmt.Printf("  · noop    %s/%s in project %s\n", p.Kind, p.Name, p.Project)
		}
	}
}

func confirmApply() bool {
	fmt.Print("\nApply this plan? [y/N] ")
	var resp string
	fmt.Scanln(&resp)
	return strings.EqualFold(resp, "y") || strings.EqualFold(resp, "yes")
}

func executePlan(ctx context.Context, c *client.Client, plan []planEntry) error {
	var failed int
	for _, p := range plan {
		sessionID, err := c.ResolveSessionID(ctx, p.Project)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  FAILED resolve project %s: %v\n", p.Project, err)
			failed++
			continue
		}
		switch p.Action {
		case planCreate:
			if _, err := alertCreate(ctx, c, sessionID, *p.desired); err != nil {
				fmt.Fprintf(os.Stderr, "  FAILED create %s/%s in %s: %v\n", p.Kind, p.Name, p.Project, err)
				failed++
				continue
			}
			fmt.Printf("  + created %s/%s in %s\n", p.Kind, p.Name, p.Project)
		case planUpdate:
			if _, err := alertUpdate(ctx, c, sessionID, p.existingID, *p.desired); err != nil {
				fmt.Fprintf(os.Stderr, "  FAILED update %s/%s in %s: %v\n", p.Kind, p.Name, p.Project, err)
				failed++
				continue
			}
			fmt.Printf("  ~ updated %s/%s in %s\n", p.Kind, p.Name, p.Project)
		case planDelete:
			if err := alertDelete(ctx, c, sessionID, p.existingID); err != nil {
				fmt.Fprintf(os.Stderr, "  FAILED delete %s/%s in %s: %v\n", p.Kind, p.Name, p.Project, err)
				failed++
				continue
			}
			fmt.Printf("  - deleted %s/%s in %s\n", p.Kind, p.Name, p.Project)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d apply step(s) failed", failed)
	}
	return nil
}

// --- tiny helpers. ---

// oneOf reports whether v is in allowed. Named `oneOf` (not `contains`) to
// avoid colliding with the existing `contains(s, substr)` helper in
// evaluator_test.go (substring check, different signature).
func oneOf(allowed []string, v string) bool {
	for _, a := range allowed {
		if a == v {
			return true
		}
	}
	return false
}

func floatPtrEq(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func intPtrEq(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func stringPtrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func floatPtrStr(p *float64) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%g", *p)
}
