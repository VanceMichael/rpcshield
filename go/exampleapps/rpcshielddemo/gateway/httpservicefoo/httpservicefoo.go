// Package httpservicefoo provides a Gateway to an HTTP service.
package httpservicefoo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/httpclient"
	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/wrappedclient"
)

// Client represents the HTTP client interface.
type Client = *httpclient.Client

// Gateway provides access to HTTP service operations.
type Gateway interface {
	Call(context.Context, string) (string, error)
}

type gateway struct {
	client  Client
	baseURL string
}

type callRequest struct {
	Message string `json:"message"`
}

type callResponse struct {
	Result string `json:"result"`
}

// New returns a Gateway for the HTTP service.
func New(client Client) Gateway {
	return &gateway{
		client:  client,
		baseURL: "http://localhost:8080",
	}
}

// Call makes an HTTP POST request to the service.
func (g *gateway) Call(ctx context.Context, message string) (string, error) {
	reqBody := callRequest{Message: message}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := g.baseURL + "/api/call"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	cl := &http.Client{Timeout: 3 * time.Second}
	_, err = cl.Do(req)
	if err != nil {
		return "", fmt.Errorf("default HTTP request failed: %w", err)
	}

	_, err = wrappedclient.DefaultClient.Do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("wrapped HTTP request failed: %w", err)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var callResp callResponse
	if err := json.NewDecoder(resp.Body).Decode(&callResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return callResp.Result, nil
}
