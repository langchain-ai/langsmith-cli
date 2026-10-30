//go:build !windows

package introspect

// renameRefused reports whether a rename failed because something held a file
// in the directory open, which passes. Only Windows refuses for that.
var renameRefused = func(error) bool { return false }
