package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is a small cached JSON getter shared by every catalog adapter. It
// exists so a provider file contains only the shape of its API, nothing else.
type Client struct {
	base    string
	http    *http.Client
	cache   *Cache
	gate    chan struct{}
	headers map[string]string
}

func New(base, cacheDir string, headers map[string]string) *Client {
	return &Client{
		base:    base,
		http:    &http.Client{Timeout: 15 * time.Second},
		cache:   NewCache(cacheDir),
		gate:    make(chan struct{}, 4),
		headers: headers,
	}
}

func (c *Client) GetJSON(ctx context.Context, path string, q url.Values, ttl time.Duration, out any) error {
	full := path
	if len(q) > 0 {
		full += "?" + q.Encode()
	}

	if body, ok := c.cache.Get(c.base+full, ttl); ok {
		return json.Unmarshal(body, out)
	}

	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+full, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := ReadLimited(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: %s: %s", path, resp.Status, Snippet(body))
	}

	c.cache.Put(c.base+full, body)
	return json.Unmarshal(body, out)
}
