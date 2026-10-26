package gosdk

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitClient(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a strongly-typed, zero-external-dependency API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
	headers    map[string]string
}

// ClientOption configures a Client instance.
type ClientOption func(*Client)

// WithHTTPClient replaces the underlying standard HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithAuthToken sets an Authorization: Bearer <token> header on all outgoing requests.
func WithAuthToken(token string) ClientOption {
	return func(c *Client) {
		c.authToken = token
	}
}

// WithBearerToken is an alias for WithAuthToken.
func WithBearerToken(token string) ClientOption {
	return WithAuthToken(token)
}

// WithHeader adds a custom default HTTP header to all outgoing requests.
func WithHeader(key, value string) ClientOption {
	return func(c *Client) {
		c.headers[key] = value
	}
}

// NewClient creates a new API client configured with the provided base URL and options.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		headers:    make(map[string]string),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) doRequest(ctx context.Context, method, reqPath string, query url.Values, body interface{}, out interface{}) error {
	fullURL := c.baseURL + reqPath
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return decodeError(resp.StatusCode, respBody)
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
`)
	return sb.String()
}
