# Sofar

Personal media tracker: series, anime, films, books, games, podcasts, courses.
Go + htmx + SQLite, server-rendered, self-hosted, single user.

Read [README.md](README.md) for what it is and how to run it, and
[PLAN.md](PLAN.md) for settled decisions before proposing architecture changes.

## Language

Code, comments, commits and documentation are **English**. Interface strings,
user-facing errors and toasts are **Ukrainian** — that is the product, not a
translation.

## Run

```bash
SOFAR_DEV=1 go run .     # disk templates, hot reload, fixtures on an empty db
go test ./...            # live catalog tests skip without keys
```

Dev mode reloads templates but not Go code — an old binary with new templates
produces a 500 that looks like a bug. Restart after Go changes.

## Conventions

- Comments explain *why*, never *what*. No comment beats a redundant one.
- Every POST answers with an HTML fragment. There is no JSON API — adding one
  invites a client, versioning and a second source of truth.
- `net/http.ServeMux` with Go 1.22 patterns (`POST /entry/{id}/advance`).
- New page or fragment: add the file to `templates/` and its name to `pages` or
  `partials` in `render.go`.
- No frontend build step. One hand-written `app.css` driven by the tokens at the
  top of the file. No Tailwind.
- Catalogs fill in metadata but never define progress units, and never become a
  hard dependency — the manual form must always work.
- Commit messages: lowercase, short, no co-author trailer. Commit as you go.

## Invariants

`progress` is an append-only log and the source of truth. `entry.position` is a
cache of the last non-undone `progress.to_pos`; if they ever disagree, recompute
from the log. Streaks, pace and yearly figures all derive from `progress`.

`unit.idx` runs 1..N across all seasons, so `position` is one integer everywhere.
Season boundaries come from `unit.season` changing. TMDB season 0 holds specials
and is excluded — including it would shift that numbering.

Deletion is soft (`entry.deleted_at`) so it can be undone like everything else.
Every query filters `deleted_at IS NULL`.

A day starts at 04:00 Europe/Kyiv — use `config.Day`, never `time.Now().Day()`.

## Client-side gotchas

Shortcuts match both the letter and `e.code`, because a Ukrainian layout sends
different letters and Chrome keeps `⌘1`–`⌘9` for its own tabs. htmx keeps moving
nodes during `afterSwap`; client state such as the selected
row must be re-applied on `afterSettle`. Keyboard handlers accept both Latin and
Cyrillic key values — `⌘K` arrives as `к` on a Ukrainian layout.
