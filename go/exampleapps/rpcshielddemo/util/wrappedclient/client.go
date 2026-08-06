// Package wrappedclient provides a second, distinct wrapped-HTTP-client type -- structurally
// analogous to an internal example.internal/go-common.git/x/net/xhttp.Client-style wrapper, but generic
// -- so finder's outbound detection of "extra" wrapped HTTP clients (configured via
// config.Config.AdditionalHTTPClientTypes, not recognized by default) has something to detect
// against outside the monorepo.
package wrappedclient

import (
	"context"
	"net/http"
)

// Client is a context-aware wrapped HTTP client, mirroring the shape of an internal
// xhttp.Client-style wrapper (a Do(ctx, req) method instead of the stdlib's Do(req)).
type Client struct {
	inner *http.Client
}

// DefaultClient is a package-level default instance, mirroring xhttp.DefaultClient.
var DefaultClient = &Client{inner: http.DefaultClient}

// Do executes an HTTP request and returns the response.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.inner.Do(req.WithContext(ctx))
}
