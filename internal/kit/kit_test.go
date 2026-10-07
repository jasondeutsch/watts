package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func memKit(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for p, c := range files {
		m["kit/"+p] = &fstest.MapFile{Data: []byte(c)}
	}
	return m
}

func TestInitializeCreatesMissingAssetsAndPreservesExistingFiles(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "kit")
	source := memKit(map[string]string{"scripts/check.sh": "#!/bin/sh\n", "skills/example/SKILL.md": "skill"})
	require.NoError(t, initializeFrom(source, "kit", directory))
	info, err := os.Stat(filepath.Join(directory, "scripts", "check.sh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
	script := filepath.Join(directory, "scripts", "check.sh")
	writeFile(t, script, "local contents")
	require.NoError(t, initializeFrom(source, "kit", directory))
	assert.Equal(t, "local contents", readFile(t, script))
	require.NoError(t, os.Remove(filepath.Join(directory, "skills", "example", "SKILL.md")))
	require.NoError(t, initializeFrom(source, "kit", directory))
	assert.Equal(t, "skill", readFile(t, filepath.Join(directory, "skills", "example", "SKILL.md")))
}

func TestEmbeddedKitContents(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kit")
	{
		err := Initialize(dir)
		require.NoError(t, err)
	}

	for _, f := range []string{
		"scripts/lib.sh", "scripts/new-task.sh", "scripts/preflight.sh", "scripts/pre-review.sh", "scripts/verify.sh",
		"scripts/close-task.sh", "scripts/check-approval.sh", "scripts/check-plan.sh", "scripts/check-stories.sh",
		"scripts/check-rework.sh", "scripts/check-verdict.sh", "scripts/gates.sh", "scripts/evidence.sh",
		"templates/RALPH.build.md", "templates/RALPH.review.md",
	} {
		assert.True(t, exists(filepath.Join(dir, filepath.FromSlash(f))),
			"embedded kit is missing %s", f)
	}
	skills := EmbeddedSkillNames()
	want := []string{"admission-rubric", "plan-template", "retrospective-log", "risk-tiering", "spec-template"}
	require.Equal(t, strings.Join(want, ","), strings.Join(skills, ","),
		"skills = %v, want %v", skills, want)

	for _, s := range skills {
		body := readFile(t, filepath.Join(dir, "skills", s, "SKILL.md"))
		assert.True(t, strings.HasPrefix(body, "---\nname: "+s+"\n"),
			"skill %s: frontmatter name must match its directory", s)
		assert.Contains(t, body, "\ndescription: ",
			"skill %s has no description", s)
	}
	// Check formatting throughout the installed kit.
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body := readFile(t, p)
		for _, bad := range []string{"\u2013", "\u2014"} {
			assert.NotContains(t, body, bad,
				"%s contains an en or em dash", strings.TrimPrefix(p, dir))
		}
		return nil
	})
	require.NoError(t, err)

	for _, tpl := range []string{"RALPH.build.md", "RALPH.review.md"} {
		body := readFile(t, filepath.Join(dir, "templates", tpl))
		assert.False(t, !strings.Contains(body, "__KIT__") || !strings.Contains(body, "__NAME__"),
			"%s lost its placeholders", tpl)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))

	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }
