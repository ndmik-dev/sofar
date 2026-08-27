# Sofar

Personal media tracker: series, films, anime, books, games. Go + htmx + SQLite,
server-rendered, self-hosted, single user. See `PLAN.md` for the full plan and
the decisions already settled — read it before proposing architecture changes.

## Run

```bash
go run .                              # embedded assets, sofar.db in cwd
SOFAR_DEV=1 go run .                  # templates and static read from disk
go test ./...
```

Env: `SOFAR_ADDR` (типово `:8099`) `SOFAR_DB` `SOFAR_DEV` `SOFAR_TZ` `SOFAR_DAY_START`
`SOFAR_CACHE` `TMDB_TOKEN` `GOOGLE_BOOKS_KEY` `RAWG_KEY`. Read from `.env` when present.

## Layout

```
main.go                          config, db, server, graceful shutdown
internal/config                  env parsing, day-boundary helper
internal/fetch                   cached JSON GET shared by every catalog adapter
internal/catalog                 TMDB import plus Google Books, RAWG and iTunes
internal/store                   SQLite, migrations. No HTML, no external APIs.
  migrations/*.sql               applied in filename order, tracked in schema_migrations
internal/server                  routing, rendering, handlers
  templates/                     layout.html + one file per page/fragment
  static/                        app.css, keys.js, vendored htmx
```

## Conventions

- Comments explain *why*, never *what*. No comment is better than a redundant one.
- Every POST answers with an HTML fragment. There is no JSON API — adding one
  invites a client, versioning, and a second source of truth.
- `net/http.ServeMux` with Go 1.22 patterns (`POST /entry/{id}/advance`). No router.
- New page or fragment: add the file to `templates/` and its name to `pages` in
  `render.go`.
- No frontend build step. One hand-written `app.css` driven by the tokens at the
  top of the file. No Tailwind.
- Catalogs (TMDB and later) fill in metadata but never define progress units, and
  never become a hard dependency — the manual form must always work.

## Data model

`progress` is an append-only log and the source of truth. `entry.position` is a
cache of the last non-undone `progress.to_pos`; if they ever disagree, recompute
from the log. Streak, tempo and the year page all derive from `progress`.

`unit.idx` runs 1..N across all seasons, so `position` is a single integer
everywhere. Season boundaries come from `unit.season` changing.

A day starts at 04:00 Europe/Kyiv — use `config.Day`, never `time.Now().Day()`.
