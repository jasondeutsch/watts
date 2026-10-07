package setup

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/application"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	projectruntime "github.com/jasondeutsch/watts/internal/runtime"
)

func Init(a *terminal.Context, args []string) error {
	// Initialization targets the invocation directory, even inside another Watts project.
	a.Root = a.WorkingDirectory
	fs := arguments.NewFlags(a.ErrorOutput, "init")
	cfg := projectconfig.Default()
	existing, loadErr := a.LoadConfig()
	if loadErr == nil {
		cfg = existing
	}
	tasksDir := fs.String("tasks-dir", cfg.TasksDir, "where task folders live, relative to the repository")
	var pass arguments.StringList
	fs.Var(&pass, "env-passthrough", "environment variable name to pass into Pi (repeatable)")
	force := fs.Bool("force", false, "regenerate models.json")
	noInstall := fs.Bool("no-install", false, "do not install the pi-ralph-loop extension")
	if _, err := arguments.Parse(fs, args); err != nil {
		return arguments.UsageError(err)
	}
	Set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { Set[f.Name] = true })
	cfg.TasksDir = strings.TrimSuffix(*tasksDir, "/")
	if Set["env-passthrough"] {
		cfg.EnvPassthrough = []string(pass)
	}
	// A first init in a terminal asks where tasks should live; scripts and tests take the default.
	if loadErr != nil && !Set["tasks-dir"] && terminal.IsInteractive(a.Input) {
		cfg.TasksDir = a.Ask("Where should task folders live?", cfg.TasksDir)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := kit.Initialize(a.KitDir()); err != nil {
		return err
	}
	a.Say("Project assets initialized in %s", a.Rel(a.KitDir()))
	if err := a.SaveConfig(cfg); err != nil {
		return err
	}
	a.Say("config: %s", a.Rel(a.ConfigPath()))
	if err := a.WriteLocalIgnores(); err != nil {
		return err
	}
	if err := a.ConfigAgents(cfg, *force); err != nil {
		return err
	}

	if *noInstall {
		a.Say("\nSkipped the extension install. Run: watts install ralph")
	} else if application.PiVersion() == "" {
		a.Say("\npi is not installed yet. Run: watts install pi, then: watts install ralph")
	} else if err := a.InstallRalph(cfg, nil); err != nil {
		a.Warn("the extension install failed: %v", err)
		a.Say("If you are behind a proxy that re-signs TLS, run: watts config env add NODE_EXTRA_CA_CERTS HTTPS_PROXY, then: watts install ralph")
	}
	a.Say(`
Done. Tasks live in %s. Watts does not use git or any other version control, so it does not matter whether this project is tracked.
  1. Keep %s and project workflow template files; share them with your team.
     .watts and the tasks folder (%s) are optional: nothing in Watts depends on where they are
     stored, and deleting .watts and running watts init rebuilds it.
  2. watts doctor
  3. watts task new <slug>      start a task`, cfg.TasksDir, projectconfig.Filename, cfg.TasksDir)
	return nil
}

func InstallAll(a *terminal.Context, args []string) error {
	fs := arguments.NewFlags(a.ErrorOutput, "install all")
	var pass arguments.StringList
	fs.Var(&pass, "env-passthrough", "environment variable name to pass into Pi for the extension install (repeatable)")
	if _, err := arguments.Parse(fs, args); err != nil {
		return arguments.UsageError(err)
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	if err := a.InstallPi(cfg); err != nil {
		return err
	}
	if err := a.ConfigAgents(cfg, false); err != nil {
		return err
	}
	if err := a.InstallRalph(cfg, pass); err != nil {
		return err
	}
	if err := a.WriteLocalIgnores(); err != nil {
		return err
	}
	return a.Doctor(cfg, true)
}

func InstallPi(a *terminal.Context, args []string) error {
	cfg := projectconfig.Default()
	if a.Root != "" {
		if c, err := a.LoadConfig(); err == nil {
			cfg = c
		}
	}
	return a.InstallPi(cfg)
}

func InstallRalph(a *terminal.Context, args []string) error {
	fs := arguments.NewFlags(a.ErrorOutput, "install ralph")
	var pass arguments.StringList
	fs.Var(&pass, "env-passthrough", "environment variable name to pass into Pi, such as NODE_EXTRA_CA_CERTS (repeatable)")
	if _, err := arguments.Parse(fs, args); err != nil {
		return arguments.UsageError(err)
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	return a.InstallRalph(cfg, pass)
}

func Doctor(a *terminal.Context, args []string) error {
	fs := arguments.NewFlags(a.ErrorOutput, "doctor")
	live := fs.Bool("live", false, "also send one tiny completion to each agent's gateway, from outside the sandbox")
	asJSON := fs.Bool("json", false, "print the checks as JSON")
	if _, err := arguments.Parse(fs, args); err != nil {
		return arguments.UsageError(err)
	}
	cfg, err := a.LoadConfig()
	return a.DoctorWith(cfg, err == nil, terminal.DoctorOptions{Live: *live, JSON: *asJSON})
}

func Start(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "start")
	detached := flags.Bool("d", false, "run worker and web UI in the background")
	positional, err := arguments.Parse(flags, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return errors.New("usage: watts start [-d]")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return projectruntime.Start(ctx, app.Service, *detached)
}
func Stop(app *terminal.Context, args []string) error {
	if len(args) > 0 {
		return errors.New("usage: watts stop")
	}
	return projectruntime.Stop(context.Background(), app.Service)
}
