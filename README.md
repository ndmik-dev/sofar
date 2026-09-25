# Sofar

A personal media tracker: series, anime, films, books, manga, games and
courses on one shelf, with progress down to the episode, the page or the
minute you stopped at. Self-hosted, one binary, Go + htmx + SQLite. The
interface is Ukrainian.

![The main list: figures on top, then every title with its progress](docs/screens/active.png)

Every tracker answers "have you seen it?". Sofar answers the evening
question: **what is waiting for me, and where did I stop?** The first thing
on the screen is how many episodes have aired that you have not watched, how
long they would take, and which one to start with. Below it, progress is a
shape you read before you read the title.

## The track

Each cell is one episode. Filled — watched. Outlined with a caret — next.
Coloured outline — aired, waiting. Empty — not out yet. That last distinction
is what a progress bar cannot show, and it comes free from air dates.

Finished seasons collapse into a block, the season you are in keeps full-size
cells. Click a cell to set your position there; click a block to open that
season. Books and films get a bar you can click the same way, manga gets a
cell per volume, anything you only track by status gets three buttons.

## The panel

![The detail panel: list, progress, next episode, description, links](docs/screens/panel.png)

`↵` or a click on the title. Rating, which list it is on, the full track
with season tabs, what to watch next and how long is left, description, links
to where you watch it, a note.

## Adding

![The palette: your shelf first, then TMDB](docs/screens/palette.png)

`⌘K` is one field: your own shelf first, then the catalog. A prefix narrows
the type — `с:` series, `а:` anime, `ф:` films, `к:` books, `м:` manga,
`і:` games, `н:` courses. `↵` adds to «у процесі», `⇧↵` to «колись», `⌘↵` to
«завершено» and keeps the field open — that is how an archive gets typed in.

A series asks *season and episode*, not an absolute number. A book arrives as
a prefilled form, because the catalog's page count belongs to some edition,
not necessarily yours. Manga is searched in a Ukrainian shop, so the result is
what is actually printed here — and the same shop is asked every night whether
a new volume is out.

## The year

![The year: hours, episodes, books, streak, a heatmap and months](docs/screens/year.png)

All of it derives from the progress log. Nothing is entered by hand, and the
position you declared when adding a title does not count — that is setup, not
watching.

## Also

- **Catalogs fill in metadata, never define progress.** TMDB, Google Books,
  RAWG, ComicsMania — all optional; the manual form always works.
- **What aired.** New episodes in the last two weeks move to «Вийшло нове». A
  nightly job re-imports running shows, so they appear on their own.
- **Undo everything.** Progress is an append-only log; `⌘Z` reverses the last
  move, delete is soft and restorable from the toast.
- **Import and export.** Paste a list of titles and TMDB matches them;
  `/export` dumps the shelf as JSON.
- **Backups.** A nightly `VACUUM INTO` copy beside the database, newest
  fourteen kept, the latest downloadable from settings.
- **One password**, no accounts. The server refuses to start on a public
  address without one.
- **Installs on a phone** as an app; both themes follow the system.

## Keyboard

| | |
|---|---|
| `↑` `↓` | select a row |
| `Space` | advance by the type's step; `3 Space` by three |
| `⇧Space` | back one |
| `⇧S` | to the end of the season |
| `↵` | open or close the panel |
| `E` `1`–`9` | rate, `0` is ten |
| `N` · `O` | note · open the link |
| `⌘⌫` · `⌘Z` | drop · undo |
| `⌘K` | search or add |
| `⌥1`–`⌥4` | у процесі · колись · завершено · рік |

Shortcuts match the physical key as well as the letter, so a Ukrainian layout
works without switching.

## Run

```bash
go run .                 # sofar.db in the working directory, :8099
SOFAR_DEV=1 go run .     # templates from disk, sample entries on an empty db
go test ./...
```

| Variable | Default | |
|---|---|---|
| `SOFAR_ADDR` | `:8099` | listen address |
| `SOFAR_DB` | `sofar.db` | the SQLite file |
| `SOFAR_PASSWORD` | — | **required** unless dev or loopback |
| `TMDB_TOKEN` | — | v4 read token; films and series |
| `GOOGLE_BOOKS_KEY`, `RAWG_KEY` | — | books, games |
| `SOFAR_BACKUPS` | `backups` beside the db | nightly copies |
| `SOFAR_TZ` | `Europe/Kyiv` | a day starts at 04:00 |

Environment first, then `.env` in the working directory.

```bash
docker compose up --build
```

Multi-stage build into distroless, `CGO_ENABLED=0`, nonroot, state on a
`/data` volume — about 14 MB and one dependency. On Dokploy it is a Compose
application: join `dokploy-network`, add the domain (service `sofar`, port
`8080`), keys in Environment.

## How it is built

```
internal/domain      tracks, units, statistics — no I/O
internal/store       SQLite and every query — no HTML, no network
internal/catalog     TMDB, Google Books, RAWG, ComicsMania
internal/nightly     refresh shows, poll manga, purge, backup
internal/server      routes, handlers, templates, one app.css, keys.js
```

`progress` is an append-only log and the source of truth; `entry.position` is
a cache of it. `unit.idx` runs 1..N across all seasons, so a position is one
integer. Every POST answers with an HTML fragment; there is no JSON API, no
frontend build, no router, no ORM. Decisions and their reasons are in
[PLAN.md](PLAN.md).

## Attribution

This product uses the TMDB API but is not endorsed or certified by TMDB. Game
data comes from RAWG, book data from Google Books, Ukrainian manga listings
from ComicsMania.
