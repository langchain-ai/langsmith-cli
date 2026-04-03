package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/spf13/cobra"
)

// getConfigPath returns the config file path from env or the default.
func getConfigPath() string {
	if v := os.Getenv("LANGSMITH_CONFIG_FILE"); v != "" {
		return v
	}
	return config.DefaultConfigPath()
}

// promptInput reads a line from stdin, printing the prompt to stderr.
func promptInput(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// promptInputDefault reads a line from stdin, showing a default value in the prompt.
// Returns the default value if the input is empty.
func promptInputDefault(label, defaultVal string) string {
	var prompt string
	if defaultVal != "" {
		prompt = fmt.Sprintf("%s [%s]: ", label, defaultVal)
	} else {
		prompt = fmt.Sprintf("%s: ", label)
	}
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		if val == "" {
			return defaultVal
		}
		return val
	}
	return defaultVal
}

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage named configuration profiles",
		Long: `Manage named configuration profiles for LangSmith.

Profiles store API keys, API URLs, and workspace IDs so you can
switch between multiple LangSmith environments (e.g., production vs. staging).

Examples:
  langsmith profile create prod --api-key ls-xxx --api-url https://api.smith.langchain.com
  langsmith profile list
  langsmith profile use prod
  langsmith profile show prod
  langsmith profile delete prod`,
	}

	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileShowCmd())
	cmd.AddCommand(newProfileCreateCmd())
	cmd.AddCommand(newProfileDeleteCmd())
	cmd.AddCommand(newProfileUseCmd())

	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all named profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := getConfigPath()
			cfg, err := config.LoadFrom(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			activeName := cfg.ResolveProfileName(flagProfile, os.Getenv("LANGSMITH_PROFILE"))

			// Collect and sort profile names
			names := make([]string, 0, len(cfg.Profiles))
			for name := range cfg.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)

			type profileEntry struct {
				Name   string `json:"name"`
				APIURL string `json:"api_url"`
				Active bool   `json:"active"`
			}

			entries := make([]profileEntry, 0, len(names))
			for _, name := range names {
				p := cfg.Profiles[name]
				entries = append(entries, profileEntry{
					Name:   name,
					APIURL: p.APIURL,
					Active: name == activeName,
				})
			}

			out, err := json.Marshal(entries)
			if err != nil {
				return fmt.Errorf("marshaling output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME",
		Short: "Show a profile's configuration (API key is masked)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfgPath := getConfigPath()
			cfg, err := config.LoadFrom(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			p, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("profile %q not found", name)
			}

			type showEntry struct {
				Name        string `json:"name"`
				APIKey      string `json:"api_key"`
				APIURL      string `json:"api_url"`
				WorkspaceID string `json:"workspace_id,omitempty"`
			}
			entry := showEntry{
				Name:        name,
				APIKey:      config.MaskAPIKey(p.APIKey),
				APIURL:      p.APIURL,
				WorkspaceID: p.WorkspaceID,
			}

			out, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("marshaling output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}

func newProfileCreateCmd() *cobra.Command {
	var (
		apiKey      string
		apiURL      string
		workspaceID string
	)

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a new named profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := config.ValidateProfileName(name); err != nil {
				return err
			}

			cfgPath := getConfigPath()
			cfg, err := config.LoadFrom(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if _, exists := cfg.Profiles[name]; exists {
				return fmt.Errorf("profile %q already exists", name)
			}

			// Prompt interactively for missing values only when flags weren't passed.
			interactive := !cmd.Flags().Changed("api-key")
			if apiKey == "" && interactive {
				apiKey = promptInput("API Key: ")
			}
			if apiURL == "" && interactive {
				apiURL = promptInputDefault("API URL", "https://api.smith.langchain.com")
			}
			if apiURL == "" {
				apiURL = "https://api.smith.langchain.com"
			}
			if workspaceID == "" && interactive {
				workspaceID = promptInputDefault("Workspace ID (optional)", "")
			}

			cfg.Profiles[name] = config.Profile{
				APIKey:      apiKey,
				APIURL:      apiURL,
				WorkspaceID: workspaceID,
			}

			// First profile becomes current
			if len(cfg.Profiles) == 1 {
				cfg.CurrentProfile = name
			}

			if err := cfg.SaveTo(cfgPath); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			out, err := json.Marshal(map[string]string{
				"status":  "created",
				"profile": name,
			})
			if err != nil {
				return fmt.Errorf("marshaling output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for this profile")
	cmd.Flags().StringVar(&apiURL, "api-url", "", "API URL for this profile")
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Workspace ID for this profile (optional)")

	return cmd
}

func newProfileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a named profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfgPath := getConfigPath()
			cfg, err := config.LoadFrom(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}

			delete(cfg.Profiles, name)

			// Clear current_profile if we deleted it
			if cfg.CurrentProfile == name {
				cfg.CurrentProfile = ""
			}

			if err := cfg.SaveTo(cfgPath); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			out, err := json.Marshal(map[string]string{
				"status":  "deleted",
				"profile": name,
			})
			if err != nil {
				return fmt.Errorf("marshaling output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}

func newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use NAME",
		Short: "Switch to a named profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfgPath := getConfigPath()
			cfg, err := config.LoadFrom(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}

			cfg.CurrentProfile = name

			if err := cfg.SaveTo(cfgPath); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			out, err := json.Marshal(map[string]string{
				"status":  "switched",
				"profile": name,
			})
			if err != nil {
				return fmt.Errorf("marshaling output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}
