package cmd

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go/option"
)

var orgAdminHintOnce sync.Once

// orgAdminScopeHint points a logged-in user at `ant auth login --admin` when
// the API refuses an organization request. The API does not say which scope
// was missing: a login without org:admin gets a bare 403 from most
// /v1/organizations routes and a 404 from the rest, so the hint goes on the
// route and the login's scope rather than on the error body. grantedScope is
// the profile's login scope; a token that has org:admin gets no hint, since
// its 403 or 404 is about something else.
func orgAdminScopeHint(w io.Writer, profile, grantedScope string) option.Middleware {
	return func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		resp, err := next(req)
		if err != nil || resp == nil || hasScope(grantedScope, scopeOrgAdmin) {
			return resp, err
		}
		refused := resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound
		if refused && strings.Contains(req.URL.Path, "/v1/organizations/") {
			orgAdminHintOnce.Do(func() {
				fmt.Fprintf(w, "Hint: the organization commands need the %s scope, which profile %q was not granted at login. Run: ant auth login --admin --profile %s\n",
					scopeOrgAdmin, profile, profile)
			})
		}
		return resp, err
	}
}

// profileGrantedScope is the scope the profile's current login was granted,
// falling back to what its config asks for when the credentials don't say.
func profileGrantedScope(creds storedCredentials, credsErr error, requested string) string {
	if credsErr == nil && creds.Scope != "" {
		return creds.Scope
	}
	return requested
}
