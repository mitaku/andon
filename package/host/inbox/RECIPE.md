# Inbox

A column of two sections: **Needs you** (blocked agents, oldest first) and **Projects** (each project's Chief of Staff and Chief Engineers, always, with an unread dot while a reply waits), and above them one line per project with owner waits that call you, read from `andon waiting --json --project <slug>` (ADR-0043, ADR-0062). Click a row to answer it in its terminal. An issue's agent shows only while blocked. The mod only reads; it writes no label.

## What you see

Top to bottom, on the theme's `chrome` background, 13px text:

1. A 31px header like Herdr's Spaces header: "Inbox" (11.5px, `dim`) on the left; on the right, when more than one machine is known, the machine button, then three 24px icon buttons (13px icon, tooltip naming the key): next waiting (`next.svg`, `ctrl-shift-j`), the kanban-board mod (`kanban-board.svg`, `run("mod.open", {name: "kanban-board"})`, `ctrl-shift-y`) and the waiting screen (`waiting.svg`, "Waiting on you (right panel)": `run("layout.show", {id: "panel"})` then `run("layout.select", {stack: "panel-body", child: "waiting"})`; no count).
2. A filter line `/<text>` while filtering.
3. Waiting lines, no heading: per project with owner waits that call you (radius 8, padding 5×8, gap 8, clickable): a 7px amber `#F0A202` dot, the project (`foreground`), the counts (11.5px, `muted`: `2 to merge · 1 to decide · 1 to do`, a kind left out at zero; what is left for later not counted), the oldest wait that calls (11px, `dim`). Then, per project whose waits can't be read, one line (11px, `dim`): `<project>: replies light the lantern (<why>)`.
4. **Needs you** heading (11px, `foreground`, after a 7px amber `#F0A202` dot) with the count, then its rows; with none, "Nothing needs you." (12px, `dim`), left out while a waiting line shows (kept as "Nothing matches." while filtering).
5. **Projects** heading (11px, `dim`) with the count, then its rows: by project name, the Chief of Staff, then Chief Engineers by issue. Left out when there are none.

The machine button: 24px, radius 6, 11.5px, Herdr's `server.svg` (12px), the pick (`All machines` in `dim`, else one or two names or `3 machines` in `foreground`) and a chevron (right, down while open). Under the header, when items wait on machines left out: `+n waiting elsewhere` (11px, `blocked`; click: all again), and `n offline` (`dim`). Open, a list (radius 8, `popover`, 1px `border`, 8px in from the column's sides) of 28px lines: a 14px checkbox (checked: `solid` with Herdr's `check.svg` in `solid_foreground`; else a 1.5px `input` border), the `monitor`/`server` icon, the name (`This computer` for this one), the number waiting (`blocked`) or "not connected", and Herdr's 6px connection dot (`ansi2`, `dim` when away); then an `All machines` line. While the queue has the keyboard, a 24px key hint sits at the bottom.

A row (radius 8, padding 6×8, gap 9), like Herdr's agents rows: the program icon (16px) on the left; then (1) the name (13px; `muted` for idle), the wait (11px, `dim`; with `+` when the time comes from the core's `status_ms` rather than a change the mod saw), and the status mark; (2) 11.5px `dim`: the machine (Herdr's `monitor`/`server` icon and `This computer` or its name, in `muted`) `·` the space, left out when one machine is picked; (3) 11.5px `dim`: the tab when someone named it and it isn't the name, a branch other than main/master, the report message cut to 60 characters.

## How it behaves

- Status mark, as Herdr's agents list draws it: a row in Needs you, a lit pill (radius 5, 18px, 10.5px semibold) with Permission / Question / Auth from the report's `kind`, Failed, Reply, else "Needs you"; Failed is red (`#D8453B`, text `#FFFFFF`), the rest amber (`#F0A202`, text `#1F2328`): the logo's colors (`GLOW`), never the theme's `blocked`; working, Herdr's `spinner.svg` turning (`.spin(900)`); a role's unread reply, a filled 7px dot in `muted` (tooltip "Unread reply"); otherwise a ring.
- Section: blocked (a terminal `error` counts) → Needs you; a role that talks with you (`role=chief-of-staff`, `role=chief-engineer`, on the terminal or a sub-agent under its pane) and `done` is an unseen reply, unless it is paneless and its row was opened at this `turn` (kept in `shared("inbox.memory")` as `seen`): in a project whose waits are read, it stays in Projects with the unread dot; where they can't be read (not yet, 3 failures in a row, no Chief of Staff on its machine, a label not shaped like a slug) → Needs you (`Reply`); any other such role → Projects; anything else → not shown. `role=dispatcher` is not a role that talks with you.
- Calling: an item calls when its `call` is `true`; `false` is left for later. An item without a boolean `call` (an andon older than the field) calls when its `kind` is `merge` or `decision` (`isCalling`, exported). Only `merge`, `decision` and `task` items are counted.
- Owner waits: per machine and project with a `role=chief-of-staff` terminal, `exec("andon", ["waiting", "--json", "--project", slug], {on, timeout: 15000})` (`on`: the terminal's machine, null for this computer), only for a slug shaped `^[a-z0-9][a-z0-9_-]*$` (a kanban project's). Read on the 30-second timer and on a role's state change, each pair at most once per 10 seconds across copies (`shared("inbox.waiting")`: `{at, fails, why, ok: {at, items: [{kind, since, call?}]}}` per `<machine>/<project>`). Unreadable: not exit 0, not JSON, `version` not 1, `project` not the slug. Fewer than 3 in a row: the last good output stands.
- A waiting line's click: when `layout()` holds the block `waiting` (mod `waiting`), `layout.show` `panel` and `layout.select` `panel-body`/`waiting`; else the project's Chief of Staff (`agent.focus`, or `terminal.peek` without a pane).
- Click a row or Enter: `agent.focus`; if the parent isn't blocked but a sub-agent without a pane is, `terminal.peek` that sub-agent.
- `next` (key command): open the first row that isn't the agent in view, walking Needs you oldest first, then the roles with an unread reply oldest first.
- `call` (command without a key, ADR-0045 decision 9): the one after what `call` opened last in this copy, else the first, of Needs you oldest first and then the waiting lines by oldest wait, neither narrowed by the pick or the filter; open it as a click does; nothing with an empty list.
- `focus` (key command): `handle.focus()` on the column.
- "The agent in view" is `focused().agent` (an agent block), else `focused().terminal` (a tab layout like `andon`); it is the row highlighted, and what `next` skips.
- Lantern: on every model, `shared("inbox.lantern")` is set (only when it changes) to `"red"` if any row of Needs you is Failed, else `"amber"` if any row is blocked or a Reply, or a waiting line has anything that calls, else null; all machines, no filter. `shared("inbox.lamp")` holds `{version: 2, at, light, failed, blocked, merge, decision, task, later, reply, unread, unreadable}` (`merge`, `decision`, `task`: what calls; `later`: the items left for later in every project read; `unread` and `later` don't light): set when anything but `at` differs (compare field by field), or `at` is 30 seconds old, ahead of now or no number; decided again right after each read of owner waits, without waiting for a paint. The andon mod's brand reads `inbox.lantern`; the Stream Deck plugin reads `inbox.lamp`.
- The machine pick (checkboxes; `m` opens the panel, `j`/`k`, `Space`, `a`) narrows both sections, the waiting lines (their waits join `+n waiting elsewhere`), `next` and `setAgentOrder`; the filter doesn't narrow the waiting lines; checking every machine means all again. The pick is kept in `localStorage`. With exactly one machine picked, rows drop the machine name.
- Keys while focused: `j`/`k`/arrows move (through both sections), Enter/`l` open, `g` then `g` within 1s first row, `G` last row, `/` start a filter (typed characters from `key_char`, Backspace deletes, Enter keeps, Esc clears), Esc otherwise `pane.go` right.

## Built from

- `mod.json`: `{ "place": "none", "title": "Inbox", "focusable": true }`. The `andon` layout is Herdr's `main` with this mod in place of `agents` (left column: the andon mod's brand, this mod at weight 2 with min 120, `spaces` at weight 1, the `workspace-chrome` footer; right: `tabs`, the shown tab with `pane` headers, `files` / `changes`, `workspace-status`).
- `herdr`: `agents()`, `terminals()` (labels, `turn`, `status_ms`, `report`, `parent`), `spaces()` (tab names and `named`), `panes()` (branch), `machines()`, `focused().agent`, `layout()` (is the waiting pane there), `exec` (only `andon waiting`), `setAgentOrder(needs + projects)`, `run`, `nextCommand()`, `shared` / `setShared`, `theme()`, `localStorage`. No `hc`, no `notify`.
- `parseWaiting` (exported): what `andon waiting` printed, as the items' kinds, times and `call` (when it is a boolean) or why it can't be read.
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
- The lantern at the top is unlit with nothing waiting (a role's reply only marks its row), amber with a blocked terminal or a merge, decision or hand work on the ledger that calls, red with a failed turn. With only what is left for later, unlit, no waiting line, and `later` counts it.
- An issue's agent that finished, works or is idle shows nowhere.
- The Chief of Staff shows under Projects; its unseen reply puts the unread dot on its row, and looking takes it off. With the cell's `andon` older than `andon waiting`, after three reads the reply is a `Reply` in Needs you again, with the reason line.
- `hc get <t> --json` shows no label written by the mod.
