# Waiting on you

A right-panel screen that draws `andon waiting --json` (Andon's owner waits and parallel slots) for the focused pane's project, and counts nothing itself.

## What you see

On the theme's `background`:

- Header, 31px, 11.5px `dim`: "Waiting on you" on the left, the output's `project` on the right.
- Slots line, 26px, 11.5px `muted` (`blocked` when `waiting_full`): `Slots <working>/<limit> working · <waiting> waiting on you`, then ` · full` when `waiting_full`.
- Rows (radius 8, `selected` while open, `hover`): a chevron (Herdr's `chevron-right.svg` / `chevron-down.svg`, 11px `dim`), the kind as an 18px `raised` badge (10.5px semibold: `Merge`, `Decide`, `Do`), then `<id> <title>` (12.5px, truncated) over the wait (`<1m`, `5m`, `3h`, `2d`) and `· N waiting behind` (11px `dim`).
- Open row, indented 24px: the question (12.5px), "Facts" with one `· fact` per line, "Why" with the reason, "Options" numbered with ` (recommended)` on the `pick` one (`foreground`, the rest `muted`); or the item's `reason`; `Question <id>#<n>`; `Answer it to the Chief of Staff.` on a decision; "Waiting behind it" with the ids; then 22px `raised` buttons **Open PR** (merge with `pr`) and **Go to Chief of Staff**.
- Instead of the list: `No folder in view.`, `Reading…`, `Nothing is waiting on you` (under the slots line), or the reason in `blocked`.

## How it behaves

- Place: the focused pane's folder and machine (Herdr's Files panel's `location()`); a new place drops what was read and the open rows.
- Reading: `exec("andon", ["waiting", "--json"], { cwd, on, timeout: 15000 })` at start, every 10 s (`cx.timer.every`), and when the place changes; one run at a time; a place that changed during a run is read as soon as the run ends. Exit 127: no `andon` in the cell. Exit 2 (andon's usage error, with fixed arguments): an andon older than `andon waiting`. Another non-zero exit: its first stderr line. Then JSON, `version` 1, `slots` and `items`, or a one-line reason.
- Order: the output's `items` as they are.
- Chief of Staff: a live terminal with labels `role=chief-of-staff` and `project=<output project>` on the same machine (`<machine>/` prefix), else none.

## Built from

- `mod.json`: `{ "place": "none", "title": "Waiting on you" }`; placed by the `andon` layout in the `panel-body` stack as `waiting`.
- `herdr`: `spaces()`, `panes()`, `focused()`, `terminals()`, `exec`, `run`, `theme()`, `window()`.

## Done when

- `herdr-app check` passes.
- In a project with a gate question open on a review issue, the pane shows a `Merge` row; **Open PR** opens it; the inbox's waiting button shows the panel on this screen.
- In the cell's own folder (no project), the pane shows andon waiting's reason in one line.
