package application

import (
	"path/filepath"

	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workflowlang"
)

func (app *Service) WritePackageMetadata(taskPath string, metadata *workflowlang.Metadata) error {
	if metadata == nil {
		return nil
	}
	return storage.WriteJSONAtomically(filepath.Join(app.Root, taskPath, ".watts-state", "package.json"), metadata)
}
