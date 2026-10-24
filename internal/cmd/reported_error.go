package cmd

// ReportedError is an error a command has already reported in its own output
// format. The CLI exits with a failure status without printing it again.
type ReportedError struct {
	Err error
}

func (e ReportedError) Error() string { return e.Err.Error() }

func (e ReportedError) Unwrap() error { return e.Err }
