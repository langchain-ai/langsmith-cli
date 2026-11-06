package project

import (
	"fmt"
	"strings"
)

// maxAgentIDLength caps Agent Platform IDs at a DNS label so a hostname can
// carry them.
const maxAgentIDLength = 63

// NormalizeName normalizes a name to lowercase ASCII words separated by single
// hyphens.
func NormalizeName(value string) (string, error) {
	normalized := canonicalName(value)
	if normalized == "" {
		return "", fmt.Errorf("name must contain at least one ASCII letter or digit")
	}
	if legacyDeploymentName(value) != normalized {
		return "", fmt.Errorf("name %q is ambiguous after normalization; use %q explicitly", value, normalized)
	}
	return normalized, nil
}

// ValidateAgentID validates an Agent Platform agent ID: 1-63 lowercase ASCII
// letters, digits, or hyphens, starting with a letter and ending with a letter
// or digit.
func ValidateAgentID(value string) error {
	valid := len(value) <= maxAgentIDLength &&
		value != "" &&
		isASCIILower(value[0]) &&
		(isASCIILower(value[len(value)-1]) || isASCIIDigit(value[len(value)-1])) &&
		!strings.ContainsFunc(value, func(c rune) bool {
			return !(c < 0x80 && (isASCIILower(byte(c)) || isASCIIDigit(byte(c)) || c == '-'))
		})
	if !valid {
		return fmt.Errorf("agent ID %q must be 1-%d lowercase letters, digits, or "+
			"hyphens, start with a letter, and end with a letter or digit", value, maxAgentIDLength)
	}
	return nil
}

func canonicalName(value string) string {
	var out strings.Builder
	previousWasDash := false
	for _, ch := range value {
		if ch < 0x80 && isASCIIAlphanumeric(byte(ch)) {
			out.WriteRune(toASCIILower(ch))
			previousWasDash = false
		} else if out.Len() > 0 && !previousWasDash {
			out.WriteByte('-')
			previousWasDash = true
		}
	}
	return strings.TrimSuffix(out.String(), "-")
}

func legacyDeploymentName(value string) string {
	var out strings.Builder
	previousWasDash := false
	for _, ch := range strings.ToLower(value) {
		if ch < 0x80 && (isASCIILower(byte(ch)) || isASCIIDigit(byte(ch))) {
			out.WriteRune(ch)
			previousWasDash = false
		} else if ch == '-' || !previousWasDash {
			out.WriteByte('-')
			previousWasDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func isASCIILower(c byte) bool {
	return c >= 'a' && c <= 'z'
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func toASCIILower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
