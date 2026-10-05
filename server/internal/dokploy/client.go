// Package dokploy implements Dokploy's REST API, independently of Docker editing.
package dokploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

type Client struct {
	apiURL   string
	token    string
	serverID string
	http     *http.Client
}

type MultiClient struct{ clients map[string]*Client }

func NewClient(cfg config.DokployHostConfig) *Client {
	return &Client{apiURL: strings.TrimSuffix(strings.TrimRight(cfg.APIURL, "/"), "/api"), token: cfg.APIToken, serverID: cfg.ServerID, http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}
func NewMultiClient(hosts []config.DokployHostConfig) *MultiClient {
	if len(hosts) == 0 {
		return nil
	}
	m := &MultiClient{clients: make(map[string]*Client)}
	for _, h := range hosts {
		m.clients[h.HostName] = NewClient(h)
	}
	return m
}
func (m *MultiClient) GetClient(host string) *Client {
	if m == nil {
		return nil
	}
	return m.clients[host]
}

func (c *Client) request(ctx context.Context, method, endpoint string, body any, result any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+"/api/"+endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("dokploy request could not be confirmed; reload configuration before retrying: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch resp.StatusCode {
		case 401:
			return fmt.Errorf("dokploy rejected the API token (401)")
		case 403:
			return fmt.Errorf("dokploy denied permission for %s (403)", strings.Split(endpoint, "?")[0])
		case 404:
			return fmt.Errorf("dokploy resource or operation is unavailable (404)")
		default:
			return fmt.Errorf("dokploy %s returned status %d; reload to check whether configuration changed", strings.Split(endpoint, "?")[0], resp.StatusCode)
		}
	}
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("dokploy returned an unreadable response: %w", err)
		}
	}
	return nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.Resources(ctx)
	return err
}
