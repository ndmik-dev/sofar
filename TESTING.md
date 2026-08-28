# Manual test pass

Run before every deploy. Roughly 40 minutes end to end.

```bash
rm -f sofar.db sofar.db-wal sofar.db-shm    # the -wal holds writes the .db does not
SOFAR_FIXTURES=0 SOFAR_DEV=1 go run .       # empty list, hot reload still on
```

Open http://localhost:8099. Without `SOFAR_FIXTURES=0`, dev mode seeds seven
fixture entries into an empty database — TC-1 needs the empty one. Templates
reload from disk, **Go code does not**, so restart after any `.go` change.

Report a failure as the case id plus the step number: `TC-3 step 4`.

---

## TC-1 · First run

| # | Do | Expect |
|---|---|---|
| 1 | Open the app on an empty database | "Порожньо" with two buttons |
| 2 | Click **Заповнити архів** | The palette opens |
| 3 | Type a series name, press `↵` | The palette **stays open**, the search field now holds the **title** instead of your query, and below it "Додано · …" with a **Де ти зупинився?** field and a hint like "усього 86 серій" |
| 4 | Type `12`, press `↵` | Palette closes; the row appears reading `12 / 86`; "Порожньо" is gone and the figures bar is there **without a reload** |
| 5 | Reload the page | The row and its position are still there |

*The step in 3 exists so you never have to click twelve episodes by hand — it
declares your starting point.*

## TC-2 · Progress, three ways

| # | Do | Expect |
|---|---|---|
| 1 | Click `+` on the right of a row | Position +1, toast with "⌘Z скасувати" |
| 2 | Click a cell two positions ahead of the filled ones | Position jumps exactly there |
| 2a | Click the last filled cell (the position you are already on) | Nothing happens and **no toast** — it is not an error |
| 3 | Select a row (`↑` `↓`), press `Space` | +1 |
| 4 | Press `3`, then `Space` | +3 in one move |
| 5 | Press `⌘Z` | Exactly −3, back to where step 3 left it |
| 6 | Reload | Every position survived |

## TC-3 · Undo stops at your starting point

The case that used to wipe a series back to zero.

| # | Do | Expect |
|---|---|---|
| 1 | Add a long series and declare position `22` in the step | Row reads `22 / 62` |
| 2 | Press `Space` twice | `24 / 62` |
| 3 | Press `⌘Z` | `23 / 62` |
| 4 | Press `⌘Z` | `22 / 62` |
| 5 | Press `⌘Z` three more times | Stays `22 / 62`, toast reads **"Нема чого скасовувати"** |
| 6 | Reload | Still `22 / 62`; the page loads normally, no spinner |

*Undo reverses what you watched, never the starting point you declared.*

## TC-4 · Rapid clicking

| # | Do | Expect |
|---|---|---|
| 1 | Open a series panel with `↵` | Panel on the right with the episode list |
| 2 | Click 8–10 different episodes as fast as you can | No freeze |
| 3 | Wait two seconds | Row and panel show the **same** position — the last click wins |
| 4 | Click `+` | Responds immediately |
| 5 | Reload the page | Loads normally; the tab is not stuck loading |

## TC-5 · Rating

| # | Do | Expect |
|---|---|---|
| 1 | Select a row, press `Е` (Ukrainian layout) | A hint appears: "Оцінка: 1–9, 0 — це десять, Esc — скасувати" |
| 2 | Press `8` | Toast "· оцінка 8", badge **8** next to the type |
| 3 | Open the panel, hover across the rating squares | Preview: squares up to the cursor light up, the rest go quiet |
| 4 | Move the mouse away without clicking | The eight squares are exactly as they were |
| 5 | Click the tenth square | Rating 10 (`0` after `Е` does the same) |

## TC-6 · Delete returns to its place

| # | Do | Expect |
|---|---|---|
| 1 | Note the order of rows | — |
| 2 | Hover the **second** row, click `×` | Row disappears, toast "· видалено · ⌘Z повернути" |
| 3 | Press `⌘Z` | Row returns to **position two**; the whole order is identical to step 1 |

## TC-7 · Drop and change your mind

| # | Do | Expect |
|---|---|---|
| 1 | Note the sidebar counts for **У процесі** and **Кинуто** | — |
| 2 | Select a row, press `⌘⌫` | Row leaves the list, toast "· у кинутих · ⌘Z повернути", both counts change **immediately** |
| 3 | Press `⌘Z` | Row is back in the list and both counts return to step 1 — **without clicking anything else** |
| 4 | Open **Кинуто** | Empty |

## TC-8 · Backfilling the archive

| # | Do | Expect |
|---|---|---|
| 1 | `⌘K`, type something you have finished | Results |
| 2 | Press **`⌘↵`** | Field clears, cursor stays in it, a log line "Назва · у завершених ✓" appears below |
| 3 | Repeat twice with other titles | The log grows, the rhythm never breaks |
| 4 | `Esc`, open **Завершено** | All three are there with fully filled tracks |
| 5 | Open **Рік** | The "серій" figure did **not** jump — archive backfill is not counted as watching |

## TC-9 · Where the sidebar numbers lead

| # | Do | Expect |
|---|---|---|
| 1 | Open **Кинуто** (empty) | Every type in the sidebar shows **0** |
| 2 | Go back to **У процесі** | Type counts describe this list |
| 3 | Click **Аніме** | Bar "Тільки аніме · N", exactly N rows below, `✕` clears it |
| 4 | On **Колись**, pick a time filter and a type | Both hold at once; switching one keeps the other |

## TC-10 · Your own book with a catalog

| # | Do | Expect |
|---|---|---|
| 1 | `⌘K` → `к:` plus a book title | Catalog results with author and page counts |
| 2 | Click one | A form, prefilled, noting "звір зі своїм виданням" |
| 3 | Correct the page count to the one in **your** copy, add a position, `↵` | The row shows **your** number, not the catalog's |
| 4 | Open the panel | A progress bar, not an empty section |

## TC-11 · A series still airing

| # | Do | Expect |
|---|---|---|
| 1 | Find a row with thin-outlined cells (aired, unwatched) and empty ones (not out yet) — Frieren in the fixtures | — |
| 2 | Press `⇧S` | Position jumps to the **last aired** episode; the empty cells stay empty |

## TC-12 · The panel follows the cursor

| # | Do | Expect |
|---|---|---|
| 1 | Press `↵` | Panel opens on the right |
| 2 | Press `↓` three times | The panel redraws for each new row |
| 3 | Press `Space` | Row and the panel's "Далі" block update together |
| 4 | Press `Н`, write a note, click Зберегти | Toast confirms |
| 5 | Reload, reopen the panel | The note is there |
| 6 | Press `Esc` | Panel closes |

## TC-13 · Games and status-only types

| # | Do | Expect |
|---|---|---|
| 1 | Select a game row, press `Space` | **Nothing happens** — a status-only type has no progress to advance |
| 2 | Click "завершив" on the game | Row leaves "У процесі" with a toast offering the way back |
| 3 | **Налаштування** → switch Книги to "статус" | Book rows immediately show three states instead of a bar |
| 4 | Switch back to "сторінки" | Bars return with the same positions |

## TC-14 · The year is honest

| # | Do | Expect |
|---|---|---|
| 1 | Count roughly how many episodes you actually pressed today | — |
| 2 | Press `⌘4` | "серій" matches that count, not the fixture totals |
| 3 | Press `⌘C`, paste into a chat | Readable plain text |
| 4 | Do one more `+1`, return to the year | The number went up by one |

## TC-15 · Looks

| # | Do | Expect |
|---|---|---|
| 1 | Switch the system theme to dark | List, panel, palette, year, settings — no dark text on dark ground anywhere |
| 2 | Narrow the window to phone width | Nothing overflows sideways; long tracks scroll inside themselves |
| 3 | Find a row with a very long title | Ellipsis, the grid holds |
| 4 | Hover a track cell | Tooltip with the episode name and air date |
| 5 | Open the palette and the panel in both themes | Text and controls readable in both |

---

## Known behaviour, not bugs

- Changing **Go code** needs a server restart; dev mode only reloads templates.
- On **Завершено**, a newly added title appears after a page refresh — rows
  there are grouped by year.
- `⇧Space` on a book steps back one page, not ten. Use `⌘Z` to reverse a `+10`.
- Fixture entries give a streak of **0**: backfill never counts as watching.
- Seven familiar titles after clearing the database are the fixtures reseeding,
  not leftover data. Run with `SOFAR_FIXTURES=0`.
- If the keyboard or the layout misbehaves in a way the code says is fixed,
  hard-reload once (`⌘⇧R`). Dev mode now sends `Cache-Control: no-store`, but a
  page loaded before that change can still hold a cached `app.css` or `keys.js`.
