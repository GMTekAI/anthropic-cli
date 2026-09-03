package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Subprocess rather than in-process: the package-level Command tree caches
// parsed flag state across Runs, so a flag set in one case shadows the env
// source in the next.
func TestWorkspaceIDHeader(t *testing.T) {
	for _, tc := range []struct {
		name, env, want string
		global, leaf    []string
	}{
		{name: "global flag", global: []string{"--workspace-id", "wrkspc_flag"}, want: "wrkspc_flag"},
		{name: "env", env: "ANTHROPIC_WORKSPACE_ID=wrkspc_env", want: "wrkspc_env"},
		{name: "per-command flag", leaf: []string{"--workspace-id", "wrkspc_leaf"}, want: "wrkspc_leaf"},
		{name: "per-command flag overrides env", env: "ANTHROPIC_WORKSPACE_ID=wrkspc_env", leaf: []string{"--workspace-id", "wrkspc_leaf"}, want: "wrkspc_leaf"},
		{name: "per-command flag overrides global flag", global: []string{"--workspace-id", "wrkspc_flag"}, leaf: []string{"--workspace-id", "wrkspc_leaf"}, want: "wrkspc_leaf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Values("anthropic-workspace-id")
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"data":[],"has_more":false,"first_id":null,"last_id":null}`)
			}))

			args := append([]string{"run", "./cmd/ant", "--api-key", "k", "--base-url", srv.URL}, tc.global...)
			args = append(args, "models", "list")
			cmd := exec.Command("go", append(args, tc.leaf...)...)
			cmd.Dir = "../.."
			cmd.Env = append(cmd.Environ(), "ANTHROPIC_CONFIG_DIR="+t.TempDir(), tc.env)
			out, err := cmd.CombinedOutput()
			srv.Close()

			require.NoError(t, err, "%s", out)
			if tc.want == "" {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, []string{tc.want}, got, "exactly one anthropic-workspace-id value must reach the wire")
			}
		})
	}
}

// The worker subcommands build their own client (newWorkerClient); their
// requests must carry the header too. The handler cancels the poll loop once
// the first request lands.
func TestWorkspaceIDHeaderOnWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	gotCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case gotCh <- r.Header.Get("anthropic-workspace-id"):
			cancel()
		default:
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	t.Setenv("ANTHROPIC_CONFIG_DIR", t.TempDir())

	_ = Command.Run(ctx, []string{"ant", "beta:worker", "poll",
		"--environment-id", "env_t", "--environment-key", "ek_t",
		"--base-url", srv.URL, "--workspace-id", "wrkspc_worker"})

	select {
	case got := <-gotCh:
		assert.Equal(t, "wrkspc_worker", got)
	default:
		t.Fatal("worker made no requests")
	}
}
