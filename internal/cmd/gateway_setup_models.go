package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

var gatewayModelFamilies = []string{"haiku", "sonnet", "opus", "fable"}

func gatewayModelEnv(family string) string {
	return "ANTHROPIC_DEFAULT_" + strings.ToUpper(family) + "_MODEL"
}

// Resolve the effective saved aliases first so rerunning setup can change one
// family without losing the others. Explicit flags then shell environment win;
// gatewayCheckOtherSettings rejects higher scopes that would mask those choices.
func gatewayModelOverrides(cmd *cobra.Command) (map[string]string, error) {
	userPath, err := claudeSettingsPath("user")
	if err != nil {
		return nil, err
	}
	saved := map[string]string{}
	for _, p := range []string{userPath, filepath.Join(".claude", "settings.json"), filepath.Join(".claude", "settings.local.json")} {
		_, doc, err := gatewayReadSettings(p)
		if err != nil {
			return nil, err
		}
		env, err := gatewaySettingsEnv(doc)
		if err != nil {
			return nil, err
		}
		for _, family := range gatewayModelFamilies {
			key := gatewayModelEnv(family)
			if value, present := env[key]; present {
				saved[key] = value
			}
		}
	}
	models := map[string]string{}
	missing := []string{}
	requested := false
	for _, family := range gatewayModelFamilies {
		key, flag := gatewayModelEnv(family), family+"-model"
		value := saved[key]
		if v := os.Getenv(key); v != "" {
			value = v
		}
		if cmd.Flags().Changed(flag) {
			requested = true
			value, err = cmd.Flags().GetString(flag)
			if err != nil {
				return nil, err
			}
			if value == "" {
				return nil, fmt.Errorf("--%s (%s) must be a nonempty provider/model slug", flag, key)
			}
		}
		if value == "" {
			missing = append(missing, "--"+flag+" ("+key+")")
			continue
		}
		requested = true
		provider, model, ok := strings.Cut(value, "/")
		if !ok || provider == "" || model == "" || strings.Contains(model, "//") || strings.HasSuffix(model, "/") || strings.ContainsAny(value, "\\?#") || strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
			return nil, fmt.Errorf("--%s (%s) must be a provider/model slug with no whitespace or control characters", flag, key)
		}
		models[key] = value
	}
	if requested && len(missing) > 0 {
		return nil, fmt.Errorf("partial model overrides are unsafe: the bare gateway requires all four families (Haiku, Sonnet, Opus, Fable); missing %s; supply the missing overrides or remove all family overrides to use Anthropic passthrough; no settings were written", strings.Join(missing, ", "))
	}
	return models, nil
}
