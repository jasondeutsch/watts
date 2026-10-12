package manifest

import "github.com/jasondeutsch/watts/internal/workload"

func validEnvironment(name string) bool { return workload.EnvironmentTarget(name) }
