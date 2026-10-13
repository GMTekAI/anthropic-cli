package cmd

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/anthropics/anthropic-sdk-go/aws"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/urfave/cli/v3"
)

// One-shot guard for the stderr notice below. Reassigned to a fresh sync.Once
// by tests via a reset helper.
var awsBaseURLNoticeOnce sync.Once

// awsPlatform holds the inputs that address Claude Platform on AWS. The
// gateway URL and the request signing are handled by the SDK's aws package;
// this type is only the CLI's flag/env carrier plus the "configured or not"
// classification used by `auth status` and the precedence switch in
// getDefaultRequestOptions.
type awsPlatform struct {
	// Region picks the gateway and the signing region. When empty the SDK
	// falls back to AWS_REGION, then AWS_DEFAULT_REGION.
	Region string
	// WorkspaceID is the Anthropic workspace tagged ID (prefix `wrkspc_`),
	// sent as the anthropic-workspace-id header.
	WorkspaceID string
	// APIKey is an AWS API key from --aws-api-key-stdin or
	// ANTHROPIC_AWS_API_KEY. When empty, requests are signed with AWS
	// credentials from the default credential chain.
	APIKey string
	// BaseURL replaces the regional gateway URL. Like the Google Cloud tier,
	// this tier never takes --base-url / ANTHROPIC_BASE_URL.
	BaseURL string
}

// awsFromRoot builds an awsPlatform from the root command's flag values, so a
// subcommand's same-named local flag can't shadow them. applyStdinCredential
// must run first for the API key.
func awsFromRoot(root *cli.Command) awsPlatform {
	apiKey, _ := directCredential(root, "aws-api-key")
	return awsPlatform{
		Region:      root.String("aws-region"),
		WorkspaceID: root.String("aws-workspace-id"),
		APIKey:      apiKey,
		BaseURL:     root.String("aws-base-url"),
	}
}

// AnySet reports whether the user addressed AWS at all: a region or a
// workspace selects the tier. AWS_REGION and AWS_PROFILE on their own do not
// count; they are set on most AWS machines for unrelated reasons. Nor do the
// API key and base URL, for the same reasons as the Google Cloud tier.
func (a awsPlatform) AnySet() bool {
	return a.Region != "" || a.WorkspaceID != ""
}

// apiKeySource names where the API key came from, without printing it.
func (a awsPlatform) apiKeySource(root *cli.Command) string {
	if credentialFromStdin(root, "aws-api-key") {
		return "--aws-api-key-stdin"
	}
	return "ANTHROPIC_AWS_API_KEY"
}

// host is where this tier sends requests, for display and for the origin
// `ant apply` records: the base URL override, or the regional gateway. Empty
// when no region is set anywhere, since the SDK then has no gateway to derive.
func (a awsPlatform) host() string {
	region := cmp.Or(a.Region, os.Getenv("AWS_REGION"), os.Getenv("AWS_DEFAULT_REGION"))
	if a.BaseURL != "" || region == "" {
		return a.BaseURL
	}
	return fmt.Sprintf("https://aws-external-anthropic.%s.api.aws", region)
}

// noteIgnoredBaseURL tells the user, once, that the generic base URL is not
// applied, for the same reason as the Google Cloud tier's notice.
func (a awsPlatform) noteIgnoredBaseURL(w io.Writer, baseURL string) {
	if baseURL == "" {
		return
	}
	awsBaseURLNoticeOnce.Do(func() {
		fmt.Fprintf(w,
			"Note: --base-url / ANTHROPIC_BASE_URL is not used with AWS credentials; requests go to %s. To send them elsewhere: --aws-base-url / ANTHROPIC_AWS_BASE_URL.\n",
			cmp.Or(a.host(), "the regional gateway"))
	})
}

// requestOptions returns the request options for the AWS credential tier: the
// gateway base URL, the workspace header, and either the API key or a
// middleware that signs each request with AWS credentials.
func (a awsPlatform) requestOptions(ctx context.Context) ([]option.RequestOption, error) {
	if err := ValidateBaseURL(a.BaseURL, "--aws-base-url / ANTHROPIC_AWS_BASE_URL"); err != nil {
		return nil, err
	}
	client, err := aws.NewClient(ctx, aws.ClientConfig{
		AWSRegion:   a.Region,
		WorkspaceID: a.WorkspaceID,
		APIKey:      a.APIKey,
		BaseURL:     a.BaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("%w\n"+
			"  Credentials come from the AWS default credential chain (AWS_PROFILE, environment keys, an IAM role),\n"+
			"  or from an AWS API key in ANTHROPIC_AWS_API_KEY / --aws-api-key-stdin.\n"+
			"  Region:    --aws-region / AWS_REGION / AWS_DEFAULT_REGION\n"+
			"  Workspace: --aws-workspace-id / ANTHROPIC_AWS_WORKSPACE_ID", err)
	}
	return client.Options, nil
}
