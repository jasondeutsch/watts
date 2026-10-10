package settings

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

func Show(a *terminal.Context, args []string) error {
	fs := arguments.NewFlags(a.ErrorOutput, "config show")
	resolved := fs.Bool("resolved", false, "show every agent with its defaults filled in, and where each value came from")
	if _, err := arguments.Parse(fs, args); err != nil {
		return arguments.UsageError(err)
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	if !*resolved {
		data, err := os.ReadFile(a.ConfigPath())
		if err != nil {
			return err
		}
		a.Say("%s", data)
		return nil
	}
	type view struct {
		Name       string            `json:"name"`
		Path       string            `json:"path"`
		Provider   string            `json:"provider"`
		Model      string            `json:"model"`
		Thinking   string            `json:"thinking"`
		SkillsDirs []string          `json:"skills_dirs"`
		Passthru   []string          `json:"env_passthrough"`
		Forbidden  []string          `json:"forbidden_paths"`
		From       map[string]string `json:"from"`
	}
	var views []view
	for _, n := range cfg.AgentNames() {
		r, _ := cfg.Agent(n)
		ov := cfg.Agents[n]
		from := func(over string) string {
			if over != "" {
				return "agents." + n
			}
			return "top level or built-in default"
		}
		views = append(views, view{n, r.Path, r.Provider, r.Model, r.Thinking, r.SkillsDirs, cfg.PassthroughNames(r, nil), r.Forbidden,
			map[string]string{"path": from(ov.Path), "provider": "workflow", "model": "workflow", "thinking": func() string {
				if ov.Thinking != "" {
					return "agents." + n
				}
				return "built-in default"
			}()}})
	}
	data, _ := terminal.JSON(map[string]any{"tasks_dir": cfg.TasksDir, "limits": cfg.Limits, "agents": views})
	a.Say("%s", data)
	return nil
}

func Set(a *terminal.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: watts config set <key> <value>   keys: %s", strings.Join(projectconfig.ConfigKeys, ", "))
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	if err := cfg.Set(args[0], args[1:]); err != nil {
		return err
	}
	if err := a.SaveConfig(cfg); err != nil {
		return err
	}
	a.Say("saved %s. Run: watts config apply   to apply it to the agent directories", a.Rel(a.ConfigPath()))
	return nil
}

func Apply(a *terminal.Context, args []string) error {
	fs := arguments.NewFlags(a.ErrorOutput, "config apply")
	force := fs.Bool("force", false, "regenerate models.json even if you edited it")
	if _, err := arguments.Parse(fs, args); err != nil {
		return arguments.UsageError(err)
	}
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	return a.ConfigAgents(cfg, *force)
}

// Env lists, adds to, or removes from env_passthrough.
func Env(a *terminal.Context, args []string) error {
	cfg, err := a.LoadConfig()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		if len(cfg.EnvPassthrough) == 0 {
			a.Say("env_passthrough is empty. Pi receives no variable from your shell except those named by a provider's api_key_env.")
		}
		for _, n := range cfg.EnvPassthrough {
			state := "set"
			if v, ok := os.LookupEnv(n); !ok || v == "" {
				state = "not set in your shell"
			}
			a.Say("%s (%s)", n, state)
		}
		return nil
	}
	if len(args) < 2 || (args[0] != "add" && args[0] != "remove") {
		return errors.New("usage: watts config env [add|remove] NAME...")
	}
	names := append([]string(nil), cfg.EnvPassthrough...)
	for _, v := range args[1:] {
		for _, n := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
			if args[0] == "add" && !projectconfig.Contains(names, n) {
				names = append(names, n)
			}
			if args[0] == "remove" {
				var kept []string
				for _, x := range names {
					if x != n {
						kept = append(kept, x)
					}
				}
				names = kept
			}
		}
	}
	cfg.EnvPassthrough = names
	if cfg.EnvPassthrough == nil {
		cfg.EnvPassthrough = []string{}
	}
	if err := a.SaveConfig(cfg); err != nil {
		return err
	}
	a.Say("env_passthrough is now: %s", strings.Join(cfg.EnvPassthrough, ", "))
	return nil
}
