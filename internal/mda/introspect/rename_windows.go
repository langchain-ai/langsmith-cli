package introspect

import (
	"errors"
	"syscall"
)

// errorSharingViolation is ERROR_SHARING_VIOLATION: another process has a file
// open without sharing it.
const errorSharingViolation syscall.Errno = 32

// renameRefused reports whether a rename failed because something held a file
// in the directory open, which passes.
var renameRefused = func(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) || errors.Is(err, errorSharingViolation)
}
