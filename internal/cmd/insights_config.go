package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/google/uuid"
	langsmith "github.com/langchain-ai/langsmith-go"
)

// The file schema deliberately excludes credentials, routing, and scheduler fields.
type insightsFileConfig struct {
	Name          string                       `json:"name"`
	Model         string                       `json:"model"`
	Start         string                       `json:"start_time"`
	End           string                       `json:"end_time"`
	LastNHours    int64                        `json:"last_n_hours"`
	Sample        int64                        `json:"sample"`
	Filter        string                       `json:"filter"`
	SummaryPrompt *string                      `json:"summary_prompt"`
	UserContext   map[string]string            `json:"user_context"`
	Partitions    map[string]string            `json:"partitions"`
	Attributes    map[string]insightsAttribute `json:"attribute_schemas"`
	ClusterModel  string                       `json:"cluster_model"`
	SummaryModel  string                       `json:"summary_model"`
}

type insightsAttribute struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	FilterBy    bool   `json:"filter_by,omitempty"`
}

func readInsightsJSON(path string, target any) error {
	const maxSize = 1024 * 1024
	b, err := readJSONInput(path, maxSize)
	if err != nil {
		return fmt.Errorf("reading Insights JSON file: %w", err)
	}
	if len(b) > maxSize {
		return fmt.Errorf("Insights JSON file exceeds 1 MiB")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(b)), "{") {
		return fmt.Errorf("Insights JSON file must contain an object")
	}
	if err := uniqueInsightsJSONKeys(json.NewDecoder(strings.NewReader(string(b))), 0); err != nil {
		return err
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return fmt.Errorf("invalid Insights JSON file (check field names and types)")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("Insights JSON file must contain exactly one object")
	}
	return nil
}

func (f insightsFileConfig) options() insightsCreateOptions {
	context := ""
	if f.UserContext != nil {
		b, _ := json.Marshal(f.UserContext)
		context = string(b)
	}
	prompt := ""
	if f.SummaryPrompt != nil {
		prompt = *f.SummaryPrompt
	}
	return insightsCreateOptions{name: f.Name, model: f.Model, start: f.Start, end: f.End,
		lastNHours: f.LastNHours, sample: f.Sample, filter: f.Filter, summaryPrompt: prompt, summaryPromptSet: f.SummaryPrompt != nil,
		userContext: context, categories: f.Partitions, attributes: f.Attributes,
		clusterModel: f.ClusterModel, summaryModel: f.SummaryModel}
}

func addInsightsAnalysisParams(o insightsCreateOptions, p *langsmith.CreateRunClusteringJobRequestParam) error {
	if o.categories != nil {
		if len(o.categories) == 0 || len(o.categories) > 10 {
			return fmt.Errorf("categories must contain 1 to 10 name/description pairs")
		}
		seen := map[string]bool{}
		for name, description := range o.categories {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(description) == "" {
				return fmt.Errorf("category names and descriptions must not be blank")
			}
			key := strings.TrimSpace(name)
			if seen[key] {
				return fmt.Errorf("category names must be unique after trimming whitespace")
			}
			seen[key] = true
		}
		p.Partitions = langsmith.F(o.categories)
	}
	if o.attributes != nil {
		if len(o.attributes) == 0 {
			return fmt.Errorf("attributes must not be empty")
		}
		attributes := make(map[string]interface{}, len(o.attributes))
		for name, a := range o.attributes {
			if strings.IndexFunc(name, unicode.IsSpace) >= 0 {
				return fmt.Errorf("attribute names must not contain whitespace")
			}
			if strings.TrimSpace(name) == "" || strings.TrimSpace(a.Description) == "" {
				return fmt.Errorf("attribute names and descriptions must not be blank")
			}
			if a.Type != "string" && a.Type != "number" && a.Type != "boolean" {
				return fmt.Errorf("attribute type must be string, number, or boolean")
			}
			attributes[name] = a
		}
		p.AttributeSchemas = langsmith.F(attributes)
	}
	for _, model := range []string{o.clusterModel, o.summaryModel} {
		if model != "" && model != "openai" && model != "anthropic" {
			if _, err := uuid.Parse(model); err != nil {
				return fmt.Errorf("model references must be openai, anthropic, or a workspace model-settings UUID")
			}
		}
	}
	if o.clusterModel != "" {
		p.ClusterModel = langsmith.F(o.clusterModel)
	}
	if o.summaryModel != "" {
		p.SummaryModel = langsmith.F(o.summaryModel)
	}
	return nil
}

// Reject duplicate keys before decoding maps, which would otherwise silently keep
// the last value. Bound nesting as well as file size for untrusted input.
func uniqueInsightsJSONKeys(d *json.Decoder, depth int) error {
	if depth > 128 {
		return fmt.Errorf("Insights JSON nesting exceeds 128 levels")
	}
	token, err := d.Token()
	if err != nil {
		return fmt.Errorf("invalid Insights JSON")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	for d.More() {
		if delim == '{' {
			token, err = d.Token()
			if err != nil {
				return fmt.Errorf("invalid Insights JSON")
			}
			key, ok := token.(string)
			if !ok || seen[key] {
				return fmt.Errorf("Insights JSON contains a duplicate or invalid key")
			}
			seen[key] = true
		}
		if err := uniqueInsightsJSONKeys(d, depth+1); err != nil {
			return err
		}
	}
	if _, err := d.Token(); err != nil {
		return fmt.Errorf("invalid Insights JSON")
	}
	return nil
}
