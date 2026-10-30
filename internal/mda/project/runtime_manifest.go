package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// RuntimeManifest is what the installed `managed-deepagents` package's own
// manifest declares about its runtime. Compile vendors that runtime into the
// build, so it carries these over with it: the build must install what the
// runtime imports, at the ranges the runtime was released with. The mda CLI
// wrote them from constants, since it ships in lockstep with its runtime;
// `langsmith deploy` vendors whichever release the project installed.
type RuntimeManifest struct {
	// Dependencies are the package's runtime dependencies, in manifest order:
	// the npm `dependencies`, or each PyPI `Requires-Dist` no extra gates,
	// split into its name (extras included) and the rest of the requirement.
	Dependencies []Dependency
	// Main is the npm package's `main`, as a path into `dist/` (the vendored
	// runtime is the contents of `dist/`); empty for none.
	Main string
	// Exports are the npm package's subpath exports that resolve into `dist/`,
	// as paths into it.
	Exports []Export
	// RequiresPython is the PyPI package's `Requires-Python`; empty for none.
	RequiresPython string
}

// Export is one npm subpath export: `"./runtime": "./runtime/index.js"`.
type Export struct {
	Subpath string
	Target  string
}

// npmDist is the directory of the npm package that compile vendors.
const npmDist = "./dist/"

// parseNpmRuntimeManifest reads the runtime half of the installed npm
// package's `package.json`.
func parseNpmRuntimeManifest(contents []byte) (RuntimeManifest, error) {
	var manifest struct {
		Main         string          `json:"main"`
		Exports      json.RawMessage `json:"exports"`
		Dependencies json.RawMessage `json:"dependencies"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return RuntimeManifest{}, err
	}
	var runtime RuntimeManifest
	if target, ok := strings.CutPrefix(manifest.Main, npmDist); ok {
		runtime.Main = "./" + target
	}
	dependencies, err := orderedMembers(manifest.Dependencies)
	if err != nil {
		return RuntimeManifest{}, fmt.Errorf("`dependencies`: %w", err)
	}
	for _, member := range dependencies {
		var version string
		if err := json.Unmarshal(member.value, &version); err != nil {
			return RuntimeManifest{}, fmt.Errorf("`dependencies.%s` is not a string", member.key)
		}
		runtime.Dependencies = append(runtime.Dependencies, Dependency{Name: member.key, Version: version})
	}
	exports, err := npmExports(manifest.Exports)
	if err != nil {
		return RuntimeManifest{}, fmt.Errorf("`exports`: %w", err)
	}
	for _, export := range exports {
		if target, ok := strings.CutPrefix(export.Target, npmDist); ok {
			runtime.Exports = append(runtime.Exports, Export{Subpath: export.Subpath, Target: "./" + target})
		}
	}
	return runtime, nil
}

// npmExports lists `exports` as subpath → the file Node.js imports, resolving
// conditions the way an `import` from Node.js does.
func npmExports(raw json.RawMessage) ([]Export, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return []Export{{Subpath: ".", Target: single}}, nil
	}
	members, err := orderedMembers(raw)
	if err != nil {
		return nil, err
	}
	// An object of conditions, rather than of subpaths, exports ".".
	if len(members) > 0 && !strings.HasPrefix(members[0].key, ".") {
		target, err := importTarget(raw)
		if err != nil || target == "" {
			return nil, err
		}
		return []Export{{Subpath: ".", Target: target}}, nil
	}
	var exports []Export
	for _, member := range members {
		target, err := importTarget(member.value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", member.key, err)
		}
		if target != "" {
			exports = append(exports, Export{Subpath: member.key, Target: target})
		}
	}
	return exports, nil
}

// importConditions are the export conditions an `import` from Node.js matches.
var importConditions = []string{"import", "node", "default"}

// importTarget resolves an export target: a path, or conditions mapping to
// targets. Empty when no condition matches.
func importTarget(raw json.RawMessage) (string, error) {
	var path string
	if json.Unmarshal(raw, &path) == nil {
		return path, nil
	}
	members, err := orderedMembers(raw)
	if err != nil {
		return "", err
	}
	for _, member := range members {
		for _, condition := range importConditions {
			if member.key == condition {
				return importTarget(member.value)
			}
		}
	}
	return "", nil
}

type jsonMember struct {
	key   string
	value json.RawMessage
}

// orderedMembers lists the members of a JSON object in document order, which
// a Go map would lose. null is an empty object.
func orderedMembers(raw json.RawMessage) ([]jsonMember, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, errors.New("not an object")
	}
	var members []jsonMember
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		members = append(members, jsonMember{key: token.(string), value: value})
	}
	return members, nil
}

// pythonRuntimeDependencies are the `Requires-Dist` entries of the installed
// distribution that no extra gates, as dependencies.
func pythonRuntimeDependencies(requires []string) []Dependency {
	var dependencies []Dependency
	for _, requirement := range requires {
		spec, marker, hasMarker := strings.Cut(requirement, ";")
		if hasMarker && strings.Contains(marker, "extra") {
			continue
		}
		spec = strings.TrimSpace(spec)
		end := strings.IndexFunc(spec, func(c rune) bool {
			return !(c == '-' || c == '_' || c == '.' || c < 0x80 && isASCIIAlphanumeric(byte(c)))
		})
		if end < 0 {
			end = len(spec)
		}
		if strings.HasPrefix(spec[end:], "[") {
			if closing := strings.Index(spec[end:], "]"); closing >= 0 {
				end += closing + 1
			}
		}
		version := strings.TrimSpace(spec[end:])
		if hasMarker {
			version += "; " + strings.TrimSpace(marker)
		}
		dependencies = append(dependencies, Dependency{Name: spec[:end], Version: version})
	}
	return dependencies
}
