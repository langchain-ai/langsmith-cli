# Profiles Feature Design

## Overview

Add named profile support to the langsmith CLI, allowing users to store and switch between multiple LangSmith configurations (API endpoint, credentials, workspace). Profiles are stored in a TOML config file and selected via CLI flag, environment variable, or a persistent `current_profile` setting.

## Config File

- **Location:** `~/.langsmith/config.toml`
- **Format:** TOML with a top-level `current_profile` key and one `[section]` per profile

```toml
current_profile = "default"

[default]
api_url = "https://api.smith.langchain.com"
api_key = "lsv2_pt_abc123..."
workspace_id = "optional-uuid"

[staging]
api_url = "https://staging.api.smith.langchain.com"
api_key = "lsv2_pt_def456..."
```

### Profile Fields

| Field          | Required | Default                                |
|----------------|----------|----------------------------------------|
| `api_key`      | yes      | —                                      |
| `api_url`      | no       | `https://api.smith.langchain.com`      |
| `workspace_id` | no       | —                                      |

## Resolution Priority

Credential and endpoint resolution follows AWS-style precedence (highest wins):

1. `--api-key` / `--api-url` CLI flags
2. `LANGSMITH_API_KEY` / `LANGSMITH_ENDPOINT` / `LANGSMITH_WORKSPACE_ID` env vars
3. Named profile selected via `--profile` flag or `LANGSMITH_PROFILE` env var
4. `current_profile` value from config file
5. `[default]` profile in config file
6. Hardcoded default URL (`https://api.smith.langchain.com`)

If no config file exists and no env vars or flags are set, the CLI behaves exactly as it does today (requires `LANGSMITH_API_KEY`).

## Subcommands

All under `langsmith profile`:

### `langsmith profile list`

Lists all profiles. Marks the active profile with `*`.

- JSON output: array of objects with `name`, `api_url`, `active` fields (key never shown)
- Pretty output: table with name, URL, active marker

### `langsmith profile show <name>`

Shows a single profile's configuration. The API key is masked (e.g., `lsv2_pt_...c123`).

- JSON output: object with `name`, `api_url`, `api_key` (masked), `workspace_id`
- Pretty output: key-value display

### `langsmith profile create <name>`

Creates a new profile. Accepts flags for scripting; prompts interactively for missing required fields.

**Flags:**
- `--api-key` (required, or prompted)
- `--api-url` (optional, defaults to `https://api.smith.langchain.com`)
- `--workspace-id` (optional)

If `~/.langsmith/config.toml` does not exist, it is created. If this is the first profile and no `current_profile` is set, it becomes the current profile automatically.

Errors if a profile with that name already exists.

### `langsmith profile delete <name>`

Removes a profile from the config file.

Errors if the profile does not exist. If the deleted profile was the `current_profile`, clears `current_profile` (the user must `profile use` another one or rely on env vars).

### `langsmith profile use <name>`

Sets `current_profile` in the config file to the given profile name.

Errors if the profile does not exist.

## Root Command Changes

- New `--profile` persistent flag on the root command
- `GetAPIKey()`, `GetAPIURL()` updated to consult profile config as a fallback after checking flags and env vars
- New helper to resolve workspace ID from profile

## New Package: `internal/config`

Handles reading, writing, and querying the TOML config file.

### Key types and functions:

```go
// Profile represents a single named profile.
type Profile struct {
    APIKey      string `toml:"api_key"`
    APIURL      string `toml:"api_url,omitempty"`
    WorkspaceID string `toml:"workspace_id,omitempty"`
}

// Config represents the full config file.
type Config struct {
    CurrentProfile string             `toml:"current_profile,omitempty"`
    Profiles       map[string]Profile // each TOML section is a profile
}

func Load() (*Config, error)           // reads ~/.langsmith/config.toml, returns empty Config if missing
func (c *Config) Save() error          // writes back to ~/.langsmith/config.toml
func (c *Config) ActiveProfileName(flagProfile, envProfile string) string
func (c *Config) ResolveProfile(flagProfile, envProfile string) *Profile
```

## Behavioral Notes

- The config file is only created on first `profile create`, never eagerly.
- All existing behavior is preserved when no config file exists.
- `profile delete` of the active profile clears `current_profile` rather than picking a new one automatically.
- API key masking shows first 8 and last 4 characters (e.g., `lsv2_pt_...c123`).

## Out of Scope

- `langsmith login` (OAuth/browser-based auth flow) — future work
- Config file locking for concurrent access — single-user CLI
- Profile import/export
