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

	body, err := c.doWithRetry(ctx, req, path)
	if err != nil {
		return err
	}

	c.cache.Put(c.base+full, body)
	return json.Unmarshal(body, out)
}

// Google Books hands out a 503 every so often for no reason. One quiet retry
// turns that blip into nothing, instead of a "catalog is down" message the user
// has to think about.
func (c *Client) doWithRetry(ctx context.Context, req *http.Request, path string) ([]byte, error) {
	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := c.http.Do(req.Clone(ctx))
		if err != nil {
			lastErr = fmt.Errorf("get %s: %w", path, err)
			continue
		}

		body, readErr := ReadLimited(resp)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusOK {
			return body, nil
		}

		lastErr = fmt.Errorf("get %s: %s: %s", path, resp.Status, Snippet(body))
		if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return nil, lastErr
		}
	}
	return nil, lastErr
}
