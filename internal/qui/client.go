// Package qui is a minimal client for the qui HTTP API — the qBittorrent
// WebUI front end. The daemon uses it to aggregate torrents across all
// qui-managed instances for the dashboard overview. qui runs locally with
// auth disabled for 127.0.0.1, so no API key is sent.
package qui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	api "github.com/ogglord/homelab-api"
)

// pageLimit is the per-request torrent page size. qui paginates; we loop
// until we have every torrent for an instance.
const pageLimit = 200

// Client talks to a single qui instance over HTTP.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client for the qui base URL (e.g. http://127.0.0.1:7476).
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Instance is a qui-managed qBittorrent instance (subset of qui's schema).
type Instance struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// torrent mirrors qui's Torrent schema (camelCase speeds). Kept unexported;
// callers receive normalized api.QbitTorrent.
type torrent struct {
	Hash     string  `json:"hash"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	DlSpeed  int64   `json:"dlSpeed"`
	UpSpeed  int64   `json:"upSpeed"`
	Eta      int64   `json:"eta"`
	State    string  `json:"state"`
}

// torrentsResponse is qui's paginated torrent list envelope.
type torrentsResponse struct {
	Torrents []torrent `json:"torrents"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
}

// AllTorrents returns every torrent across all instances, tagged with its
// instance name and sorted by download speed descending (active first).
func (c *Client) AllTorrents(ctx context.Context) ([]api.QbitTorrent, error) {
	instances, err := c.listInstances(ctx)
	if err != nil {
		return nil, err
	}

	out := []api.QbitTorrent{}
	for _, inst := range instances {
		ts, err := c.instanceTorrents(ctx, inst.ID)
		if err != nil {
			return nil, fmt.Errorf("instance %q: %w", inst.Name, err)
		}
		for _, t := range ts {
			out = append(out, api.QbitTorrent{
				Hash:     t.Hash,
				Name:     t.Name,
				Size:     t.Size,
				Progress: t.Progress,
				Dlspeed:  t.DlSpeed,
				Upspeed:  t.UpSpeed,
				Eta:      t.Eta,
				State:    t.State,
				Instance: inst.Name,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Dlspeed > out[j].Dlspeed
	})
	return out, nil
}

// listInstances fetches all qui-managed instances.
func (c *Client) listInstances(ctx context.Context) ([]Instance, error) {
	var instances []Instance
	if err := c.getJSON(ctx, "/api/instances", &instances); err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	return instances, nil
}

// instanceTorrents pages through every torrent for one instance, sorted by
// download speed descending at the source.
func (c *Client) instanceTorrents(ctx context.Context, id int) ([]torrent, error) {
	var all []torrent
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("limit", strconv.Itoa(pageLimit))
		q.Set("sort", "dlspeed")
		q.Set("order", "desc")

		var resp torrentsResponse
		path := fmt.Sprintf("/api/instances/%d/torrents?%s", id, q.Encode())
		if err := c.getJSON(ctx, path, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Torrents...)
		if len(resp.Torrents) == 0 || len(all) >= resp.Total {
			break
		}
	}
	return all, nil
}

// getJSON performs a GET against the qui base URL and decodes the JSON body.
func (c *Client) getJSON(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qui GET %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
