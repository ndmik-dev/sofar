# Sofar — build plan

Roadmap and decision log. What the app *is* and how to run it lives in
[README.md](README.md); this file records what was decided and what is left.

Working document: update it as we go.

---

## 1. Settled decisions

Not up for rediscussion — written down so they are not relitigated.

| Question | Decision | Why |
|---|---|---|
| Name | **Sofar** | "so far" — everything in the app answers how far you have got |
| Form | Web app, server-rendered | The design is keyboard-first; this is a web app, not an adaptation |
| Stack | Go + htmx + SQLite | One binary, no frontend build, ~30 MB RAM |
| Design | Track of cells | Progress has a shape, not just a percentage |
| Catalogs | TMDB, Google Books, RAWG, iTunes | Metadata only |
| Courses | Own kind, manual only | No catalog for them exists anywhere |
| Catalog role | Fill metadata, **never define units**, never mandatory | All the pain was in the second part |
| Friends | Not in v1 | But `user_id` in the schema from day one |
| Progress | Append-only `progress` log | Undo, streaks, pace and stats fall out of it free |
| Pause | No manual status | "Stale" is derived from `updated_at` |
| Tracking depth | Per type | Games get three states, series get episodes |
| Ongoing things | Open counter, own folded section | A podcast has no finish line, so "how far" has no answer |
| Deploy | Dokploy droplet, one container | Already there, 4 GB, Caddy in front |
| Auth | One password from the environment | Not accounts. Year-long cookie, key derived from the password so a change logs everyone out |
| Day boundary | **04:00, Europe/Kyiv** | An episode at 01:30 belongs to the previous day |

**Deliberately not in v1:** AniList, Open Library, HowLongToBeat, IGDB, Trakt
sync, Plex/Jellyfin webhooks, recommendations, an activity feed, reviews,
anything social, a native app.

---

## 2. Two free insurances, taken on day one

Both cost one column. Without them a later change means migrating all the data.

1. **`entry.user_id`** — so growing from one person to a handful is an evening.
2. **`entry.run`** — a rewatch must not overwrite the first pass.
   Uniqueness is `(user_id, media_id, run)`.

---

## 3. Schema

The key idea: `unit.idx` runs **1..N straight through every season**. The track
draws exactly that, `entry.position` is one integer, and season boundaries come
from `unit.season` changing.

```sql
users (id, name, created_at)

media (
  id, kind,        -- show|movie|anime|book|game|podcast
  source,          -- tmdb|manual|gbooks|rawg|itunes
  tmdb_type, tmdb_id, ext_id,
  title, title_orig, year, overview, poster_path,
  runtime_min,     -- per episode, or film length
  total_units,     -- NULL = open ended
  unit,            -- episode|page|hour|chapter|none
  airing,          -- returning|ended, tv only
  refreshed_at, created_at
)

unit (media_id, idx, season, number, title, air_date, runtime_min)

entry (
  id, user_id, media_id, run,
  status,          -- active|backlog|done|dropped
  position,        -- cache of the last non-undone progress.to_pos
  rating, note, started_at, finished_at, updated_at, created_at,
  deleted_at       -- soft delete, undoable
)

progress (id, entry_id, from_pos, to_pos, at, source, undone_at)

type_settings (user_id, kind, depth, step)   -- depth: status|units
streak_freeze (user_id, day)
```

**Derived, deliberately not stored:**
stale = `status='active' AND updated_at < now-30d` ·
streak = consecutive days with progress ·
pace = progress over a period ·
time left = `(total_units - position) * runtime_min`.

---

## 4. Milestones

| | Milestone | Est. | Outcome |
|---|---|---|---|
| ✅ | **M0** Skeleton | 3 h | Server, migrations, Docker |
| ✅ | **M1** Schema and track | 6 h | Four-state cells on real data |
| ✅ | **M2** `+1` and undo | 5 h | **Usable as a tracker** |
| ✅ | **M3** TMDB | 8 h | Real series with real episodes |
| ✅ | **M4** Adding | 5 h | `⌘K`, prefixes, manual entries |
| ✅ | **M5** Filling up | 6 h | Flow mode, backlog, archive |
| ✅ | **M5.5** Other catalogs | 6 h | Books, games, podcasts |
| ✅ | **M6** Keyboard | 5 h | A full session without the mouse |
| ✅ | **M7** Panel | 5 h | Description, episodes, pace, note |
| ✅ | **M8** Streak and year | 6 h | Gamification, per-type depth |
| ✅ | **M8.5** Edit and import | 3 h | Fix a saved entry, paste an archive |
| ✅ | **M8.6** Password | 1.5 h | Nothing public without it |
| ✅ | **M8.7** Links, nightly, shelf search, export | 5 h | The list keeps itself current |
| | **M9** Deploy | 5 h | Live on a domain, with backups |
| | **M10** Pocket | 3 h | PWA, installs on a phone |

About 58 hours spent of roughly 61. Dark theme and the responsive layout landed
early, during the QA pass that followed M8, so M10 is only the PWA now.

### M7 · Panel — 5 h ✅

Done. `↵` opens it, `↑↓` drags it along the cursor, `Esc` closes it. Inside:
the large track, next episode with a season-finish shortcut, synopsis, the
current season's episode list (each row clickable to jump there), pace from the
progress log, an inline note, rating as ten clickable dots. Every keyboard
action funnels through one `post()` helper, which is the single place that keeps
an open panel in sync with the row it describes.

### M8 · Streak and year — 6 h ✅

Done. Streak walks the manual progress log backwards with one auto-spent freeze
per calendar month; a frozen day counts toward the length, because a streak is
an unbroken stretch of calendar, not a tally of active days. Backfill and import
rows are excluded from every statistic — dumping an archive must not read as a
heroic watching day. `/year` shows four headline numbers, a day calendar
coloured by the kind that dominated each day, and months as stacked bars.
`⌘C` copies the year as plain text. `/settings` switches per-type depth and
step live; `⌥1`–`⌥4` jump between pages.

### M8.5 · Edit and import — 3 h ✅

Done. The panel folds an edit form: title, subtitle and — only while no real
episodes exist — the total. A total below the current position clamps it, and
the clamp goes through the progress log rather than writing `position` behind
its back. `/import` takes a pasted list against TMDB, tolerating a trailing
year, a `| position` suffix and a CSV first column; an exact title beats a
popular one. Everything imported is `backfill`, so an archive dump never shows
up in the year.

### M8.6 · Password — 1.5 h ✅

Done. One password from `SOFAR_PASSWORD`, a year-long cookie signed with a key
derived from it, per-address throttling that reads the proxy headers, and a
startup that fails rather than serve a public address without a password.

### M8.7 · Links, nightly, shelf search, export — 5 h ✅

Done. Links hang off the entry with a label read from the domain and `O` to
open one. A nightly job re-imports running shows and purges deletions older
than 30 days, which is what makes «Вийшло нове» possible at all — that section
is pure derivation from air dates. `⌘K` searches the shelf before the catalog,
matching in Go because SQLite's `LOWER` folds ASCII only and «дюна» would never
find «Дюна». `/export` writes the whole shelf to one JSON file. Films are
tracked in minutes, with a migration that moves already-finished ones to their
runtime through the progress log rather than behind it.

### M9 · Deploy — 5 h

A nightly job in the same process refreshing `airing='returning'` shows and
purging entries deleted more than 30 days ago. Dokploy, domain, HTTPS, a daily
`sqlite3 .backup` to S3.

**Done when:** it runs on a domain and a backup has completed.

### M10 · Pocket — 3 h

Manifest, icons, service worker. The responsive layout is done: rows reflow on
their own container, so opening the panel and narrowing the window take the
same path. The dark theme is done too, with a three-state switch in settings
(system / light / dark) kept in `localStorage`.

**Done when:** it installs on a phone and reads offline.

---

## 5. Traps worth not stepping in

- **A catalog fills metadata but never defines progress units.** TMDB is the one
  exception, because episodes are unambiguous. Anything that would define units
  — AniList absolute numbering, another edition's page count, HowLongToBeat —
  stays out. This is what kills projects like this.
- **No catalog is mandatory.** Every adapter degrades to the manual form. Proven
  in the wild: Google Books returned 503 on the first live request and the user
  saw a working form rather than an error.
- **`progress` is the source of truth, `position` is a cache.** If they ever
  disagree, recompute from the log.
- **No confirmations.** A toast with undo for five seconds, plus `⌘Z`. Mistakes
  are cheap, so asking is noise.
- **Do not call TMDB during a render.** Only on add and in the nightly job.
- **No Tailwind.** One `app.css`, tokens at the top.
- **No JSON API.** The moment it exists, so do a client and versioning.

---

## 6. Lessons that cost time

Recorded so they are not repeated.

- **Dev mode reloads templates but not Go code.** An old binary with new
  templates is a 500 that looks like a bug. Restart.
- **htmx keeps moving nodes during `afterSwap`.** Client-side state such as the
  selected row must be re-applied on `afterSettle`, or it vanishes.
- **A flex container sized from shrinkable children collapses to their minimum.**
  The current-season strip shrank to a sliver until the cells were given a fixed
  width with a scroll fallback.
- **Thresholds belong to the smallest unit they describe.** "Cells up to 60
  episodes" was wrong for a whole show; per season it is right, and single-season
  anime never notices the rule exists.
- **Advertise nothing that does not exist.** A toast promising `⌘Z` before the
  keyboard layer shipped was worse than no label at all.
- **A control that sometimes does nothing reads as broken.** Clicking a season
  block set the position to that season's end — which, when you were already
  standing there, changed nothing and showed nothing. It opens the season now
  instead of moving anything.
- **Collapsing something hides its controls too.** Seasons collapsed into
  blocks to keep cells full-size, and with them went the only way back into a
  season already passed. Anything folded away needs its own way in.
- **Changing a unit changes every number built on it.** Films became minutes
  and the year began reporting "+342 фільми": the counter had been adding
  deltas, which used to be one tick per film. Anything derived from a unit has
  to be re-read when the unit moves.
- **`unit.idx` is right for the machine and wrong for the person.** One integer
  across all seasons is what makes the track and undo simple, but nobody knows
  that S4E7 is episode 46. Ask in seasons, store in idx.
- **SQLite's `LOWER` and `LIKE` fold ASCII only.** Searching «дюна» found
  nothing at all in a Ukrainian shelf. Matching moved into Go, where case
  folding knows about Cyrillic.
- **Never query the pool from inside an open transaction.** With one pooled
  connection that is a guaranteed deadlock: the query waits for the connection
  the transaction is holding, the request never returns, and the browser sits on
  a spinner. Two code paths had it, both on early returns.
- **The browser wins the shortcut fight.** Chrome reserves `⌘1`–`⌘9` for its
  own tabs and never delivers them to the page, so a shortcut on them is dead
  however correct the handler is. Page navigation moved to `⌥`, matched on
  `e.code` because `⌥1` types `¡` on macOS.
- **A shortcut must match the letter, not the key position.** `E` on a Ukrainian
  layout arrives as `е`, and matching only the positional `у` left the key dead
  exactly when the user was typing Ukrainian.

---

## 7. Open questions

1. ~~**Domain**~~ — settled: a subdomain of `the author's domain`.
2. **Posters** — TMDB returns image URLs and they are stored. Nothing displays
   them yet; the M7 panel is the first place they would earn their space.
3. **Importing history** — done as a pasted list against TMDB. A real Trakt or
   Simkl export parser stays out until there is one to parse.
4. **Fira fonts** — currently a system stack with Fira first. Vendor the woff2
   files whenever the typography starts to matter.
