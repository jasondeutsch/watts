package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

// Check is one line of `watts doctor`. Level is ok, warn or fail.
type Check struct {
	Level  string `json:"level"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

func (a *Service) DoctorChecks(cfg projectconfig.Config, haveConfig, live bool) []Check {
	var out []Check
	ok := func(name, detail string) { out = append(out, Check{Level: "ok", Name: name, Detail: detail}) }
	bad := func(name, detail, fix string) {
		out = append(out, Check{Level: "fail", Name: name, Detail: detail, Fix: fix})
	}
	warn := func(name, detail, fix string) {
		out = append(out, Check{Level: "warn", Name: name, Detail: detail, Fix: fix})
	}

	tools := []string{"bash", "awk", "find", "xargs"}
	if !LookPath("sha256sum") && !LookPath("shasum") {
		bad("sha256sum or shasum not found", "snapshots need one of them", "")
	}
	for _, t := range tools {
		if LookPath(t) {
			ok(t+" found", "")
		} else {
			bad(t+" not found", "the check scripts need it", "")
		}
	}
	if v := NodeVersion(); v == "" {
		bad("node not found", "needs "+NodeMin+" or later", "")
	} else if VersionAtLeast(v, NodeMin) {
		ok("node "+v, "needs "+NodeMin+" or later")
	} else {
		bad("node "+v+" is too old", "needs "+NodeMin+" or later", "")
	}
	if LookPath("npm") {
		ok("npm found", "")
	} else {
		bad("npm not found", "", "")
	}
	pv := PiVersion()
	switch {
	case pv == "":
		bad("pi not found", "", "watts install pi")
	case VersionAtLeast(pv, PiMin):
		ok("pi "+pv, "needs "+PiMin+" or later")
	default:
		bad("pi "+pv+" is too old", "needs "+PiMin+" or later", "watts install pi")
	}

	if _, err := os.Stat(filepath.Join(a.KitDir(), "scripts", "lib.sh")); err != nil {
		bad("the kit is not installed in "+a.Rel(a.KitDir()), "", "watts init")
	} else {
		ok("project assets installed in "+a.Rel(a.KitDir()), "")
	}
	if !haveConfig {
		bad(projectconfig.Filename+" is missing or invalid", "", "watts init")
		return out
	}
	ok(projectconfig.Filename+" is valid", "")
	if st, err := os.Stat(a.TasksPath(cfg)); err == nil && st.IsDir() {
		ok("tasks directory "+cfg.TasksDir+" exists", "")
	} else {
		warn("tasks directory "+cfg.TasksDir+" does not exist yet", "it is created by the first task", "watts task new <slug>")
	}

	usesOllama := false
	for _, name := range cfg.AgentNames() {
		agent, _ := cfg.Agent(name)
		usesOllama = usesOllama || agent.Provider == "ollama"
		if _, err := os.Stat(a.AgentDir(cfg, name)); err != nil {
			bad("agent directory for the "+name+" agent is missing", "", "watts config apply")
			continue
		}
		ok("agent directory for the "+name+" agent exists", a.Rel(a.AgentDir(cfg, name)))
		for _, f := range []string{"settings.json", "models.json"} {
			if data, err := os.ReadFile(filepath.Join(a.AgentDir(cfg, name), f)); err == nil && !json.Valid(data) {
				bad(a.AgentRel(cfg, name)+"/"+f+" is not valid JSON", "", "watts config apply --force")
			}
		}
		for _, d := range agent.SkillsDirs {
			if _, err := os.Stat(filepath.Join(a.Root, filepath.FromSlash(d))); err != nil {
				bad("skills directory "+d+" for the "+name+" agent does not exist", "", "create it or remove it from "+projectconfig.Filename)
			}
		}
		if pv != "" {
			outp, err := a.CapturePi(cfg, name, "list")
			if err == nil && strings.Contains(outp, RalphPackage) {
				ok(RalphPackage+" installed for the "+name+" agent", "")
			} else {
				bad(RalphPackage+" is not installed for the "+name+" agent", "", "watts install ralph")
			}
		}
		findings := a.CredentialCheck(cfg, name, nil)
		for _, f := range findings {
			lvl := f.Level
			out = append(out, Check{Level: lvl, Name: "credentials for the " + name + " agent", Detail: f.Message, Fix: f.Fix})
		}
		if len(findings) == 0 {
			ok("credentials for the "+name+" agent", "the key variable is named, passed through and set")
		}
		if live {
			out = append(out, a.LiveCheck(cfg, name))
		}
	}
	if usesOllama {
		client := http.Client{Timeout: 3 * time.Second}
		if resp, err := client.Get(strings.TrimRight(cfg.Providers["ollama"].BaseURL, "/") + "/models"); err == nil {
			resp.Body.Close()
			ok("Ollama reachable at "+cfg.Providers["ollama"].BaseURL, "")
		} else {
			warn("Ollama not reachable at "+cfg.Providers["ollama"].BaseURL, "optional: ignore this if you use a hosted provider", "")
		}
	}
	return out
}

// liveCheck sends one tiny chat completion with the same URL, key and model the agent will use,
// from outside the sandbox. It tells a missing key apart from a rejected one.
func (a *Service) LiveCheck(cfg projectconfig.Config, name string) Check {
	agent, _ := cfg.Agent(name)
	label := "live gateway call for the " + name + " agent"
	data, err := os.ReadFile(filepath.Join(a.AgentDir(cfg, name), "models.json"))
	var doc struct {
		Providers map[string]struct {
			BaseURL string `json:"baseUrl"`
			APIKey  string `json:"apiKey"`
		} `json:"providers"`
	}
	if err != nil || json.Unmarshal(data, &doc) != nil {
		return Check{Level: "warn", Name: label, Detail: "no readable models.json, so there is no URL to call", Fix: "watts config apply"}
	}
	p, found := doc.Providers[agent.Provider]
	if !found || p.BaseURL == "" {
		return Check{Level: "warn", Name: label, Detail: "models.json has no base URL for " + agent.Provider}
	}
	key := p.APIKey
	if decl, ok := cfg.Providers[agent.Provider]; ok && len(decl.APIKeyCommand) > 0 {
		k, err := RunKeyCommand(decl.APIKeyCommand)
		if err != nil {
			return Check{Level: "fail", Name: label, Detail: err.Error()}
		}
		key = k
	} else if ReEnvLike.MatchString(key) {
		key = os.Getenv(key)
	} else if m := ReDollarName.FindStringSubmatch(key); m != nil {
		key = os.Getenv(m[1])
	}
	body, _ := json.Marshal(map[string]any{
		"model": agent.Model, "max_tokens": 8,
		"messages": []map[string]string{{"role": "user", "content": "ping"}},
	})
	req, err := http.NewRequest("POST", strings.TrimRight(p.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Check{Level: "fail", Name: label, Detail: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" && key != "none" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return Check{Level: "fail", Name: label, Detail: "could not reach " + p.BaseURL + ": " + OneLine(err.Error(), 160)}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == 200:
		return Check{Level: "ok", Name: label, Detail: fmt.Sprintf("%s answered 200 for %s", p.BaseURL, agent.Model)}
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return Check{Level: "fail", Name: label, Detail: fmt.Sprintf("the gateway rejected the key (HTTP %d). Checked from outside the sandbox, so the key itself is wrong, expired or not entitled to this model", resp.StatusCode)}
	default:
		return Check{Level: "fail", Name: label, Detail: fmt.Sprintf("HTTP %d from %s for model %s", resp.StatusCode, p.BaseURL, agent.Model)}
	}
}
