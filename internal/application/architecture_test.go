package application

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These dependency checks protect the UI boundary as new commands and backends are added.
func TestPackageBoundaries(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	err := filepath.WalkDir(filepath.Join(projectRoot, "internal"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, dependency := range file.Imports {
			name, _ := strconv.Unquote(dependency.Path.Value)
			assert.False(t, name == "flag" || (name == "github.com/jasondeutsch/watts/cli" || strings.HasPrefix(name, "github.com/jasondeutsch/watts/cli/")),
				"internal package %s depends on UI package %s", path, name)
		}
		return nil
	})
	require.NoError(t, err)

	cliRoot := filepath.Join(projectRoot, "cli")
	err = filepath.WalkDir(cliRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(cliRoot, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		owner := ""
		if len(parts) > 2 && parts[0] == "internal" {
			owner = parts[1]
		}
		const cliModule = "github.com/jasondeutsch/watts/cli"
		for _, dependency := range file.Imports {
			name, _ := strconv.Unquote(dependency.Path.Value)
			assert.False(t, strings.HasPrefix(name, "go.temporal.io/") || name == "os/exec" || name == "crypto/sha256",
				"CLI adapter %s directly imports execution infrastructure %s", path, name)

			if owner == "" || (name != cliModule && !strings.HasPrefix(name, cliModule+"/")) {
				continue
			}
			allowed := name == cliModule+"/internal/arguments" || name == cliModule+"/internal/terminal"
			if owner == "arguments" || owner == "terminal" {
				allowed = false
			}
			assert.True(t, allowed,
				"CLI package %s imports another command domain or the dispatcher: %s", path, name)
		}
		return nil
	})
	require.NoError(t, err)
}
