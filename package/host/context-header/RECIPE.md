# Context header

A two-line pane header: what this terminal's work is, where it lives, and what it waits for.

![Context header](preview.png)

## What you see

On the theme's `background`, 1px `border` under it, 10px side padding, 46px high (set by the layout):

- Line 1 (20px, 12.5px text): the program icon (14px, `dim` when tinted), the name (`foreground`, medium, truncating), then on the right in 11px `dim`: `n sub-agent(s)`, `settled`, the time in the current state (`<1m`, `5m`, `2h`, `3d`), and the status mark.
- Line 2 (16px, 11.5px, `dim`): `This computer` or the machine name · space · named tab · branch (not main/master) · folder (`~` for home). While blocked with a message: the message in `blocked`, cut to 120 characters.

## How it behaves

- Status mark as Herdr's agents list: blocked, a `blocked`-tinted pill reading Permission / Question / Auth from the report's `kind`, else "Needs you"; working, `spinner.svg` turning; done, a `done` dot; idle, a ring. A plain shell has none.
- Name: a tab someone named; else the terminal title when it says something (not a path, not `user@host`, not the program's name); else the program's name. Plain shells show their title.
- Time: the core's `status_ms`, else the tab's `changed_at`. Redraws every 30s.
- Sub-agents: live terminals whose `parent` is this one (prefixing the machine for remote ones) and that have no pane.
- `settled` shows while the terminal carries the `settled` label (written by inbox) and isn't working or blocked.
- Hover shows four 14px icon buttons (`mods/pane/split-right.svg`, `split-down`, `zoom`/`unzoom`, `close`) running `pane.split`, `pane.zoom`, `pane.close` for this pane; a click on the header runs `pane.focus`.

## Built from

- `mod.json`: `{ "place": "none", "title": "Context header" }`. Used as a tab block's `pane.header` mod; the app gives `props()` = `{ pane, terminal }`.
- `herdr`: `panes()` (title, cwd, branch, status, program, zoomed), `terminals()` (report, labels, status_ms, parent), `spaces()` (space and tab names, machine, `changed_at`), `run`, `theme()`, `home()`.
- `name.js`: the display name rule, the same as inbox's (each mod keeps its own copy).

## Watch out

- `hc ls` (what `terminals()` mirrors) can leave `status_ms` out for local terminals; fall back to the tab's `changed_at`.
- A sub-agent's `parent` is a bare address on its own machine.
- Keep hover as state and stop the click on the buttons, so they don't also focus the pane.

## Done when

- `herdr-app check` passes.
- A blocked agent's header shows its kind and time; a remote agent's shows its machine; hovering shows the four buttons and they act on that pane.
