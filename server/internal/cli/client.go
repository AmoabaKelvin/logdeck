package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const requestTimeout = 30 * time.Second

const authHint = "authenticate with an API token created in LogDeck Settings, via --token or LOGDECK_TOKEN"

// client is a thin HTTP client for the LogDeck API (/api/v1).
type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(baseURL, token string) *client {
	return &client{baseURL: baseURL, token: token, http: &http.Client{}}
}

func (c *client) newRequest(ctx context.Context, method, path string, query url.Values, body any) (*http.Request, error) {
	endpoint := c.baseURL + "/api/v1" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

// do performs a one-shot request and decodes the JSON response into out
// (which may be nil to discard the body).
func (c *client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := c.newRequest(ctx, method, path, query, body)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach LogDeck server at %s: %v", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

func (c *client) post(ctx context.Context, path string, query url.Values, body, out any) error {
	return c.do(ctx, http.MethodPost, path, query, body, out)
}

func (c *client) put(ctx context.Context, path string, query url.Values, body, out any) error {
	return c.do(ctx, http.MethodPut, path, query, body, out)
}

// postRaw performs a POST and returns the status code and raw body. Used for
// endpoints (compose actions) that return a useful JSON body on failure too.
func (c *client) postRaw(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := c.newRequest(ctx, http.MethodPost, path, query, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("cannot reach LogDeck server at %s: %v", c.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}

// stream opens a streaming GET request (no client-side timeout) and returns
// the response body. The caller must close it.
func (c *client) stream(ctx context.Context, path string, query url.Values) (io.ReadCloser, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach LogDeck server at %s: %v", c.baseURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, responseError(resp)
	}
	return resp.Body, nil
}

// responseError turns a non-2xx response into an error carrying the server's
// message. 401 responses get a hint about API token authentication.
func responseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(body))

	// Server errors may be JSON like {"error": "..."} or plain text.
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Error != "" {
		message = payload.Error
	}
	if message == "" {
		message = resp.Status
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("HTTP 401: %s (%s)", message, authHint)
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
}
