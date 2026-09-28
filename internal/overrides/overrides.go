// Package overrides holds handwritten replacements for generated commands,
// keyed by catalog operation ID. Each one must also be declared with
// `override:` in the catalog overlay; TestOverridesMatchCatalog enforces it.
package overrides

import "github.com/langchain-ai/langsmith-cli/internal/gen"

// All returns every override, keyed by operation ID.
func All() map[string]gen.Override {
	return map[string]gen.Override{
		"annotation_queues.items.list": annotationQueueItemList,
	}
}
