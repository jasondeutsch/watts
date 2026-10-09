package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jasondeutsch/watts/internal/orchestration"
)

// Config is watts.json, at the project root. It declares execution infrastructure;
// workflow definitions declare stage identities, providers and models.
// Secrets never go in here: they live in the environment and are named, not stored (see
// EnvPassthrough and Provider.APIKeyEnv).
type Config struct {
	agentWorkflow *orchestration.Definition

	Version int  `json:"version"`
	Offline bool `json:"offline"`

	// EnvPassthrough lists environment variable NAMES copied from your shell into every agent's
	// otherwise empty environment. Watts adds the api_key_env of each agent's provider on its own.
	EnvPassthrough []string `json:"env_passthrough"`

	// TasksDir is where task folders live, relative to the repository root.
	TasksDir string `json:"tasks_dir"`
	// SnapshotExclude lists extra paths, relative to the project, that snapshots skip.
	// Watts' own files, the tasks folder, .git, node_modules and .venv are always skipped.
	SnapshotExclude []string `json:"snapshot_exclude,omitempty"`

	Limits          Limits                    `json:"limits"`
	Providers       map[string]Provider       `json:"providers,omitempty"`
	SkillsDirs      []string                  `json:"skills_dirs,omitempty"`
	PiSettings      map[string]any            `json:"pi_settings,omitempty"`
	ForbiddenPaths  []string                  `json:"forbidden_paths,omitempty"`
	Agents          map[string]AgentConfig    `json:"agents,omitempty"`
	DefaultWorkflow string                    `json:"default_workflow,omitempty"`
	MCPs            map[string]MCPServer      `json:"mcps,omitempty"`
	Capabilities    map[string]Capability     `json:"capabilities,omitempty"`
	Workflow        *orchestration.Definition `json:"workflow,omitempty"`
	Temporal        *TemporalSettings         `json:"temporal,omitempty"`
}

// Limits cap a run. Zero means no limit.
type Limits struct {
	// MaxMinutes is the wall-clock limit for one Pi run.
	MaxMinutes int `json:"max_minutes"`
	// MaxAttempts is how many times one stage may be started on one task, counting retries.
	MaxAttempts int `json:"max_attempts"`
}

// Provider declares a model provider so Watts can write models.json for it. The key is always
// named, never stored: APIKeyEnv names a variable in your shell, and APIKeyCommand is run by
// Watts outside the sandbox at launch (for example to mint a short-lived key). The command's
// output is handed to Pi in memory and is never written to disk.
type Provider struct {
	Name          string          `json:"name,omitempty"`
	BaseURL       string          `json:"base_url"`
	API           string          `json:"api,omitempty"`
	APIKeyEnv     string          `json:"api_key_env,omitempty"`
	APIKeyCommand []string        `json:"api_key_command,omitempty"`
	Models        []ProviderModel `json:"models,omitempty"`
}

type ProviderModel struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	Reasoning     bool   `json:"reasoning,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`
	MaxTokens     int    `json:"max_tokens,omitempty"`
}

// AgentConfig defines agent directories, skills and execution settings. The built-in
// agents are build and review. Workflow stages select each agent’s provider and model.
type AgentConfig struct {
	Thinking string `json:"thinking,omitempty"`
	// Path is the agent directory (Pi's PI_CODING_AGENT_DIR), relative to the repository root.
	// Empty means .watts/pi-agent-<name>.
	Path           string         `json:"path,omitempty"`
	SkillsDirs     []string       `json:"skills_dirs,omitempty"`
	PiSettings     map[string]any `json:"pi_settings,omitempty"`
	ForbiddenPaths []string       `json:"forbidden_paths,omitempty"`
}

// Capability names a project-configured executable plugin. It reads JSON on stdin and returns JSON on stdout.
type Capability struct {
	Command []string `json:"command"`
}

// MCPServer defines a Pi-compatible server connection; stages opt in by name.
type MCPServer struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	URL         string            `json:"url,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Exposure    string            `json:"exposure,omitempty"`
	Description string            `json:"description,omitempty"`
}

type TemporalSettings struct {
	Address   string `json:"address,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	TaskQueue string `json:"task_queue,omitempty"`
}

const (
	ConfigVersion = 1
	Filename      = "watts.json"

	DefaultTasksDir = "tasks"
)

func Default() Config {
	return Config{
		Version:        ConfigVersion,
		Providers:      map[string]Provider{"ollama": {BaseURL: "http://localhost:11434/v1", API: "openai-completions"}, "openrouter": {BaseURL: "https://openrouter.ai/api/v1", API: "openai-completions", APIKeyEnv: "OPENROUTER_API_KEY", Models: []ProviderModel{{ID: "deepseek/deepseek-v4-flash", Reasoning: true, ContextWindow: 1048576, MaxTokens: 16384}}}},
		Agents:         map[string]AgentConfig{"build": {Thinking: "medium"}, "review": {Thinking: "medium"}},
		EnvPassthrough: []string{},
		TasksDir:       DefaultTasksDir,
	}
}

var (
	ReProvider = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	ReModel    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]*$`)
	ReURL      = regexp.MustCompile(`^https?://[^\s]+$`)
	ReEnvName  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	ReAgent    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	ThinkingLevels = []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

	// Names the wrapper sets itself. They cannot be passed through.
	ReservedEnv = map[string]bool{
		"HOME": true, "PATH": true, "TERM": true, "LANG": true,
		"PI_CODING_AGENT_DIR": true, "PI_SKIP_VERSION_CHECK": true, "PI_TELEMETRY": true, "PI_OFFLINE": true,
		"WATTS_ROLE": true, "WATTS_STEP_INPUT": true, "WATTS_STEP_RESULT": true, "WATTS_TASK": true, "WATTS_REPO": true,
	}
)

// builtinAgents are always present.
var BuiltinAgents = []string{"build", "review"}

// ResolvedAgent is an agent with every default filled in.
type ResolvedAgent struct {
	Thinking   string
	Name       string
	Path       string // relative to the repository root
	Provider   string
	Model      string
	Identity   string
	SkillsDirs []string
	PiSettings map[string]any
	Forbidden  []string
	Builtin    bool
}

// agentNames lists the builtin agents first, then custom ones in name order.
func (c Config) AgentNames() []string {
	names := append([]string(nil), BuiltinAgents...)
	var extra []string
	for n := range c.Agents {
		if !Contains(BuiltinAgents, n) {
			extra = append(extra, n)
		}
	}
	sort.Strings(extra)
	return append(names, extra...)
}

// agent resolves one agent. The second result is false for an unknown name.
func (c Config) Agent(name string) (ResolvedAgent, bool) {
	ov, custom := c.Agents[name]
	if !Contains(BuiltinAgents, name) && !custom {
		return ResolvedAgent{}, false
	}
	r := ResolvedAgent{Name: name, Builtin: Contains(BuiltinAgents, name)}
	r.Path = filepath.ToSlash(filepath.Join(".watts", "pi-agent-"+name))
	r.Thinking, r.Identity = "medium", name
	definition := c.AgentWorkflow()
	for _, step := range definition.Steps {
		if step.Agent == name {
			r.Provider, r.Model = step.Provider, step.Model
			if step.Identity != "" {
				r.Identity = step.Identity
			}
			break
		}
	}
	if ov.Path != "" {
		r.Path = filepath.ToSlash(filepath.Clean(ov.Path))
	}
	if ov.Thinking != "" {
		r.Thinking = ov.Thinking
	}
	r.SkillsDirs = append(append([]string(nil), c.SkillsDirs...), ov.SkillsDirs...)
	r.PiSettings = map[string]any{}
	for k, v := range c.PiSettings {
		r.PiSettings[k] = v
	}
	for k, v := range ov.PiSettings {
		r.PiSettings[k] = v
	}
	r.Forbidden = append(append(append([]string(nil), DefaultForbidden...), c.ForbiddenPaths...), ov.ForbiddenPaths...)
	return r, true
}

// defaultForbidden are the paths no agent may change: Watts' own configuration and check scripts.
// A run that changes any of them is reported as failed.
var DefaultForbidden = []string{Filename, "workflows", ".watts/kit", ".git"}

// passthroughNames is every variable name that is copied into an agent's environment: the
// top-level list, the extras given on the command line, and the key variable of the agent's own
// provider (so declaring a provider is enough, and nobody has to remember a second list).
func (c Config) PassthroughNames(agent ResolvedAgent, extra []string) []string {
	var out []string
	add := func(n string) {
		if n != "" && !Contains(out, n) {
			out = append(out, n)
		}
	}
	for _, n := range c.EnvPassthrough {
		add(n)
	}
	for _, n := range extra {
		add(n)
	}
	if p, ok := c.Providers[agent.Provider]; ok {
		add(p.APIKeyEnv)
	}
	return out
}

// providerKeyVar is the variable Watts fills from api_key_command, named after the provider.
func ProviderKeyVar(provider string) string {
	var b strings.Builder
	b.WriteString("WATTS_APIKEY_")
	for _, r := range strings.ToUpper(provider) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func ValidTasksDir(d string) error {
	if d == "" {
		return errors.New("tasks_dir must not be empty")
	}
	if filepath.IsAbs(d) || strings.HasPrefix(d, "/") || strings.Contains(d, `\`) {
		return fmt.Errorf("tasks_dir %q must be a relative path with forward slashes", d)
	}
	if c := filepath.ToSlash(filepath.Clean(d)); c != d || d == "." || d == ".." || strings.HasPrefix(d, "../") {
		return fmt.Errorf("tasks_dir %q must be a clean path inside the repository (no .., no trailing slash)", d)
	}
	if d == "workflows" || strings.HasPrefix(d, "workflows/") {
		return errors.New("tasks_dir cannot overlap the project workflows directory")
	}
	if d == ".watts" || strings.HasPrefix(d, ".watts/") || d == ".git" || strings.HasPrefix(d, ".git/") {
		return fmt.Errorf("tasks_dir %q cannot be inside .watts or .git: .watts is disposable and rebuilt from %s", d, Filename)
	}
	if strings.ContainsAny(d, " \t\n\"'$`;|&<>*?#:") {
		return fmt.Errorf("tasks_dir %q contains characters the check scripts cannot handle", d)
	}
	return nil
}

func ValidRelPath(what, p string) error {
	if p == "" || filepath.IsAbs(p) || strings.HasPrefix(p, "..") || strings.Contains(p, `\`) {
		return fmt.Errorf("%s %q must be a relative path inside the repository", what, p)
	}
	return nil
}

func (c Config) Validate() error {
	var errs []string
	add := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	if c.Version != ConfigVersion {
		add("version must be %d, found %d", ConfigVersion, c.Version)
	}
	for _, n := range c.EnvPassthrough {
		switch {
		case !ReEnvName.MatchString(n):
			add("env_passthrough: %q is not a valid environment variable name", n)
		case ReservedEnv[n]:
			add("env_passthrough: %s is set by Watts itself and cannot be passed through", n)
		}
	}
	if err := ValidTasksDir(c.TasksDir); err != nil {
		add("%v", err)
	}
	for _, p := range c.SnapshotExclude {
		if err := ValidRelPath("snapshot_exclude", p); err != nil || strings.ContainsAny(p, "\n\"'") {
			add("snapshot_exclude %q must be a relative path inside the project", p)
		}
	}
	if c.Limits.MaxMinutes < 0 || c.Limits.MaxAttempts < 0 {
		add("limits cannot be negative")
	}
	for _, p := range append(append([]string(nil), c.SkillsDirs...), c.ForbiddenPaths...) {
		if err := ValidRelPath("path", p); err != nil {
			add("%v", err)
		}
	}

	for _, name := range SortedKeys(c.Providers) {
		p := c.Providers[name]
		switch {
		case !ReProvider.MatchString(name):
			add("providers: %q is not a valid provider name", name)
			continue
		case !ReURL.MatchString(p.BaseURL):
			add("providers.%s.base_url %q must be an http or https URL", name, p.BaseURL)
		case p.APIKeyEnv != "" && len(p.APIKeyCommand) > 0:
			add("providers.%s: set api_key_env or api_key_command, not both", name)
		case p.APIKeyEnv != "" && !ReEnvName.MatchString(p.APIKeyEnv):
			add("providers.%s.api_key_env %q is not a valid environment variable name", name, p.APIKeyEnv)
		case p.APIKeyEnv != "" && ReservedEnv[p.APIKeyEnv]:
			add("providers.%s.api_key_env: %s is set by Watts itself", name, p.APIKeyEnv)
		case len(p.Models) == 0 && name != "ollama":
			add("providers.%s.models must list at least one model", name)
		}
		for _, a := range p.APIKeyCommand {
			if a == "" {
				add("providers.%s.api_key_command has an empty argument", name)
				break
			}
		}
		for _, m := range p.Models {
			if !ReModel.MatchString(m.ID) {
				add("providers.%s: %q is not a valid model id", name, m.ID)
			}
		}
	}

	for _, name := range SortedKeys(c.Agents) {
		a := c.Agents[name]
		if !ReAgent.MatchString(name) {
			add("agents: %q must be lowercase letters, digits and dashes, starting with a letter", name)
			continue
		}
		if a.Path != "" {
			if err := ValidRelPath("agents."+name+".path", a.Path); err != nil {
				add("%v", err)
			} else if cl := filepath.ToSlash(filepath.Clean(a.Path)); cl == ".watts" || cl == ".." {
				add("agents.%s.path %q is not a usable directory", name, a.Path)
			}
		}
		if a.Thinking != "" && !Contains(ThinkingLevels, a.Thinking) {
			add("agents.%s.thinking must be one of %s", name, strings.Join(ThinkingLevels, " "))
		}
		for _, p := range append(append([]string(nil), a.SkillsDirs...), a.ForbiddenPaths...) {
			if err := ValidRelPath("agents."+name+" path", p); err != nil {
				add("%v", err)
			}
		}
	}

	// Every agent must resolve, use a model its declared provider actually offers, and keep its
	// directory apart from the others.
	dirs := map[string]string{}
	for _, name := range c.AgentNames() {
		r, _ := c.Agent(name)
		if prev, dup := dirs[r.Path]; dup {
			add("agents %s and %s share the directory %s", prev, name, r.Path)
		}
		dirs[r.Path] = name

	}
	if c.DefaultWorkflow != "" && (filepath.IsAbs(c.DefaultWorkflow) || strings.ContainsAny(c.DefaultWorkflow, "\x00\n\r") || strings.HasPrefix(filepath.Clean(c.DefaultWorkflow), "..")) {
		add("invalid default_workflow %q", c.DefaultWorkflow)
	}
	if c.DefaultWorkflow != "" && c.Workflow != nil {
		add("choose default_workflow or an inline workflow, not both")
	}
	for name, server := range c.MCPs {
		if !orchestration.ValidName(name) || (server.Command == "") == (server.URL == "") {
			add("mcp %q requires a valid name and exactly one of command or url", name)
		}
		if server.URL != "" && (!ReURL.MatchString(server.URL) || server.Cwd != "" || len(server.Args) > 0 || len(server.Env) > 0) {
			add("mcp %q has invalid HTTP connection settings", name)
		}
		if server.Command != "" && len(server.Headers) > 0 {
			add("mcp %q: headers require an HTTP connection", name)
		}
		if server.Cwd != "" && server.Cwd != "." {
			if err := ValidRelPath("mcp cwd", server.Cwd); err != nil {
				add("%v", err)
			}
		}
		switch server.Exposure {
		case "", "direct", "codemode", "deferred", "hidden":
		default:
			add("mcp %q has invalid exposure", name)
		}
	}
	for name, capability := range c.Capabilities {
		if !orchestration.ValidName(name) || len(capability.Command) == 0 || strings.TrimSpace(capability.Command[0]) == "" {
			add("capability %q requires a valid name and command", name)
		}
		for _, argument := range capability.Command {
			if strings.ContainsRune(argument, '\x00') {
				add("capability %q contains a NUL command argument", name)
			}
		}
	}
	if c.Workflow != nil {
		if err := c.ValidateWorkflow(*c.Workflow); err != nil {
			add("%s", err)
		}
	}
	if c.Temporal != nil {
		if strings.ContainsAny(c.Temporal.Address+c.Temporal.Namespace+c.Temporal.TaskQueue, "\x00\n\r") {
			add("temporal settings contain invalid characters")
		}
	}
	if len(errs) > 0 {
		return errors.New("invalid config: " + strings.Join(errs, "; "))
	}
	return nil
}

func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var ConfigKeys = []string{
	"env_passthrough", "offline",
	"tasks_dir", "max_minutes", "max_attempts",
}

// set changes one key. For env_passthrough, values may be given as separate words or a comma
// separated list, and no values clears the list.
func (c *Config) Set(key string, vals []string) error {
	one := func() (string, error) {
		if len(vals) != 1 {
			return "", fmt.Errorf("%s takes exactly one value", key)
		}
		return vals[0], nil
	}
	boolean := func(dst *bool) error {
		v, err := one()
		if err != nil {
			return err
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("%s must be true or false, found %q", key, v)
		}
		*dst = b
		return nil
	}
	number := func(dst *int) error {
		v, err := one()
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return fmt.Errorf("%s must be a whole number of zero or more, found %q", key, v)
		}
		*dst = n
		return nil
	}
	switch key {
	case "env_passthrough":
		var names []string
		for _, v := range vals {
			for _, n := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
				if !Contains(names, n) {
					names = append(names, n)
				}
			}
		}
		if names == nil {
			names = []string{}
		}
		c.EnvPassthrough = names
	case "offline":
		if err := boolean(&c.Offline); err != nil {
			return err
		}
	case "max_minutes":
		if err := number(&c.Limits.MaxMinutes); err != nil {
			return err
		}
	case "max_attempts":
		if err := number(&c.Limits.MaxAttempts); err != nil {
			return err
		}
	default:
		v, err := one()
		if err != nil {
			return err
		}
		switch key {
		case "default_workflow":
			c.DefaultWorkflow = v
		case "tasks_dir":
			c.TasksDir = strings.TrimSuffix(v, "/")
		default:
			keys := append([]string(nil), ConfigKeys...)
			sort.Strings(keys)
			return fmt.Errorf("unknown key %q. Keys: %s", key, strings.Join(keys, ", "))
		}
	}
	return c.Validate()
}

func Contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
