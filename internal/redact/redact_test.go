package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamingEverySplitAndOverlappingValues(t *testing.T) {
	secret := "synthetic-key-123"
	input := "diagnostic " + secret + " / " + secret + " done"
	expected := "diagnostic [REDACTED] / [REDACTED] done"
	set := New(secret, "synthetic-key")
	for split := 0; split <= len(input); split++ {
		var output bytes.Buffer
		stream := set.Writer(&output)
		_, err := stream.Write([]byte(input[:split]))
		require.NoError(t, err)
		_, err = stream.Write([]byte(input[split:]))
		require.NoError(t, err)
		require.NoError(t, stream.Flush())
		require.Equal(t, expected, output.String(), "split %d", split)
	}
	var output bytes.Buffer
	stream := set.Writer(&output)
	for _, b := range []byte(input) {
		_, err := stream.Write([]byte{b})
		require.NoError(t, err)
	}
	require.NoError(t, stream.Flush())
	require.Equal(t, expected, output.String())
}
func TestEscapedValuesAndErrors(t *testing.T) {
	secret := "synthetic\"key\\line\n<&>"
	encoded, err := json.Marshal(secret)
	require.NoError(t, err)
	set := New(secret)
	for _, text := range []string{secret, string(encoded[1 : len(encoded)-1]), url.QueryEscape(secret), url.PathEscape(secret)} {
		require.Equal(t, Marker, set.Text(text))
	}
	original := fmt.Errorf("provider failure: %w", errors.New(secret))
	sanitized := set.Error(original)
	require.NotContains(t, sanitized.Error(), secret)
	require.Nil(t, errors.Unwrap(sanitized), "SDK must not serialize the raw cause")
	unchanged := io.EOF
	require.Same(t, unchanged, set.Error(unchanged))
	require.Equal(t, "ordinary error", set.Text("ordinary error"))
}
func TestWriterFailureCannotDiscloseDestinationError(t *testing.T) {
	stream := New("synthetic-key").Writer(failingWriter{})
	_, err := stream.Write([]byte("safe diagnostic"))
	require.EqualError(t, err, "redacted diagnostic output could not be written")
	require.Error(t, stream.Flush())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("synthetic-key") }

func TestJSONLRedactsStringsWithoutChangingTokenMetrics(t *testing.T) {
	input := []byte("{\"message\":\"1\",\"usage\":{\"input_tokens\":1,\"cost_usd\":0.1}}\n{\"details\":{\"value\":\"1\"}}\n")
	safe := New("1").JSON(input)
	require.Contains(t, string(safe), `"input_tokens":1`)
	require.Contains(t, string(safe), `"cost_usd":0.1`)
	require.Contains(t, string(safe), `"message":"[REDACTED]"`)
	require.Contains(t, string(safe), `"value":"[REDACTED]"`)
	require.Equal(t, "invalid JSON [REDACTED]", string(New("synthetic-key").JSON([]byte("invalid JSON synthetic-key"))))
}
