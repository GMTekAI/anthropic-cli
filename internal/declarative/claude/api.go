package claude

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// sdkClient implements core.Client on top of the Anthropic Go SDK.
//
// Requests are sent as raw JSON bodies rather than through the SDK's typed
// parameter structs, matching how pkg/cmd's generated commands work: pass an
// empty params value and override the body with a request option. That keeps
// the reconciler from needing a Go type for every union in the spec, and means
// a field added to the API works the day it ships.
type sdkClient struct {
	sdk anthropic.Client
	// opts are the per-request options the CLI resolved (auth, base URL,
	// headers, debug middleware).
	opts []option.RequestOption
}

// NewClient wraps an SDK client for use by the reconciler.
func NewClient(client anthropic.Client, opts ...option.RequestOption) core.Client {
	return &sdkClient{sdk: client, opts: opts}
}

func (c *sdkClient) Get(ctx context.Context, kind core.Kind, id string) (map[string]any, error) {
	switch kind {
	case KindSkill:
		return c.readSkill(ctx, id)
	case KindEnvironment:
		return c.call(nil, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Environments.Get(ctx, id, anthropic.BetaEnvironmentGetParams{}, o...))
		})
	case KindMemoryStore:
		return c.call(nil, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.MemoryStores.Get(ctx, id, anthropic.BetaMemoryStoreGetParams{}, o...))
		})
	case KindAgent:
		return c.call(nil, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Agents.Get(ctx, id, anthropic.BetaAgentGetParams{}, o...))
		})
	case KindDeployment:
		return c.call(nil, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Deployments.Get(ctx, id, anthropic.BetaDeploymentGetParams{}, o...))
		})
	}
	return nil, fmt.Errorf("get: unsupported kind %q", kind)
}

func (c *sdkClient) Create(ctx context.Context, kind core.Kind, req core.Request) (map[string]any, error) {
	switch kind {
	case KindSkill:
		return c.createSkill(ctx, bundleOf(req))
	case KindEnvironment:
		return c.call(req.Body, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Environments.New(ctx, anthropic.BetaEnvironmentNewParams{}, o...))
		})
	case KindMemoryStore:
		return c.call(req.Body, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.MemoryStores.New(ctx, anthropic.BetaMemoryStoreNewParams{}, o...))
		})
	case KindAgent:
		return c.call(req.Body, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Agents.New(ctx, anthropic.BetaAgentNewParams{}, o...))
		})
	case KindDeployment:
		return c.call(req.Body, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Deployments.New(ctx, anthropic.BetaDeploymentNewParams{}, o...))
		})
	}
	return nil, fmt.Errorf("create: unsupported kind %q", kind)
}

func (c *sdkClient) Update(ctx context.Context, kind core.Kind, id string, req core.Request) (map[string]any, error) {
	switch kind {
	case KindSkill:
		// A skill has no update endpoint: a change is a new version, and the
		// JSON body has nothing to say about it.
		return c.createSkillVersion(ctx, id, bundleOf(req))
	case KindEnvironment:
		return c.call(req.Body, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Environments.Update(ctx, id, anthropic.BetaEnvironmentUpdateParams{}, o...))
		})
	case KindMemoryStore:
		return c.call(req.Body, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.MemoryStores.Update(ctx, id, anthropic.BetaMemoryStoreUpdateParams{}, o...))
		})
	case KindAgent:
		return c.call(req.Body, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Agents.Update(ctx, id, anthropic.BetaAgentUpdateParams{}, o...))
		})
	case KindDeployment:
		return c.call(req.Body, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Deployments.Update(ctx, id, anthropic.BetaDeploymentUpdateParams{}, o...))
		})
	}
	return nil, fmt.Errorf("update: unsupported kind %q", kind)
}

// Destroy archives or deletes each kind as its KindSpec.Destroy in Registry
// says. The plan has already printed that verb, so the two must agree: a kind
// archived here is `byArchiving` there.
func (c *sdkClient) Destroy(ctx context.Context, kind core.Kind, id string) error {
	switch kind {
	case KindSkill:
		return c.destroySkill(ctx, id)
	case KindEnvironment:
		return discard(c.call(nil, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Environments.Archive(ctx, id, anthropic.BetaEnvironmentArchiveParams{}, o...))
		}))
	case KindMemoryStore:
		// Archive rather than delete: delete takes every memory in the store
		// with it, and those were written by the agent, not by this config.
		return discard(c.call(nil, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.MemoryStores.Archive(ctx, id, anthropic.BetaMemoryStoreArchiveParams{}, o...))
		}))
	case KindAgent:
		return discard(c.call(nil, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Agents.Archive(ctx, id, anthropic.BetaAgentArchiveParams{}, o...))
		}))
	case KindDeployment:
		return discard(c.call(nil, func(o []option.RequestOption) error {
			return discard(c.sdk.Beta.Deployments.Archive(ctx, id, anthropic.BetaDeploymentArchiveParams{}, o...))
		}))
	}
	return fmt.Errorf("destroy: unsupported kind %q", kind)
}

// call issues a request and decodes the raw response body into a map.
func (c *sdkClient) call(body map[string]any, fn func(opts []option.RequestOption) error) (map[string]any, error) {
	var raw []byte
	opts := slices.Clone(c.opts)
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithRequestBody("application/json", encoded))
	}
	opts = append(opts, option.WithResponseBodyInto(&raw))

	if err := fn(opts); err != nil {
		return nil, translateError(err)
	}
	return decodeObject(raw)
}

// decodeObject parses a response body as a JSON object. Numbers stay
// json.Number so an integer is never rounded through float64, and an empty
// body decodes as an empty object.
func decodeObject(raw []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding API response: %w", err)
	}
	return out, nil
}

// translateError maps an SDK error onto what core understands. A 404 wraps
// core.ErrNotFound, which is how the planner and applier tell a resource that
// is gone from a request that failed. Other errors pass through, some with a
// hint.
func translateError(err error) error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	if apiErr.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", core.ErrNotFound, apiErr.Error())
	}
	// A skill's display name is unique across the whole organization, not just
	// this config, so two configs that declare the same skill collide, and so does
	// a skill removed from this config that still exists in the Console. The bare
	// 400 does not say what to do; the hint does.
	if apiErr.StatusCode == http.StatusBadRequest && strings.Contains(apiErr.RawJSON(), "cannot reuse an existing display_") {
		return core.WithHint(err, "skill names are unique per organization: set a distinct `display_name:` (or `name:`) in SKILL.md, or remove the existing skill with `ant beta:skills delete`")
	}
	return err
}

// discard drops a call's value, leaving the error. The reconciler reads the
// raw JSON body instead — see the note on sdkClient — so the typed value is
// never wanted, and dropping it inline keeps each dispatch arm to a single line.
func discard[T any](_ T, err error) error { return err }

// readSkill reads a skill and, from its latest version, the `name` its
// SKILL.md declared. The skill object does not carry it, but it is the identity
// the API holds every later version to, so the plan needs it to refuse a
// rename. A version that cannot be found leaves the name unknown rather than
// failing the read.
func (c *sdkClient) readSkill(ctx context.Context, id string) (map[string]any, error) {
	obj, err := withCurrentSkillFields(c.call(nil, func(o []option.RequestOption) error {
		return discard(c.sdk.Beta.Skills.Get(ctx, id, anthropic.BetaSkillGetParams{}, o...))
	}))
	if err != nil {
		return nil, err
	}
	var versionID string
	switch v := obj["latest_version_id"].(type) {
	case string:
		versionID = v
	case json.Number:
		versionID = v.String()
	}
	if versionID == "" {
		return obj, nil
	}
	version, err := c.call(nil, func(o []option.RequestOption) error {
		return discard(c.sdk.Beta.Skills.Versions.Get(ctx, versionID, anthropic.BetaSkillVersionGetParams{SkillID: id}, o...))
	})
	if errors.Is(err, core.ErrNotFound) {
		return obj, nil
	}
	if err != nil {
		return nil, err
	}
	if name, _ := version["name"].(string); name != "" {
		obj["name"] = name
	}
	return obj, nil
}

// bundleOf recovers the skill bundle from a request. Core carries payloads as
// an opaque interface; only this package knows what shape they are.
func bundleOf(req core.Request) *skillBundle {
	b, _ := req.Payload.(*skillBundle)
	return b
}

func (c *sdkClient) createSkill(ctx context.Context, bundle *skillBundle) (map[string]any, error) {
	files, err := readSkillFiles(bundle)
	if err != nil {
		return nil, err
	}
	params := anthropic.BetaSkillNewParams{Files: files}
	if bundle.DisplayName != "" {
		params.DisplayName = anthropic.String(bundle.DisplayName)
	}
	return withCurrentSkillFields(c.call(nil, func(o []option.RequestOption) error {
		return discard(c.sdk.Beta.Skills.New(ctx, params, o...))
	}))
}

// withCurrentSkillFields rewrites the skill fields display_title and
// latest_version, which some servers still return, to display_name and
// latest_version_id, the names the API specification uses. A response that
// already has the current name keeps it.
func withCurrentSkillFields(obj map[string]any, err error) (map[string]any, error) {
	if err != nil {
		return obj, err
	}
	for legacy, current := range map[string]string{"display_title": "display_name", "latest_version": "latest_version_id"} {
		if v, ok := obj[legacy]; ok {
			if _, has := obj[current]; !has {
				obj[current] = v
			}
			delete(obj, legacy)
		}
	}
	return obj, nil
}

func (c *sdkClient) createSkillVersion(ctx context.Context, skillID string, bundle *skillBundle) (map[string]any, error) {
	files, err := readSkillFiles(bundle)
	if err != nil {
		return nil, err
	}
	params := anthropic.BetaSkillVersionNewParams{Files: files}
	obj, err := c.call(nil, func(o []option.RequestOption) error {
		return discard(c.sdk.Beta.Skills.Versions.New(ctx, skillID, params, o...))
	})
	if err != nil {
		return nil, err
	}
	// The endpoint answers with the version object, which carries its own
	// identifier under `id`; the applier reads the version a payload upload
	// minted from `version`, so copy it there. An `id` equal to the skill's
	// is the skill object itself, not a version, and is left alone.
	if _, has := obj["version"]; !has {
		if id, ok := obj["id"].(string); ok && id != "" && id != skillID {
			obj["version"] = id
		}
	}
	return obj, nil
}

// readSkillFiles buffers a skill bundle into memory, because the multipart
// encoder consumes every reader during marshalling and a bundle can hold more
// open files than the default per-process descriptor limit allows.
//
// Each part is named with the file's path under the bundle's upload folder:
// that is how the server rebuilds the tree. A bare *os.File would name itself,
// flattening every subdirectory into the root.
func readSkillFiles(bundle *skillBundle) ([]io.Reader, error) {
	if bundle == nil {
		return nil, errors.New("a skill write carries no files to upload")
	}
	files := make([]io.Reader, 0, len(bundle.Files))
	for _, f := range bundle.Files {
		data, err := os.ReadFile(f.AbsPath)
		if err != nil {
			return nil, err
		}
		files = append(files, anthropic.File(
			bytes.NewReader(data),
			f.UploadName(bundle.UploadDir),
			f.ContentType(),
		))
	}
	return files, nil
}

// destroySkill removes a skill and its versions. Some servers refuse to delete
// a skill that still has versions; others refuse to delete a skill's last
// version and remove it with the skill. Deleting every version, tolerating the
// last-version refusal and versions already gone, then deleting the skill,
// satisfies both.
func (c *sdkClient) destroySkill(ctx context.Context, id string) error {
	versions := c.sdk.Beta.Skills.Versions.ListAutoPaging(ctx, id,
		anthropic.BetaSkillVersionListParams{}, c.opts...)
	var versionIDs []string
	for versions.Next() {
		v := versions.Current()
		// Some servers name a version by "version" rather than "id".
		versionIDs = append(versionIDs, cmp.Or(v.ID, v.JSON.ExtraFields["version"].Raw()))
	}
	if err := versions.Err(); err != nil {
		return translateError(err)
	}
	for _, versionID := range versionIDs {
		_, err := c.sdk.Beta.Skills.Versions.Delete(ctx, versionID,
			anthropic.BetaSkillVersionDeleteParams{SkillID: id}, c.opts...)
		if err == nil || isLastVersionRefusal(err) {
			continue
		}
		if err = translateError(err); !errors.Is(err, core.ErrNotFound) {
			return fmt.Errorf("deleting version %s: %w", versionID, err)
		}
	}
	_, err := c.sdk.Beta.Skills.Delete(ctx, id, anthropic.BetaSkillDeleteParams{}, c.opts...)
	return translateError(err)
}

// isLastVersionRefusal reports whether err is the refusal destroySkill
// describes: a server that will not delete a skill's last version. The refusal
// has no error type of its own, so it is recognized by its message.
func isLastVersionRefusal(err error) bool {
	var apiErr *anthropic.Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest &&
		strings.Contains(apiErr.RawJSON(), "only version")
}
