package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tarGz(t *testing.T, entries []*tar.Header, bodies []string) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i, h := range entries {
		require.NoError(t, tw.WriteHeader(h))
		if h.Typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(bodies[i]))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return bytes.NewReader(buf.Bytes())
}

func TestExtractTarGzRefusesPathTraversal(t *testing.T) {
	dest := t.TempDir()
	outside := filepath.Join(filepath.Dir(dest), "escaped.txt")

	archive := tarGz(t,
		[]*tar.Header{{Name: "../escaped.txt", Typeflag: tar.TypeReg, Size: 5, Mode: 0o644}},
		[]string{"pwned"})

	err := extractTarGz(archive, dest)
	require.ErrorContains(t, err, "escapes the extraction directory")
	assert.NoFileExists(t, outside)
}

func TestExtractTarGzSkipsSymlinks(t *testing.T) {
	dest := t.TempDir()
	archive := tarGz(t, []*tar.Header{
		{Name: "skill/SKILL.md", Typeflag: tar.TypeReg, Size: 2, Mode: 0o644},
		{Name: "skill/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777},
	}, []string{"hi", ""})

	require.NoError(t, extractTarGz(archive, dest))
	assert.FileExists(t, filepath.Join(dest, "skill", "SKILL.md"))
	_, err := os.Lstat(filepath.Join(dest, "skill", "link"))
	assert.Error(t, err, "a symlink in a downloaded bundle is never worth honoring")
}

func TestExtractTarGzWritesNestedFiles(t *testing.T) {
	dest := t.TempDir()
	archive := tarGz(t, []*tar.Header{
		{Name: "repo/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "repo/skills/s/SKILL.md", Typeflag: tar.TypeReg, Size: 4, Mode: 0o644},
	}, []string{"", "body"})

	require.NoError(t, extractTarGz(archive, dest))
	data, err := os.ReadFile(filepath.Join(dest, "repo", "skills", "s", "SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "body", string(data))
}

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		url     string
		want    githubRef
		wantErr string
	}{
		{
			url:  "https://github.com/vercel-labs/agent-skills/tree/main/skills/composition-patterns",
			want: githubRef{Owner: "vercel-labs", Repo: "agent-skills", Rest: []string{"main", "skills", "composition-patterns"}},
		},
		{
			url:  "https://github.com/o/r",
			want: githubRef{Owner: "o", Repo: "r"},
		},
		{
			url:  "https://github.com/o/r/tree/v1.2.3",
			want: githubRef{Owner: "o", Repo: "r", Rest: []string{"v1.2.3"}},
		},
		{url: "https://gitlab.com/o/r", wantErr: "only github.com URLs"},
		{url: "https://github.com/o", wantErr: "expected a URL like"},
		{url: "https://github.com/o/r/releases/latest", wantErr: "expected /tree/"},
		{
			// Would otherwise resolve outside the download cache.
			url:     "https://github.com/o/r/tree/main/../../../etc",
			wantErr: `may not contain ".."`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			got, err := parseGitHubURL(tc.url)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGitHubRefSplit(t *testing.T) {
	ref := githubRef{Rest: []string{"team", "new-skill", "pdf"}}

	gitRef, subpath := ref.split(1)
	assert.Equal(t, "team", gitRef)
	assert.Equal(t, "new-skill/pdf", subpath)

	gitRef, subpath = ref.split(2)
	assert.Equal(t, "team/new-skill", gitRef)
	assert.Equal(t, "pdf", subpath)
}

// A branch name may contain slashes, so where the ref ends and the path begins
// can only be settled by asking the API.
func TestResolveTriesLongerRefsWhenTheShortOneIsNotABranch(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ref := strings.TrimPrefix(r.URL.Path, "/repos/o/skills/commits/")
		asked = append(asked, ref)
		if ref != "team/new-skill" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	}))
	defer server.Close()

	g := &GitHubFetcher{HTTPClient: server.Client(), apiBase: server.URL}
	pin, err := g.resolve(context.Background(),
		githubRef{Owner: "o", Repo: "skills", Rest: []string{"team", "new-skill", "pdf"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"team", "team/new-skill"}, asked)
	assert.Equal(t, URLPin{Revision: strings.Repeat("a", 40), Subpath: "pdf"}, pin)
}

func TestResolveReportsTheFirstFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	g := &GitHubFetcher{HTTPClient: server.Client(), apiBase: server.URL}
	_, err := g.resolve(context.Background(),
		githubRef{Owner: "o", Repo: "r", Rest: []string{"main", "skills"}})
	require.ErrorContains(t, err, "/commits/main")
}

// A pin read back from the lockfile is somebody else's input: it must not be
// able to aim the bundle at a directory outside the download.
func TestPinnedFetchRefusesAPinThatEscapes(t *testing.T) {
	g := &GitHubFetcher{CacheDir: t.TempDir(), cache: map[string]*FetchedURL{}}
	sha := strings.Repeat("e", 40)
	for _, pin := range []URLPin{
		{Revision: sha, Subpath: "../../etc"},
		{Revision: sha, Subpath: "/etc"},
		{Revision: "../../../../x", Subpath: "s"},
		{Revision: "deadbeef", Subpath: "s"},
	} {
		_, err := g.Fetch(context.Background(), "https://github.com/o/r/tree/main/s", pin)
		assert.Error(t, err, "%+v", pin)
	}
}

// A branch with slashes must keep resolving to the same directory once its
// commit is pinned. The pin records where the branch name ended, so a second
// apply neither asks the API again nor guesses.
func TestPinnedFetchUsesTheRecordedSubpathWithoutAskingTheAPI(t *testing.T) {
	sha := strings.Repeat("d", 40)
	cache := t.TempDir()
	writeTreeAt(t, filepath.Join(cache, "o-r-"+sha[:12]), map[string]string{
		"pdf/SKILL.md":           "---\nname: pdf\n---\nbody\n",
		"new-skill/pdf/notes.md": "a decoy the old probing code would have had to rule out\n",
	})

	g := &GitHubFetcher{HTTPClient: nil, apiBase: "http://unreachable.invalid", CacheDir: cache, cache: map[string]*FetchedURL{}}
	got, err := g.Fetch(context.Background(),
		"https://github.com/o/r/tree/team/new-skill/pdf", URLPin{Revision: sha, Subpath: "pdf"})

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cache, "o-r-"+sha[:12], "pdf"), got.Dir)
	assert.Equal(t, URLPin{Revision: sha, Subpath: "pdf"}, got.URLPin)
}
