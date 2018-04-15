package utils

import (
	"net/http"
)

// KubeLegoUserAgentPrefix is the user agent prefix that http clients in this codebase should use
var KubeLegoUserAgentPrefix = "jetstack-kube-lego/"

// UserAgentRoundTripper implements the http.RoundTripper interface and adds a User-Agent
// header.
type userAgentRoundTripper struct {
	version string

	inner http.RoundTripper
}

// UserAgentRoundTripper returns a RoundTripper that functions identically to
// the provided 'inner' round tripper, other than also setting a user agent.
func UserAgentRoundTripper(inner http.RoundTripper, version string) http.RoundTripper {
	return userAgentRoundTripper{
		version: version,
		inner:   inner,
	}
}

// RoundTrip implements http.RoundTripper
func (u userAgentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", KubeLegoUserAgentPrefix+u.version)
	return u.inner.RoundTrip(req)
}
