package arguments

import (
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSeparatesFlagsAndVerbatimArguments(t *testing.T) {
	flags := NewFlags(io.Discard, "test")
	agent := flags.String("agent", "build", "")
	positionals, err := Parse(flags, []string{"task-folder", "--agent", "review", "other", "--", "--agent", "untouched", "--"})
	require.NoError(t, err)
	assert.Equal(t, "review", *agent,
		"agent = %q", *agent)

	expected := []string{"task-folder", "other", "--agent", "untouched", "--"}
	assert.True(t, reflect.DeepEqual(positionals, expected),
		"arguments = %q, want %q", positionals, expected)
}
