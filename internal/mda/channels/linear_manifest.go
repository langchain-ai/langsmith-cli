package channels

// Read the Linear channel manifest the runtime emits into
// `channel-manifests.json`.
//
// `channels.linear(...)` works out what its app needs, from the events it
// handles and its options, and puts it in `providerConfig`: the handled
// events, whether it answers agent sessions, the webhook resource types, the
// OAuth scopes and how the app looks. The CLI uses it as given, and checks
// only the icon file, which only it can see in the build.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
)

const linearProvider = "linear"

const (
	// linearAppNameMaxChars is Linear's limit on app names.
	linearAppNameMaxChars = 80
	// linearIconMaxBytes is the largest icon the deployment serves.
	linearIconMaxBytes    = 1024 * 1024
	linearIconLabel       = "Linear channel app.icon"
	jpegMagic             = "\xff\xd8\xff"
	defaultAppDescription = "A Managed Deep Agent that works on issues in this workspace."
)

// linearChannel is a validated Linear channel, ready for app setup.
type linearChannel struct {
	// name is the channel name (file stem), also the manifest key.
	name string
	// eventsPath is the events route on the Agent Server, e.g.
	// `/channels/linear/events`.
	eventsPath string
	// handled are the handled event keys, e.g. `Issue.create`, sorted.
	handled []string
	// sessions reports whether the channel answers agent sessions.
	sessions bool
	// resourceTypes are the webhook resource types the app subscribes to,
	// sorted.
	resourceTypes []string
	// scopes are the OAuth scopes the install asks for.
	scopes []string
	// allowedScopes are the most the install may be granted: the scopes, and
	// those Linear adds.
	allowedScopes []string
	// appName is the name of a new app: the configured one, or the agent's
	// within Linear's rules.
	appName string
	// appDescription is the description of a new app.
	appDescription string
	// appIcon is a PNG or JPEG under `channels/` the deployment serves as the
	// app's icon, at `/channels/{name}/icon`; "" for none.
	appIcon string
}

type linearProviderConfig struct {
	Events               []string         `json:"events" strict:"required"`
	Sessions             bool             `json:"sessions" strict:"required"`
	WebhookResourceTypes []string         `json:"webhookResourceTypes" strict:"required"`
	Scopes               []string         `json:"scopes" strict:"required"`
	AllowedScopes        []string         `json:"allowedScopes" strict:"required"`
	App                  linearAppOptions `json:"app"`
}

// UnmarshalJSON decodes strictly.
func (c *linearProviderConfig) UnmarshalJSON(data []byte) error {
	type plain linearProviderConfig
	return decodeStrict(data, (*plain)(c), true)
}

type linearAppOptions struct {
	Name        *string `json:"name" strict:"nullable"`
	Description *string `json:"description" strict:"nullable"`
	Icon        *string `json:"icon" strict:"nullable"`
}

// UnmarshalJSON decodes strictly.
func (o *linearAppOptions) UnmarshalJSON(data []byte) error {
	type plain linearAppOptions
	return decodeStrict(data, (*plain)(o), true)
}

// prepareLinear finds the Linear channel, if the project declares one.
//
// wait is false for `langsmith deploy --no-wait`: app setup needs the live
// Agent Server URL, so that combination is rejected before anything is
// uploaded. projectFiles are the files the build carries, relative to
// projectRoot.
func prepareLinear(requirements introspect.ChannelRequirements, wait bool, assistantID, projectRoot string, projectFiles []string) (*linearChannel, error) {
	manifest, err := findAppChannelManifest[linearProviderConfig](requirements, linearProvider, "Linear")
	if err != nil || manifest == nil {
		return nil, err
	}
	config := manifest.ProviderConfig
	appIcon := ""
	if config.App.Icon != nil {
		if appIcon, err = validateIconFile(projectRoot, projectFiles, *config.App.Icon); err != nil {
			return nil, err
		}
	}
	if !wait {
		return nil, errors.New("An agent with a Linear channel cannot be deployed with --no-wait.")
	}

	appName := defaultLinearAppName(assistantID)
	if config.App.Name != nil {
		appName = *config.App.Name
	}
	appDescription := defaultAppDescription
	if config.App.Description != nil {
		appDescription = *config.App.Description
	}
	return &linearChannel{
		name:           manifest.Name,
		eventsPath:     manifest.EventsPath,
		handled:        config.Events,
		sessions:       config.Sessions,
		resourceTypes:  config.WebhookResourceTypes,
		scopes:         config.Scopes,
		allowedScopes:  config.AllowedScopes,
		appName:        appName,
		appDescription: appDescription,
		appIcon:        appIcon,
	}, nil
}

// validateIconFile checks the icon the deployment serves: a PNG or JPEG file
// inside `channels/`, as for the Slack channel's icon, of at most
// [linearIconMaxBytes], whose content matches its extension. It must be one
// of projectFiles, the files the build carries, since the deployment reads it
// from the build.
func validateIconFile(projectRoot string, projectFiles []string, icon string) (string, error) {
	relative := strings.TrimSpace(icon)
	iconPath, err := relativeIconPath(relative, linearIconLabel)
	if err != nil {
		return "", err
	}
	var magic string
	switch strings.ToLower(strings.TrimPrefix(path.Ext(iconPath), ".")) {
	case "png":
		magic = pngSignature
	case "jpg", "jpeg":
		magic = jpegMagic
	default:
		return "", fmt.Errorf("%s must be a PNG or JPEG image, such as assets/icon.png.", linearIconLabel)
	}
	// A link, or a file in a folder the build leaves out (such as build/ or
	// dist/), never reaches the deployment, which would then serve nothing.
	built := filepath.Join("channels", filepath.FromSlash(iconPath))
	if !slices.ContainsFunc(projectFiles, func(file string) bool { return filepath.Clean(file) == built }) {
		if _, err := os.Lstat(filepath.Join(projectRoot, built)); err == nil {
			return "", fmt.Errorf("%s %s isn't part of the build: use a regular file, not a link, outside folders the build leaves out such as build/ or dist/.", linearIconLabel, relative)
		}
		return "", fmt.Errorf("%s %s does not exist.", linearIconLabel, relative)
	}
	file, err := channelIconFile(projectRoot, relative, linearIconLabel, linearIconMaxBytes)
	if err != nil {
		return "", err
	}
	head, err := readHead(file, len(pngSignature))
	if err != nil {
		return "", fmt.Errorf("failed to read %s %s: %w", linearIconLabel, relative, err)
	}
	if !bytes.HasPrefix(head, []byte(magic)) {
		return "", fmt.Errorf("%s %s is not the image its extension names.", linearIconLabel, relative)
	}
	return relative, nil
}

// readHead is up to n bytes from the start of the file at name.
func readHead(name string, n int) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	head, readErr := io.ReadAll(io.LimitReader(file, int64(n)))
	if closeErr := file.Close(); readErr == nil {
		readErr = closeErr
	}
	return head, readErr
}

// defaultLinearAppName names a new app after the agent. Linear refuses names
// that mention Linear, so those get a generic one.
func defaultLinearAppName(assistantID string) string {
	runes := []rune(strings.TrimSpace(assistantID))
	if len(runes) > linearAppNameMaxChars {
		runes = runes[:linearAppNameMaxChars]
	}
	name := string(runes)
	if utf8.RuneCountInString(name) < 2 || strings.Contains(strings.ToLower(name), "linear") {
		return "Deep Agent"
	}
	return name
}
