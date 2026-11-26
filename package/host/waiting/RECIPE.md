# Waiting on you

A right-panel screen that draws `andon waiting list --json` (Andon's owner waits and parallel slots) for the focused pane's project, and counts nothing itself. On an answerable decision it can answer with one of the options through `andon waiting answer`.

## What you see

On the theme's `background`:

- Header, 31px, 11.5px `dim`: "Waiting on you" on the left; on the right the output's `project`, then, when the shown layout has this block inside the `panel` column, a 24px close button (radius 6, `hover`, Herdr's `x.svg` at 13px, tooltip `Close (Esc)`).
- Slots line, 26px, 11.5px `muted`: `Slots <working>/<limit> working · <waiting> waiting on you`, always in this form and color (`waiting_full`, if an older andon prints it, is not read).
- Rows (radius 8, `selected` while open, `hover`): a chevron (Herdr's `chevron-right.svg` / `chevron-down.svg`, 11px `dim`), the kind as an 18px `raised` badge (10.5px semibold: `Merge`, `Decide`, `Do`), then `<id> <title>` (12.5px, truncated) over the wait (`<1m`, `5m`, `3h`, `2d`) and `· N waiting behind` (11px `dim`).
- Open row, indented 24px: the question (12.5px), "Facts" with one `· fact` per line, "Why" with the reason, "Options" numbered with ` (recommended)` on the `pick` one (`foreground`, the rest `muted`); or the item's `reason`; on a task, "Steps" with the output's `steps` one line per line (an empty line as a blank line, 12px `foreground`) and "Links" with each `links` entry that is `http(s)` (12px `accent`, radius 6, `hover`), in the output's order; `Question <id>#<n>`; on a decision with `answerable: true` the options are clickable (radius 6, `hover`, the picked one on `raised` in `foreground`) under `Pick an option, then answer. Anything else, tell the Chief of Staff.`, on another decision `Answer it to the Chief of Staff.`; once answered `Answered: <label>` (also `· Answered: <label>` on the row's second line, `foreground`) and the options no longer click; a refusal in one `blocked` line; "Waiting behind it" with the ids; then 22px `raised` buttons **Answer "<label>"** (`Answering…` while it runs; only after a pick), **Open PR** (merge with `pr`) and **Go to Chief of Staff**.
- Later: after the rows that call (`call` not `false`), the rows with `call` `false` under a heading (11px medium `dim`, padding top 10: `Later` and their number), in the output's order; with none, no heading. With no row that calls, `Nothing calls you now` (12px `dim`) first. An opened later row has "Later" with the item's `later` (first line; `Left for later` when empty) and ` (until <later_until>)` when there is one (12px `muted`), before the question's number.
- Instead of the list: `No folder in view.`, `Reading…`, `Nothing is waiting on you` (under the slots line), or the reason in `blocked`.
- While the pane holds the keyboard and can close: a 24px key hint `Esc close` (10.5px `dim`) at the bottom.

## How it behaves

- Place: the focused pane's folder and machine (Herdr's Files panel's `location()`); a new place drops what was read and the open rows. While this pane holds the keyboard (a click in it can take it), the place stays as it was, whatever `focused()` reports.
- Reading: `exec("andon", ["waiting", "list", "--json"], { cwd, on, timeout: 15000 })` at start, every 10 s (`cx.timer.every`), and when the place changes; one run at a time; a place that changed during a run is read as soon as the run ends. Exit 127: no `andon` in the cell. Exit 2 with andon's refusal of the name (`知らない` from an older andon, `名前が変わった` from a newer one; the arguments are fixed): the cell's andon and the pane are different versions, reinstall the host parts. The same for `andon waiting answer`. Another non-zero exit: its first stderr line. Then JSON, `version` 1, `slots` and `items`, or a one-line reason.
- Order: the output's `items` as they are, split in two: `isLater(i)` (exported) is `i.call === false`; a row without `call` calls.
- Links: a click on a task row's link opens it as **Open PR** does (`openWeb`: only `^https?://[^\s]+$`, through `xdg-open`, `open` or `explorer.exe`). A task row without `steps` or `links` (no brief, or an andon older than the fields) shows its `reason` only.
- Answering: `answerable(i)` is `kind` `decision`, `answerable === true`, a `question` matching `^[A-Za-z0-9][A-Za-z0-9._-]*#[1-9][0-9]*$`, options in `ask`, and not answered here yet. A click on an option only sets the pick (per question). **Answer** runs `exec("andon", ["waiting", "answer", question, "--option", String(n + 1)], { cwd, on, timeout: 15000 })` with the place the row was read from, one run per question at a time. Exit 0: answered (kept per question until a read no longer lists it). Exit 127: no `andon`. Exit 2: an andon older than `andon waiting answer`. Another exit: `Not answered: <first stderr line>`. A place changed during the run drops the result with the rest. A click on a row drawn from output no longer on screen, or while the focused place differs from the place the output was read from (even before the pane reads again), does nothing.
- Keys: a decision row is keyed by its question (`<id>-<n>`, since an issue can have two open decisions), a merge or task row by its issue; the key names the row's element ids and what is open.
- Closing: the close button, or Esc while the pane holds the keyboard (no control, alt or platform key), runs `run("layout.hide", {id: "panel"})`; holding the keyboard, it first runs `run("pane.go", {direction: "left"})` so the keyboard doesn't stay on a hidden pane. Only when `layout()` has a block with mod `waiting` inside the node `panel`; elsewhere there is no button and Esc does nothing. The whole right panel hides, Files and Changes with it (they share its `panel-body` stack): the shown tab takes the width. The inbox's waiting button or a waiting line shows it again on this screen; the tab bar's Files and Changes buttons show it on theirs.
- While closed: the copy stays alive (Herdr keeps a hidden block's copy), so it keeps reading every 10 s. While the focus stays in the same folder and machine, the open rows, the picks, an answer in flight and its result are there when the panel shows again; a focus moved elsewhere meanwhile is a new place (above), and drops them, an answer's result with them.
- Not `close`: Herdr calls a View's `close(cx)` as its cleanup when the block goes and on quit, so the pane's own closing is `closePanel()`, and the cleanup hides nothing.
- Keyboard: `focusable`, so `pane.go` reaches the pane; on `"focus"` from `nextCommand()` it takes the keyboard (`handle.focus()`), and keys come through `on_key_down`.
- Chief of Staff: a live terminal with labels `role=chief-of-staff` and `project=<output project>` on the same machine (`<machine>/` prefix), else none.

## Built from

- `mod.json`: `{ "place": "none", "title": "Waiting on you", "focusable": true }`; placed by the `andon` layout in the `panel-body` stack as `waiting`.
- `herdr`: `spaces()`, `panes()`, `focused()`, `terminals()`, `exec`, `run` (`agent.focus`, `layout.hide`, `pane.go`), `theme()`, `window()`, `layout()` (is this block in the panel), `nextCommand()` (`focus`).

## Done when

- `herdr-app check` passes.
- In a project with a gate question open on a review issue, the pane shows a `Merge` row; **Open PR** opens it; the inbox's waiting button shows the panel on this screen.
- On a task whose brief has steps and a link, opening the row shows the brief line by line and the link; pressing the link opens it in the browser.
- In the cell's own folder (no project), the pane shows andon waiting list's reason in one line.
- Pressing the header's close button hides the right panel, and the inbox's waiting button shows it again on this screen. With the pane holding the keyboard (`pane.go` right from the shown tab), Esc hides it too and the keyboard is back on the left. A row opened and an option picked before closing are still open and picked after showing it again.
- On a decision with options, picking an option and pressing **Answer** writes that option as the answer (`kanban questions` shows it answered), the row shows `Answered: <label>`, and the Chief of Staff gets the `answered` notice.
