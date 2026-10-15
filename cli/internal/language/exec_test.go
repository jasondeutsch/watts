package language

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("input failed") }

func TestRequestSourcesPreserveMultilineText(t *testing.T) {
	text := strings.Repeat("One paragraph.\n\n", 200000) + "No final newline"
	filename := filepath.Join(t.TempDir(), "request.md")
	require.NoError(t, os.WriteFile(filename, []byte(text), 0600))
	for _, source := range []string{filename, "-"} {
		actual, err := requestText("", source, false, true, strings.NewReader(text))
		require.NoError(t, err)
		require.Equal(t, text, actual)
	}
	_, err := requestText("", filename, true, true, nil)
	require.ErrorContains(t, err, "cannot be combined")
	_, err = requestText("", filename+"-missing", false, true, nil)
	require.ErrorContains(t, err, "read request from")
	_, err = requestText("", "-", false, true, brokenReader{})
	require.ErrorContains(t, err, "input failed")
	_, err = requestText("", "-", false, true, strings.NewReader(""))
	require.ErrorContains(t, err, "empty")
}
