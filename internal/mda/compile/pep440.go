package compile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	pep440 "github.com/aquasecurity/go-pep440-version"
)

// The PEP 440 subset compile needs for `requires-python`, which the Rust CLI
// reads with `uv-pep440`: comma-separated specifiers, membership of a version,
// and each specifier's release segments. Matching is go-pep440-version's; the
// clause syntax is checked here, since that library also accepts forms PEP 440
// does not (`||`, a bare version, `=`).

// versionSpecifier is one clause of a `requires-python` (`>=3.11`).
type versionSpecifier struct {
	matcher pep440.Specifiers
	// release is the release segment of the clause's version (`[3, 11]`).
	release []uint64
}

// specifierClause is a PEP 440 version specifier: an operator and a version,
// optionally followed by `.*` for `==` and `!=`.
var specifierClause = regexp.MustCompile(`^\s*(===|==|!=|<=|>=|~=|<|>)\s*(\S+?)\s*$`)

// releaseSegment is the release of a PEP 440 version: optional `v` and epoch,
// then dot-separated numbers.
var releaseSegment = regexp.MustCompile(`^[vV]?(?:\d+!)?(\d+(?:\.\d+)*)`)

// parseVersionSpecifiers parses comma-separated version specifiers.
func parseVersionSpecifiers(text string) ([]versionSpecifier, error) {
	var specifiers []versionSpecifier
	for _, clause := range strings.Split(text, ",") {
		match := specifierClause.FindStringSubmatch(clause)
		if match == nil {
			return nil, fmt.Errorf("invalid version specifier %q: expected an operator followed by a version",
				strings.TrimSpace(clause))
		}
		operator, version := match[1], match[2]
		if strings.HasSuffix(version, ".*") && operator != "==" && operator != "!=" {
			return nil, fmt.Errorf("invalid version specifier %q: `.*` is only allowed with `==` and `!=`",
				strings.TrimSpace(clause))
		}
		matcher, err := pep440.NewSpecifiers(operator + version)
		if err != nil {
			return nil, fmt.Errorf("invalid version specifier %q: %w", strings.TrimSpace(clause), err)
		}
		release := releaseSegment.FindStringSubmatch(version)
		if release == nil {
			return nil, fmt.Errorf("invalid version specifier %q", strings.TrimSpace(clause))
		}
		var segments []uint64
		for _, part := range strings.Split(release[1], ".") {
			segment, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid version specifier %q: %w", strings.TrimSpace(clause), err)
			}
			segments = append(segments, segment)
		}
		specifiers = append(specifiers, versionSpecifier{matcher: matcher, release: segments})
	}
	return specifiers, nil
}

// specifiersContain reports whether version satisfies every specifier.
func specifiersContain(specifiers []versionSpecifier, version pep440.Version) bool {
	for _, specifier := range specifiers {
		if !specifier.matcher.Check(version) {
			return false
		}
	}
	return true
}

// releaseVersion is the stable version with release segments `release`.
func releaseVersion(release ...uint64) pep440.Version {
	parts := make([]string, len(release))
	for i, segment := range release {
		parts[i] = strconv.FormatUint(segment, 10)
	}
	return pep440.MustParse(strings.Join(parts, "."))
}
