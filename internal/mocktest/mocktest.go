package mocktest

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mockServerURL *url.URL

func init() {
	mockServerURL, _ = url.Parse("http://localhost:4010")
	if testURL := os.Getenv("TEST_API_BASE_URL"); testURL != "" {
		if parsed, err := url.Parse(testURL); err == nil {
			mockServerURL = parsed
		}
	}
}

// OnlyMockServerDialer only allows network connections to the mock server
type OnlyMockServerDialer struct{}

func (d *OnlyMockServerDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if address == mockServerURL.Host {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}

	return nil, fmt.Errorf("BLOCKED: connection to %s not allowed (only allowed: %s)", address, mockServerURL.Host)
}

func blockNetworkExceptMockServer() (http.RoundTripper, http.RoundTripper) {
	restricted := &http.Transport{
		DialContext: (&OnlyMockServerDialer{}).DialContext,
	}

	origClient, origDefault := http.DefaultClient.Transport, http.DefaultTransport
	http.DefaultClient.Transport, http.DefaultTransport = restricted, restricted
	return origClient, origDefault
}

func restoreNetwork(origClient, origDefault http.RoundTripper) {
	http.DefaultClient.Transport, http.DefaultTransport = origClient, origDefault
}

// TestRunMockTestWithFlags runs a test against a mock server with the provided
// CLI args and ensures it succeeds
func TestRunMockTestWithFlags(t *testing.T, args ...string) {
	TestRunMockTestWithPipeAndFlags(t, nil, args...)
}

// TestRunMockTestWithPipeAndFlags runs a test against a mock server with the provided
// data piped over stdin and CLI args and ensures it succeeds
func TestRunMockTestWithPipeAndFlags(t *testing.T, pipeData []byte, args ...string) {
	origClient, origDefault := blockNetworkExceptMockServer()
	defer restoreNetwork(origClient, origDefault)

	// Check if mock server is running
	conn, err := net.DialTimeout("tcp", mockServerURL.Host, 2*time.Second)
	if err != nil {
		require.Fail(t, "Mock server is not running on "+mockServerURL.Host+". Please start the mock server before running tests.")
	} else {
		conn.Close()
	}

	// Get the path to the main command
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "Could not get current file path")
	dirPath := filepath.Dir(filename)
	project := filepath.Join(dirPath, "..", "..", "cmd", "ant")

	args = append([]string{"run", project, "--base-url", mockServerURL.String()}, args...)

	t.Logf("Testing command: go run ./cmd/ant %s", strings.Join(args[2:], " "))

	cmd := exec.Command("go", args...)
	cmd.Stdin = bytes.NewReader(pipeData)
	output, err := cmd.CombinedOutput()
	assert.NoError(t, err, "Test failed\nError: %v\nOutput: %s", err, output)

	t.Logf("Test passed successfully\nOutput:\n%s", string(output))
}

// ServerWithQuery starts a server that hands each request to the mock server with the given query
// params set, and returns its URL to pass as `--base-url`. The mock server needs them to tell two
// operations with the same path apart.
func ServerWithQuery(t *testing.T, query string) string {
	params, err := url.ParseQuery(query)
	require.NoError(t, err)

	server := httptest.NewServer(&httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(mockServerURL)
			values := r.Out.URL.Query()
			for name, value := range params {
				values[name] = value
			}
			r.Out.URL.RawQuery = values.Encode()
		},
	})
	t.Cleanup(server.Close)
	return server.URL
}

func TestFile(t *testing.T, contents string) string {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(filename, []byte(contents), 0644))
	return filename
}
