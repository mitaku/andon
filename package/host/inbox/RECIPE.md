# Inbox

A column of the agents that need you: blocked ones first, then finished ones you haven't settled, each oldest first, with two lines of context. Click one to answer it in its terminal; settle it when handled and the next one opens. Working, idle and settled agents wait in folded shelves.

## What you see

Top to bottom, on the theme's `chrome` background, 13px text:

1. A 31px header like Herdr's Spaces header: "Inbox" (11.5px, `dim`) on the left; on the right, when more than one machine is known, the machine button.
2. A filter line `/<text>` while filtering.
3. **Needs you** heading (11px, `blocked` color) with the count, then its rows.
4. **Done** heading (11px, `done` color) with the count, then its rows.
5. With both empty: "Nothing needs you." (12.5px, `dim`).
6. Shelf headers like Herdr's group headers (30px, 11px medium, `dim`): `Working n`, `Idle n`, `Settled n`, with a chevron on the right; folded by default. Open, they list their rows; Settled shows 10, then "Show 25 more".

The machine button: 24px, radius 6, 11.5px, Herdr's `server.svg` (12px), the pick (`All machines` in `dim`, else one or two names or `3 machines` in `foreground`) and a chevron (right, down while open). Under the header, when items wait on machines left out: `+n waiting elsewhere` (11px, `blocked`; click: all again), and `n offline` (`dim`). Open, a list (radius 8, `popover`, 1px `border`, 8px in from the column's sides) of 28px lines: a 14px checkbox (checked: `solid` with Herdr's `check.svg` in `solid_foreground`; else a 1.5px `input` border), the `monitor`/`server` icon, the name (`This computer` for this one), the number waiting (`blocked`) or "not connected", and Herdr's 6px connection dot (`ansi2`, `dim` when away); then an `All machines` line. While the queue has the keyboard, a 24px key hint sits at the bottom.

A row (radius 8, padding 6×8, gap 9), like Herdr's agents rows: the program icon (16px) on the left; then (1) the name (13px; `muted` for idle and settled), the wait (11px, `dim`; with `+` when the time comes from the core's `status_ms` rather than a change the mod saw), and the status mark; (2) 11.5px `dim`: the machine (Herdr's `monitor`/`server` icon and `This computer` or its name, in `muted`) `·` the space, left out when one machine is picked; (3) 11.5px `dim`: the tab when someone named it and it isn't the name, a branch other than main/master, the report message cut to 60 characters.

## How it behaves

- Status mark, as Herdr's agents list draws it: blocked, a `blocked`-tinted pill (radius 5, 18px) with Permission / Question / Auth from the report's `kind`, else "Needs you"; working, Herdr's `spinner.svg` turning (`.spin(900)`); done, a filled `done` dot; idle and settled, a ring.
- Section: settled if the `settled` label (or a pending write) is set; else blocked → Needs you; done, or idle with a turn later than the turn first seen → Done; working → Working; else Idle.
- Click a row or Enter: `agent.focus`; if the parent isn't blocked but a sub-agent without a pane is, `terminal.peek` that sub-agent.
- Hover a row: the wait is replaced by an icon button, Herdr's `check.svg` (settle) or, on settled rows, `chevron-left.svg` (unsettle). Hover is kept as state and cleared when the row moves away.
- `next` (key command): open the first queue row that isn't the agent in view.
- `settle` (key command): settle the agent in view and run next; with nothing in view in the queue, just next.
- `focus` (key command): `handle.focus()` on the column.
- "The agent in view" is `focused().agent` (an agent block), else `focused().terminal` (a tab layout like `andon`); it is the row highlighted, and what `next` skips and `settle` settles.
- The machine pick (checkboxes; `m` opens the panel, `j`/`k`, `Space`, `a`) narrows every section, `next`, `settle` and `setAgentOrder`; checking every machine means all again. The pick is kept in `localStorage`. With exactly one machine picked, rows drop the machine name.
- Keys while focused: `j`/`k`/arrows move, Enter/`l` open, `d` settle and select the next row, `u` undo the last settle, `g` then `g` within 1s first row, `G` last row, `/` start a filter (typed characters from `key_char`, Backspace deletes, Enter keeps, Esc clears), Esc otherwise `pane.go` right.
- New work: when a settled agent's `turn` passes the stored turn, or it (or a sub-agent) is working or blocked and its `status_ms` is later than the settle time, the label is removed.

## The board (as a dialog)

- `mod.json` adds `"overlay": { "kind": "dialog", "width": 0.92, "height": 0.88 }`; `mod.open:inbox` toggles it. The column's header has a 24px icon button (Herdr's `layers.svg`, 13px, tooltip "Board") at the right, after the machine button, running `run("mod.open", {name: "inbox"})`. When `props().overlay` is set, the copy draws a board instead of the column, starts no `nextCommand()` loop and never calls `setAgentOrder`.
- Header 44px: "Agents" (14px semibold), "n need you · n done" (`dim`), a **By machine** toggle (`selected` when on), a close `x`. A row of column titles (11.5px medium, in the section's color, with counts), then lanes: a lane header (Herdr's monitor/server icon, `This computer` or the machine, "not connected" when away) over five equal columns (`surface`, 1px `border`, radius 8, 6px padding, at least 44px high). Cards are the column's rows on `background` with a 1px border (`blocked` for Needs you, `accent` for the keyboard selection); in lanes the row drops its machine.
- The board takes the keyboard 30 ms after it opens (a `cx.timer.after`; focus can't move during a render). Opening a card runs `agent.focus` / `terminal.peek` and closes the dialog.
- `changed` and `base` live in `shared("inbox.memory")` so both copies agree; an older `localStorage` copy is read once.

## Built from

- `mod.json`: `{ "place": "none", "title": "Inbox", "focusable": true }`. The `andon` layout is Herdr's `main` with this mod in place of `agents` (left column: `workspace-chrome` brand, this mod at weight 2 with min 120, `spaces` at weight 1, the `workspace-chrome` footer; right: `tabs`, the shown tab with `pane` headers, `files` / `changes`, `workspace-status`).
- `herdr`: `agents()`, `terminals()` (labels, `turn`, `status_ms`, `report`, `parent`), `spaces()` (tab names and `named`), `machines()`, `focused().agent`, `setAgentOrder(needs + done + working)`, `run`, `hc(["label", addr, "settled=…"])`, `notify` on failure, `nextCommand()`, `theme()`, `localStorage`.
- `ROLE_ICON` (exported): the icon file of each role that talks with you (`chief-of-staff.svg`, `chief-engineer.svg`); a row of such a role draws it instead of the program's icon.
- `name.js`: the display name: a named tab, else a telling title (not a path, `user@host` or the program's name; a leading mark stripped), else the program's name.
- A label write is kept as pending so the row moves at once; it's dropped when `terminals()` shows the same value, or on failure (then an error notification).

## Watch out

- `render` doesn't run while the window is in the background, so a settled agent turning working is noticed only when the window draws again; the turn check still catches it.
- The busy rule must ignore a state that was already there when settling, or a blocked agent bounces straight back.
- Done and idle are tracked as one state for wait times: looking turns done into idle.
- A sub-agent's `parent` is a bare address on its own machine; prefix the machine to match a remote parent.
- `<mod>.<command>` reaches only a shown copy; in other layouts it's an unknown command.
- Don't put the shipped `agents` mod in the same layout: both call `setAgentOrder`.

## Done when

- `herdr-app check` passes on the folder.
- A blocked agent shows under Needs you with its kind; `inbox.next` opens it in the focused block.
- A finished agent shows under Done; settling it writes `settled=<ms>:<turn>` (see `hc get <t> --json`) and moves it to Settled.
- Giving it new work removes the label and brings it back.
- With nothing waiting, "Nothing needs you." shows.
