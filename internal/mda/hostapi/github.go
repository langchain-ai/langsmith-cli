package hostapi

// GitHub values the CLI handles while setting up a GitHub channel.
//
// The CLI makes no GitHub API calls: Agent Auth creates the App, keeps its
// private key, and records the installation. What remains here is input
// validation for values the developer types or points at.

import (
	"errors"
	"fmt"
	"strings"
)

// PrivateKeyPem is a GitHub App private key in PEM form. Formatting it never
// prints the key.
type PrivateKeyPem struct {
	pem string
}

// ParsePrivateKeyPem accepts PEM text that looks like a private key.
func ParsePrivateKeyPem(value string) (PrivateKeyPem, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "-----BEGIN") || !strings.Contains(trimmed, "PRIVATE KEY-----") {
		return PrivateKeyPem{}, errors.New("The file is not a PEM private key. Download a private key from the GitHub App's settings page.")
	}
	return PrivateKeyPem{pem: trimmed + "\n"}, nil
}

// Expose is the PEM text.
func (k PrivateKeyPem) Expose() string {
	return k.pem
}

// String redacts the key.
func (k PrivateKeyPem) String() string {
	return "PrivateKeyPem([redacted])"
}

// GoString redacts the key.
func (k PrivateKeyPem) GoString() string {
	return k.String()
}

// ValidateLogin checks a GitHub login: 1–39 ASCII alphanumerics or single
// hyphens, not at either end.
func ValidateLogin(login string) error {
	valid := login != "" &&
		len(login) <= 39 &&
		!strings.HasPrefix(login, "-") &&
		!strings.HasSuffix(login, "-") &&
		!strings.Contains(login, "--") &&
		strings.IndexFunc(login, func(c rune) bool {
			return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-')
		}) < 0
	if !valid {
		return fmt.Errorf("'%s' is not a valid GitHub account name.", login)
	}
	return nil
}
