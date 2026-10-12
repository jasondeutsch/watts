package settings

import (
	"fmt"
	"os"

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
	data, _ := terminal.JSON(cfg)
	a.Say("%s", data)
	return nil
}

func Set(a *terminal.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: wctl config set <key> <value>   keys: %s", fmt.Sprint(projectconfig.ConfigKeys))
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
	a.Say("saved %s; submitted tasks retain their pinned settings", a.Rel(a.ServiceSettingsPath()))
	return nil
}
