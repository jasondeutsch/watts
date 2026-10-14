package workflowlang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func write(t *testing.T, root, name, text string) {
	t.Helper()
	target := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0755))
	require.NoError(t, os.WriteFile(target, []byte(text), 0644))
}

const basic = `package "fixture" { version = "0.1.0" }
variable "image" {
 type = string
 default = "fixture:0.1.0"
}
procedure "build" {
 execution { type = "container" }
 container {
  image = var.image
  command = ["build"]
 }
}
procedure "review" {
 execution { type = "container" }
 container {
  image = var.image
  command = ["review"]
 }
}
workflow "coding" {
 stage "build" {
  executor = procedure.build
  outputs = { candidate = "candidate.md" }
 }
 stage "review" {
  executor = procedure.review
  inputs = [stage.build.outputs.candidate]
  on "changes-requested" {
   return_to = stage.build.name
   max_iterations = 2
  }
 }
}
`

func TestCompileConnectionsVariablesAndBoundedFeedback(t *testing.T) {
	root := t.TempDir()
	write(t, root, Filename, basic)
	pkg, err := Load(root, Options{Variables: map[string]cty.Value{"image": cty.StringVal("custom:0.1.0")}})
	require.NoError(t, err)
	require.Equal(t, "custom:0.1.0", pkg.Definition.Steps[0].Container.Image)
	require.Equal(t, []string{"candidate.md"}, pkg.Definition.Steps[1].Inputs)
	require.Equal(t, []string{"build"}, pkg.Definition.Steps[1].DependsOn)
	require.Equal(t, "build", pkg.Definition.Steps[1].Transitions["changes-requested"])
	require.Equal(t, 2, pkg.Definition.Steps[1].TransitionLimits["changes-requested"])
	repeated, err := Load(root, Options{Variables: map[string]cty.Value{"image": cty.StringVal("custom:0.1.0")}})
	require.NoError(t, err)
	require.Equal(t, pkg.Digest, repeated.Digest)
	require.NoError(t, pkg.Validate())
}
func TestModulesComposeReusableResourcesAndCaptureAssets(t *testing.T) {
	root := t.TempDir()
	write(t, root, "library/watts.acl", `package "library" {
 version = "0.1.0"
 assets = ["scripts"]
}
variable "image" { type = string }
procedure "check" {
 execution { type = "container" }
 container {
  image = var.image
  command = ["bash", "/inputs/resources/library/scripts/check.sh"]
  resources = { library = package.root }
 }
}
output "checker" { value = procedure.check }
`)
	write(t, root, "library/scripts/check.sh", "echo captured\n")
	write(t, root, Filename, `package "team" { version = "0.1.0" }
module "checks" {
 source = "./library"
 version = "0.1.0"
 variables = { image = "test:0.1.0" }
}
workflow "team" {
 stage "verify" { executor = module.checks.outputs.checker }
}
`)
	pkg, err := Load(root, Options{})
	require.NoError(t, err)
	require.Equal(t, "test:0.1.0", pkg.Definition.Steps[0].Container.Image)
	require.Contains(t, pkg.Definition.Steps[0].Container.Resources["library"], "packages/library/")
	target := filepath.Join(root, "compiled.json")
	data, err := json.Marshal(pkg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, data, 0600))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "library")))
	require.NoError(t, os.Remove(filepath.Join(root, Filename)))
	compiled, err := ReadCompiled(target)
	require.NoError(t, err)
	workspace := t.TempDir()
	require.NoError(t, compiled.Materialize(workspace))
	asset := filepath.Join(workspace, compiled.Definition.Steps[0].Container.Resources["library"], "scripts/check.sh")
	contents, err := os.ReadFile(asset)
	require.NoError(t, err)
	require.Equal(t, "echo captured\n", string(contents))
	require.NoError(t, compiled.Materialize(workspace))
	require.NoError(t, os.WriteFile(asset, []byte("modified"), 0644))
	require.ErrorContains(t, compiled.Materialize(workspace), "modified")
}
func TestInvalidSourceAndTamperedArtifact(t *testing.T) {
	for _, test := range []struct{ name, source, message string }{
		{"unknown variable", basic, "unknown variable"},
		{"unsupported field", `package "bad" { version="0.1.0" }
procedure "bad" { mystery = "x" }
workflow "bad" {
 stage "bad" { executor=procedure.bad }
}`, "unsupported argument"},
		{"module cycle", `package "cycle" {version="0.1.0"}
module "self" {source="."}`, "cycle"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, Filename, test.source)
			options := Options{}
			if test.name == "unknown variable" {
				options.Variables = map[string]cty.Value{"missing": cty.StringVal("x")}
			}
			_, err := Load(root, options)
			require.ErrorContains(t, err, test.message)
		})
	}
	root := t.TempDir()
	write(t, root, Filename, basic)
	pkg, err := Load(root, Options{})
	require.NoError(t, err)
	pkg.Definition.Steps[0].Container.Image = "tampered:latest"
	target := filepath.Join(root, "compiled.json")
	data, err := json.Marshal(pkg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, data, 0600))
	_, err = ReadCompiled(target)
	require.ErrorContains(t, err, "fingerprint")
}

func TestModuleReferencesDoNotDependOnDeclarationOrder(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings/watts.acl", `package "settings" {version="0.1.0"}
output "image" {value="fixture:0.1.0"}
`)
	write(t, root, "checks/watts.acl", `package "checks" {version="0.1.0"}
variable "image" {type=string}
procedure "verify" {
 execution {type="container"}
 container {
  image=var.image
  command=["verify"]
 }
}
`)
	write(t, root, Filename, `package "consumer" {version="0.1.0"}
module "checks" {
 source="./checks"
 variables={image=module.settings.outputs.image}
}
module "settings" {source="./settings"}
workflow "consumer" {
 stage "verify" {executor=module.checks.procedure.verify}
}
`)
	pkg, err := Load(root, Options{})
	require.NoError(t, err)
	require.Equal(t, "fixture:0.1.0", pkg.Definition.Steps[0].Container.Image)
}

func TestCoderGolangModulesAndPolicy(t *testing.T) {
	source := filepath.Join("..", "..", "resources", "examples", "coder_golang")
	pkg, err := Load(source, Options{Variables: map[string]cty.Value{"model": cty.StringVal("team/model")}})
	require.NoError(t, err)
	require.Len(t, pkg.Definition.Steps, 6)
	for _, step := range pkg.Definition.Steps {
		if step.Agent != "" {
			require.Equal(t, "team/model", step.Model)
			require.Equal(t, "OPENROUTER_API_KEY", step.SecretEnvironment["OPENROUTER_API_KEY"])
		}
		if step.Name == "build" {
			require.Len(t, step.Gates, 5)
			require.Contains(t, step.MCPServices, "gopls-service")
			require.NotEmpty(t, step.Skills)
			require.Equal(t, "gopls-service", step.Tools[4].MCP.Ref)
		}
	}
	require.Equal(t, 2, pkg.Definition.Steps[5].TransitionLimits["changes-required"])
	require.NoError(t, pkg.Validate())
}

func TestWholeWorkflowEntrypointAndSharedModuleAssets(t *testing.T) {
	root := t.TempDir()
	write(t, root, "library/watts.acl", `package "library" {
 version="0.1.0"
 assets=["scripts"]
}
procedure "verify" {
 execution {type="container"}
 container {
  image="fixture:0.1.0"
  command=["verify"]
  resources={library=package.root}
 }
}
workflow "checks" {
 stage "verify" {executor=procedure.verify}
}
`)
	write(t, root, "library/scripts/check.sh", "echo captured\n")
	write(t, root, Filename, `package "consumer" {
 version="0.1.0"
 entrypoint=module.first.workflow.checks
}
module "first" {source="./library"}
module "second" {source="./library"}
`)
	pkg, err := Load(root, Options{})
	require.NoError(t, err)
	require.NoError(t, pkg.Validate())
	require.Len(t, pkg.Definition.Steps, 1)
	require.Len(t, pkg.Assets, 1)
}

func TestACLDoesNotLoadUnrelatedHCLFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, Filename, basic)
	write(t, root, "terraform.hcl", "This belongs to another language and is intentionally invalid ACL.")
	pkg, err := Load(root, Options{})
	require.NoError(t, err)
	require.Len(t, pkg.Definition.Steps, 2)
	require.NotContains(t, pkg.SourcePaths, filepath.Join(root, "terraform.hcl"))
	_, err = Load(filepath.Join(root, "terraform.hcl"), Options{})
	require.ErrorContains(t, err, "ACL files")
	require.NoError(t, os.Rename(filepath.Join(root, Filename), filepath.Join(root, "custom.acl")))
	pkg, err = Load(filepath.Join(root, "custom.acl"), Options{})
	require.NoError(t, err)
	require.Equal(t, "fixture", pkg.Name)
}
