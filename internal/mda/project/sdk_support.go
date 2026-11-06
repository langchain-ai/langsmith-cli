package project

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// sdkRelease is the release segment of a `managed-deepagents` version:
// `0.10.1` of npm's `0.10.1-dev.3` and PyPI's `0.10.1.dev3` alike.
type sdkRelease [3]int

// parseSDKRelease reads the release segment at the start of version.
func parseSDKRelease(version string) (sdkRelease, bool) {
	var release sdkRelease
	rest := version
	for i := range release {
		end := strings.IndexFunc(rest, func(c rune) bool { return c < '0' || c > '9' })
		if end < 0 {
			end = len(rest)
		}
		number, err := strconv.Atoi(rest[:end])
		if err != nil {
			return sdkRelease{}, false
		}
		release[i] = number
		rest = rest[end:]
		if i < len(release)-1 {
			var ok bool
			if rest, ok = strings.CutPrefix(rest, "."); !ok {
				return sdkRelease{}, false
			}
		}
	}
	return release, true
}

func (r sdkRelease) less(other sdkRelease) bool {
	for i := range r {
		if r[i] != other[i] {
			return r[i] < other[i]
		}
	}
	return false
}

func (r sdkRelease) String() string {
	return fmt.Sprintf("%d.%d.%d", r[0], r[1], r[2])
}

// supportedSDK is the `managed-deepagents` releases `langsmith deploy` builds
// for, from minimum up to (not including) limit, compared by release segment
// so the pre-releases of a supported release are supported too.
//
// Compile vendors the installed runtime and generates an entry that calls its
// API, so the range is what that entry needs. Every served entry passes the
// general-purpose subagent's graph (`asyncSubagentGraphs` /
// `async_subagent_graphs`) and builds async subagents with `asyncSubagent` /
// `async_subagent`; browser projects re-export `browserHandoffWatcher` /
// `managed_deepagents.browser.handoff_watcher`. All of these arrived in 0.10.0
// on npm and PyPI. 0.x minors change that API, so the next one is excluded
// until compile is checked against it.
var supportedSDK = struct{ minimum, limit sdkRelease }{
	minimum: sdkRelease{0, 10, 0},
	limit:   sdkRelease{0, 11, 0},
}

// SupportedSDKRange is [supportedSDK] as a version range.
func SupportedSDKRange() string {
	return fmt.Sprintf(">=%s, <%s", supportedSDK.minimum, supportedSDK.limit)
}

// checkSupportedSDK fails unless installation is a release in
// [supportedSDK].
//
// A package installed from a local directory (an editable or `file:` install,
// `npm link`) is a source tree, whose version is whatever it last released as
// rather than what its runtime has, so it is taken as it is.
func checkSupportedSDK(root string, installation *Installation) error {
	if installation.local {
		return nil
	}
	release, ok := parseSDKRelease(installation.version)
	if ok && !release.less(supportedSDK.minimum) && release.less(supportedSDK.limit) {
		return nil
	}
	install := installation.language.Ident(
		fmt.Sprintf("npm install managed-deepagents@%d.%d", supportedSDK.minimum[0], supportedSDK.minimum[1]),
		fmt.Sprintf("uv add 'managed-deepagents%s'", strings.ReplaceAll(SupportedSDKRange(), " ", "")),
	)
	return errs.Categorized(errs.Project, fmt.Sprintf(
		"managed-deepagents %s is installed for the project at %s, but `langsmith deploy` builds "+
			"managed-deepagents %s: the entry it generates calls the runtime API of those releases.\n"+
			"Install a supported release with `%s`, or deploy with the SDK's own `mda deploy`.",
		installation.version, root, SupportedSDKRange(), install))
}
