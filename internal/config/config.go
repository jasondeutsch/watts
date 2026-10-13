// Package config holds worker preferences. Workload software and providers are
// declared in manifests, never selected by worker defaults.
package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jasondeutsch/watts/internal/orchestration"
)

type Config struct {
	Version         int                       `json:"version" yaml:"version"`
	TasksDir        string                    `json:"tasks_dir" yaml:"tasks_dir"`
	SnapshotExclude []string                  `json:"snapshot_exclude,omitempty" yaml:"snapshot_exclude,omitempty"`
	Limits          Limits                    `json:"limits" yaml:"limits"`
	Temporal        *TemporalSettings         `json:"temporal,omitempty" yaml:"temporal,omitempty"`
	Workflow        *orchestration.Definition `json:"-" yaml:"-"`
}
type Limits struct {
	MaxMinutes  int `json:"max_minutes" yaml:"max_minutes"`
	MaxAttempts int `json:"max_attempts" yaml:"max_attempts"`
}
type TemporalSettings struct {
	Address   string `json:"address,omitempty" yaml:"address,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	TaskQueue string `json:"task_queue,omitempty" yaml:"task_queue,omitempty"`
}

const ConfigVersion = 1
const Filename = "watts.acl"
const DefaultTasksDir = "tasks"

func Default() Config { return Config{Version: ConfigVersion, TasksDir: DefaultTasksDir} }
func Contains(values []string, value string) bool {
	for _, entry := range values {
		if entry == value {
			return true
		}
	}
	return false
}
func ValidRelPath(what, value string) error {
	if !orchestration.ValidArtifact(value) {
		return fmt.Errorf("%s %q must be a relative path inside the project", what, value)
	}
	return nil
}
func ValidTasksDir(value string) error {
	if err := ValidRelPath("tasks_dir", value); err != nil {
		return err
	}
	if value == ".watts" || strings.HasPrefix(value, ".watts/") || value == ".git" || strings.HasPrefix(value, ".git/") {
		return fmt.Errorf("tasks_dir cannot overlap private state")
	}
	return nil
}
func (config Config) Validate() error {
	if config.Version != ConfigVersion {
		return fmt.Errorf("unsupported settings version %d", config.Version)
	}
	if err := ValidTasksDir(config.TasksDir); err != nil {
		return err
	}
	if config.Limits.MaxMinutes < 0 || config.Limits.MaxAttempts < 0 {
		return fmt.Errorf("execution limits cannot be negative")
	}
	for _, value := range config.SnapshotExclude {
		if err := ValidRelPath("snapshot_exclude", value); err != nil {
			return err
		}
	}
	if config.Temporal != nil && strings.ContainsAny(config.Temporal.Address+config.Temporal.Namespace+config.Temporal.TaskQueue, "\x00\r\n") {
		return fmt.Errorf("invalid Temporal settings")
	}
	return nil
}
func (config Config) ValidateWorkflow(definition orchestration.Definition) error {
	return definition.Validate()
}
func SortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var ConfigKeys = []string{"tasks_dir", "max_minutes", "max_attempts"}

func (config *Config) Set(key string, values []string) error {
	if len(values) != 1 {
		return fmt.Errorf("%s takes one value", key)
	}
	if key == "tasks_dir" {
		if err := ValidTasksDir(values[0]); err != nil {
			return err
		}
		config.TasksDir = filepath.ToSlash(values[0])
		return nil
	}
	number, err := strconv.Atoi(values[0])
	if err != nil || number < 0 {
		return fmt.Errorf("%s must be a nonnegative integer", key)
	}
	switch key {
	case "max_minutes":
		config.Limits.MaxMinutes = number
	case "max_attempts":
		config.Limits.MaxAttempts = number
	default:
		return fmt.Errorf("unknown worker setting %q", key)
	}
	return nil
}
