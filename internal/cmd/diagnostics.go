package cmd

// commandDiagnostic contains only fixed, safe text. Keep argument values and
// upstream error bodies out of diagnostics rendered for machine consumers.
type commandDiagnostic struct {
	code    string
	message string
	next    string
}

func (e commandDiagnostic) Error() string { return e.message }

func (e commandDiagnostic) CLIDiagnostic() (string, string, string) {
	return e.code, e.message, e.next
}
