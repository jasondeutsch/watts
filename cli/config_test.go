package cli

import (
	"path/filepath"
	"strings"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSameReviewModelIsAllowedWithoutWarnings(t *testing.T) {
	directory := t.TempDir()
	cfg := projectconfig.Default()
	setWorkflowModels(&cfg, "ollama", "devstral:24b", "devstral:24b")
	require.NoError(t, manifest.Write(filepath.Join(directory, projectconfig.Filename), manifest.Project, "test", cfg))
	for _, args := range [][]string{{"config", "show"}, {"config", "set", "max_attempts", "3"}} {
		code, _, errors := runCLI(t, directory, args...)
		require.Equal(t, 0, code, errors)
		assert.Empty(t, errors)
	}
}
func TestProofOfConceptRelease(t *testing.T) {
	code, out, errs := runCLI(t, t.TempDir(), "version")
	require.False(t, code != 0 || strings.TrimSpace(out) != "watts 0.1.0",
		"version: %d %q %s", code, out, errs)
}
