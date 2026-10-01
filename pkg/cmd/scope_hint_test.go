package cmd

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrgAdminScopeHint: a login without org:admin is refused on the
// organization routes with a bare 403 or a 404, neither of which names the
// scope, so the hint keys on the route and the login's scope. It fires once
// per process and never for a login that has the scope.
func TestOrgAdminScopeHint(t *testing.T) {
	call := func(t *testing.T, granted, path string, status int) string {
		t.Helper()
		orgAdminHintOnce = sync.Once{}
		var w strings.Builder
		mw := orgAdminScopeHint(&w, "work", granted)
		_, err := mw(&http.Request{URL: &url.URL{Path: path}}, func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: http.NoBody}, nil
		})
		require.NoError(t, err)
		return w.String()
	}
	const want = "Run: ant auth login --admin --profile work"

	t.Run("403 on an organization route", func(t *testing.T) {
		assert.Contains(t, call(t, oauthScope, "/v1/organizations/users", http.StatusForbidden), want)
	})
	t.Run("404 on an organization route", func(t *testing.T) {
		assert.Contains(t, call(t, oauthScope, "/v1/organizations/service_accounts", http.StatusNotFound), want)
	})
	t.Run("base URL with a path prefix", func(t *testing.T) {
		assert.Contains(t, call(t, oauthScope, "/gateway/v1/organizations/me", http.StatusForbidden), want)
	})
	t.Run("login has org:admin: the refusal is about something else", func(t *testing.T) {
		assert.Empty(t, call(t, oauthScope+" org:admin", "/v1/organizations/users/user_missing", http.StatusNotFound))
	})
	t.Run("not an organization route", func(t *testing.T) {
		assert.Empty(t, call(t, oauthScope, "/v1/messages", http.StatusForbidden))
		assert.Empty(t, call(t, oauthScope, "/v1/models/nope", http.StatusNotFound))
	})
	t.Run("other statuses", func(t *testing.T) {
		assert.Empty(t, call(t, oauthScope, "/v1/organizations/rbac_groups", http.StatusUnauthorized))
		assert.Empty(t, call(t, oauthScope, "/v1/organizations/users", http.StatusOK))
	})
	t.Run("once per process", func(t *testing.T) {
		orgAdminHintOnce = sync.Once{}
		var w strings.Builder
		mw := orgAdminScopeHint(&w, "work", oauthScope)
		for range 2 {
			_, err := mw(&http.Request{URL: &url.URL{Path: "/v1/organizations/me"}}, func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusForbidden, Body: http.NoBody}, nil
			})
			require.NoError(t, err)
		}
		assert.Equal(t, 1, strings.Count(w.String(), "Hint:"))
	})
}
