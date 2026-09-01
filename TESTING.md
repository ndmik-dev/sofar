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
| 3 | Type a multi-season series, press `↵` | The palette **stays open**, the search field holds the **title**, and below it **Де ти зупинився?** with **сезон** and **серія** fields plus a hint like `S1·13 S2·13 …` |
| 4 | Type season `4`, episode `7`, press `↵` | Palette closes; the row reads `46 / 86` and «далі S4E8» — the season lengths were added up for you |
| 4a | Add a **single-season** title the same way | One field asking for the episode, no season |
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
| 1 | Select a row, press `E` (any layout) | A hint appears: "Оцінка: 1–9, 0 — це десять, Esc — скасувати" |
| 2 | Press `8` | Toast "· оцінка 8", badge **8** next to the type |
| 3 | Open the panel, hover across the rating squares | Preview: squares up to the cursor light up, the rest go quiet |
| 4 | Move the mouse away without clicking | The eight squares are exactly as they were |
| 5 | Click the tenth square | Rating 10 (`0` after `E` does the same) |

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
| 1 | `⌘K`, type something you have finished | Results, the first one highlighted |
| 1a | Press `↓` twice, then `↑` | The highlight walks the list and comes back one |
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
| 4 | Press `N` (the key left of M — on a Ukrainian layout it types `т`) | The panel scrolls to the note and the caret lands in it |
| 4a | Write a note, click Зберегти | Toast confirms |
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

The year page counts **what you did in the app**, never what you declared when
adding a title. That is the whole point of the case.

| # | Do | Expect |
|---|---|---|
| 1 | On the list, press `Space` exactly three times on one row | Position moved by three |
| 2 | Press `⌥4` | The year page. "серій" reads **3**, not the show's total |
| 3 | Press `⌘C`, paste into a chat | Readable plain text |
| 4 | Go back, press `Space` once, return to the year | "серій" reads **4** |

## TC-15 · Looks

| # | Do | Expect |
|---|---|---|
| 1 | **Налаштування** → Тема → темна | Everything turns dark at once; reload keeps it |
| 2 | Switch to системна | Follows the OS again |
| 3 | Open a series with many episodes, press `↵` | With the panel open the rows go two-line: **cells stay the same size**, every season block visible, nothing overlaps the position |
| 4 | Drag the window narrower, a bit at a time | Rows switch to the two-line layout before anything collides — no width where the track sits on top of the numbers |
| 5 | Add a title with a very long name (`⌘K`, paste a 60-character title) | The name is cut with `…`; the type, track, position and button stay on their own columns |
| 6 | Hover a track cell | Tooltip with the episode name and air date |
| 7 | Rate a row `E` `8` | `★8` stands out from the type label beside it, in both themes |

## TC-16 · Fixing what you typed

| # | Do | Expect |
|---|---|---|
| 1 | Add a book through `к:` with **Всього** left empty | Row reads `50 · без межі` with a thin line, not a bar |
| 2 | Open the panel, expand **Редагувати запис** | Назва, Автор and Всього, filled in |
| 3 | Set Всього to `222`, Зберегти | Row turns into a real bar, `50 / 222`, toast "· збережено" |
| 4 | Edit Всього to `20` | Position clamps to `20 / 20` — you cannot be past the last page |
| 5 | Open a TMDB series panel and expand the same block | **No Всього field**: its episodes come from TMDB |
| 6 | Clear the title, Зберегти | Refused, the row does not change |

## TC-17 · Import a list

| # | Do | Expect |
|---|---|---|
| 1 | Open **Імпорт**, paste four lines: a title, one with `(2008)`, one with `\| 96`, one CSV row | — |
| 2 | Leave **завершено**, press Імпортувати | A line per row: matched title, year, `✓` |
| 3 | Add a nonsense line and repeat | That one reads "не знайдено", the rest still import |
| 4 | Repeat the same list | Every row reads "вже було" — no duplicates |
| 5 | Open **Рік** | The figures did **not** move: an import is backfill, not watching |
| 6 | Open **Завершено** | The titles are there with full tracks |

## TC-18 · The password

Run this one with `SOFAR_PASSWORD=… SOFAR_DEV=1 go run .`

| # | Do | Expect |
|---|---|---|
| 1 | Open the app in a private window | The login page, not the list |
| 2 | Enter a wrong password | "Не той пароль", still on the login page |
| 3 | Enter the right one | The list, and a reload does not ask again |
| 4 | **Налаштування** → Вийти | Back to the login page |
| 5 | Restart with a **different** password, reload | Asked again: the old cookie died with the old password |
| 6 | `SOFAR_PASSWORD= SOFAR_ADDR=:8099 go run .` (no dev) | Refuses to start, and says why |

## TC-19 · Links

| # | Do | Expect |
|---|---|---|
| 1 | Open a panel, paste `netflix.com/title/123` in Посилання, Додати | A link labelled **Netflix** — the label came from the domain |
| 2 | Select that row, press `O` | The link opens in a **new** tab; the list stays where it was |
| 3 | Paste `javascript:alert(1)` | Refused — only http and https are accepted |
| 4 | Press `✕` next to the link | It is gone, and `O` does nothing |

## TC-20 · A film in minutes

| # | Do | Expect |
|---|---|---|
| 1 | **Налаштування** → Фільми → «хвилини», крок 5 | — |
| 2 | Add a film through `ф:` | The row shows a bar and `0 / <runtime>` |
| 3 | Click the bar about a third along | The position jumps to that minute |
| 4 | Press `Space` | +5 minutes |
| 5 | Check a film you had marked finished **before** this change | It reads `runtime / runtime`, not `1 / runtime` |

## TC-11a · Going back into an earlier season

A collapsed season used to be unreachable: only the current season had cells.

| # | Do | Expect |
|---|---|---|
| 1 | On a multi-season show, click the **S1** block in the row | The panel opens on season 1; the **position does not change** |
| 2 | Click E3 in that list | Position becomes S1E3 exactly, the row says «далі S1E4» |
| 3 | Click a **future** season block, e.g. **S5** | The panel shows season 5, position still untouched |
| 4 | Stand exactly at the end of a season and click that season's block | It opens the season — it does not look broken by doing nothing |
| 5 | Use the season tabs in the panel header | Same thing without leaving the panel |

## TC-20a · Going backwards on a bar

The gap this closes: a track lets you click any episode, a bar had no way back
except `⇧Space`, one page or one minute at a time.

| # | Do | Expect |
|---|---|---|
| 1 | On a book or film row, click the bar about a quarter along | The position jumps to a quarter of the total, toast with `⌘Z` |
| 2 | Press `⌘Z` | Back where it was |
| 3 | Open the panel, type a number in **Зупинився на**, Ок | Exactly that position, row and panel together |
| 4 | Set it below the total on something marked finished | It returns to «у процесі» — a finished thing you rolled back is not finished |
| 5 | Type a number larger than the total | Clamped to the total, not refused |

## TC-9a · Type prefixes do not hide the answer

| # | Do | Expect |
|---|---|---|
| 1 | `⌘K`, `а: ` and the name of an anime **film** | It is found — an animated Japanese feature is anime, not a plain film |
| 2 | Add it | The row is coloured as аніме and counted in minutes |
| 3 | `а: ` and something that genuinely does not exist | «TMDB не знає такої назви серед цього типу», and the manual form below has **аніме** preselected — not книга |
| 4 | Repeat with `ф:`, `к:`, `м:` | The form always keeps the type you typed |

## TC-20b · Manga

| # | Do | Expect |
|---|---|---|
| 1 | `⌘K`, type `м: фрірен` | Series from the shop, e.g. «Проводжальниця Фрірен · до тому 6» — the **series**, never one volume of it |
| 1a | Click one | The form prefilled: title, and the newest volume as an editable total in **томи** |
| 1b | Type `м: наруто` | Nothing, and the manual form below with **манґа** selected — it is not published in Ukrainian, and that is not a bug |
| 2 | Fill a volume count and a position, save | A bar with `2 / 9` and a percentage, counted in **томи** |
| 3 | Add one and leave **Всього** empty | An open counter: a thin line and «без межі», which is what an unfinished series is |
| 4 | Check the sidebar and the year legend | Манґа has its own colour and its own count |
| 5 | Look for **Подкасти** anywhere | Gone — sidebar, prefixes, settings, year legend |

## TC-20c · Watching for new volumes

| # | Do | Expect |
|---|---|---|
| 1 | Open a manga panel | A **Нові томи** block with the title prefilled as the query |
| 2 | Shorten the query to what the shop lists (`Фрірен`), press Стежити | Toast «Стежу · зараз у продажу том N» — it checks at once rather than waiting for the night |
| 3 | Set your position below that number and reload | The row is in **Вийшло нове** reading «вийшов том N» |
| 4 | Advance past that volume | It leaves the section |
| 5 | Type a query that matches nothing | «томів поки не знайшов» — no wrong series is adopted |
| 6 | Press **Не стежити** | The block goes back to offering to watch |

## TC-21 · Shelf search and export

| # | Do | Expect |
|---|---|---|
| 1 | `⌘K`, type a Ukrainian title you already have, in **lowercase** | A **Уже на полиці** section above the catalog results |
| 2 | Click your own result | The right list opens, scrolled to that row, with it selected |
| 3 | Type something you do not have | Only catalog results, no shelf section |
| 4 | **Імпорт** → Завантажити JSON | A `sofar-YYYY-MM-DD.json` with every list, positions, ratings, notes and links |

## TC-22 · What aired

Needs a running show with a recent episode you have not watched.

| # | Do | Expect |
|---|---|---|
| 1 | Open **У процесі** | A **Вийшло нове** section above the list |
| 2 | Look at the row | A green badge with the episode, e.g. `S5E1` |
| 3 | Watch it (`Space` until past that episode) and reload | The row leaves the section for the normal list |
| 4 | Check a show whose last episode aired years ago | Not in the section — old backlog is not news |

---

## Known behaviour, not bugs

- Changing **Go code** needs a server restart; dev mode only reloads templates.
- On **Завершено**, a newly added title appears after a page refresh — rows
  there are grouped by year.
- `⇧Space` on a book steps back one page, not ten. Use `⌘Z` to reverse a `+10`.
- Fixture entries give a streak of **0**: backfill never counts as watching.
- Shortcuts are matched by physical key as well as by letter, so the layout does
  not matter. The note is on **N**, not H — Cyrillic `Н` looks like a Latin H on
  screen, which is a good reason to read these tables as Latin letters.
- A book added without a page count shows a thin open line and "без межі"
  instead of a bar: there is no total to be a percentage of. The panel's edit
  block can give it one later.
- A film counts toward «фільмів» only when you reach its end; before that it
  still adds its minutes to the hours.
- The nightly job runs twenty minutes after the day starts (04:20 by default).
  Nothing refreshes while you watch; restarting the server does not trigger it.
- In the "where did you stop" step, an episode past the end of its season lands
  on that season's last one instead of being refused. Leaving the episode empty
  means "finished everything before this season".
- Import matches one title per line and takes a single best guess. Check the
  matched names in the result list; a year in brackets settles the ambiguous
  ones.
- Under a kind filter the figures bar is hidden: it describes the whole list,
  not the filtered subset, and showing it there would contradict the rows.
- Seasons come from TMDB, which does not always match how a service markets a
  show — Disenchantment ships as five Netflix "parts" but is three TMDB seasons
  of 20 + 20 + 10, and the app follows TMDB.
- Seven familiar titles after clearing the database are the fixtures reseeding,
  not leftover data. Run with `SOFAR_FIXTURES=0`.
- If the keyboard or the layout misbehaves in a way the code says is fixed,
  hard-reload once (`⌘⇧R`). Dev mode now sends `Cache-Control: no-store`, but a
  page loaded before that change can still hold a cached `app.css` or `keys.js`.
