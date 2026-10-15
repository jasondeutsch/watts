package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
)

func TestTaskDirectoryOrderingAndCollisions(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "tasks"), 0755))
	timestamp := time.Date(2026, 10, 12, 8, 9, 10, 123000000, time.FixedZone("local", -5*3600))
	first, err := createTaskDirectory(root, "tasks", "example", timestamp)
	require.NoError(t, err)
	second, err := createTaskDirectory(root, "tasks", "example", timestamp)
	require.NoError(t, err)
	third, err := createTaskDirectory(root, "tasks", "example", timestamp.Add(2*time.Millisecond))
	require.NoError(t, err)
	require.Equal(t, "tasks/2026-10-12t130910-123z-example", first)
	require.Less(t, first, second)
	require.Less(t, second, third)
	require.NotEqual(t, first, second)
}

func TestTaskReportActivityReplacesWholeDocument(t *testing.T) {
	app := &Service{Root: t.TempDir()}
	task := "tasks/demo"
	require.NoError(t, os.MkdirAll(filepath.Join(app.Root, task), 0755))
	request := orchestration.ReportInput{Input: orchestration.Input{Task: task}, State: orchestration.State{Status: "completed"}}
	require.NoError(t, app.WriteTaskReport(context.Background(), request))
	target := filepath.Join(app.Root, task, "report.html")
	require.Contains(t, readFile(t, target), "Status: completed")
	request.State.Status = "cancelled"
	require.NoError(t, app.WriteTaskReport(context.Background(), request))
	require.Contains(t, readFile(t, target), "Status: cancelled")
	require.NotContains(t, readFile(t, target), "Status: completed")
	require.NoError(t, os.Remove(target))
	require.NoError(t, os.Symlink(filepath.Join(app.Root, "outside.html"), target))
	require.ErrorContains(t, app.WriteTaskReport(context.Background(), request), "symlink")
}

func TestTaskReportRedactsHTMLCredentialForms(t *testing.T) {
	const secret = "synthetic<&>credential"
	t.Setenv("FIXTURE_REPORT_KEY", secret)
	app := &Service{Root: t.TempDir()}
	task := "tasks/report"
	require.NoError(t, os.MkdirAll(filepath.Join(app.Root, task), 0700))
	request := orchestration.ReportInput{
		Input: orchestration.Input{Task: task, Definition: orchestration.Definition{Steps: []orchestration.Step{{Name: "build", SecretEnvironment: map[string]string{"TOKEN": "FIXTURE_REPORT_KEY"}}}}},
		State: orchestration.State{Status: "failed", History: []orchestration.StageResult{{Step: "build", Attempt: 1, Error: "provider failed: " + secret}}},
	}
	require.NoError(t, app.WriteTaskReport(context.Background(), request))
	report := readFile(t, filepath.Join(app.Root, task, "report.html"))
	require.NotContains(t, report, secret)
	require.NotContains(t, report, "synthetic&lt;&amp;&gt;credential")
	require.Contains(t, report, "[REDACTED]")
	require.Contains(t, report, "Status: failed")
}

func TestNewTaskNamesAreNormalizedWithoutChangingLookup(t *testing.T) {
	for _, name := range []string{"Improve CSS", "IMPROVE_CSS", "  Improve---CSS  "} {
		slug, err := taskSlug(name)
		require.NoError(t, err)
		require.Equal(t, "improve-css", slug)
	}
	for _, name := range []string{"", "---", "../task", "/tmp/task", "a\\b", "a.b"} {
		_, err := taskSlug(name)
		require.Error(t, err)
	}
}
