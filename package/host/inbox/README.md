# inbox

Two sections, in one column (ADR-0015):

- **Needs you**: what only you can move, longest waiting first. Any blocked agent (an issue's agent, a reviewer or a role; a sub-agent's `error` counts; a parent takes its most urgent sub-agent's state), and a turn the Chief of Staff or a Chief Engineer ended (a reply, until you look). The badge says Permission, Question or Auth when the agent reported which, Failed when its turn failed (the terminal reports `error`), and Reply for a role's reply.
- **Projects**: the way to each project's roles: its Chief of Staff, then its Chief Engineers (by issue), projects by name. They show idle or working; one that needs you moves up to Needs you and comes back once you look.

An issue's agent or a reviewer that finished, works or is idle is not shown: the dispatcher and the Chief of Staff handle it, and kanban-board and the Spaces list show the flow. The dispatcher (`role=dispatcher`) isn't in Projects; it shows in Needs you only when blocked or failed.

Andon's agents are rows of their own even without a pane, when they show: a terminal labelled `issue=<id>` (an agent `andon issue start` started, or a reviewer), or `project=<slug>` with no pane above it. Their second line says `project › issue`. One under a pane stays in that pane's row, so it isn't counted twice. Other sub-agents without a pane fold into their parent as before.

The Chief of Staff and a Chief Engineer (`role=chief-of-staff`, `role=chief-engineer`, or a pane holding one) talk with you, so a turn they end is a reply: while you haven't looked (Herdr's `done`) it is in Needs you with a `Reply` badge, and once you look (Herdr turns it `idle` by itself) it is back in Projects. One without a pane has only the core's status, which stays `done`: opening its row counts as looking, and the inbox remembers that turn (in `shared("inbox.memory")`), so the reply comes back only with a later turn (ADR-0015 decision 7, provisional). Their icon is the role's own, in the theme's accent color, not the program's (they run the same Claude Code as the issue agents): Lucide's `clipboard-list` for the Chief of Staff (`chief-of-staff.svg`), `drafting-compass` for a Chief Engineer (`chief-engineer.svg`). A terminal with a `title` label is named by it (`andon issue start` puts the issue's title there); otherwise the tab, the agent's title or the hc name.

## Lights

A row that needs you is lit in Andon's colors, the logo's rather than the theme's (ADR-0023): its badge is amber (`#F0A202`, dark text) when an answer moves it on (Permission, Question, Auth, Needs you, Reply), red (`#D8453B`, white text) when its turn failed (Failed). The Needs you heading has an amber dot. Nothing blinks; Herdr makes the sounds and notifications.

The lantern at the top of the column (the andon mod's brand, `../andon/`) shows the same rows at a glance: red while any row that needs you failed, amber while rows wait for an answer, unlit when nothing needs you. The inbox decides it and keeps it in `shared("inbox.lantern")` (`"red"`, `"amber"` or null). It counts every row that needs you, on every machine, whatever the machine pick or filter hides (as `+3 waiting elsewhere` does).

The inbox names no tabs: Andon's Spaces and tabs are named by the andon mod (`../andon/`), from the same labels.

Each row, drawn like Herdr's own agents list: the program's icon; the name, how long it has waited (`3m+` means "at least": the mod hasn't seen the change itself, so it uses the core's time) and the status mark; then the machine (`This computer` or its name, with Herdr's monitor/server icon) and the space; then the tab when named, a branch other than main/master, and the agent's message when it gives one.

With more than one machine known, the header's button names the machines the inbox shows (`All machines`, one or two names, or `3 machines`). It opens a list in the column: a checkbox per machine, how many wait there, and Herdr's connection dot (or "not connected"). `next` stays inside the pick. While some machines are left out, `+3 waiting elsewhere` says how many wait on them (click it to show all again), and `1 offline` counts the machines not connected. With one machine picked, rows drop the machine.

Click a row (or select it and press Enter) to open the agent: `agent.focus`, or `terminal.peek` for a blocked sub-agent or one of Andon's agents without a pane. A row stays in Needs you until the terminal's state changes (the Chief of Staff or you answer it, you look at a reply). The mod writes nothing: no label, no sound, no alert (Herdr does those).

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
    "ctrl-shift-h": "inbox.focus"
  }
}
```

- `ctrl-shift-j`: open the longest-waiting row of Needs you that isn't the one in view.
- `ctrl-shift-h`: move the keyboard to the queue from wherever you are typing (Herdr documents `pane.go`, `alt-h`, as reaching focusable mods too; not verified here).

Keys get forgotten, so the column's header has a button for each, its tooltip naming the key: ⏭ next waiting (`ctrl-shift-j`) and the kanban-board mod (`ctrl-shift-y`, its own key).

An older copy of this README also had `ctrl-shift-d` (`inbox.settle`) and `ctrl-shift-u` (`mod.open:inbox`). They are gone; remove them from your `keys.json` (left there they are only unknown commands).

While the queue has the keyboard:

| Key | Does |
|---|---|
| `j` / `k`, `↓` / `↑` | next / previous row |
| `Enter` / `l` | open the selected agent |
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
| `run("mod.open", {name: "kanban-board"})` | the header's kanban-board button |

Addresses come from `agents()` / `terminals()` only. No `hc`, no `exec`, no file reads, no `fetch`, no sounds, no notifications: the mod only reads. `shared("inbox.memory")`: when each agent last changed state, and the turn of a paneless role's reply you opened. `shared("inbox.lantern")`: what the lantern shows. `localStorage` (per copy): the machine picked.

A `settled=` label an older inbox wrote is no longer read; it ends with its terminal.

## Icon

`chief-of-staff.svg`, `chief-engineer.svg`, `next.svg` and `kanban-board.svg` are [Lucide](https://lucide.dev)'s `clipboard-list`, `drafting-compass`, `skip-forward` and `square-kanban` (lucide-static 0.460.0), ISC (`LICENSE-Lucide.txt`), with `stroke-width` 1.5 to match the workspace-chrome icons.

## Origin

Copied from herdr-flightdeck (a private repository that Andon came out of) `herdr-app/mods/inbox` at commit `afc57ac5ec85d3123cbf5c067a9cd4ee3cf5b268`. Changed for Andon: the roles that talk with you are `chief-of-staff` and `chief-engineer` (ADR-0002) instead of `coordinator`; it no longer names tabs (the andon mod does); the kanban-board button and the icons' file names; cut to Needs you and Projects, without Done, the shelves, settle and the board dialog (ADR-0015). The Needs you badges lit amber and red, and the lantern's state (ADR-0023). The one Chief of Staff icon (`network`) became one per role. The preview picture was not copied. It was named `focus-queue` there until 2026-11-04; what such a copy remembered in `shared("focus-queue.memory")` is still read once. Under Andon's licence (Apache-2.0), with the Lucide icons under ISC as above.
