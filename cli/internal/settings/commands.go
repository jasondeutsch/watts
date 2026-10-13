package settings

import (
	"fmt"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

func Show(a *terminal.Context, args []string) error {
	fs := arguments.NewFlags(a.ErrorOutput, "config show")
	if _, err := arguments.Parse(fs, args); err != nil {
		return arguments.UsageError(err)
	}
	cfg, err := a.LoadServiceConfig()
	if err != nil {
		return err
	}
	data, err := terminal.JSON(cfg)
	if err != nil {
		return err
	}
	a.Say("%s", data)
	return nil
}

func Set(a *terminal.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: wctl config set <key> <value>   keys: %s", fmt.Sprint(projectconfig.ConfigKeys))
	}
	cfg, err := a.LoadServiceConfig()
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
