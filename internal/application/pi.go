package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// agentDir is where an agent's Pi state lives. It is .watts/pi-agent-<name> unless the
// configuration points the agent somewhere else.
func (a *Service) AgentDir(cfg projectconfig.Config, role string) string {
	if agent, ok := cfg.Agent(role); ok {
		return filepath.Join(a.Root, filepath.FromSlash(agent.Path))
	}
	return a.Wd("pi-agent-" + role)
}

// homeDir is the private HOME of an agent. It is always under .watts, so it is disposable.
func (a *Service) HomeDir(role string) string { return a.Wd("home-" + role) }

func (a *Service) Identity(cfg projectconfig.Config, role string) string {
	if r, ok := cfg.Agent(role); ok {
		return r.Identity
	}
	return role
}

func ValidAgentName(role string) bool { return role != "" && projectconfig.ReAgent.MatchString(role) }

// piEnv builds the complete environment for a Pi process. It is the same idea as `env -i`: nothing
// is inherited, so API keys in your shell, your real HOME and your normal Pi login never reach the
// agent. Only the names in cfg.Pass and extraPass are copied across, if they are set.
func (a *Service) PiEnv(cfg projectconfig.Config, role string, extraPass []string, offline bool) ([]string, error) {
	return a.BuildEnv(cfg, role, extraPass, offline, true)
}

// buildEnv is piEnv. With secrets false it does not run a provider's api_key_command, which is
// what a dry run needs.
func (a *Service) BuildEnv(cfg projectconfig.Config, role string, extraPass []string, offline, secrets bool) ([]string, error) {
	agent, ok := cfg.Agent(role)
	if !ValidAgentName(role) || !ok {
		return nil, fmt.Errorf("unknown agent %q (defined agents: %s)", role, strings.Join(cfg.AgentNames(), ", "))
	}
	term := os.Getenv("TERM")
	if term == "" {
		term = "xterm-256color"
	}
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "en_US.UTF-8"
	}
	env := []string{
		"WATTS_ROLE=" + role,
		"HOME=" + a.HomeDir(role),
		"PATH=" + os.Getenv("PATH"),
		"TERM=" + term,
		"LANG=" + lang,
		"PI_CODING_AGENT_DIR=" + a.AgentDir(cfg, role),
		"PI_SKIP_VERSION_CHECK=1",
		"PI_TELEMETRY=0",
	}
	if cfg.Offline || offline {
		env = append(env, "PI_OFFLINE=1")
	}
	seen := map[string]bool{}
	for _, n := range cfg.PassthroughNames(agent, extraPass) {
		if !projectconfig.ReEnvName.MatchString(n) {
			return nil, fmt.Errorf("invalid environment variable name to pass: %q", n)
		}
		if projectconfig.ReservedEnv[n] {
			return nil, fmt.Errorf("%s is set by Watts itself and cannot be passed through", n)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		if v, ok := os.LookupEnv(n); ok {
			env = append(env, n+"="+v)
		}
	}
	// A provider that mints its key with a command gets it here, in memory, outside the sandbox.
	if p, ok := cfg.Providers[agent.Provider]; ok && len(p.APIKeyCommand) > 0 && secrets {
		key, err := RunKeyCommand(p.APIKeyCommand)
		if err != nil {
			return nil, fmt.Errorf("providers.%s.api_key_command: %w", agent.Provider, err)
		}
		env = append(env, projectconfig.ProviderKeyVar(agent.Provider)+"="+key)
	}
	return env, nil
}

// runKeyCommand runs a provider's api_key_command and returns its output without the trailing
// newline. The output is a secret: it is never logged and never written to disk.
func RunKeyCommand(argv []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return "", fmt.Errorf("the command failed: %v %s", err, msg)
	}
	key := strings.TrimRight(stdout.String(), "\r\n")
	if strings.TrimSpace(key) == "" {
		return "", errors.New("the command printed no key")
	}
	return key, nil
}

func PiBinary() (string, error) {
	name := os.Getenv("WATTS_PI_BIN")
	if name == "" {
		name = "pi"
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", errors.New("pi is not installed. Run: watts install pi")
	}
	return p, nil
}

func (a *Service) RequireAgentDir(cfg projectconfig.Config, role string) error {
	if _, err := os.Stat(a.AgentDir(cfg, role)); err != nil {
		return fmt.Errorf("the %s agent directory is missing. Run: watts config apply", role)
	}
	return nil
}

func IsPrintMode(args []string) bool {
	for _, x := range args {
		if x == "-p" || x == "--print" {
			return true
		}
	}
	return false
}

// newPiCmd prepares a Pi process for a role. Print mode (-p) reads stdin when it is not a terminal
// and waits for it to close, so a loop started from a script would hang. Non-interactive runs
// therefore get an empty stdin.
func (a *Service) NewPiCmd(cfg projectconfig.Config, role string, extraPass []string, offline bool, args ...string) (*exec.Cmd, error) {
	bin, err := PiBinary()
	if err != nil {
		return nil, err
	}
	if err := a.RequireAgentDir(cfg, role); err != nil {
		return nil, err
	}
	env, err := a.PiEnv(cfg, role, extraPass, offline)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = a.Root
	cmd.Env = env
	if !IsPrintMode(args) {
		cmd.Stdin = a.Input
	}
	return cmd, nil
}

func (a *Service) RunPi(cfg projectconfig.Config, role string, extraPass []string, offline bool, args ...string) error {
	cmd, err := a.NewPiCmd(cfg, role, extraPass, offline, args...)
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = a.Output, a.ErrorOutput
	err = workspace.RunCommandTimeout(cmd, a.RunLimit(cfg))
	if err != nil && !IsPrintMode(args) {
		return err
	}
	if err != nil {
		var ee workspace.ExitError
		if errors.As(err, &ee) && ee.Code != 124 {
			a.warn("pi exited with status %d. If it reported \"No API key found\" or a 401, run: watts doctor", ee.Code)
		}
	}
	return err
}

// runLimit is the wall-clock limit for one Pi run, from limits.max_minutes.
func (a *Service) RunLimit(cfg projectconfig.Config) time.Duration {
	if cfg.Limits.MaxMinutes <= 0 {
		return 0
	}
	unit := a.TimeoutUnit
	if unit == 0 {
		unit = time.Minute
	}
	return time.Duration(cfg.Limits.MaxMinutes) * unit
}

func (a *Service) CapturePi(cfg projectconfig.Config, role string, args ...string) (string, error) {
	cmd, err := a.NewPiCmd(cfg, role, nil, false, args...)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err = workspace.RunCommand(cmd)
	return buf.String(), err
}

// piVersion reports the installed Pi version, or "" if Pi is missing or unreadable.
func PiVersion() string {
	bin, err := PiBinary()
	if err != nil {
		return ""
	}
	cmd := exec.Command(bin, "--version")
	cmd.Env = append(os.Environ(), "PI_OFFLINE=1", "PI_SKIP_VERSION_CHECK=1", "PI_TELEMETRY=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return FindVersion(string(out))
}

func NodeVersion() string {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return ""
	}
	return FindVersion(string(out))
}

func LookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (a *Service) AgentRel(cfg projectconfig.Config, role string) string {
	return filepath.ToSlash(a.Rel(a.AgentDir(cfg, role)))
}

func TrimLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}
