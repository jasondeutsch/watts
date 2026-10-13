package application

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"github.com/jasondeutsch/watts/internal/orchestration"
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
	return storage.WriteFileAtomically(target, document.Bytes())
}
