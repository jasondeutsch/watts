package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWorkerPreferences(t *testing.T) {
	config := Default()
	require.NoError(t, config.Validate())
	require.NoError(t, config.Set("tasks_dir", []string{"work/tasks"}))
	require.NoError(t, config.Set("max_minutes", []string{"30"}))
	require.NoError(t, config.Set("max_attempts", []string{"3"}))
	require.Equal(t, "work/tasks", config.TasksDir)
	require.Equal(t, 30, config.Limits.MaxMinutes)
	require.Equal(t, 3, config.Limits.MaxAttempts)
	for _, test := range []struct{ key, value string }{{"tasks_dir", "../outside"}, {"tasks_dir", ".watts/tasks"}, {"max_minutes", "-1"}, {"max_attempts", "bad"}, {"provider", "anything"}} {
		require.Error(t, config.Set(test.key, []string{test.value}))
	}
}

func TestInvalidWorkerSettings(t *testing.T) {
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Version = 9 },
		func(c *Config) { c.RuntimeRetention = "unsafe" },
		func(c *Config) { c.TasksDir = "/outside" },
		func(c *Config) { c.SnapshotExclude = []string{"../outside"} },
		func(c *Config) { c.Limits.MaxMinutes = -1 },
		func(c *Config) { c.Temporal = &TemporalSettings{Address: "bad\naddress"} },
	} {
		config := Default()
		mutate(&config)
		require.Error(t, config.Validate())
	}
}
