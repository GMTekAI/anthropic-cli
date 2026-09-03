package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// WhoAmI asks the server which organization and workspace these credentials
// act in. Every response names both in headers, so the cheapest authenticated
// read will do; local configuration is never consulted, which keeps the answer
// right for API keys and tokens that no profile describes.
// An ID whose header the server does not send comes back "", with a nil error.
func WhoAmI(ctx context.Context, client anthropic.Client) (organizationID, workspaceID string, err error) {
	var resp *http.Response
	var body json.RawMessage
	if err = client.Get(ctx, "/v1/models?limit=1", nil, &body, option.WithResponseInto(&resp)); err != nil {
		return "", "", err
	}
	return resp.Header.Get("Anthropic-Organization-Id"), resp.Header.Get("Anthropic-Workspace-Id"), nil
}

// CheckOrigin refuses to run against a lockfile whose resources live somewhere
// the current credentials do not reach: another API host, organization or
// workspace. Without this, every read comes back not-found and the plan
// offers to recreate everything under new ids — the wrong reading of "you are
// logged in elsewhere".
func CheckOrigin(lock *core.Lockfile, current core.Origin) error {
	recorded := lock.Origin
	if len(lock.Resources) == 0 || recorded == nil {
		return nil
	}
	if was, now := host(recorded.BaseURL), host(current.BaseURL); recorded.BaseURL != "" && current.BaseURL != "" && was != now {
		return mismatch(lock, "API host", was, now)
	}
	if conflict(recorded.OrganizationID, current.OrganizationID) {
		return mismatch(lock, "organization", recorded.OrganizationID, current.OrganizationID)
	}
	if conflict(recorded.WorkspaceID, current.WorkspaceID) {
		return mismatch(lock, "workspace", recorded.WorkspaceID, current.WorkspaceID)
	}
	return nil
}

// OriginMismatchError is the error CheckOrigin returns, so a caller can add
// what it knows about how to reach the right target — a profile name, say.
type OriginMismatchError struct {
	// Lockfile is the lockfile's path, as Error prints it.
	Lockfile string
	// What names what differs: "API host", "organization" or "workspace".
	What string
	// Want is what the lockfile recorded; Got is what the credentials reach.
	Want, Got string
}

func (e *OriginMismatchError) Error() string {
	return fmt.Sprintf("%s tracks resources on %s %s, but the credentials in use target %s", e.Lockfile, e.What, e.Want, e.Got)
}

func mismatch(lock *core.Lockfile, what, want, got string) error {
	return &OriginMismatchError{Lockfile: lock.Path, What: what, Want: want, Got: got}
}

// conflict reports whether two recorded values are both known and differ.
func conflict(a, b string) bool { return a != "" && b != "" && a != b }

// host reduces a base URL to its lower-cased host and port, so two spellings
// of one API compare equal. A base with no scheme has no host to parse out
// and is compared as written, minus a trailing slash.
func host(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return strings.TrimSuffix(base, "/")
	}
	return strings.ToLower(u.Host)
}
