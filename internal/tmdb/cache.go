package tmdb

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const maxBody = 4 << 20

type Cache struct {
	dir string
}

func NewCache(dir string) *Cache { return &Cache{dir: dir} }

// Get returns a cached body when it is younger than ttl. Cache misses and
// unreadable files are the same thing to the caller: fetch it again.
func (c *Cache) Get(key string, ttl time.Duration) ([]byte, bool) {
	if c.dir == "" {
		return nil, false
	}
	path := c.path(key)
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > ttl {
		return nil, false
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return body, true
}

// Put is best-effort: a failed write costs one extra API call, never a request.
func (c *Cache) Put(key string, body []byte) {
	if c.dir == "" {
		return
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return
	}
	path := c.path(key)
	tmp, err := os.CreateTemp(c.dir, "tmp-*")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmp.Name(), path)
}

// The key is hashed rather than slugified so a token accidentally present in a
// query string can never end up as a filename.
func (c *Cache) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:16])+".json")
}

func readLimited(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) > maxBody {
		return nil, errors.New("tmdb response too large")
	}
	return body, nil
}

func snippet(body []byte) string {
	const n = 200
	if len(body) > n {
		return string(body[:n]) + "…"
	}
	return string(body)
}
