package cmd

import (
	"encoding/json"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/shared"
	"github.com/spf13/cobra"
)

type exampleReadFilters struct {
	asOf, metadata, filter string
	splits, search         []string
}

func (f *exampleReadFilters) flags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.asOf, "as-of", "", "Dataset version timestamp or tag (default: latest)")
	cmd.Flags().StringSliceVar(&f.splits, "split", nil, "Named subsets to include (repeat or comma-separate)")
	cmd.Flags().StringVar(&f.metadata, "metadata", "", "Metadata containment filter: JSON object or @file")
	cmd.Flags().StringVar(&f.filter, "filter", "", "Example filter DSL (metadata fields only)")
	cmd.Flags().StringArrayVar(&f.search, "search", nil, "Full-text search term (repeat for multiple terms)")
}

func (f *exampleReadFilters) apply(p *langsmith.ExampleListParams) error {
	if f.asOf != "" {
		if strings.TrimSpace(f.asOf) == "" {
			return datasetInputError("--as-of must not be blank")
		}
		p.AsOf = langsmith.F[langsmith.ExampleListParamsAsOfUnion](shared.UnionString(f.asOf))
	}
	if err := validateExampleSplits(f.splits); err != nil {
		return err
	}
	if len(f.splits) > 0 {
		p.Splits = langsmith.F(f.splits)
	}
	if f.metadata != "" {
		obj, err := resourceObject(f.metadata)
		if err != nil {
			return err
		}
		b, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		p.Metadata = langsmith.F(string(b))
	}
	if f.filter != "" {
		p.Filter = langsmith.F(f.filter)
	}
	if len(f.search) > 0 {
		p.FullTextContains = langsmith.F(f.search)
	}
	return nil
}

func datasetInputError(message string) error {
	return commandDiagnostic{"invalid_dataset_request", message, "Check command --help. Use explicit dataset/example IDs and review the request before applying changes."}
}

func validateExampleSplits(splits []string) error {
	seen := map[string]bool{}
	for _, split := range splits {
		if strings.TrimSpace(split) == "" || split != strings.TrimSpace(split) || seen[split] {
			return datasetInputError("split names must be nonblank, trimmed, and unique")
		}
		seen[split] = true
	}
	return nil
}
