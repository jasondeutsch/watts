package devtools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockGateway(t *testing.T) {
	srv := httptest.NewServer(GatewayHandler("canned", 200))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"abc"}`))
	require.NoError(t, err)

	defer resp.Body.Close()
	var body struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	assert.False(t, resp.StatusCode != 200 || body.Model != "abc" || body.Choices[0].Message.Content != "canned",
		"mock completion: %d %+v", resp.StatusCode, body)

	fail := httptest.NewServer(GatewayHandler("", 401))
	defer fail.Close()
	r2, _ := http.Post(fail.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{}`))
	assert.Equal(t, 401, r2.StatusCode,
		"the mock must be able to fail: %d", r2.StatusCode)
}
