package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nixomose/zfstool/internal/api"
)

var _ api.Client = (*HTTPClient)(nil)

// HTTPClient talks to the zfstool agent over HTTP (Unix socket or TCP).
type HTTPClient struct {
	base    *url.URL
	client  *http.Client
	headers map[string]string
}

// NewUnixHTTPClient dials unix socket at socketPath.
func NewUnixHTTPClient(socketPath string) *HTTPClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	u, _ := url.Parse("http://localhost")
	return &HTTPClient{
		base: u,
		client: &http.Client{
			Transport: transport,
			Timeout:   120 * time.Second,
		},
		headers: map[string]string{},
	}
}

// NewTCPHTTPClient connects to host like http://127.0.0.1:8787
func NewTCPHTTPClient(hostURL string) (*HTTPClient, error) {
	u, err := url.Parse(hostURL)
	if err != nil {
		return nil, err
	}
	return &HTTPClient{
		base: u,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		headers: map[string]string{},
	}, nil
}

func (c *HTTPClient) absPath(path string) string {
	r := c.base.ResolveReference(&url.URL{Path: path})
	return r.String()
}

func (c *HTTPClient) get(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.absPath(path), nil)
	if err != nil {
		return err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

func (c *HTTPClient) postJSON(ctx context.Context, path string, payload, out interface{}) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.absPath(path), bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// GetHost fetches /v1/host.
func (c *HTTPClient) GetHost(ctx context.Context) (*api.HostInfo, error) {
	var h api.HostInfo
	if err := c.get(ctx, "/v1/host", &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// ListPools fetches /v1/pools.
func (c *HTTPClient) ListPools(ctx context.Context) ([]api.PoolSummary, error) {
	var pools []api.PoolSummary
	if err := c.get(ctx, "/v1/pools", &pools); err != nil {
		return nil, err
	}
	return pools, nil
}

// PoolStatus fetches pool status.
func (c *HTTPClient) PoolStatus(ctx context.Context, pool string) (*api.PoolStatus, error) {
	var st api.PoolStatus
	path := "/v1/pools/" + url.PathEscape(pool) + "/status"
	if err := c.get(ctx, path, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// PoolHistory fetches paginated history.
func (c *HTTPClient) PoolHistory(ctx context.Context, pool string, offset, limit int) ([]api.PoolHistoryEntry, error) {
	q := fmt.Sprintf("/v1/pools/%s/history?offset=%d&limit=%d", url.PathEscape(pool), offset, limit)
	var e []api.PoolHistoryEntry
	if err := c.get(ctx, q, &e); err != nil {
		return nil, err
	}
	return e, nil
}

// ListDatasets lists datasets.
func (c *HTTPClient) ListDatasets(ctx context.Context, pool string) ([]api.DatasetRow, error) {
	path := "/v1/datasets"
	if pool != "" {
		path += "?pool=" + url.QueryEscape(pool)
	}
	var rows []api.DatasetRow
	if err := c.get(ctx, path, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ListDisks fetches block devices (all disks, with partitions and pool membership).
func (c *HTTPClient) ListDisks(ctx context.Context) ([]api.DiskSummary, error) {
	var disks []api.DiskSummary
	if err := c.get(ctx, "/v1/disks", &disks); err != nil {
		return nil, err
	}
	return disks, nil
}

// ListMounts fetches mounted filesystems including non-ZFS volumes.
func (c *HTTPClient) ListMounts(ctx context.Context) ([]api.MountEntry, error) {
	var mounts []api.MountEntry
	if err := c.get(ctx, "/v1/mounts", &mounts); err != nil {
		return nil, err
	}
	return mounts, nil
}
