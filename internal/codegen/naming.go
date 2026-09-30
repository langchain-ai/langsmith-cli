package codegen

import "strings"

// Go identifiers for path args and page fields; Stainless-specific names
// (params fields, enum types) come from the catalog instead.
var acronyms = map[string]string{"id": "ID", "ids": "IDs", "url": "URL", "urls": "URLs", "api": "API"}

func pascal(snake string) string {
	var b strings.Builder
	for _, w := range strings.Split(snake, "_") {
		if w == "" {
			continue
		}
		if a, ok := acronyms[strings.ToLower(w)]; ok {
			b.WriteString(a)
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	return b.String()
}

func kebab(snake string) string {
	return strings.ToLower(strings.ReplaceAll(snake, "_", "-"))
}

var singularWords = map[string]bool{"feedback": true, "info": true, "status": true, "bulk": true}

// singular turns the last word of a resource name singular, matching the
// CLI's existing noun style (`dataset list`, `project list`).
func singular(name string) string {
	words := strings.Split(name, "_")
	last := words[len(words)-1]
	lower := strings.ToLower(last)
	switch {
	case singularWords[lower], !strings.HasSuffix(lower, "s"), strings.HasSuffix(lower, "ss"):
	case strings.HasSuffix(lower, "ies"):
		last = last[:len(last)-3] + "y"
	case strings.HasSuffix(lower, "xes"), strings.HasSuffix(lower, "ches"), strings.HasSuffix(lower, "shes"):
		last = last[:len(last)-2]
	default:
		last = last[:len(last)-1]
	}
	words[len(words)-1] = last
	return strings.Join(words, "_")
}

// verb maps a Stainless method name to a CLI verb: retrieve_count -> get-count.
func verb(action string) string {
	if rest, ok := strings.CutPrefix(action, "retrieve"); ok {
		action = "get" + rest
	}
	return kebab(action)
}
