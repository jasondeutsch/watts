package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workflowlang"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// CompileWorkflow is shared by CLI and future MCP adapters. Compilation has no
// runtime side effects and does not need Docker, Temporal or credentials.
func (app *Service) CompileWorkflow(source string, options workflowlang.Options) (workflowlang.Package, error) {
	return workflowlang.Load(source, options)
}

func (app *Service) CreateCompiledTask(cfg projectconfig.Config, slug string, pkg workflowlang.Package) (string, error) {
	if err := pkg.Validate(); err != nil {
		return "", err
	}
	if err := cfg.ValidateWorkflow(pkg.Definition); err != nil {
		return "", err
	}
	metadata := workflowlang.Metadata{Name: pkg.Name, Version: pkg.Version, Digest: pkg.Digest, SourcePaths: pkg.SourcePaths}
	metadata.Layers = append(metadata.Layers, workflowlang.Identity{Name: pkg.Name, Version: pkg.Version, Digest: pkg.Digest, Source: pkg.Source})
	for _, dependency := range pkg.Dependencies {
		if dependency.Kind == "module" {
			metadata.Layers = append(metadata.Layers, workflowlang.Identity{Name: dependency.Name, Version: dependency.Version, Digest: dependency.Digest, Source: dependency.Source})
		}
	}
	selection := TaskDefinition{Name: pkg.Name, Definition: pkg.Definition, Package: &metadata, Captured: &pkg}
	return app.createTask(cfg, slug, selection)
}

// ExecuteCompiled creates a task from a frozen artifact and submits it through
// the existing container/Temporal runtime. It never compiles authoring files.
func (app *Service) ExecuteCompiled(ctx context.Context, source, slug, request string, options *RunOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	pkg, err := workflowlang.ReadCompiled(source)
	if err != nil {
		return "", err
	}
	return app.executePackage(ctx, pkg, slug, request, options)
}

// ExecutePackage compiles authored ACL once for a new task. Existing task retries
// call RunTask and never return through this source-loading path.
func (app *Service) ExecutePackage(ctx context.Context, source, name, request string, compilation workflowlang.Options, options *RunOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	pkg, err := app.CompileWorkflow(source, compilation)
	if err != nil {
		return "", err
	}
	return app.executePackage(ctx, pkg, name, request, options)
}

func (app *Service) executePackage(ctx context.Context, pkg workflowlang.Package, name, request string, options *RunOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !options.DryRun {
		app.say("Selected %s@%s · workflow %s · %d stages", pkg.Name, pkg.Version, pkg.Workflow, len(pkg.Definition.Steps))
	}
	if options.DryRun {
		encoder := json.NewEncoder(app.Output)
		encoder.SetIndent("", "  ")
		return "", encoder.Encode(pkg.Definition)
	}
	cfg, err := app.LoadServiceConfig()
	if err != nil {
		return "", err
	}
	task, err := app.CreateCompiledTask(cfg, name, pkg)
	if err != nil {
		return "", err
	}
	if request != "" {
		target, err := workspace.TaskArtifactOutput(app.Root, task, "SPEC.md")
		if err != nil {
			return task, err
		}
		if err := storage.WriteFileAtomically(target, []byte(request)); err != nil {
			return task, err
		}
	}
	if err := app.RunTask(cfg, task, options); err != nil {
		return task, fmt.Errorf("task %s created; execution was not submitted: %w", task, err)
	}
	return task, nil
}

// Compiled tasks use JSON executor contracts. YAML is only consulted for tasks
// created by the existing manifest path, never as a DSL source during execution.
func compiledTaskPath(root, task string) string { return filepath.Join(root, task, "workflow.json") }
func isCompiledTask(root, task string) bool {
	_, err := os.Stat(compiledTaskPath(root, task))
	return err == nil
}
