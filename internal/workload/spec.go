// Package workload defines execution contracts independent of agents and tools.
package workload

import (
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"
)

type Spec struct {
	ResourceHashes map[string]string `json:"resource_hashes,omitempty" yaml:"-"`
	Image          string            `json:"image" yaml:"image"`
	Command        []string          `json:"command" yaml:"command"`
	WorkingDir     string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	User           string            `json:"user,omitempty" yaml:"user,omitempty"`
	Network        string            `json:"network,omitempty" yaml:"network,omitempty"`
	Workspace      string            `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Result         string            `json:"result,omitempty" yaml:"result,omitempty"`
	MemoryMB       int64             `json:"memory_mb,omitempty" yaml:"memory_mb,omitempty"`
	CPUs           float64           `json:"cpus,omitempty" yaml:"cpus,omitempty"`
	Files          map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	Resources      map[string]string `json:"resources,omitempty" yaml:"resources,omitempty"`
}

func Relative(name string) bool {
	return name != "" && name != "." && !path.IsAbs(name) && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\\\x00")
}

func (spec Spec) Validate() error {
	if strings.TrimSpace(spec.Image) == "" || strings.ContainsAny(spec.Image, "\x00\r\n ") {
		return fmt.Errorf("container needs an image reference")
	}
	if len(spec.Command) == 0 || strings.TrimSpace(spec.Command[0]) == "" {
		return fmt.Errorf("container needs an explicit command array")
	}
	for _, arg := range spec.Command {
		if strings.ContainsRune(arg, 0) {
			return fmt.Errorf("container command contains NUL")
		}
	}
	switch spec.Network {
	case "", "none", "bridge":
	default:
		return fmt.Errorf("container network must be none or bridge")
	}
	switch spec.Workspace {
	case "", "read-only", "write":
	default:
		return fmt.Errorf("container workspace must be read-only or write")
	}
	switch spec.Result {
	case "", "file", "exit-code":
	default:
		return fmt.Errorf("container result must be file or exit-code")
	}
	if spec.MemoryMB < 0 || spec.MemoryMB > math.MaxInt64/(1024*1024) || spec.CPUs < 0 || math.IsNaN(spec.CPUs) || math.IsInf(spec.CPUs, 0) || spec.CPUs > float64(math.MaxInt64)/1e9 {
		return fmt.Errorf("container resource limits cannot be negative")
	}
	user := strings.Split(spec.User, ":")[0]
	uid, numeric := strconv.ParseUint(user, 10, 32)
	if user == "root" || numeric == nil && uid == 0 || strings.ContainsAny(spec.User, "\x00\r\n ") {
		return fmt.Errorf("container user must be non-root")
	}
	if err := ValidateWorkingDirectory(spec.WorkingDir); err != nil {
		return err
	}
	for name, contents := range spec.Files {
		if !Relative(name) || strings.ContainsRune(contents, 0) {
			return fmt.Errorf("invalid container file %q", name)
		}
	}
	for name, source := range spec.Resources {
		if !Relative(name) || !Relative(source) {
			return fmt.Errorf("invalid container resource binding %q", name)
		}
	}
	return nil
}
