package application

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/redact"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/taskreport"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// WriteTaskReport is idempotent: retries replace the report, never add usage.
func (app *Service) WriteTaskReport(ctx context.Context, request orchestration.ReportInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := workspace.TaskArtifactOutput(app.Root, request.Input.Task, "report.html")
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer root.Close()
	var document bytes.Buffer
	if err := taskreport.Render(&document, root, request); err != nil {
		return err
	}
	var values []string
	for _, step := range request.Input.Definition.Steps {
		for _, source := range step.SecretEnvironment {
			values = append(values, os.Getenv(source))
		}
	}
	return storage.WriteFileAtomically(target, []byte(redact.New(values...).Text(document.String())))
}
