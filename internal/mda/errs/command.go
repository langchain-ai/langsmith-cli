package errs

// CommandCompletion is how a CLI command ended: [Succeeded], [Cancelled], or
// [Failed] with a category. Completions compare with ==.
type CommandCompletion struct {
	outcome  outcome
	category ErrorCategory
}

type outcome int

const (
	succeeded outcome = iota
	cancelled
	failed
)

var (
	// Succeeded is a command that finished its requested work.
	Succeeded = CommandCompletion{outcome: succeeded}
	// Cancelled is a command the caller cancelled before it performed its
	// guarded work.
	Cancelled = CommandCompletion{outcome: cancelled}
)

// Failed is a command that ended with a failure of category.
func Failed(category ErrorCategory) CommandCompletion {
	return CommandCompletion{outcome: failed, category: category}
}

func (c CommandCompletion) String() string {
	switch c.outcome {
	case succeeded:
		return "succeeded"
	case cancelled:
		return "cancelled"
	default:
		return "failed(" + c.category.String() + ")"
	}
}
