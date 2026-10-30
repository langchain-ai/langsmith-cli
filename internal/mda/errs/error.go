// Package errs sorts the failures of `langsmith deploy` into the categories
// the mda CLI reports (`error.rs`), and says how a command ended
// (`command.rs`).
//
// The variants of the two Rust enums are package-level names without the type
// prefix: `ErrorCategory::UserInput` is [UserInput],
// `CommandCompletion::Failed(c)` is [Failed](c).
package errs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/langchain-ai/langsmith-go"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
)

// ErrorCategory is the kind of failure a command ended with.
type ErrorCategory int

// The categories, serialized as their snake_case names.
const (
	Unclassified ErrorCategory = iota
	Project
	Compilation
	Authentication
	Permission
	Remote
	Network
	Toolchain
	Deployment
	UserInput
)

var categoryNames = [...]string{
	Unclassified:   "unclassified",
	Project:        "project",
	Compilation:    "compilation",
	Authentication: "authentication",
	Permission:     "permission",
	Remote:         "remote",
	Network:        "network",
	Toolchain:      "toolchain",
	Deployment:     "deployment",
	UserInput:      "user_input",
}

// String returns the category's documented snake_case name.
func (c ErrorCategory) String() string {
	if c < 0 || int(c) >= len(categoryNames) {
		return fmt.Sprintf("ErrorCategory(%d)", int(c))
	}
	return categoryNames[c]
}

// MarshalText serializes the category as its documented snake_case name.
func (c ErrorCategory) MarshalText() ([]byte, error) {
	if c < 0 || int(c) >= len(categoryNames) {
		return nil, fmt.Errorf("unknown error category %d", int(c))
	}
	return []byte(categoryNames[c]), nil
}

// ErrCancelled is the error for a prompt the user aborted with Ctrl-C or Esc.
// [IsCancellation] reports it, like a cancelled context, as a cancellation.
var ErrCancelled = errors.New("operation cancelled")

// StatusError is a failed API call that carries its HTTP status. The status
// decides its category and the detail never does. `hostapi.ApiError`
// implements it.
type StatusError interface {
	error
	HTTPStatus() int
}

// Categorized returns an error with message and category.
func Categorized(category ErrorCategory, message string) error {
	return &categorized{category: category, message: message}
}

// WithCategory returns err with category, leaving its message and causes as
// they were. Unlike [Categorized], the chain survives, so a cancellation or a
// child's stderr below it is still found.
func WithCategory(category ErrorCategory, err error) error {
	return &recategorized{category: category, inner: err}
}

// CategorizeUnclassified returns err with category unless it already has one,
// as a cancellation or a LangSmith API status would.
func CategorizeUnclassified(category ErrorCategory, err error) error {
	if IsCancellation(err) || CategoryOf(err) != Unclassified {
		return err
	}
	return WithCategory(category, err)
}

// ChildFailed is the error for a child process that exited unsuccessfully:
// message, caused by the child's stderr when it wrote any. The stderr is its
// own link in the chain, so reporters can tell it apart from our messages.
func ChildFailed(output *proc.Captured, message string) error {
	return StderrFailed(output.Stderr, message)
}

// StderrFailed is [ChildFailed] for a child whose stderr was collected some
// other way.
func StderrFailed(stderr []byte, message string) error {
	text := strings.TrimSpace(strings.ToValidUTF8(string(stderr), "�"))
	if text == "" {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %w", message, ChildStderr(text))
}

// ChildStderr is what a failed child process wrote to stderr.
type ChildStderr string

func (s ChildStderr) Error() string {
	return string(s)
}

// IsCancellation reports whether err is, or was caused by, a cancelled context
// or a prompt the user aborted.
func IsCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, ErrCancelled)
}

// CategoryOf returns the category of the outermost cause of err that has one:
// a [Categorized] or [WithCategory] error, an API status, or a transport
// failure. Anything else is [Unclassified].
func CategoryOf(err error) ErrorCategory {
	category := Unclassified
	walk(err, func(cause error) bool {
		found, ok := categoryOfCause(cause)
		if ok {
			category = found
		}
		return ok
	})
	return category
}

type categorized struct {
	category ErrorCategory
	message  string
}

func (e *categorized) Error() string {
	return e.message
}

// recategorized stands in for the error it wraps: the same message and the
// same causes.
type recategorized struct {
	category ErrorCategory
	inner    error
}

func (e *recategorized) Error() string {
	return e.inner.Error()
}

func (e *recategorized) Unwrap() error {
	return e.inner
}

func categoryOfCause(cause error) (ErrorCategory, bool) {
	switch cause := cause.(type) {
	case *recategorized:
		return cause.category, true
	case *categorized:
		return cause.category, true
	case *langsmith.Error:
		return apiCategory(cause.StatusCode), true
	case StatusError:
		return apiCategory(cause.HTTPStatus()), true
	}
	if isOwnCause(cause, client.IsForbidden) {
		return apiCategory(http.StatusForbidden), true
	}
	if isOwnCause(cause, client.IsNotFound) ||
		isOwnCause(cause, client.IsConflict) ||
		isOwnCause(cause, client.IsBadRequest) {
		return Remote, true
	}
	return transportCategory(cause)
}

func apiCategory(status int) ErrorCategory {
	switch status {
	case http.StatusUnauthorized:
		return Authentication
	case http.StatusForbidden:
		return Permission
	default:
		return Remote
	}
}

// transportCategory is [Network] for a request that timed out or never
// connected.
func transportCategory(cause error) (ErrorCategory, bool) {
	if timeout, ok := cause.(net.Error); ok && timeout.Timeout() {
		return Network, true
	}
	if op, ok := cause.(*net.OpError); ok && op.Op == "dial" {
		return Network, true
	}
	if _, ok := cause.(*net.DNSError); ok {
		return Network, true
	}
	return Unclassified, false
}

// isOwnCause reports whether cause itself, rather than something it wraps, is
// what matches. The `internal/client` predicates unwrap on their own, so a
// match that one of cause's direct causes also has belongs further down the
// chain, below whatever category sits between the two.
func isOwnCause(cause error, matches func(error) bool) bool {
	if !matches(cause) {
		return false
	}
	for _, inner := range causes(cause) {
		if matches(inner) {
			return false
		}
	}
	return true
}

// walk visits err and its causes outermost first, depth first, until visit
// returns true.
func walk(err error, visit func(error) bool) bool {
	if err == nil {
		return false
	}
	if visit(err) {
		return true
	}
	for _, inner := range causes(err) {
		if walk(inner, visit) {
			return true
		}
	}
	return false
}

func causes(err error) []error {
	switch err := err.(type) {
	case interface{ Unwrap() error }:
		if inner := err.Unwrap(); inner != nil {
			return []error{inner}
		}
	case interface{ Unwrap() []error }:
		return err.Unwrap()
	}
	return nil
}
