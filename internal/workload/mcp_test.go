package workload_test

import (
	"testing"

	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

func TestMCPPlatformIsTheWorkloadEnvironment(t *testing.T) {
	service := workload.MCPService{Requirements: workload.MCPRequirements{OS: []string{"linux"}, Arch: []string{"arm64", "amd64"}}}
	require.NoError(t, service.ValidatePlatform("linux", "arm64"))
	require.NoError(t, service.ValidatePlatform("linux", "amd64"))
	require.ErrorContains(t, service.ValidatePlatform("darwin", "arm64"), "requires os")
	require.ErrorContains(t, service.ValidatePlatform("linux", "386"), "requires arch")
}
