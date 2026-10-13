package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/containerengine"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workflowlang"
	"github.com/jasondeutsch/watts/internal/workload"
)

func (runtime *activityRuntime) prepareWorkspace(engine *containerengine.Engine, attempt *attemptWorkspace, input []byte, inputHash string) error {
	receipt := filepath.Join(attempt.root, "prepared.json")
	if data, err := os.ReadFile(receipt); err == nil {
		if err = json.Unmarshal(data, &attempt.preparation); err != nil {
			return err
		}
		if attempt.preparation.InputHash != inputHash {
			return fmt.Errorf("attempt inputs changed; refusing to reuse container state")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	// No process can be launched before this receipt is saved. A partial capture
	// can therefore be discarded safely, without mixing old and new input files.
	if err := os.RemoveAll(attempt.root); err != nil {
		return err
	}
	for _, name := range []string{attempt.input, attempt.output, attempt.private, filepath.Join(attempt.output, "task"), filepath.Join(attempt.private, "home"), filepath.Join(attempt.private, "tmp")} {
		if err := os.MkdirAll(name, 0700); err != nil {
			return err
		}
	}
	baseline, err := workload.Copy(runtime.project.Root, attempt.candidate, runtime.exclusions())
	if err != nil {
		return err
	}
	image, err := engine.Pin(runtime.context, runtime.request.Step.Container.Image)
	if err != nil {
		return err
	}
	attempt.preparation = attemptPreparation{InputHash: inputHash, Baseline: baseline, Image: image}
	var envelope StepInput
	if err = json.Unmarshal(input, &envelope); err != nil {
		return err
	}
	if envelope.Agent != nil && len(envelope.Agent.MCPServices) > 0 {
		operatingSystem, architecture, err := engine.Platform(runtime.context, image)
		if err != nil {
			return err
		}
		for name, service := range envelope.Agent.MCPServices {
			if err := service.ValidatePlatform(operatingSystem, architecture); err != nil {
				return fmt.Errorf("MCP %s: platform: %w", name, err)
			}
		}
	}
	for _, artifact := range envelope.Inputs {
		data, mode, err := workload.ReadFile(runtime.project.Root, filepath.Join(runtime.request.Input.Task, artifact.Path), workload.MaximumFileBytes)
		if err != nil {
			return err
		}
		if storage.Digest(data) != artifact.SHA256 {
			return fmt.Errorf("input %s changed during capture", artifact.Path)
		}
		for _, root := range []string{attempt.input, attempt.output} {
			if err = workload.WriteFile(root, "task/"+artifact.Path, data, mode); err != nil {
				return err
			}
		}
	}
	spec := runtime.request.Step.Container
	for name, source := range spec.Resources {
		if expected := spec.ResourceHashes[name]; expected != "" {
			digest, err := workload.ResourceHash(runtime.project.Root, source)
			if err != nil {
				return err
			}
			if digest != expected {
				return fmt.Errorf("pinned resource %s changed", name)
			}
		}
		if err = workload.CopyResource(runtime.project.Root, source, filepath.Join(attempt.input, "resources", name)); err != nil {
			return err
		}
	}
	for name, contents := range spec.Files {
		if err = workload.WriteFile(attempt.input, "config/"+name, []byte(contents), 0600); err != nil {
			return err
		}
	}
	if err = workload.WriteFile(attempt.input, "input.json", input, 0600); err != nil {
		return err
	}
	return storage.WriteJSONAtomically(receipt, attempt.preparation)
}

func (app *Service) protectPackageSources(taskPath string, cfg *config.Config) error {
	data, err := os.ReadFile(filepath.Join(app.StateDir(taskPath), "package.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var metadata workflowlang.Metadata
	if err = json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	for _, source := range metadata.SourcePaths {
		name, err := filepath.Rel(app.Root, source)
		if err != nil || name == "." || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			continue
		}
		name = filepath.ToSlash(name)
		if !contains(cfg.SnapshotExclude, name) {
			cfg.SnapshotExclude = append(cfg.SnapshotExclude, name)
		}
	}
	return cfg.Validate()
}

// Capture inputs at submission. Executions do not read mutable authoring assets.
func (app *Service) captureContainerResources(taskPath, stage string, spec *workload.Spec) error {
	sources := map[string]string{}
	hashes := map[string]string{}
	for name, source := range spec.Resources {
		digest, err := workload.ResourceHash(app.Root, source)
		if err != nil {
			return err
		}
		target := filepath.Join(taskPath, StateDirName, "resources", stage, digest, name)
		if err = workload.CopyResource(app.Root, source, filepath.Join(app.Root, target)); err != nil {
			return err
		}
		actual, err := workload.ResourceHash(app.Root, target)
		if err != nil {
			return err
		}
		if actual != digest {
			return fmt.Errorf("resource %s changed during submission", name)
		}
		sources[name] = filepath.ToSlash(target)
		hashes[name] = digest
	}
	spec.Resources, spec.ResourceHashes = sources, hashes
	return nil
}
