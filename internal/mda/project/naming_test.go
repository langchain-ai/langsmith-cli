package project

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeNameLowercasesAndReplacesSeparators(t *testing.T) {
	name, err := NormalizeName("My_Super Bot.Agent")
	require.NoError(t, err)
	assert.Equal(t, "my-super-bot-agent", name)
}

func TestNormalizeNameCollapsesAndTrimsSeparators(t *testing.T) {
	name, err := NormalizeName("__My_Agent__")
	require.NoError(t, err)
	assert.Equal(t, "my-agent", name)
}

func TestNormalizeNameRejectsEmptyOutput(t *testing.T) {
	_, err := NormalizeName("!!!")
	assert.EqualError(t, err, "name must contain at least one ASCII letter or digit")
}

func TestValidateAgentIDAcceptsPlatformIDs(t *testing.T) {
	for _, id := range []string{"a", "support-agent", "billing-v2", strings.Repeat("a", 63)} {
		assert.NoError(t, ValidateAgentID(id), id)
	}
}

func TestValidateAgentIDRejectsIDsOutsidePlatformRules(t *testing.T) {
	for _, id := range []string{"", "2-support", "support-", "Support", "support_agent", "suppört", strings.Repeat("a", 64)} {
		assert.EqualError(t, ValidateAgentID(id), "agent ID \""+id+"\" must be 1-63 lowercase letters, "+
			"digits, or hyphens, start with a letter, and end with a letter or digit", id)
	}
}

func TestNormalizeNameRejectsLegacyMappingChange(t *testing.T) {
	_, err := NormalizeName("foo--bar")

	assert.EqualError(t, err, `name "foo--bar" is ambiguous after normalization; use "foo-bar" explicitly`)
}
