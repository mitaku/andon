# inbox

The agents waiting for you, in one column, longest waiting first:

- **Needs you**: blocked agents (a sub-agent's `error` counts; a parent takes its most urgent sub-agent's state). The badge says Permission, Question or Auth when the agent reported which, and Failed when its turn failed (the terminal reports `error`).
- **Done**: agents that finished and that you haven't settled. Looking at a done agent doesn't settle it.
- Folded shelves: **Working**, **Idle**, **Settled** (newest first, 10, then "Show 25 more").

Andon's agents are rows of their own even without a pane: a terminal labelled `issue=<id>` (an agent `andon issue start` started, or a reviewer), or `project=<slug>` with no pane above it. Their second line says `project › issue`. One under a pane stays in that pane's row, so it isn't counted twice. Other sub-agents without a pane fold into their parent as before.

The Chief of Staff and a Chief Engineer (`role=chief-of-staff`, `role=chief-engineer`, or a pane holding one) are never Done. They talk with you, so a turn they end is a reply: while you haven't looked (Herdr's `done`) it is in Needs you with a `Reply` badge, and once you look (Herdr turns it `idle` by itself) it sits in Idle, no settle needed. One without a pane has only the core's status, which stays `done`: settle it. Their icon is Lucide's `network` in the theme's accent color (`chief-of-staff.svg`), not the program's: they run the same Claude Code as the issue agents. A terminal with a `title` label is named by it (`andon issue start` puts the issue's title there); otherwise the tab, the agent's title or the hc name.

The inbox names no tabs: Andon's Spaces and tabs are named by the andon mod (`../andon/`), from the same labels.

Each row, drawn like Herdr's own agents list: the program's icon; the name, how long it has waited (`3m+` means "at least": the mod hasn't seen the change itself, so it uses the core's time) and the status mark; then the machine (`This computer` or its name, with Herdr's monitor/server icon) and the space; then the tab when named, a branch other than main/master, and the agent's message when it gives one.

With more than one machine known, the header's button names the machines the queue shows (`All machines`, one or two names, or `3 machines`). It opens a list in the column: a checkbox per machine, how many wait there, and Herdr's connection dot (or "not connected"). `next` and `settle` stay inside the pick. While some machines are left out, `+3 waiting elsewhere` says how many wait on them (click it to show all again), and `1 offline` counts the machines not connected. With one machine picked, rows drop the machine.

Click a row (or select it and press Enter) to open the agent: `agent.focus`, or `terminal.peek` for a blocked sub-agent or one of Andon's agents without a pane. Settle with the row's hover `✓`, `d`, or `ctrl-shift-d`: the agent leaves the queue and, if it was the one in view, the next one opens. Settled rows have `↺` to put them back. New work brings a settled agent back by itself; nothing is ever settled automatically. The mod plays no sound and posts no alert (Herdr does that).

## The board

The board button at the right of the queue's header, or `ctrl-shift-u` (`mod.open:inbox`), opens the same queue as a large Kanban dialog, and closes it again: a column per section (Needs you, Done, Working, Idle, Settled); cards name their machine. **By machine** (or `s`) splits it into a swimlane per machine. Cards are the queue's rows and act the same: click to open the agent (the dialog closes), hover for ✓ / ↺. A Needs-you card has a `blocked` border. The machine pick applies here too.

Keys on the board: `h`/`l` columns, `j`/`k` cards (into the next lane at the end), `Enter` open, `d` settle, `u` undo, `s` by machine, `Esc` close.

The column and the board are two copies of one mod; they share what they remember (when each agent changed state, the turn first seen) through `shared("inbox.memory")`, so both judge Done and waiting time alike. Only the column takes key commands and sets the agent order.

## Place it

The `andon` layout (`../layouts/andon.json`) puts it on the left. In your own copy of `main` (`herdr-app do layout.clone '{"name":"main","as":"my-main"}'`), replace the shipped `agents` block in the `sidebar` column with:

```json
{ "id": "inbox", "type": "block", "content": { "mod": "inbox" } }
```

Replace, don't add: two mods that set the agent order in one layout fight over it.

## Keys

Add this to your `keys.json` (also in `keys.json` here):

```json
{
  "global": {
    "ctrl-shift-j": "inbox.next",
    "ctrl-shift-d": "inbox.settle",
    "ctrl-shift-h": "inbox.focus",
    "ctrl-shift-u": "mod.open:inbox"
  }
}
```

- `ctrl-shift-j`: open the longest-waiting agent that isn't the one in view.
- `ctrl-shift-d`: settle the agent in view and open the next.
- `ctrl-shift-u`: open or close the board.
- `ctrl-shift-h`: move the keyboard to the queue from wherever you are typing (Herdr documents `pane.go`, `alt-h`, as reaching focusable mods too; not verified here).

Keys get forgotten, so the column's header has a button for each, its tooltip naming the key: ⏭ next waiting (`ctrl-shift-j`), the board (`ctrl-shift-u`), and the kanban-board mod (`ctrl-shift-y`, its own key). Settling the agent in view (`ctrl-shift-d`) is the `✓` on each row.

While the queue has the keyboard:

| Key | Does |
|---|---|
| `j` / `k`, `↓` / `↑` | next / previous row |
| `Enter` / `l` | open the selected agent |
| `d` | settle the selected row, select the next (the keyboard stays) |
| `u` | undo the last settle made in this window |
| `gg` / `G` | first / last row |
| `m` | open or close the machine panel; in it `j`/`k` move, `Space` checks, `a` picks all |
| `/` | filter by name, space, tab or machine; Enter keeps it, Esc clears it |
| `Esc` | clear the filter, else back to the terminal (`pane.go` right) |

The commands reach only a shown copy of the mod: in a layout without it, `inbox.next` is an unknown command.

## What it runs

| Call | When |
|---|---|
| `run("agent.focus", {terminal})` | opening a row, next |
| `run("terminal.peek", {terminal})` | opening a blocked sub-agent, or one of Andon's agents, without a pane |
| `run("pane.go", {direction: "right"})` | `Esc` in the queue |
| `hc(["label", <address>, "settled=<ms>:<turn>"])` | settle |
| `hc(["label", <address>, "settled="])` | unsettle (`↺`, `u`, or new work) |
| `notify({level: "error"})` | only when a settle or unsettle fails |
| `run("mod.open", {name: "inbox"})` | the header's board button |
| `run("mod.open", {name: "kanban-board"})` | the header's kanban-board button |

Addresses come from `agents()` / `terminals()` only. No `exec`, no file reads, no `fetch`, no sounds. `shared("inbox.memory")`: when each agent last changed state and the turn it had when first seen. `localStorage` (per copy): which shelves are open, the machine picked, whether the board is by machine. The board also runs `closeOverlay` when it opens an agent or closes.

## The `settled` label

`settled=<ms>:<turn>`: when it was settled (epoch ms) and the core's `turn` then. It lives on the terminal, so every window sees it and it ends with the terminal. A settled agent comes back when its `turn` goes past the stored one, or when it (or a sub-agent) turns working or blocked after the settle. Same key and format as herdr-t3-focus, so both share the mark.

## Icon

`chief-of-staff.svg`, `next.svg` and `kanban-board.svg` are [Lucide](https://lucide.dev)'s `network`, `skip-forward` and `square-kanban` (lucide-static 0.460.0), ISC (`LICENSE-Lucide.txt`), with `stroke-width` 1.5 to match the workspace-chrome icons.

## Origin

Copied from herdr-flightdeck (a private repository that Andon came out of) `herdr-app/mods/inbox` at commit `afc57ac5ec85d3123cbf5c067a9cd4ee3cf5b268`. Changed for Andon: the roles that talk with you are `chief-of-staff` and `chief-engineer` (ADR-0002) instead of `coordinator`; it no longer names tabs (the andon mod does); the kanban-board button and the icons' file names. The preview picture was not copied. It was named `focus-queue` there until 2026-11-04; what such a copy remembered in `shared("focus-queue.memory")` is still read once. Under Andon's licence (Apache-2.0), with the Lucide icons under ISC as above.
