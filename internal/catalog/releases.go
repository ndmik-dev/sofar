package catalog

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ndmik-dev/sofar/internal/fetch"
)

// Release is one printed volume that exists: the number, when the shop listed
// it, and where to look at it.
type Release struct {
	Volume int
	Title  string
	URL    string
	At     time.Time
}

// Series is what a search offers: a name and how many volumes are on sale.
type Series struct {
	Title   string
	Volumes int
	Newest  int
	URL     string
}

// Releases watches a Ukrainian shop for manga volumes. No catalog knows when a
// Ukrainian edition ships, and the publishers that do print it mostly have no
// API — but this shop carries all of them and runs WooCommerce with its REST
// API open.
type Releases struct {
	fetch *fetch.Client
}

const (
	ReleaseSource = "comicsmania"
	// The shop sells keychains and photocards beside the books. This category
	// is "манга українською мовою" and nothing else.
	mangaCategory = "86"
)

func NewReleases(cacheDir string) *Releases {
	return &Releases{fetch: fetch.New("https://comicsmania.shop", cacheDir, nil)}
}

type wooProduct struct {
	Date  string `json:"date"`
	Link  string `json:"link"`
	Title struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
}

// "Берсерк. Том 3", "Given, Том 3", "Tomie Vol. 2".
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

// seriesOf drops the volume from a title: "Берсерк. Том 3" is the same series
// as "Берсерк. Том 1".
func seriesOf(title string) string {
	if loc := volumePattern.FindStringIndex(title); loc != nil {
		title = title[:loc[0]]
	}
	return strings.Trim(strings.TrimSpace(title), " .,–—«»\"")
}

func (r *Releases) products(ctx context.Context, query string) ([]wooProduct, error) {
	q := url.Values{
		"search":      {query},
		"product_cat": {mangaCategory},
		"per_page":    {"60"},
		"orderby":     {"date"},
		"order":       {"desc"},
		"_fields":     {"date,link,title"},
	}
	var raw []wooProduct
	if err := r.fetch.GetJSON(ctx, "/wp-json/wp/v2/product", q, 6*time.Hour, &raw); err != nil {
		return nil, fmt.Errorf("comicsmania: %w", err)
	}
	return raw, nil
}

// Volumes returns every volume the shop lists for a query. The shop's search is
// fuzzy, so results that do not carry the query in their title are dropped: a
// watch that quietly follows the wrong series is worse than one that finds
// nothing.
func (r *Releases) Volumes(ctx context.Context, query string) ([]Release, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	raw, err := r.products(ctx, query)
	if err != nil {
		return nil, err
	}

	want := strings.ToLower(query)
	var out []Release
	for _, p := range raw {
		title := cleanTitle(p.Title.Rendered)
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

// SearchManga groups the shop's volumes into series, so adding one is picking a
// name rather than picking volume seven of it.
func (r *Releases) SearchManga(ctx context.Context, query string, limit int) ([]Found, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	raw, err := r.products(ctx, query)
	if err != nil {
		return nil, err
	}

	want := strings.ToLower(query)
	byName := map[string]*Series{}
	var order []string
	for _, p := range raw {
		title := cleanTitle(p.Title.Rendered)
		if !strings.Contains(strings.ToLower(title), want) {
			continue
		}
		vol := volumeOf(title)
		if vol == 0 {
			continue
		}
		name := seriesOf(title)
		if name == "" {
			continue
		}
		s, seen := byName[name]
		if !seen {
			s = &Series{Title: name, URL: p.Link}
			byName[name] = s
			order = append(order, name)
		}
		s.Volumes++
		if vol > s.Newest {
			s.Newest = vol
			s.URL = p.Link
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		return byName[order[i]].Newest > byName[order[j]].Newest
	})

	out := make([]Found, 0, len(order))
	for _, name := range order {
		s := byName[name]
		out = append(out, Found{
			Source: ReleaseSource,
			Kind:   "manga",
			Title:  s.Title,
			// The newest volume on sale is a suggestion, not a fact about the
			// series: it lands in an editable field like every other total.
			Total: s.Newest,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

func cleanTitle(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagPattern.ReplaceAllString(s, "")))
}
