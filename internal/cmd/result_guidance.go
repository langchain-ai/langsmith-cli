package cmd

// Add guidance only to successful empty reads. Callers retain their original
// resource fields and pagination; an empty page does not imply an empty account.
func emptyResultGuidance(result map[string]any, count int, message string, next ...string) map[string]any {
	if count == 0 {
		result["message"] = message
		result["next_steps"] = next
	}
	return result
}
