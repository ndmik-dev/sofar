package catalog

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ndmik-dev/sofar/internal/fetch"
)

// Release is one printed volume that exists: the number, when the publisher
// listed it, and where to look at it.
type Release struct {
	Volume int
	Title  string
	URL    string
	At     time.Time
}

// Releases watches a Ukrainian publisher's shop for new volumes. No catalog
// knows when a Ukrainian edition ships — the publisher's own listing is the
// only source there is, and this one happens to run WooCommerce with its REST
// API left open.
type Releases struct {
	fetch *fetch.Client
}

const ReleaseSource = "nashaidea"

func NewReleases(cacheDir string) *Releases {
	return &Releases{fetch: fetch.New("https://nashaidea.com", cacheDir, nil)}
}

type wooProduct struct {
	Date  string `json:"date"`
	Link  string `json:"link"`
	Title struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
}

// "Given, Том 3", "Дрон-Сіті Том 1 Робовечірка", "Tomie Vol. 2".
var volumePattern = regexp.MustCompile(`(?i)(?:том|vol\.?)\s*(\d{1,3})`)

func volumeOf(title string) int {
	m := volumePattern.FindStringSubmatch(title)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// Volumes returns every volume the shop lists for a query, newest first. The
// search is fuzzy on the shop's side, so results that do not carry the query in
// their title are dropped: a watch that quietly follows the wrong series is
// worse than one that finds nothing.
func (r *Releases) Volumes(ctx context.Context, query string) ([]Release, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	q := url.Values{
		"search":   {query},
		"per_page": {"30"},
		"orderby":  {"date"},
		"order":    {"desc"},
	}
	var raw []wooProduct
	if err := r.fetch.GetJSON(ctx, "/wp-json/wp/v2/product", q, 6*time.Hour, &raw); err != nil {
		return nil, fmt.Errorf("nashaidea: %w", err)
	}

	want := strings.ToLower(query)
	var out []Release
	for _, p := range raw {
		title := strings.TrimSpace(html.UnescapeString(stripTags(p.Title.Rendered)))
		if !strings.Contains(strings.ToLower(title), want) {
			continue
		}
		vol := volumeOf(title)
		if vol == 0 {
			continue
		}
		at, err := time.Parse("2006-01-02T15:04:05", p.Date)
		if err != nil {
			at = time.Time{}
		}
		out = append(out, Release{Volume: vol, Title: title, URL: p.Link, At: at})
	}
	return out, nil
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string { return tagPattern.ReplaceAllString(s, "") }
