# Sofar

Personal media tracker: series, anime, films, books, games, podcasts, courses. One list
of what is in progress, progress down to the individual episode, and an archive
of what is finished.

Self-hosted, single user, server-rendered. Go + htmx + SQLite, one binary, one
dependency, no frontend build. The interface is Ukrainian.

Status: milestones M0–M8 done, M9–M10 remaining. See [PLAN.md](PLAN.md).

## Run

```bash
go run .                 # embedded assets, sofar.db in the working directory
SOFAR_DEV=1 go run .     # templates and static read from disk, fixtures seeded
go test ./...            # live catalog tests skip without keys
```

[TESTING.md](TESTING.md) holds the manual pass to run before a deploy.

Listens on `:8099`. In dev mode an empty database is seeded with seven sample
entries so the list has something to render — which means clearing the database
brings them straight back. `SOFAR_FIXTURES=0 SOFAR_DEV=1 go run .` keeps it
empty while leaving hot reload on.

## Configuration

Read from the environment, falling back to `.env` in the working directory.
A real environment variable always wins over the file.

| Variable | Default | Purpose |
|---|---|---|
| `SOFAR_ADDR` | `:8099` | listen address |
| `SOFAR_DB` | `sofar.db` | SQLite file |
| `SOFAR_CACHE` | `cache` | on-disk cache of catalog responses |
| `SOFAR_DEV` | unset | disk templates, hot reload, fixtures |
| `SOFAR_FIXTURES` | on in dev | `0` seeds nothing, so a cleared database stays cleared |
| `SOFAR_TZ` | `Europe/Kyiv` | timezone for day boundaries |
| `SOFAR_DAY_START` | `4` | hour a day begins (see below) |
| `SOFAR_ENV_FILE` | `.env` | path to the env file |
| `SOFAR_PASSWORD` | — | the one password; **required** unless dev or loopback |
| `TMDB_TOKEN` | — | v4 read access token, films and series |
| `GOOGLE_BOOKS_KEY` | — | books |
| `RAWG_KEY` | — | games |

A podcast never finishes, so "how far have you got" has no answer for it. It is
an open counter — one press is one episode, no total — and its rows live in a
folded **Слухаю постійно** section at the bottom of the list rather than among
things with a finish line. They are never called stale: a habit does not go
stale. The year counts them as випуски, separately from серії.

A film is tracked in minutes: TMDB gives the runtime, so the position is the
minute you stopped at. Films default to status depth — switch them to
«хвилини» in settings to use it.

Podcasts use the iTunes Search API, which needs no key. Every catalog is
optional: without a key the manual entry form still works. Courses have no
catalog anywhere and are always entered by hand.

## Access

One password, no accounts. `SOFAR_PASSWORD` guards everything except `/login`,
`/static/` and `/healthz`; a correct password sets a cookie that lasts a year.
The cookie is signed with a key derived from the password, so changing the
password logs every device out — the only revocation a single-password service
can offer. Settings has a logout for one browser.

The server **refuses to start** without a password unless it is in dev mode or
listening on loopback: an instance that is quietly open is worse than one that
did not come up. Failed logins are throttled per address, read from
`CF-Connecting-IP` or `X-Forwarded-For` so a proxy does not lock out everyone at
once.

## The track

The core of the interface. Each cell is one episode, and it has four states:

| State | Meaning |
|---|---|
| filled | watched |
| accent outline with a caret | next up |
| thin coloured outline | aired, not watched yet |
| empty outline | not out yet |

The distinction between the last two is what a progress bar cannot express, and
it comes free from TMDB air dates.

Seasons collapse. Everything before the season you are in becomes a labelled
block, everything after becomes an outline, and the season you are in keeps
full-size cells. Every block is the same width: a width nobody can count
episodes from is noise pretending to be information, and the exact number is
one click away. The blocks are buttons: clicking one opens that season's
episode list in the panel, which is the only way back into a season the track
has folded away. They never move your position — a click that silently rewrote
fifty episodes would be the wrong kind of shortcut. A show with 86 episodes and one with 19 therefore render cells
of the same size. A finished show collapses entirely.

Books, films and games have no episodes, so they get a bar; a game tracked by
status only gets a three-state control instead. The bar is clickable like a
cell is: a click sets the position to that share of the total, backwards as
readily as forwards. The panel has an exact field next to it, because a minute
is hard to hit with a mouse.

## Keyboard

| Key | Action |
|---|---|
| `↑` `↓` | select a row |
| `Space` | advance by the type's step |
| `3` `Space` | advance by three — digits are typed before the space |
| `⇧Space` | back one |
| `⇧S` | jump to the end of the current season |
| `E` then `1`–`9` | rate, `0` means ten |
| `⌘⌫` | drop |
| `⌘Z` | undo the last action |
| `↵` | open or close the detail panel |
| `N` | focus the note in an open panel |
| `O` | open the entry's link in a new tab |
| `⌘K` | search or add |
| `↑` `↓` in the palette | walk the results |
| `⌥1`–`⌥4` | in progress · later · finished · year |
| `?` | shortcuts |

Both layouts are handled: a shortcut matches the letter (`E` arrives as `е` on a
Ukrainian keyboard, `⌘K` as `к`) and the physical key through `e.code`, so a
layout the letter lists miss still works. The page
shortcuts sit on `⌥` because Chrome keeps `⌘1`–`⌘9` for its own tabs and never
delivers them to a page.

Undo reverses what you watched, never the starting position you declared when
adding a title — that is setup, not an action, and it is excluded from the year
statistics for the same reason.

## Adding

`⌘K` opens one field. `↑` `↓` walk the results, `↵` takes the highlighted one.
Type prefixes narrow it by type: `с:` series, `а:` anime,
`ф:` films, `к:` books, `і:` games, `п:` podcasts, `н:` courses. Latin equivalents work too
(`s: a: m: b: g: p: c:`).

A series with more than one season asks for **season and episode**, not for an
absolute number: seasons differ in length, and working out that S4E7 is episode
46 of 86 is exactly the arithmetic the app exists to avoid. The seasons and
their lengths are shown under the field. An episode past the end of its season
resolves to that season's last one rather than being refused — the question is
where you stopped, not a quiz.

Modifiers on `↵` choose where a title lands: plain for in progress, `⇧` for
later, `⌘` for finished, `⌥` for dropped. `⌘↵` and `⇧↵` also clear the field and
keep focus, which is how the archive gets backfilled — a running log of what has
landed appears below.

## Links, freshness and the nightly job

Links belong to the entry, not to the media — where *you* watch something is not
a property of the show. Paste a URL in the panel and the label comes from the
domain; `O` opens the first one in a new tab. Only `http` and `https` are
accepted, because the value ends up in an `href`.

A nightly job runs twenty minutes after the day starts. It re-imports running
shows that somebody is actually watching, so new episodes appear on their own,
and it finally deletes entries soft-deleted more than 30 days ago. Re-import
replaces the unit list wholesale; progress lives on `entry`, so it is never
disturbed.

Rows with an unwatched episode that aired in the last two weeks move to a
**Вийшло нове** section above the list. It is derived from `unit.air_date` and
`position` — nothing is stored for it.

## Importing and fixing

`⌘K` searches your own shelf first and the catalog after: reaching a title you
own is a different job from adding one. A shelf hit links to
`/{list}?focus={id}`, which selects and scrolls to that row.

`/export` downloads everything as one JSON file — a dump, not an API: it
answers once, promises no compatibility and has no client.

`/import` takes a pasted list, one title per line, and looks each up in TMDB.
A year in brackets narrows the search, `title | 12` sets a starting position,
and the first column of a CSV works as well. An exact title beats a popular
one, which is what keeps `Друзі` from becoming a 1951 film. Everything lands as
`backfill`, so importing an archive never reads as a heroic watching day.
Sixty lines per submission, because the whole batch is one blocking round of
searches.

The panel folds an edit form at the bottom: title, author or platform, and the
total. The total is only editable while nothing is drawn from real episodes —
`unit.idx` runs 1..N and a hand-typed number would contradict the rows the
track is made of. Shrinking a total below your position clamps it, and the
clamp is written to the progress log like any other move.

A book opens a prefilled form rather than saving straight away: the catalog's
page count belongs to some edition, not necessarily yours, so it arrives in an
editable field. Games and podcasts have nothing to verify and add in one click.

## Data model

`progress` is an append-only log and the single source of truth.
`entry.position` is a cache of the last non-undone `progress.to_pos`; if the two
ever disagree, recompute from the log. Streaks, pace and yearly statistics all
derive from `progress`.

`unit.idx` runs 1..N across all seasons, so `position` is one integer
everywhere. Season boundaries come from `unit.season` changing. Specials
(TMDB season 0) are excluded, because including them would shift that numbering.

Deletion is soft (`entry.deleted_at`) so it can be undone like everything else.

The theme follows the operating system. **Налаштування** can pin light or dark;
the choice lives in `localStorage`, never on the server, because it belongs to
the browser rather than to the data.

A day starts at 04:00 — an episode watched at 01:30 belongs to the evening
before. Use `config.Day`, never `time.Now().Day()`.

Tables: `users`, `media`, `unit`, `entry`, `progress`, `type_settings`,
`streak_freeze`. Full schema in [PLAN.md](PLAN.md); migrations are plain SQL in
`internal/store/migrations`, applied in filename order.

## Layout

```
main.go              config, database, server, graceful shutdown
internal/config      environment parsing, .env reader, day boundary
internal/domain      track building, pluralisation. No I/O.
internal/nightly     refresh running shows, purge old deletions
internal/fetch       cached JSON GET shared by every catalog adapter
internal/tmdb        films and series
internal/catalog     TMDB import plus Google Books, RAWG, iTunes
internal/store       SQLite and queries. No HTML, no external APIs.
internal/server      routing, rendering, handlers
  templates/         layout plus one file per page or fragment
  static/            app.css, keys.js, vendored htmx
```

## Conventions

- Comments explain *why*, never *what*. No comment beats a redundant one.
- Every POST answers with an HTML fragment. There is no JSON API — adding one
  invites a client, versioning and a second source of truth.
- `net/http.ServeMux` with Go 1.22 patterns. No router.
- No frontend build step. One hand-written `app.css` driven by tokens at the top
  of the file.
- Catalogs fill in metadata but never define progress units, and never become a
  hard dependency: the manual form always works.
- New page or fragment: add the file to `templates/` and its name to `pages` or
  `partials` in `render.go`.

## Routes

```
GET  /                      → 302 /active
GET  /active /backlog /done /dropped     ?kind= filters by type
GET  /year /year/{y}        statistics for a year
GET  /settings              per-type tracking depth
POST /settings/{kind}
GET  /import                paste a list of titles
POST /import
GET  /export                everything as one JSON file
GET  /login
POST /login
POST /logout
GET  /healthz
GET  /search?q=             ⌘K results fragment
GET  /manual?...            manual form, prefilled from a catalog hit
POST /add                   from TMDB
POST /add/manual            manual entry, optionally carrying catalog metadata
POST /entry/{id}/advance    n=1 | abs=<idx>
POST /entry/{id}/undo
POST /entry/{id}/status
POST /entry/{id}/rating
POST /entry/{id}/delete
POST /entry/{id}/restore
POST /entry/{id}/note
POST /entry/{id}/edit       title, subtitle, total
POST /entry/{id}/link
POST /entry/{id}/link/{link}/delete
GET  /entry/{id}/panel      detail panel fragment
```

## Deploy

```bash
docker compose up --build
```

Multi-stage into distroless, `CGO_ENABLED=0`, runs as nonroot, database on a
`/data` volume. About 14 MB of binary and one dependency
(`modernc.org/sqlite`, a pure-Go driver, which is what keeps cgo out).

`/data` is created in the image owned by uid 65532. A fresh volume inherits the
ownership of the directory it covers, and without that the nonroot process
cannot create the database on first start.

The image has no shell, so `-healthcheck` makes the binary call its own
`/healthz` — that is what the compose healthcheck runs.

On Dokploy the service is a **Compose** application: `dokploy-network` is
external and joined by the service, the domain is added in the Domains tab
(Dokploy writes the Traefik labels itself, service `sofar`, port `8080`), and
the keys go in the Environment tab.

## Attribution

This product uses the TMDB API but is not endorsed or certified by TMDB. Game
data comes from RAWG, book data from Google Books, podcast data from the iTunes
Search API. The application displays the attribution these terms require.
