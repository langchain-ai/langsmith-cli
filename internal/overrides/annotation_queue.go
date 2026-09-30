package overrides

import (
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

// annotationQueueItemList adds --queue <name> to the generated list command.
// The name is resolved to --queue-id before the generated action runs.
func annotationQueueItemList(generated func() *cobra.Command) *cobra.Command {
	cmd := generated()
	var name string
	cmd.Flags().StringVar(&name, "queue", "", "Annotation queue name, resolved to --queue-id (use either)")
	cmd.Long = strings.Replace(cmd.Long, "\nExamples:\n", "\nExamples:\n  langsmith annotation-queue item list --queue my-queue --status needs_my_review\n", 1)

	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if name == "" {
			return nil
		}
		if cmd.Flags().Changed("queue-id") {
			return structured.NewError(structured.CategoryInvalidInput, "pass --queue or --queue-id, not both")
		}
		id, err := resolveQueueID(cmd, name)
		if err != nil {
			return structured.Classify(err)
		}
		return cmd.Flags().Set("queue-id", id)
	}
	return cmd
}

func resolveQueueID(cmd *cobra.Command, name string) (string, error) {
	c, err := cmdutil.GetClient(cmd)
	if err != nil {
		return "", structured.NewError(structured.CategoryAuthentication, err.Error())
	}
	page, err := c.SDK.AnnotationQueues.GetAnnotationQueues(cmd.Context(), langsmith.AnnotationQueueGetAnnotationQueuesParams{
		Name:  langsmith.F(name),
		Limit: langsmith.F(int64(10)),
	})
	if err != nil {
		return "", err
	}
	var ids []string
	for _, q := range page.Items {
		if q.Name == name {
			ids = append(ids, q.ID)
		}
	}
	switch len(ids) {
	case 0:
		return "", structured.NewError(structured.CategoryNotFound, fmt.Sprintf("no annotation queue named %q", name))
	case 1:
		return ids[0], nil
	default:
		e := structured.NewError(structured.CategoryInvalidInput, fmt.Sprintf("%d annotation queues are named %q", len(ids), name))
		e.Hint = "pass --queue-id with one of: " + strings.Join(ids, ", ")
		return "", e
	}
}
