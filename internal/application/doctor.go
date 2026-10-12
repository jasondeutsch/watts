package application

import (
	"context"
	"os"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/containerengine"
)

type Check struct {
	Level  string `json:"level"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

func (app *Service) DoctorChecks(config projectconfig.Config, haveConfig bool) []Check {
	if !haveConfig || config.Workflow == nil {
		return []Check{{Level: "fail", Name: "no selected package", Fix: "select a package descriptor"}}
	}
	engine, err := containerengine.Connect()
	if err != nil {
		return []Check{{Level: "fail", Name: "container engine", Detail: err.Error()}}
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = engine.Check(ctx); err != nil {
		return []Check{{Level: "fail", Name: "container engine unavailable", Detail: err.Error(), Fix: "start the configured Docker Engine"}}
	}
	checks := []Check{{Level: "ok", Name: "container engine available"}}
	for _, step := range config.Workflow.Steps {
		if step.Container == nil {
			continue
		}
		if _, err = engine.Pin(ctx, step.Container.Image); err != nil {
			checks = append(checks, Check{Level: "fail", Name: step.Name + ": image", Detail: err.Error()})
		} else {
			checks = append(checks, Check{Level: "ok", Name: step.Name + ": image available"})
		}
		for _, source := range step.SecretEnvironment {
			if value, ok := os.LookupEnv(source); !ok || strings.TrimSpace(value) == "" {
				checks = append(checks, Check{Level: "fail", Name: step.Name + ": required secret source", Detail: source, Fix: "export the declared source before starting the worker"})
			}
		}
	}
	return checks
}
