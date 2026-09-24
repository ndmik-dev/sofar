package server

import (
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
)

// View types: what the templates see. Nothing here talks to the store.

var statusWords = map[string]string{
	"active":  "у процесі",
	"backlog": "у «колись»",
	"done":    "у завершених",
	"dropped": "у кинутих",
}

type navItem struct {
	Key    string
	Label  string
	Href   string
	Count  int
	Active bool
}

type kindFilter struct {
	Label string
	Kind  string
	Count int
	Clear string
}

type kindItem struct {
	Label string
	Kind  string
	Href  string
	Count int
	On    bool
}

type row struct {
	EntryID   int64
	Title     string
	Sub       string
	Kind      string
	KindLabel string
	Mode      string
	Blocks    []domain.Block
	Percent   int
	Status    string
	Pos       string
	PosSub    string
	Total     int
	Btn       string
	BtnWide   bool
	Rating    int
	Step      int
	SeasonEnd int
	Statuses  []statusOption
	Aired     string
	LinkURL   string
	Stale     bool
	Selected  bool
}

type summaryView struct {
	Waiting     int
	WaitingTime string
	Airing      int
	Active      int
	// What to watch if you sat down now. Information, not a button.
	Next   *nextUp
	OOB    bool
	Render bool
	Off    bool
}

type nextUp struct {
	EntryID int64
	Title   string
	Label   string
	Mins    int
}

type statusOption struct {
	Value string
	Label string
	On    bool
}

type toastView struct {
	Text       string
	EntryID    int64
	Undo       bool
	Restore    bool
	BackStatus string
}

type removedRow struct {
	EntryID int64
	Toast   toastView
	Summary summaryView
	Nav     navView
}

type navView struct {
	Nav   []navItem
	Kinds []kindItem
	Kind  string
	Base  string
	OOB   bool
}

type addedRow struct {
	Row     row
	Toast   toastView
	Summary summaryView
	Nav     navView
	Step    *positionStep
	Flow    *flowItem
	ShowRow bool
	RowOOB  string
}

type flowItem struct {
	Title  string
	Kind   string
	Status string
	Label  string
}

type positionStep struct {
	EntryID  int64
	Title    string
	Total    int
	Unit     string
	Hint     string
	BySeason bool
	Seasons  []domain.Season
	LastS    int
	LastE    int
}

type listPage struct {
	Title string
	// Nothing in this list at all, as opposed to nothing under this bucket.
	Bare    bool
	NavView navView
	Now     time.Time
	Rows    []row
	Fresh   []row
	Stale   []row
	Years   []yearGroup
	Filters []filterChip
	Filter  *kindFilter
	Summary summaryView
	Empty   bool
}

type moveResponse struct {
	Row     row
	Summary summaryView
	Nav     navView
	Toast   toastView
}

func navViewFrom(statusCounts, kindCounts map[string]int, active string, oob bool) navView {
	return navViewWithKind(statusCounts, kindCounts, active, "", oob)
}

func navViewWithKind(statusCounts, kindCounts map[string]int, active, kind string, oob bool) navView {
	nv := navView{OOB: oob, Kind: kind, Base: active}
	kindBase := active
	if _, isList := pathStatus[active]; !isList {
		// Year, settings and import do not filter by kind, so their type links
		// lead back to the main list — and the counts beside them must describe
		// that list, not everything on the shelf.
		kindBase = "/active"
	}
	for _, n := range []struct{ Label, Href, Key string }{
		{"У процесі", "/active", "active"},
		{"Колись", "/backlog", "backlog"},
		{"Завершено", "/done", "done"},
		{"Кинуто", "/dropped", "dropped"},
	} {
		nv.Nav = append(nv.Nav, navItem{
			Key: n.Key, Label: n.Label, Href: n.Href, Count: statusCounts[n.Key], Active: n.Href == active,
		})
	}
	for _, k := range kinds {
		href := kindBase + "?kind=" + k.Kind
		if k.Kind == kind {
			href = kindBase
		}
		nv.Kinds = append(nv.Kinds, kindItem{
			Label: k.Nav, Kind: k.Kind, Href: href,
			Count: kindCounts[k.Kind], On: k.Kind == kind,
		})
	}
	return nv
}

var statusPaths = map[string]string{
	"active": "/active", "backlog": "/backlog",
	"done": "/done", "dropped": "/dropped",
}

var pathStatus = map[string]string{
	"/active": "active", "/backlog": "backlog",
	"/done": "done", "/dropped": "dropped",
}
