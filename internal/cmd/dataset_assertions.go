package cmd

import "strings"

type datasetAssertion struct {
	Key     string `json:"key"`
	Comment string `json:"comment"`
}

func invalidAssertions(message string) error {
	return commandDiagnostic{"invalid_assertions", message, "Provide --assertions with a JSON array of unique nonblank key/comment pairs and one --trace-id or --run-id. Review the dry-run selection before applying it."}
}

func readDatasetAssertions(path string) ([]datasetAssertion, error) {
	var assertions []datasetAssertion
	if err := readDatasetEditFile(path, &assertions); err != nil {
		return nil, invalidAssertions("assertions file must be readable, at most 8 MiB, and strict JSON without unknown or duplicate fields")
	}
	if len(assertions) == 0 {
		return nil, invalidAssertions("assertions must contain at least one criterion")
	}
	seen := map[string]bool{}
	for _, assertion := range assertions {
		key := strings.TrimSpace(assertion.Key)
		if key == "" || key != assertion.Key || seen[key] || strings.TrimSpace(assertion.Comment) == "" {
			return nil, invalidAssertions("assertion keys must be unique and nonblank without surrounding whitespace; comments must not be blank")
		}
		seen[key] = true
	}
	return assertions, nil
}
