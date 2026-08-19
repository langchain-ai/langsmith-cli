package cmd

import (
	"errors"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/output"
)

type reportedError struct {
	message string
}

func (e *reportedError) Error() string {
	return e.message
}

// IsReportedError reports whether err was already rendered for the user.
func IsReportedError(err error) bool {
	var target *reportedError
	return errors.As(err, &target)
}

func reportJSONError(data map[string]any) error {
	if err := output.OutputJSON(data, ""); err != nil {
		return err
	}
	return &reportedError{message: fmt.Sprint(data["error"])}
}
