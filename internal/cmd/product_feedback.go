package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

var productFeedbackCategories = map[string]langsmith.ProductFeedbackNewParamsCategory{
	"bug":             langsmith.ProductFeedbackNewParamsCategoryBug,
	"feature-request": langsmith.ProductFeedbackNewParamsCategoryFeatureRequest,
	"usability":       langsmith.ProductFeedbackNewParamsCategoryUsability,
	"documentation":   langsmith.ProductFeedbackNewParamsCategoryDocumentation,
	"other":           langsmith.ProductFeedbackNewParamsCategoryOther,
}

func newProductFeedbackCmd(version string) *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "feedback <note>",
		Short: "Submit feedback about the LangSmith CLI",
		Long: `Submit concise feedback about the LangSmith CLI.

The feedback payload contains the note, category, CLI version, operating system,
architecture, and a fixed CLI source value. The CLI does not collect command
arguments, output, environment variables, file paths, traces, prompts, or other
local context. Authentication, routing, idempotency, and standard client headers
accompany the request.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			message := strings.TrimSpace(args[0])
			if message == "" {
				return fmt.Errorf("feedback note cannot be empty")
			}
			if utf8.RuneCountInString(message) > 4000 {
				return fmt.Errorf("feedback note must be at most 4000 characters")
			}
			feedbackCategory, ok := productFeedbackCategories[category]
			if !ok {
				return fmt.Errorf("invalid category %q; use bug, feature-request, usability, documentation, or other", category)
			}
			client, err := getClient()
			if err != nil {
				return err
			}
			response, err := client.SDK.ProductFeedback.New(cmd.Context(), langsmith.ProductFeedbackNewParams{
				Category: langsmith.F(feedbackCategory),
				Message:  langsmith.F(message),
				Source:   langsmith.F(langsmith.ProductFeedbackNewParamsSourceLangsmithCli),
				Client: langsmith.F(langsmith.ProductFeedbackNewParamsClient{
					Version:      langsmith.F(version),
					Os:           langsmith.F(runtime.GOOS),
					Architecture: langsmith.F(runtime.GOARCH),
				}),
				IdempotencyKey: langsmith.F(uuid.NewString()),
			}, option.WithMaxRetries(0))
			if err != nil {
				var apiErr *langsmith.Error
				if errors.As(err, &apiErr) {
					switch apiErr.StatusCode {
					case http.StatusNotFound:
						return fmt.Errorf("product feedback is unavailable on this LangSmith deployment")
					case http.StatusTooManyRequests:
						if apiErr.Response == nil {
							return fmt.Errorf("product feedback rate limit reached; try again later")
						}
						if retryAfter := apiErr.Response.Header.Get("Retry-After"); retryAfter != "" {
							return fmt.Errorf("product feedback rate limit reached; retry after %s", retryAfter)
						}
						return fmt.Errorf("product feedback rate limit reached; try again later")
					}
				}
				return fmt.Errorf("submitting product feedback: %w", err)
			}
			return writeProductFeedbackResult(cmd.OutOrStdout(), GetFormat(), response)
		},
	}
	cmd.Flags().StringVar(&category, "category", "usability", "Feedback category: bug, feature-request, usability, documentation, or other")
	return cmd
}

func writeProductFeedbackResult(w io.Writer, format string, response *langsmith.ProductFeedbackNewResponse) error {
	if format == "pretty" {
		_, err := fmt.Fprintf(w, "Feedback submitted\nID: %s\nCreated: %s\n", response.ID, response.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
		return err
	}
	return json.NewEncoder(w).Encode(map[string]any{
		"id":         response.ID,
		"created_at": response.CreatedAt,
	})
}
