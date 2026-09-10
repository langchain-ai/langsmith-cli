package cmd

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

var gatewayModelFamilies = []string{"haiku", "sonnet", "opus", "fable"}

func gatewayModelEnv(family string) string {
	return "ANTHROPIC_DEFAULT_" + strings.ToUpper(family) + "_MODEL"
}

// Each invocation supplies the complete model selection: flags override shell
// environment, never saved settings. No selection restores native defaults.
func gatewayModelOverrides(cmd *cobra.Command) (map[string]string, error) {
	models := map[string]string{}
	missing := []string{}
	requested := false
	for _, family := range gatewayModelFamilies {
		key, flag := gatewayModelEnv(family), family+"-model"
		value := os.Getenv(key)
		if cmd.Flags().Changed(flag) {
			requested = true
			var err error
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
