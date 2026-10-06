package cmd

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/anthropics/anthropic-sdk-go/googlecloud"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/urfave/cli/v3"
)

// googleCloudHost is the gateway requests are addressed to when the Google
// Cloud tier derives its own URL.
const googleCloudHost = "https://claude.googleapis.com"

// One-shot guard for the stderr notice below. Reassigned to a fresh sync.Once
// by tests via a reset helper.
var googleCloudBaseURLNoticeOnce sync.Once

// googleCloud holds the inputs that address Claude Platform on Google Cloud.
// The gateway URL and the Google credential are resolved by the SDK's
// googlecloud package; this type is only the CLI's flag/env carrier plus the
// "configured or not" classification used by `auth status` and the precedence
// switch in getDefaultRequestOptions.
type googleCloud struct {
	// Project is the Google Cloud project ID. When empty the SDK falls back to
	// GOOGLE_CLOUD_PROJECT, then to the project of the Application Default
	// Credentials.
	Project string
	// Location is the Google Cloud location. When empty the SDK uses "global".
	Location string
	// WorkspaceID is the Anthropic workspace tagged ID (prefix `wrkspc_`). It
	// is part of the gateway URL, not a header.
	WorkspaceID string
	// BaseURL replaces the derived gateway URL for every route. It is the
	// only base URL this tier takes: --base-url / ANTHROPIC_BASE_URL name a
	// host that was trusted with Anthropic credentials, not Google ones.
	BaseURL string
}

// googleCloudFromRoot builds a googleCloud from the root command's flag
// values, so a subcommand's same-named local flag can't shadow them.
func googleCloudFromRoot(root *cli.Command) googleCloud {
	return googleCloud{
		Project:     root.String("google-cloud-project"),
		Location:    root.String("google-cloud-location"),
		WorkspaceID: root.String("google-cloud-workspace-id"),
		BaseURL:     root.String("google-cloud-base-url"),
	}
}

// AnySet reports whether the user addressed Google Cloud at all. Any one of
// project, location and workspace selects the tier: the project can come from
// the Google credentials and the location has a default, so there is no
// required set to wait for. GOOGLE_CLOUD_PROJECT on its own does not count; it
// is set on most Google Cloud machines for unrelated reasons. Nor does the
// base URL: one variable must not both turn on the Google credential and name
// the host it is sent to.
func (g googleCloud) AnySet() bool {
	return g.Project != "" || g.Location != "" || g.WorkspaceID != ""
}

// host is where this tier sends requests, for display and for the origin
// `ant apply` records: the base URL override, or the gateway.
func (g googleCloud) host() string {
	return cmp.Or(g.BaseURL, googleCloudHost)
}

// noteIgnoredBaseURL tells the user, once, that the generic base URL is not
// applied. Sending the Google token there without saying so would hand a
// Google credential to a host set up for Anthropic traffic.
func (g googleCloud) noteIgnoredBaseURL(w io.Writer, baseURL string) {
	if baseURL == "" {
		return
	}
	googleCloudBaseURLNoticeOnce.Do(func() {
		fmt.Fprintf(w,
			"Note: --base-url / ANTHROPIC_BASE_URL is not used with Google Cloud credentials; requests go to %s. To send them elsewhere: --google-cloud-base-url / ANTHROPIC_GOOGLE_CLOUD_BASE_URL.\n",
			g.host())
	})
}

// requestOptions returns the request options for the Google Cloud credential
// tier: the gateway base URL and a middleware that attaches a Google access
// token from Application Default Credentials, refreshed by the SDK.
func (g googleCloud) requestOptions(ctx context.Context) ([]option.RequestOption, error) {
	if err := ValidateBaseURL(g.BaseURL, "--google-cloud-base-url / ANTHROPIC_GOOGLE_CLOUD_BASE_URL"); err != nil {
		return nil, err
	}
	client, err := googlecloud.NewClient(ctx, googlecloud.ClientConfig{
		Project:     g.Project,
		Location:    g.Location,
		WorkspaceID: g.WorkspaceID,
		BaseURL:     g.BaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("%w\n"+
			"  Credentials come from Google Application Default Credentials: run `gcloud auth application-default login`,\n"+
			"  or set GOOGLE_APPLICATION_CREDENTIALS to a service account key file.\n"+
			"  Project:   --google-cloud-project / ANTHROPIC_GOOGLE_CLOUD_PROJECT\n"+
			"  Workspace: --google-cloud-workspace-id / ANTHROPIC_GOOGLE_CLOUD_WORKSPACE_ID", err)
	}
	return client.Options, nil
}
