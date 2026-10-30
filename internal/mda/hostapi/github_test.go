package hostapi

import (
	"fmt"
	"strings"
	"testing"
)

const testPEM = "-----BEGIN RSA PRIVATE KEY-----\nMIIabc\n-----END RSA PRIVATE KEY-----\n"

func TestPrivateKeyRequiresPEM(t *testing.T) {
	key, err := ParsePrivateKeyPem(testPEM)
	if err != nil || key.Expose() != testPEM {
		t.Fatalf("key = %q, %v", key.Expose(), err)
	}
	if _, err := ParsePrivateKeyPem("-----BEGIN PRIVATE KEY-----\nx\n-----END PRIVATE KEY-----"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePrivateKeyPem("not a key"); err == nil {
		t.Fatal("accepted a non-key")
	}
	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		if got := fmt.Sprintf(format, key); got != "PrivateKeyPem([redacted])" {
			t.Errorf("%s = %q", format, got)
		}
	}
}

func TestLoginValidation(t *testing.T) {
	for _, valid := range []string{"acme", "a", "acme-inc", "A1"} {
		if err := ValidateLogin(valid); err != nil {
			t.Errorf("%s: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "-acme", "acme-", "ac--me", "acme/inc", strings.Repeat("a", 40)} {
		if err := ValidateLogin(invalid); err == nil || err.Error() != fmt.Sprintf("'%s' is not a valid GitHub account name.", invalid) {
			t.Errorf("%q: err = %v", invalid, err)
		}
	}
}
