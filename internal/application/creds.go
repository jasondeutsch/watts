package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// Finding is one problem found by a check. Level is "fail" (do not start) or "warn".
type Finding struct {
	Level   string `json:"level"`
	Agent   string `json:"agent"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

var (
	// reEnvLike is how a variable name usually looks. A lowercase value such as the Ollama dummy
	// key is a literal, not a name.
	ReEnvLike    = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{2,}$`)
	ReDollarName = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)
)

type ModelsDoc struct {
	Providers map[string]struct {
		APIKey string `json:"apiKey"`
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	} `json:"providers"`
}

func HasFail(fs []Finding) bool {
	for _, f := range fs {
		if f.Level == "fail" {
			return true
		}
	}
	return false
}

// credentialCheck answers one question before any agent starts: will this agent actually be able
// to authenticate? It reads the agent's models.json and follows the apiKey it names through the
// passthrough list to your shell. It never prints a key.
func (a *Service) CredentialCheck(cfg projectconfig.Config, name string, extraPass []string) []Finding {
	agent, ok := cfg.Agent(name)
	if !ok {
		return []Finding{{Level: "fail", Agent: name, Message: fmt.Sprintf("agent %q is not defined", name)}}
	}
	var out []Finding
	add := func(level, msg, fix string) {
		out = append(out, Finding{Level: level, Agent: name, Message: msg, Fix: fix})
	}

	provider := agent.Provider
	decl, declared := cfg.Providers[provider]
	if declared && len(decl.APIKeyCommand) > 0 {
		if _, err := exec.LookPath(decl.APIKeyCommand[0]); err != nil {
			add("fail", fmt.Sprintf("providers.%s.api_key_command runs %q, which was not found", provider, decl.APIKeyCommand[0]), "install it or fix the command in "+projectconfig.Filename)
		}
	}

	path := filepath.Join(a.AgentDir(cfg, name), "models.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if provider != "ollama" && !declared {
			add("warn", fmt.Sprintf("%s has no models.json, so only providers that Pi already knows can be used for %q", a.Rel(path), provider),
				fmt.Sprintf("declare the provider under \"providers\" in %s, or write the models.json yourself", projectconfig.Filename))
		}
		return out
	}
	var doc ModelsDoc
	if err != nil || json.Unmarshal(data, &doc) != nil {
		add("fail", fmt.Sprintf("%s cannot be read as JSON", a.Rel(path)), "fix the file, or regenerate it with: watts config apply --force")
		return out
	}
	p, found := doc.Providers[provider]
	if !found {
		if provider != "ollama" || declared {
			add("warn", fmt.Sprintf("%s has no entry for provider %q", a.Rel(path), provider), "run: watts config apply")
		}
		return out
	}
	listed := false
	for _, m := range p.Models {
		listed = listed || m.ID == agent.Model
	}
	if len(p.Models) > 0 && !listed {
		add("fail", fmt.Sprintf("the %s agent uses model %q, which %s does not list under %q", name, agent.Model, a.Rel(path), provider),
			"add the model to models.json (or to providers."+provider+".models in "+projectconfig.Filename+") or change the model")
	}

	key := strings.TrimSpace(p.APIKey)
	varName := ""
	if m := ReDollarName.FindStringSubmatch(key); m != nil {
		varName = m[1]
	} else if ReEnvLike.MatchString(key) {
		varName = key
	}
	switch {
	case key == "":
		add("warn", fmt.Sprintf("provider %q has no apiKey in %s", provider, a.Rel(path)), "")
	case varName == "":
		if len(key) >= 20 {
			add("warn", fmt.Sprintf("%s holds a literal key for %q. That is a secret at rest in the repository tree", a.Rel(path), provider),
				fmt.Sprintf("name an environment variable instead (providers.%s.api_key_env in %s)", provider, projectconfig.Filename))
		}
	case strings.HasPrefix(varName, "WATTS_APIKEY_") && declared && len(decl.APIKeyCommand) > 0:
		// Filled by Watts from api_key_command at launch.
	default:
		if !projectconfig.Contains(cfg.PassthroughNames(agent, extraPass), varName) {
			add("fail", fmt.Sprintf("%s uses %s for its API key, but %s is not in env_passthrough, so pi will not see it. Watts runs pi with a clean environment", provider, varName, varName),
				"watts config env add "+varName)
		}
		if v, Set := os.LookupEnv(varName); !Set || v == "" {
			add("fail", fmt.Sprintf("%s is not set in your shell, so there is no key to give pi (pi would send the name %q as the key and the gateway would answer 401)", varName, varName),
				"export "+varName+"=<your key> and run again")
		}
	}
	return out
}

// requireCredentials stops a run that could not authenticate, naming the real cause.
func (a *Service) RequireCredentials(cfg projectconfig.Config, name string, extraPass []string) error {
	fs := a.CredentialCheck(cfg, name, extraPass)
	if len(fs) == 0 {
		return nil
	}
	var warns []Finding
	for _, f := range fs {
		if f.Level == "warn" {
			warns = append(warns, f)
		}
	}
	if HasFail(fs) {
		a.PrintFindings(fs)
		return workspace.ExitError{Code: 1, Message: fmt.Sprintf("the %s agent cannot authenticate; nothing was started", name)}
	}
	if len(warns) > 0 {
		for _, w := range warns {
			a.warn("%s agent: %s", w.Agent, w.Message)
		}
	}
	return nil
}

// EnvEntry is one variable in an agent's environment. Values are never included.
type EnvEntry struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	State  string `json:"state"`
}

// envReport lists what an agent's process receives, by name only.
func (a *Service) EnvReport(cfg projectconfig.Config, name string, extraPass []string, offline bool) ([]EnvEntry, error) {
	agent, ok := cfg.Agent(name)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q (defined agents: %s)", name, strings.Join(cfg.AgentNames(), ", "))
	}
	var out []EnvEntry
	for _, n := range []string{"HOME", "PATH", "TERM", "LANG", "PI_CODING_AGENT_DIR", "PI_SKIP_VERSION_CHECK", "PI_TELEMETRY",
		"WATTS_ROLE"} {
		out = append(out, EnvEntry{Name: n, Source: "set by watts", State: "set"})
	}
	if cfg.Offline || offline {
		out = append(out, EnvEntry{Name: "PI_OFFLINE", Source: "set by watts", State: "set"})
	}
	for _, n := range cfg.PassthroughNames(agent, extraPass) {
		src := "env_passthrough"
		if projectconfig.Contains(extraPass, n) {
			src = "--env-passthrough flag"
		}
		if p, ok := cfg.Providers[agent.Provider]; ok && p.APIKeyEnv == n && !projectconfig.Contains(cfg.EnvPassthrough, n) {
			src = "providers." + agent.Provider + ".api_key_env"
		}
		state := "set"
		if v, Set := os.LookupEnv(n); !Set || v == "" {
			state = "not set in your shell, so it is not passed"
		}
		out = append(out, EnvEntry{Name: n, Source: src, State: state})
	}
	if p, ok := cfg.Providers[agent.Provider]; ok && len(p.APIKeyCommand) > 0 {
		out = append(out, EnvEntry{Name: projectconfig.ProviderKeyVar(agent.Provider), Source: "generated at launch by providers." + agent.Provider + ".api_key_command", State: "set at launch"})
	}
	return out, nil
}
