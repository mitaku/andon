# inbox

Two sections, in one column (ADR-0015), and above them a line per project with something on the ledger that calls you (ADR-0043, ADR-0062):

- **Waiting lines** (no heading, above Needs you): one per project with owner waits that call you: an amber dot, the project, the counts by kind (`2 to merge · 1 to decide · 1 to do`) and how long the oldest of them has waited. Click one to open the waiting pane (the waiting mod in the right panel); in a layout without that pane, it goes to the project's Chief of Staff. What is left for later shows only in the pane, and isn't counted here. With a waiting line and nothing in Needs you, "Nothing needs you." isn't shown.
- **Needs you**: terminals that wait for you, longest waiting first. Any blocked agent (an issue's agent, a reviewer or a role; a sub-agent's `error` counts; a parent takes its most urgent sub-agent's state). The badge says Permission, Question or Auth when the agent reported which, Failed when its turn failed (the terminal reports `error`). A Chief Engineer's dialog question is a blocked terminal, so it shows here.
- **Projects**: the way to each project's roles: its Chief of Staff, then its Chief Engineers (by issue), projects by name. They show idle or working, with a small dot when a reply waits unread; one that is blocked moves up to Needs you.

An issue's agent or a reviewer that finished, works or is idle is not shown: the dispatcher and the Chief of Staff handle it, and kanban-board and the Spaces list show the flow. The dispatcher (`role=dispatcher`) isn't in Projects; it shows in Needs you only when blocked or failed.

Andon's agents are rows of their own even without a pane, when they show: a terminal labelled `issue=<id>` (an agent `andon issue start` started, or a reviewer), or `project=<slug>` with no pane above it. Their second line says `project › issue`. One under a pane stays in that pane's row, so it isn't counted twice. Other sub-agents without a pane fold into their parent as before.

The Chief of Staff and a Chief Engineer (`role=chief-of-staff`, `role=chief-engineer`, or a pane holding one) talk with you, so a turn they end is a reply. A reply isn't a light (ADR-0043 decision 5): while you haven't looked (Herdr's `done`) the role's Projects row carries a small dot in the theme's `muted` color, on every unseen reply, whatever the dispatcher's `news.replied` label says (decision 6); once you look (Herdr turns it `idle` by itself) the dot goes. One without a pane has only the core's status, which stays `done`: opening its row counts as looking, and the inbox remembers that turn (in `shared("inbox.memory")`), so the dot comes back only with a later turn (ADR-0015 decision 7, provisional).

Where a project's waits can't be read (below), its roles' replies light as before: in Needs you with a `Reply` badge, amber, until you look. A line under the waiting lines says which project and why (`sample-site: replies light the lantern (andon waiting exited 1: …)`). Their icon is the role's own, in the theme's accent color, not the program's (they run the same Claude Code as the issue agents): Lucide's `clipboard-list` for the Chief of Staff (`chief-of-staff.svg`), `drafting-compass` for a Chief Engineer (`chief-engineer.svg`). A terminal with a `title` label is named by it (`andon issue start` puts the issue's title there); otherwise the tab, the agent's title or the hc name.

## Lights

The light means the person is called: something moves on only when you answer, merge or do it (ADR-0043 decisions 2 and 3, ADR-0062 decision 5). It has three states, decided here and nowhere else:

| State | Value | When | Your move |
|---|---|---|---|
| red | `"red"` | a terminal's turn failed and stopped | look into it |
| amber | `"amber"` | a terminal is blocked waiting for you, or an owner wait on the ledger calls you: a merge, a decision or hand work (or, where waits can't be read, a role's unseen reply) | answer, merge or do it |
| unlit | `null` | nothing waits for you | none |

Red wins over amber. An unread reply is a mark, not a light. What is left for later doesn't light. There is no green.

Which owner waits call you is andon's rule, not the inbox's: each item of `andon waiting --json` says so in `call` (ADR-0062 decisions 6 and 7). An item with `call` counts when `call` is true, whatever its `kind`, and is left for later when it is false. An item without `call` (a cell whose andon is older than the field) is read by the rule from before: merges and decisions call, hand work doesn't.

A row that needs you is lit in Andon's colors, the logo's rather than the theme's (ADR-0023): its badge is amber (`#F0A202`, dark text) when an answer moves it on (Permission, Question, Auth, Needs you, and Reply where waits can't be read), red (`#D8453B`, white text) when its turn failed (Failed). The Needs you heading and each waiting line have an amber dot. Nothing blinks; Herdr makes the sounds and notifications.

The lantern at the top of the column (the andon mod's brand, `../andon/`) shows the state at a glance. The inbox keeps it in `shared("inbox.lantern")` (`"red"`, `"amber"` or null), and what it counted in `shared("inbox.lamp")`:

```json
{"version": 2, "at": 1795262400000, "light": "amber", "failed": 0, "blocked": 1, "merge": 2, "decision": 1, "task": 1, "later": 3, "reply": 0, "unread": 1, "unreadable": 0}
```

| Field | Is |
|---|---|
| `version` | 2, the form of this value (1 counted every merge and decision waiting, and had no `task` or `later`) |
| `at` | when the inbox put it: milliseconds since 1970-01-01 UTC |
| `light` | equals `inbox.lantern` |
| `failed` | the failed rows |
| `blocked` | the other blocked rows |
| `merge` | the merges on the ledger that call you |
| `decision` | the decisions on the ledger that call you |
| `task` | the hand work on the ledger that calls you |
| `later` | the owner waits on the ledger left for later (not lit) |
| `reply` | the replies lit where waits can't be read |
| `unread` | the unread marks (not lit) |
| `unreadable` | the projects whose waits can't be read |

Both count every machine, whatever the machine pick or filter hides (as `+3 waiting elsewhere` does).

`inbox.lamp` is put again at once when anything but `at` changes, and otherwise when its `at` is 30 seconds old or ahead of the clock (ADR-0045 decision 5). It is decided again as soon as a project's waits are read, painted or not. So `at` tells a reader outside the App whether someone still keeps the value.

The Stream Deck plugin (`../streamdeck/andon.lamp.sdPlugin/`) reads `inbox.lamp` from the App's `shared.json`. It is not one of Andon's contracts (`schema/`): this table is its form. A change that gives a field another meaning raises `version`, and changes the plugin with it.

## Owner waits

For each machine and project with a Chief of Staff terminal (labels `role=chief-of-staff` and `project`), the inbox runs `andon waiting --json --project <slug>` on that machine and reads only `version`, `project`, and each item's `kind`, `since` and `call`; the counting is andon's (ADR-0042, ADR-0043 decision 8, ADR-0062 decision 7). The slug is the `project` label and is passed only when it has a kanban project's shape: a lowercase letter or digit, then lowercase letters, digits, `_` and `-`. It reads on the 30-second timer and when a role's state changed, at most once per pair in 10 seconds, whichever copy of the mod reads: the results and the failures in a row are kept per pair in `shared("inbox.waiting")`. So a wait on the ledger reaches the lantern within 30 seconds, and at once when the Chief of Staff ends a turn after asking.

A pair can't be read when `andon` is missing or fails, prints no JSON, prints a `version` other than 1, or names another `project`. The first and second times, the last output read is used; from the third time in a row, the project's replies light as before (decision 9) until one read succeeds. A project with no Chief of Staff terminal on that machine, or a label not shaped like a slug, can't be read either. Until the first read of a pair, its replies light as before too, without the reason line.

The inbox names no tabs: Andon's Spaces and tabs are named by the andon mod (`../andon/`), from the same labels.

Each row, drawn like Herdr's own agents list: the program's icon; the name, how long it has waited (`3m+` means "at least": the mod hasn't seen the change itself, so it uses the core's time) and the status mark; then the machine (`This computer` or its name, with Herdr's monitor/server icon) and the space; then the tab when named, a branch other than main/master, and the agent's message when it gives one.

With more than one machine known, the header's button names the machines the inbox shows (`All machines`, one or two names, or `3 machines`). It opens a list in the column: a checkbox per machine, how many wait there, and Herdr's connection dot (or "not connected"). `next` stays inside the pick. While some machines are left out, their waiting lines leave too, and `+3 waiting elsewhere` says how many wait on them, Needs you rows and ledger waits together (click it to show all again), and `1 offline` counts the machines not connected. With one machine picked, rows drop the machine.

Click a row (or select it and press Enter) to open the agent: `agent.focus`, or `terminal.peek` for a blocked sub-agent or one of Andon's agents without a pane. A row stays in Needs you until the terminal's state changes (the Chief of Staff or you answer it). The mod writes nothing: no label, no file, no kanban, no sound, no alert (Herdr does those).

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

- `ctrl-shift-j`: open the longest-waiting row of Needs you that isn't the one in view; with none, the role whose reply has waited unread longest.
- `ctrl-shift-h`: move the keyboard to the queue from wherever you are typing (Herdr documents `pane.go`, `alt-h`, as reaching focusable mods too; not verified here).

Keys get forgotten, so the column's header has a button for each, its tooltip naming the key: ⏭ next waiting (`ctrl-shift-j`) and the kanban-board mod (`ctrl-shift-y`, its own key). A third button, ⌛, shows the right panel on the waiting mod's screen (what waits on you on the ledger: merges, decisions, owner tasks). It has no count: Needs you stays terminal states.

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

One more command has no key: `inbox.call`, for something outside the App (the Stream Deck plugin runs `herdr-app do inbox.call`). It opens the next of what lights the lantern, as a click would: Needs you oldest first, then the waiting lines by their oldest wait, and round again, whatever the machine pick or the filter hides. With nothing to open it does nothing (ADR-0045 decision 9).

The commands reach only a shown copy of the mod: in a layout without it, `inbox.next` is an unknown command.

## What it runs

| Call | When |
|---|---|
| `run("agent.focus", {terminal})` | opening a row, next, `inbox.call` |
| `run("terminal.peek", {terminal})` | opening a blocked sub-agent, or one of Andon's agents, without a pane |
| `run("pane.go", {direction: "right"})` | `Esc` in the queue |
| `run("mod.open", {name: "kanban-board"})` | the header's kanban-board button |
| `run("layout.show", {id: "panel"})`, then `run("layout.select", {stack: "panel-body", child: "waiting"})` | the header's waiting button (the `andon` layout's right panel), and a waiting line (clicked, or by `inbox.call`) when the layout has the waiting pane |
| `exec("andon", ["waiting", "--json", "--project", slug], {on, timeout: 15000})` | reading a project's owner waits on its Chief of Staff's machine: on the 30-second timer and when a role's state changed, at most once per pair in 10 seconds; the App stops a call after 15 seconds |

Addresses come from `agents()` / `terminals()` only; `layout()` only tells whether the waiting pane is there. The one `exec` is the read-only command above. No `hc`, no other command, no file reads, no `fetch`, no sounds, no notifications: the mod only reads. `shared("inbox.memory")`: when each agent last changed state, and the turn of a paneless role's reply you opened. `shared("inbox.waiting")`: per machine and project, when it was last read, the items' kinds, times and `call`, and the failures in a row. `shared("inbox.lantern")` and `shared("inbox.lamp")`: what the lantern shows, and what it counted with when it was put (the Stream Deck plugin reads the second). `localStorage` (per copy): the machine picked.

A `settled=` label an older inbox wrote is no longer read; it ends with its terminal.

## Icon

`chief-of-staff.svg`, `chief-engineer.svg`, `next.svg` and `kanban-board.svg` are [Lucide](https://lucide.dev)'s `clipboard-list`, `drafting-compass`, `skip-forward` and `square-kanban` (lucide-static 0.460.0), and `waiting.svg` is its `hourglass` (lucide 1.77.0), ISC (`LICENSE-Lucide.txt`), with `stroke-width` 1.5 to match the workspace-chrome icons.

## Origin

Copied from herdr-flightdeck (a private repository that Andon came out of) `herdr-app/mods/inbox` at commit `afc57ac5ec85d3123cbf5c067a9cd4ee3cf5b268`. Changed for Andon: the roles that talk with you are `chief-of-staff` and `chief-engineer` (ADR-0002) instead of `coordinator`; it no longer names tabs (the andon mod does); the kanban-board button, the waiting button (ADR-0042) and the icons' file names; cut to Needs you and Projects, without Done, the shelves, settle and the board dialog (ADR-0015). The Needs you badges lit amber and red, and the lantern's state (ADR-0023). The lantern lit by blocked terminals and the ledger's owner waits instead of replies, the waiting lines and the unread mark (ADR-0043). `inbox.lamp` dated and kept fresh, and `inbox.call` (ADR-0045). Owner waits counted by their `call`, hand work that calls lit and counted, and `inbox.lamp` version 2 (ADR-0062). The one Chief of Staff icon (`network`) became one per role. The preview picture was not copied. It was named `focus-queue` there until 2026-11-04; what such a copy remembered in `shared("focus-queue.memory")` is still read once. Under Andon's licence (Apache-2.0), with the Lucide icons under ISC as above.
