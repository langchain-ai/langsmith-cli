package proc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToUnixPathJoinsComponentsWithForwardSlashes(t *testing.T) {
	assert.Equal(t, "agent.ts", ToUnixPath("agent.ts"))
	assert.Equal(t, "src/agents/agent.ts", ToUnixPath("src/agents/agent.ts"))
	assert.Equal(t, "./src/agent.ts", ToUnixPath("./src//./agent.ts"))
}

func TestToUnixPathPreservesNonUTF8Components(t *testing.T) {
	assert.Equal(t, "assets/�/file", ToUnixPath("assets/\xff/file"))
}
