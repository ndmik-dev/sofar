package server

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ndmik-dev/sofar/internal/domain"
	"github.com/ndmik-dev/sofar/internal/store"
)

// Building a row, a toast or the summary out of an entry.

// Asking "where did you stop" only makes sense for something you are actually
// watching, that has units to count, and that is still at zero.
func positionStepFor(e store.Entry, created bool) *positionStep {
	if !created || e.Status != "active" || e.Depth == "status" {
		return nil
	}
	if !e.Media.TotalUnits.Valid || e.Position > 0 {
		return nil
	}
	total := int(e.Media.TotalUnits.Int64)
	if total <= 1 {
		return nil
	}
	unit := domain.UnitMany(e.Media.Unit)
	hint := fmt.Sprintf("усього %d", total)
	if unit != "" {
		hint += " " + unit
	}
	step := &positionStep{
		EntryID: e.ID,
		Title:   e.Media.Title,
		Total:   total,
		Unit:    unit,
		Hint:    hint,
	}

	// Asking for an absolute episode number is asking someone to add up season
	// lengths in their head. With more than one season, ask the way people
	// actually remember it.
	if seasons := domain.Seasons(e.Units); len(seasons) > 1 {
		step.BySeason = true
		step.Seasons = seasons
		last := seasons[len(seasons)-1]
		step.LastS, step.LastE = last.Number, last.Episodes
	}
	return step
}

func unitWords(e store.Entry) (one, few, many string) {
	f, ok := domain.Units[e.Media.Unit]
	if !ok {
		return "", "", ""
	}
	return f.One, f.Few, f.Many
}

func toastFor(move store.Move) toastView {
	if !move.Changed {
		return toastView{}
	}
	one, few, many := unitWords(move.Entry)
	switch {
	case move.To < move.From:
		return toastView{
			Text:    fmt.Sprintf("%s · назад на %d", move.Entry.Media.Title, move.To),
			EntryID: move.Entry.ID,
			Undo:    true,
		}
	case move.Finished:
		return toastView{Text: move.Entry.Media.Title + " · завершено", EntryID: move.Entry.ID, Undo: true}
	case one != "":
		return toastView{
			Text:    move.Entry.Media.Title + " · " + domain.Count(move.To, one, few, many),
			EntryID: move.Entry.ID,
			Undo:    true,
		}
	default:
		return toastView{Text: move.Entry.Media.Title + " · оновлено", EntryID: move.Entry.ID}
	}
}

func summaryFrom(s store.Summary) summaryView {
	return summaryView{
		Render:      true,
		Off:         s.Active == 0,
		Waiting:     s.Waiting,
		WaitingTime: domain.HoursMins(s.WaitingMins),
		Airing:      s.Airing,
		Active:      s.Active,
	}
}

// nextUp names the episode to watch now. A failure here costs one line of
// the page, not the page, so it is logged and swallowed.
func (s *Server) nextUp(ctx context.Context, today string, staleBefore int64) *nextUp {
	id, err := s.store.NextUp(ctx, defaultUserID, today, staleBefore)
	if err != nil {
		s.log.Warn("next up", "err", err)
		return nil
	}
	if id == 0 {
		return nil
	}
	e, err := s.store.GetEntry(ctx, id)
	if err != nil {
		return nil
	}
	for _, u := range e.Units {
		if u.Idx != e.Position+1 {
			continue
		}
		label := domain.Label(u, domain.MultiSeason(e.Units))
		if u.Title != "" {
			label += " «" + u.Title + "»"
		}
		return &nextUp{EntryID: e.ID, Title: e.Media.Title, Label: label, Mins: u.Runtime}
	}
	return nil
}

// freshVolume answers the same question JustAired answers for episodes: has
// something you have not read turned up lately.
func freshVolume(e store.Entry, now time.Time) (string, bool) {
	if e.WatchVol <= e.Position || e.WatchFoundAt == 0 {
		return "", false
	}
	if now.Sub(time.Unix(e.WatchFoundAt, 0)) > domain.FreshDays*24*time.Hour {
		return "", false
	}
	return fmt.Sprintf("вийшов том %d", e.WatchVol), true
}

// volumeTrack is true for something counted in volumes with a known count and
// no unit rows of its own to draw from.
func volumeTrack(e store.Entry) bool {
	return e.Media.Unit == "volume" && len(e.Units) == 0 &&
		e.Media.TotalUnits.Valid && e.Media.TotalUnits.Int64 > 1
}

func buildRow(e store.Entry, today string) row {
	rw := row{
		EntryID:   e.ID,
		Title:     e.Media.Title,
		Rating:    int(e.Rating.Int64),
		Kind:      e.Media.Kind,
		KindLabel: kindLabels[e.Media.Kind],
		Step:      max(e.Step, 1),
		LinkURL:   e.LinkURL,
		Btn:       "+",
	}
	if rw.Step > 1 {
		// «+10» beside a bare «+» on every other row is an unexplained
		// number; «+10 стор.» is a step.
		rw.Btn = fmt.Sprintf("+%d %s", rw.Step, domain.Units[e.Media.Unit].Short)
		rw.BtnWide = true
	}

	total := int(e.Media.TotalUnits.Int64)
	hasTotal := e.Media.TotalUnits.Valid
	rw.Total = total

	switch {
	case e.Depth == "status":
		rw.Mode = "status"
		rw.Status = e.Status
		rw.Btn = "✓"
		rw.Pos = "—"
		rw.Sub = e.Media.TitleOrig
		rw.Statuses = []statusOption{
			{"backlog", "хочу", e.Status == "backlog"},
			{"active", "у процесі", e.Status == "active"},
			{"done", "завершив", e.Status == "done"},
		}
	case !hasTotal:
		rw.Mode = "open"
		rw.Pos = strconv.Itoa(e.Position)
		rw.PosSub = "без межі"
		rw.Sub = e.Media.TitleOrig
	case len(e.Units) > 0:
		rw.Mode = "cells"
		rw.Blocks = domain.BuildTiered(e.Units, e.Position, today)
		rw.SeasonEnd = domain.CurrentSeasonEnd(rw.Blocks)
		rw.Pos = fmt.Sprintf("%d / %d", e.Position, total)
		rw.PosSub = remainingLabel(domain.RemainingFrom(e.Units, e.Position, today))
		if next := domain.NextLabel(e.Units, e.Position); next != "" {
			rw.Sub = "далі " + next
		} else {
			rw.Sub = "усе переглянуто"
		}
	case volumeTrack(e):
		// Volumes are countable things you finish one by one, which is what a
		// track is for. A solid bar hides that a series has thirteen of them.
		rw.Mode = "cells"
		rw.Blocks = domain.BuildVolumes(total, e.Position)
		rw.SeasonEnd = total
		rw.Pos = fmt.Sprintf("%d / %d", e.Position, total)
		rw.PosSub = domain.Count(total-e.Position, "том", "томи", "томів") + " лишилось"
		if e.Position >= total {
			rw.PosSub = "усе прочитано"
		}
		rw.Sub = e.Media.TitleOrig
	default:
		rw.Mode = "bar"
		if total > 0 {
			rw.Percent = e.Position * 100 / total
		}
		rw.Pos = fmt.Sprintf("%d / %d", e.Position, total)
		rw.PosSub = fmt.Sprintf("%d%%", rw.Percent)
		rw.Sub = e.Media.TitleOrig
	}

	return rw
}

func remainingLabel(rem domain.Remaining) string {
	if rem.Left == 0 {
		return "завершено"
	}
	if rem.Waiting > 0 && rem.Waiting < rem.Left {
		return domain.Count(rem.Waiting, "чекає", "чекають", "чекають")
	}
	if t := domain.HoursMins(rem.LeftMins); t != "" {
		return fmt.Sprintf("%d · %s", rem.Left, t)
	}
	return fmt.Sprintf("лишилось %d", rem.Left)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
