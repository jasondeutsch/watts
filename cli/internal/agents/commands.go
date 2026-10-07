package agents

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/application"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

// Pi opens Pi interactively as one agent, in the same isolated environment a build uses.
func Pi(a *terminal.Context, args []string) error {
	fl := arguments.NewFlags(a.ErrorOutput, "agent pi")
	var rf application.RunOptions
	arguments.RunFlags(fl, &rf, true)
	rest, err := arguments.Parse(fl, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Agent(rf.Agent); !ok {
		return fmt.Errorf("unknown agent %q (defined agents: %s)", rf.Agent, strings.Join(cfg.AgentNames(), ", "))
	}
	// Interactive use is not blocked by a credential problem, but the user is told about it now
	// instead of finding out from Pi's own error.
	if fs := a.CredentialCheck(cfg, rf.Agent, rf.Pass); len(fs) > 0 {
		a.PrintFindings(fs)
	}
	return a.RunPi(cfg, rf.Agent, rf.Pass, rf.Offline, rest...)
}

func List(a *terminal.Context, args []string) error {
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	for _, n := range cfg.AgentNames() {
		r, _ := cfg.Agent(n)
		kind := "custom"
		if r.Builtin {
			kind = "built in"
		}
		a.Say("%-18s %-9s %s/%s  dir=%s", n, kind, r.Provider, r.Model, r.Path)
	}
	return nil
}

// Env prints the environment variable NAMES an agent receives, never values. It answers
// "why can't Pi see my variable" without a debugging session.
func Env(a *terminal.Context, args []string) error {
	fl := arguments.NewFlags(a.ErrorOutput, "agent env")
	var rf application.RunOptions
	arguments.RunFlags(fl, &rf, false)
	fl.BoolVar(&rf.JSONOutput, "json", false, "print JSON")
	pos, err := arguments.Parse(fl, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	name := "build"
	if len(pos) > 1 {
		return errors.New("usage: watts agent env [agent]")
	} else if len(pos) == 1 {
		name = pos[0]
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	entries, err := a.EnvReport(cfg, name, rf.Pass, rf.Offline)
	if err != nil {
		return err
	}
	if rf.JSONOutput {
		return terminal.Encoder(a.Output).Encode(map[string]any{"agent": name, "variables": entries})
	}
	a.Say("Environment of the %s agent. Watts starts Pi with nothing inherited from your shell, and", name)
	a.Say("only these names exist. Values are never shown.\n")
	for _, e := range entries {
		a.Say("  %-28s %s; %s", e.Name, e.Source, e.State)
	}
	a.Say("\nTo pass another variable: watts config env add NAME")
	return nil
}

func Follow(a *terminal.Context, args []string) error {
	fl := arguments.NewFlags(a.ErrorOutput, "agent follow")
	once := fl.Bool("once", false, "print what is there now and exit")
	tail := fl.Int("tail", 20, "how many recent events to show first (0 for all)")
	pos, err := arguments.Parse(fl, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	name := "build"
	if len(pos) > 1 {
		return errors.New("usage: watts agent follow [agent] [--once] [--tail N]")
	} else if len(pos) == 1 {
		name = pos[0]
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Agent(name); !ok {
		return fmt.Errorf("unknown agent %q (defined agents: %s)", name, strings.Join(cfg.AgentNames(), ", "))
	}
	stop := make(chan struct{})
	if !*once {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigc)
		go func() { <-sigc; close(stop) }()
		a.Say("Following the %s agent's session. Press Ctrl-C to stop.", name)
	}
	return a.Follow(cfg, name, time.Time{}, *once, *tail, stop)
}

func Clean(a *terminal.Context, args []string) error {
	fl := arguments.NewFlags(a.ErrorOutput, "agent clean")
	force := fl.Bool("force", false, "confirm the deletion")
	if _, err := arguments.Parse(fl, args); err != nil {
		return arguments.UsageError(err)
	}
	if !*force {
		return errors.New("this deletes the private agent directories (logins, installed packages, sessions). Re-run with --force")
	}
	removed, err := a.CleanAgents()
	if err != nil {
		return err
	}
	for _, path := range removed {
		a.Say("removed %s", path)
	}
	a.Say("Run: watts config apply   to set them up again. Your skills and settings come back from %s.", projectconfig.Filename)
	return nil
}

// New scaffolds a custom agent from the example shipped in the kit and registers it in
// watts.json.
func New(a *terminal.Context, args []string) error {
	fl := arguments.NewFlags(a.ErrorOutput, "agent new")
	dir := fl.String("path", "", "agent directory (default .watts.agents/<name>)")
	pos, err := arguments.Parse(fl, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	name, err := arguments.One(pos, "agent name")
	if err != nil {
		return err
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	target, err := a.CreateAgent(cfg, application.CreateAgentOptions{Name: name, Path: *dir})
	if err != nil {
		return err
	}
	a.Say("\nCreated the %s agent in %s and registered it under agents in %s.", name, target, projectconfig.Filename)
	a.Say("Next:")
	a.Say("  1. Edit %s/skills", target)
	a.Say("  2. Add a workflow step that uses it and declares provider and model. See %s/README.md and the example in %s/examples/custom-agent/watts.example.json", target, a.Rel(a.KitDir()))
	a.Say("  3. watts doctor")
	return nil
}
