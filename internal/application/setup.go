package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// wattsGitignore is written to .watts/.gitignore, inside Watts' own folder, so that if the project
// happens to be tracked by version control the local state is not picked up by accident. Watts
// itself does not use version control and never edits the project's own .gitignore. Everything
// under .watts can be rebuilt from watts.json.
const WattsGitignore = `# Local Pi state for the agents: logins, packages, sessions. Never commit.
pi-agent-*/
home-*/
.generated.json
`

// agentGitignore is written into an agent directory that lives outside .watts, so its local
// state is not committed by accident.
const AgentGitignore = `# Local Pi state: logins, packages, sessions. Never commit.
sessions/
auth.json
models-store.json
npm/
git/
bin/
`

// writeLocalIgnores writes .watts/.gitignore. Nothing outside the Watts folders is touched.
func (a *Service) WriteLocalIgnores() error {
	if err := os.MkdirAll(a.Wd(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(a.Wd(".gitignore"), []byte(WattsGitignore), 0o644)
}

type OllamaProvider struct {
	BaseURL string        `json:"baseUrl"`
	API     string        `json:"api"`
	APIKey  string        `json:"apiKey"`
	Models  []OllamaModel `json:"models"`
}

type OllamaModel struct {
	ID string `json:"id"`
}

type ModelsFile struct {
	Providers map[string]OllamaProvider `json:"providers"`
}

// ollamaModelsJSON follows https://pi.dev/docs/latest/models#configure-a-compatible-endpoint. The
// key is a dummy: it only makes the model available to Pi, and Ollama ignores it.
func OllamaModelsJSON(url string, ids ...string) []byte {
	var models []OllamaModel
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			models = append(models, OllamaModel{ID: id})
		}
	}
	data, _ := json.MarshalIndent(ModelsFile{Providers: map[string]OllamaProvider{
		"ollama": {BaseURL: url, API: "openai-completions", APIKey: "ollama", Models: models},
	}}, "", "  ")
	return append(data, '\n')
}

type PiModel struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	Reasoning     bool   `json:"reasoning,omitempty"`
	ContextWindow int    `json:"contextWindow,omitempty"`
	MaxTokens     int    `json:"maxTokens,omitempty"`
}

type PiProvider struct {
	Name       string    `json:"name,omitempty"`
	BaseURL    string    `json:"baseUrl"`
	API        string    `json:"api"`
	APIKey     string    `json:"apiKey"`
	AuthHeader bool      `json:"authHeader,omitempty"`
	Models     []PiModel `json:"models"`
}

// providerModelsJSON renders a provider declared in watts.json. apiKey is always a $NAME reference to an
// environment variable, never a key: the variable is api_key_env, or the one Watts fills from
// api_key_command.
func ProviderModelsJSON(name string, p projectconfig.Provider) []byte {
	api := p.API
	if api == "" {
		api = "openai-completions"
	}
	key := "none"
	switch {
	case p.APIKeyEnv != "":
		key = "$" + p.APIKeyEnv
	case len(p.APIKeyCommand) > 0:
		key = "$" + projectconfig.ProviderKeyVar(name)
	}
	pp := PiProvider{Name: p.Name, BaseURL: p.BaseURL, API: api, APIKey: key, AuthHeader: key != "none"}
	for _, m := range p.Models {
		mn := m.Name
		if mn == "" {
			mn = m.ID
		}
		pp.Models = append(pp.Models, PiModel{ID: m.ID, Name: mn, Reasoning: m.Reasoning, ContextWindow: m.ContextWindow, MaxTokens: m.MaxTokens})
	}
	data, _ := json.MarshalIndent(map[string]any{"providers": map[string]PiProvider{name: pp}}, "", "  ")
	return append(data, '\n')
}

func (a *Service) GeneratedState() map[string]string {
	m := map[string]string{}
	if data, err := os.ReadFile(a.Wd(".generated.json")); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

func (a *Service) SaveGeneratedState(m map[string]string) error {
	return storage.WriteJSON(a.Wd(".generated.json"), m)
}

// wantModelsJSON is the models.json Watts would write for an agent, or nil when it writes none
// (a hosted provider that Pi already knows).
func WantModelsJSON(cfg projectconfig.Config, agent projectconfig.ResolvedAgent) []byte {
	p, ok := cfg.Providers[agent.Provider]
	if !ok {
		return nil
	}
	if agent.Provider == "ollama" && len(p.Models) == 0 {
		seen := map[string]bool{}
		for _, step := range cfg.AgentWorkflow().Steps {
			if step.Provider == "ollama" && !seen[step.Model] {
				seen[step.Model] = true
				p.Models = append(p.Models, projectconfig.ProviderModel{ID: step.Model})
			}
		}
	}
	return ProviderModelsJSON(agent.Provider, p)
}

// configAgents creates or refreshes the private agent directory and private HOME for each agent.
// It never replaces a models.json you wrote yourself: a file is only regenerated while it still
// matches what Watts last wrote, or when force is set. Running it after deleting .watts rebuilds
// everything from watts.json.
func (a *Service) ConfigAgents(cfg projectconfig.Config, force bool) error {
	return a.configureAgents(cfg, cfg.AgentNames(), force)
}

func (a *Service) configureAgents(cfg projectconfig.Config, names []string, force bool) error {
	state := a.GeneratedState()
	for _, name := range names {
		agent, _ := cfg.Agent(name)
		dir, home := a.AgentDir(cfg, name), a.HomeDir(name)
		for _, d := range []string{filepath.Join(dir, "skills"), home} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return err
			}
		}
		if !strings.HasPrefix(agent.Path, ".watts/") {
			if err := WriteIfMissing(filepath.Join(dir, ".gitignore"), AgentGitignore); err != nil {
				return err
			}
		}

		// settings.json: merge, so installed packages and anything you added survive. Values under
		// pi_settings in watts.json are merged; workflow model selection is applied last.
		settingsPath := filepath.Join(dir, "settings.json")
		settings := map[string]any{}
		if data, err := os.ReadFile(settingsPath); err == nil {
			if err := json.Unmarshal(data, &settings); err != nil {
				return fmt.Errorf("%s is not valid JSON: %w", a.Rel(settingsPath), err)
			}
		}
		settings["defaultThinkingLevel"] = agent.Thinking
		settings["defaultProjectTrust"] = "never"
		settings["enableInstallTelemetry"] = false
		settings["quietStartup"] = true
		for k, v := range agent.PiSettings {
			settings[k] = v
		}
		if agent.Model != "" {
			settings["defaultProvider"] = agent.Provider
			settings["defaultModel"] = agent.Model
		} else {
			delete(settings, "defaultProvider")
			delete(settings, "defaultModel")
		}
		if err := storage.WriteJSON(settingsPath, settings); err != nil {
			return err
		}

		// models.json: generated for Ollama and for providers declared in watts.json, and never
		// over a file you edited.
		if want := WantModelsJSON(cfg, agent); want != nil {
			p := filepath.Join(dir, "models.json")
			key := a.Rel(p)
			cur, err := os.ReadFile(p)
			switch {
			case errors.Is(err, os.ErrNotExist), force, err == nil && state[key] == kit.Digest(cur):
				if err := os.WriteFile(p, want, 0o644); err != nil {
					return err
				}
				state[key] = kit.Digest(want)
			case err != nil:
				return err
			case kit.Digest(cur) == kit.Digest(want):
				state[key] = kit.Digest(want)
			default:
				a.warn("%s was edited by you and is left unchanged. Use: watts config apply --force to regenerate it.", key)
			}
		}

		// Skills: user-level skills in the agent directory are not subject to project trust.
		sources := []string{filepath.Join(a.KitDir(), "skills")}
		for _, d := range agent.SkillsDirs {
			sources = append(sources, filepath.Join(a.Root, filepath.FromSlash(d)))
		}
		for i, src := range sources {
			if err := CopySkills(src, filepath.Join(dir, "skills")); err != nil {
				if i == 0 {
					return fmt.Errorf("cannot read %s: %w. Run: watts init", a.Rel(src), err)
				}
				return fmt.Errorf("skills_dirs entry %s: %w", a.Rel(src), err)
			}
		}
		a.say("configured %s agent: provider=%s model=%s dir=%s", name, agent.Provider, agent.Model, a.Rel(dir))
	}
	return a.SaveGeneratedState(state)
}

func WriteIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// copySkills copies every skill folder found in src into dst. A folder with a SKILL.md directly
// inside src is itself a skill.
func CopySkills(src, dst string) error {
	if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err == nil {
		return kit.CopyDir(src, filepath.Join(dst, filepath.Base(src)))
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || e.Type()&fs.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(src, e.Name())); err != nil || !st.IsDir() {
				continue
			}
			if err := kit.CopyDir(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Service) InstallPi(cfg projectconfig.Config) error {
	if v := PiVersion(); v != "" && VersionAtLeast(v, PiMin) {
		a.say("pi %s is already installed (needs %s or later)", v, PiMin)
		return nil
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm is required. Install Node.js %s or later first", NodeMin)
	}
	spec := PiPackage + "@" + PiVersionRange
	a.say("installing %s globally with npm", spec)
	cmd := exec.Command(npm, "install", "-g", "--ignore-scripts", spec)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.Input, a.Output, a.ErrorOutput
	return workspace.RunCommand(cmd)
}

// installRalph installs the pinned loop extension into each agent's private directory. These are
// npm installs, so proxy and certificate variables must be passed (watts config set
// env_passthrough ...).
func (a *Service) InstallRalph(cfg projectconfig.Config, extraPass []string) error {
	spec := "npm:" + RalphPackage + "@" + RalphVersion
	for _, name := range cfg.AgentNames() {
		a.say("installing %s for the %s agent", spec, name)
		if err := a.RunPi(cfg, name, extraPass, false, "install", spec); err != nil {
			return err
		}
	}
	return nil
}

const (
	// nodeMin is what pi-ralph-loop 2.1.0 requires. Pi 1.0 itself needs 22.19.0 or later.
	NodeMin = "22.22.1"
	// piMin is the first Pi release with built-in MCP, codemode and the 1.0 API.
	PiMin = "1.0.0"
	// piPackage is Pi's npm package.
	PiPackage      = "@earendil-works/pi-coding-agent"
	PiVersionRange = "^1.0.0"
	RalphVersion   = "2.1.0"
	// ralphPackage is the loop extension.
	RalphPackage = "@lnilluv/pi-ralph-loop"
)
