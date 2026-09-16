package cmd

import (
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/shared"
)

func feedbackMatches(got *langsmith.FeedbackSchema, id, projectID, runID, key string, score float64, hasScore bool, comment string, hasComment bool) bool {
	if got == nil || !sameResourceID(got.ID, id) || !sameResourceID(got.RunID, runID) || !sameResourceID(got.SessionID, projectID) || got.Key != key {
		return false
	}
	if hasScore {
		value, ok := got.Score.(shared.UnionFloat)
		if !ok || got.JSON.Score.IsNull() || float64(value) != score {
			return false
		}
	} else if !got.JSON.Score.IsNull() {
		return false
	}
	if hasComment {
		return !got.JSON.Comment.IsNull() && got.Comment == comment
	}
	return got.JSON.Comment.IsNull() || got.Comment == ""
}
