package api

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// linkValue matches one `<uri>; params` entry of an RFC 8288 Link header. A
// quoted parameter value may contain commas and backslash-escaped quotes, so
// neither ends the entry.
var linkValue = regexp.MustCompile(`<([^>]*)>((?:[^,<"]|"(?:[^"\\]|\\.)*")*)`)

// linkParam matches one `; name[=value]` parameter of a Link entry, where the
// value is a token or a quoted string that may itself contain semicolons.
var linkParam = regexp.MustCompile(`;\s*([^=;\s]+)\s*(?:=\s*("(?:[^"\\]|\\.)*"|[^;]*))?`)

// warnIfDeprecated writes one line to w when the response marks the endpoint
// deprecated (RFC 9745 `Deprecation`), naming the removal date from `Sunset`,
// the `successor-version` link, and the `deprecation` link to the migration
// guide when the response carries them. Agents read stderr but parse stdout,
// so the warning never goes to stdout.
func warnIfDeprecated(w io.Writer, method, path string, headers http.Header) {
	if headers.Get("Deprecation") == "" || strings.EqualFold(headers.Get("Deprecation"), "false") {
		return
	}
	var msg strings.Builder
	fmt.Fprintf(&msg, "warning: %s %s is deprecated", strings.ToUpper(method), path)
	if sunset := headers.Get("Sunset"); sunset != "" {
		if t, err := http.ParseTime(sunset); err == nil {
			sunset = t.Format("2006-01-02")
		}
		fmt.Fprintf(&msg, " and will be removed after %s", sunset)
	}
	successor, guide := deprecationLinks(headers.Values("Link"))
	if successor != "" {
		fmt.Fprintf(&msg, "; use %s", successor)
	}
	msg.WriteString(".")
	if guide != "" {
		fmt.Fprintf(&msg, " See %s", guide)
	}
	fmt.Fprintln(w, msg.String())
}

func deprecationLinks(values []string) (successor, guide string) {
	for _, value := range values {
		for _, m := range linkValue.FindAllStringSubmatch(value, -1) {
			uri, params := strings.TrimSpace(m[1]), strings.ToLower(m[2])
			switch {
			case successor == "" && hasRel(params, "successor-version"):
				successor = uri
			case guide == "" && hasRel(params, "deprecation"):
				guide = uri
			}
		}
	}
	return successor, guide
}

func hasRel(params, rel string) bool {
	for _, m := range linkParam.FindAllStringSubmatch(params, -1) {
		if m[1] == "rel" && slices.Contains(strings.Fields(strings.Trim(strings.TrimSpace(m[2]), `"`)), rel) {
			return true
		}
	}
	return false
}
