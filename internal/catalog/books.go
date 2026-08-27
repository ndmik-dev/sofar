package catalog

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ndmik-dev/sofar/internal/fetch"
)

type Books struct {
	key   string
	fetch *fetch.Client
}

func NewBooks(key, cacheDir string) *Books {
	return &Books{key: key, fetch: fetch.New("https://www.googleapis.com/books/v1", cacheDir, nil)}
}

func (b *Books) Enabled() bool { return b.key != "" }

type booksResponse struct {
	Items []struct {
		ID         string `json:"id"`
		VolumeInfo struct {
			Title         string   `json:"title"`
			Subtitle      string   `json:"subtitle"`
			Authors       []string `json:"authors"`
			PublishedDate string   `json:"publishedDate"`
			PageCount     int      `json:"pageCount"`
			ImageLinks    struct {
				Thumbnail string `json:"thumbnail"`
			} `json:"imageLinks"`
		} `json:"volumeInfo"`
	} `json:"items"`
}

func (b *Books) Search(ctx context.Context, query string, limit int) ([]Found, error) {
	if !b.Enabled() {
		return nil, nil
	}
	var raw booksResponse
	q := url.Values{
		"q":          {query},
		"maxResults": {strconv.Itoa(min(limit*2, 20))},
		"printType":  {"books"},
		"key":        {b.key},
	}
	if err := b.fetch.GetJSON(ctx, "/volumes", q, time.Hour, &raw); err != nil {
		return nil, err
	}

	out := make([]Found, 0, limit)
	for _, it := range raw.Items {
		v := it.VolumeInfo
		if v.Title == "" {
			continue
		}
		out = append(out, Found{
			Source:   "gbooks",
			ExtID:    it.ID,
			Kind:     "book",
			Title:    v.Title,
			Subtitle: strings.Join(v.Authors, ", "),
			Year:     yearOf(v.PublishedDate),
			Total:    v.PageCount,
			Cover:    strings.Replace(v.ImageLinks.Thumbnail, "http://", "https://", 1),
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
