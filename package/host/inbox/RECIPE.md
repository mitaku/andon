# Inbox

A column of two sections: **Needs you** (blocked agents and the roles' unseen replies, oldest first) and **Projects** (each project's Chief of Staff and Chief Engineers, always). Click a row to answer it in its terminal. An issue's agent shows only while blocked. The mod only reads; it writes no label.

## What you see

Top to bottom, on the theme's `chrome` background, 13px text:

1. A 31px header like Herdr's Spaces header: "Inbox" (11.5px, `dim`) on the left; on the right, when more than one machine is known, the machine button, then two 24px icon buttons (13px icon, tooltip naming the key): next waiting (`next.svg`, `ctrl-shift-j`) and the kanban-board mod (`kanban-board.svg`, `run("mod.open", {name: "kanban-board"})`, `ctrl-shift-y`).
2. A filter line `/<text>` while filtering.
3. **Needs you** heading (11px, `foreground`, after a 7px amber `#F0A202` dot) with the count, then its rows; with none, "Nothing needs you." (12px, `dim`).
4. **Projects** heading (11px, `dim`) with the count, then its rows: by project name, the Chief of Staff, then Chief Engineers by issue. Left out when there are none.

The machine button: 24px, radius 6, 11.5px, Herdr's `server.svg` (12px), the pick (`All machines` in `dim`, else one or two names or `3 machines` in `foreground`) and a chevron (right, down while open). Under the header, when items wait on machines left out: `+n waiting elsewhere` (11px, `blocked`; click: all again), and `n offline` (`dim`). Open, a list (radius 8, `popover`, 1px `border`, 8px in from the column's sides) of 28px lines: a 14px checkbox (checked: `solid` with Herdr's `check.svg` in `solid_foreground`; else a 1.5px `input` border), the `monitor`/`server` icon, the name (`This computer` for this one), the number waiting (`blocked`) or "not connected", and Herdr's 6px connection dot (`ansi2`, `dim` when away); then an `All machines` line. While the queue has the keyboard, a 24px key hint sits at the bottom.

A row (radius 8, padding 6×8, gap 9), like Herdr's agents rows: the program icon (16px) on the left; then (1) the name (13px; `muted` for idle), the wait (11px, `dim`; with `+` when the time comes from the core's `status_ms` rather than a change the mod saw), and the status mark; (2) 11.5px `dim`: the machine (Herdr's `monitor`/`server` icon and `This computer` or its name, in `muted`) `·` the space, left out when one machine is picked; (3) 11.5px `dim`: the tab when someone named it and it isn't the name, a branch other than main/master, the report message cut to 60 characters.

## How it behaves

- Status mark, as Herdr's agents list draws it: a row in Needs you, a lit pill (radius 5, 18px, 10.5px semibold) with Permission / Question / Auth from the report's `kind`, Failed, Reply, else "Needs you"; Failed is red (`#D8453B`, text `#FFFFFF`), the rest amber (`#F0A202`, text `#1F2328`): the logo's colors (`GLOW`), never the theme's `blocked`; working, Herdr's `spinner.svg` turning (`.spin(900)`); otherwise a ring.
- Section: blocked (a terminal `error` counts) → Needs you; a role that talks with you (`role=chief-of-staff`, `role=chief-engineer`, on the terminal or a sub-agent under its pane) and `done` → Needs you (`Reply`), unless it is paneless and its row was opened at this `turn` (kept in `shared("inbox.memory")` as `seen`); any other such role → Projects; anything else → not shown. `role=dispatcher` is not a role that talks with you.
- Click a row or Enter: `agent.focus`; if the parent isn't blocked but a sub-agent without a pane is, `terminal.peek` that sub-agent.
- `next` (key command): open the first Needs you row that isn't the agent in view.
- `focus` (key command): `handle.focus()` on the column.
- "The agent in view" is `focused().agent` (an agent block), else `focused().terminal` (a tab layout like `andon`); it is the row highlighted, and what `next` skips.
- Lantern: on every model, `shared("inbox.lantern")` is set (only when it changes) to `"red"` if any row of Needs you is Failed, else `"amber"` if there is any, else null; all machines, no filter. The andon mod's brand reads it.
- The machine pick (checkboxes; `m` opens the panel, `j`/`k`, `Space`, `a`) narrows both sections, `next` and `setAgentOrder`; checking every machine means all again. The pick is kept in `localStorage`. With exactly one machine picked, rows drop the machine name.
- Keys while focused: `j`/`k`/arrows move (through both sections), Enter/`l` open, `g` then `g` within 1s first row, `G` last row, `/` start a filter (typed characters from `key_char`, Backspace deletes, Enter keeps, Esc clears), Esc otherwise `pane.go` right.

## Built from

- `mod.json`: `{ "place": "none", "title": "Inbox", "focusable": true }`. The `andon` layout is Herdr's `main` with this mod in place of `agents` (left column: the andon mod's brand, this mod at weight 2 with min 120, `spaces` at weight 1, the `workspace-chrome` footer; right: `tabs`, the shown tab with `pane` headers, `files` / `changes`, `workspace-status`).
- `herdr`: `agents()`, `terminals()` (labels, `turn`, `status_ms`, `report`, `parent`), `spaces()` (tab names and `named`), `panes()` (branch), `machines()`, `focused().agent`, `setAgentOrder(needs + projects)`, `run`, `nextCommand()`, `shared` / `setShared`, `theme()`, `localStorage`. No `hc`, no `notify`.
- `ROLE_ICON` (exported): the icon file of each role that talks with you (`chief-of-staff.svg`, `chief-engineer.svg`); a row of such a role draws it instead of the program's icon.
- `name.js`: the display name: a named tab, else a telling title (not a path, `user@host` or the program's name; a leading mark stripped), else the program's name.

## Watch out

- Done and idle are tracked as one state for wait times: looking turns done into idle.
- A role without a pane stays `done` after you look; only the remembered `turn` keeps its reply from showing again.
- A sub-agent's `parent` is a bare address on its own machine; prefix the machine to match a remote parent.
- `<mod>.<command>` reaches only a shown copy; in other layouts it's an unknown command.
- Don't put the shipped `agents` mod in the same layout: both call `setAgentOrder`.

## Done when

- `herdr-app check` passes on the folder.
- A blocked agent shows under Needs you with its kind, its badge amber (red when Failed) in light and dark themes; `inbox.next` opens it in the focused block.
- The lantern at the top is unlit with nothing in Needs you, amber with a reply, red with a failed turn.
- An issue's agent that finished, works or is idle shows nowhere.
- The Chief of Staff shows under Projects; its unseen reply moves it to Needs you (`Reply`), and looking moves it back.
- `hc get <t> --json` shows no label written by the mod.
