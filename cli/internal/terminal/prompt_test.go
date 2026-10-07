package terminal

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptUsesAnswerOrDefault(t *testing.T) {
	for _, test := range []struct{ input, expected string }{{"my-tasks\n", "my-tasks"}, {"\n", "tasks"}, {"", "tasks"}} {
		t.Run(test.input, func(t *testing.T) {
			var output bytes.Buffer
			terminal := New(t.TempDir(), "0.1.0", strings.NewReader(test.input), &output, &bytes.Buffer{})
			{
				got := terminal.Ask("Where?", "tasks")
				assert.Equal(t, test.expected, got,
					"answer = %q, want %q", got, test.expected)
			}
			assert.Equal(t, "Where? [tasks]: ", output.String(),
				"prompt = %q", output.String())
		})
	}
	assert.False(t, IsInteractive(strings.NewReader("")),
		"a reader is not a terminal")
}
