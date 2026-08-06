// Package httpclient provides a simple HTTP client utility.
package httpclient

import (
	"net/http"
	"time"
)

// Client wraps the standard http.Client.
type Client struct {
	inner *http.Client
}

// New creates a new HTTP client with default settings.
func New() *Client {
	return &Client{
		inner: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewWithClient creates a new HTTP client with a custom http.Client.
func NewWithClient(httpClient *http.Client) *Client {
	return &Client{inner: httpClient}
}

// Do executes an HTTP request and returns the response.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.inner.Do(req)
}
