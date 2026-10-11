package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workflowpackage"
)

func (app *Service) PackagePath() string {
	if app.PackageDescriptor != "" {
		return app.PackageDescriptor
	}
	return filepath.Join(app.Root, workflowpackage.Filename)
}

// ResolvePackage renders once from captured inputs and installs its declared
// assets locally. The authored descriptor is never rewritten by this operation.
func (app *Service) ResolvePackage(cfg projectconfig.Config) (WorkflowSelection, error) {
	pkg, err := workflowpackage.Load(app.PackagePath())
	if err != nil {
		return WorkflowSelection{}, err
	}
	result, err := pkg.Render()
	if err != nil {
		return WorkflowSelection{}, err
	}
	definition, err := manifest.CompileWorkflow(result.Data)
	if err != nil {
		return WorkflowSelection{}, err
	}
	if err := cfg.ValidateWorkflow(definition); err != nil {
		return WorkflowSelection{}, err
	}
	if err := pkg.Materialize(app.Root); err != nil {
		return WorkflowSelection{}, err
	}
	return WorkflowSelection{Name: result.Metadata.Name, Definition: definition, Package: &result.Metadata}, nil
}

func (app *Service) WritePackageMetadata(taskPath string, metadata *workflowpackage.Metadata) error {
	if metadata == nil {
		return nil
	}
	return storage.WriteJSONAtomically(filepath.Join(app.Root, taskPath, ".watts-state", "package.json"), metadata)
}

// InitializePackage supplies an editable source package for a new local project.
func (app *Service) InitializePackage(cfg projectconfig.Config) error {
	if _, err := os.Stat(app.ConfigPath()); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return app.writeWorkflowSource(cfg, true)
}

func definitionHash(definition orchestration.Definition) string {
	data, _ := json.Marshal(definition)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
