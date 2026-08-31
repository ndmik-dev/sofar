package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ndmik-dev/sofar/internal/catalog"
	"github.com/ndmik-dev/sofar/internal/store"
)

type watchView struct {
	On        bool
	Query     string
	LastVol   int
	LastTitle string
	LastURL   string
	Found     string
	Checked   string
}

func buildWatch(w store.Watch, on bool, loc *time.Location) watchView {
	v := watchView{On: on, Query: w.Query, LastVol: w.LastVol,
		LastTitle: w.LastTitle, LastURL: w.LastURL}
	if w.FoundAt > 0 {
		v.Found = time.Unix(w.FoundAt, 0).In(loc).Format("02.01.2006")
	}
	if w.CheckedAt > 0 {
		v.Checked = time.Unix(w.CheckedAt, 0).In(loc).Format("02.01.2006")
	}
	return v
}

// handleWatch starts following a series. The query is what the publisher's shop
// is asked for, so it is editable: "Given" finds it, the full Ukrainian subtitle
// would not.
func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	query := strings.TrimSpace(r.FormValue("query"))
	if query == "" {
		http.Error(w, "Порожній запит нічого не знайде", http.StatusBadRequest)
		return
	}

	now, today, _ := s.now()
	// Check the entry exists first: without this a missing id reaches SQLite
	// and comes back as a foreign-key failure, which is a 500 for what is
	// plainly a 404.
	if _, err := s.store.GetEntry(r.Context(), id); err != nil {
		s.failEntry(w, r, err)
		return
	}
	if err := s.store.SetWatch(r.Context(), id, catalog.ReleaseSource, query, now); err != nil {
		s.fail(w, r, err)
		return
	}

	// Check once immediately: a watch that says nothing until tomorrow morning
	// looks like it did not take.
	text := "Стежу за «" + query + "»"
	if vols, err := s.catalog.Volumes(r.Context(), query); err != nil {
		s.log.Warn("first watch check", "query", query, "err", err)
		text = "Стежу за «" + query + "» · видавця зараз не чути"
	} else if newest, found := newestVolume(vols); found {
		if err := s.store.RecordRelease(r.Context(), id, newest.Volume, newest.Title, newest.URL, now); err != nil {
			s.fail(w, r, err)
			return
		}
		text = "Стежу · зараз у продажу том " + strconv.Itoa(newest.Volume)
	} else {
		text = "Стежу за «" + query + "» · томів поки не знайшов"
	}

	s.respondEntry(w, r, id, today, toastView{Text: text})
}

func (s *Server) handleUnwatch(w http.ResponseWriter, r *http.Request) {
	id, ok := s.entryID(w, r)
	if !ok {
		return
	}
	if err := s.store.DropWatch(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	_, today, _ := s.now()
	s.respondEntry(w, r, id, today, toastView{Text: "Більше не стежу"})
}

func newestVolume(vols []catalog.Release) (catalog.Release, bool) {
	var best catalog.Release
	for _, v := range vols {
		if v.Volume > best.Volume {
			best = v
		}
	}
	return best, best.Volume > 0
}
