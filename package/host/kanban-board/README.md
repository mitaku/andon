# kanban-board

The work ledger in `kanban/` ([format 3](../../../docs/ledger.md): one ledger per project) as a Jira-like board: columns of cards, a list grouped by parent, the flow of issues in order (Flow), the done issues newest first (History), and a detail panel with the issue's brief and report. It reads only; people and agents change the ledger with `kanban` (`cmd/kanban`).

## What it shows

- **The ledger of the project you are in:** found the way `kanban` finds it ([ledger.md](../../../docs/ledger.md), "Finding the root"): up from the focused pane's folder to a `kanban/config.yaml` with a whole-number `version`, else the project in the cell (`projects/*/kanban/`) whose `repos` name the repository's main checkout, so a worktree anywhere finds it. On this computer or on another machine. Two projects claiming the same repository is shown as such (set `KANBAN_ROOT` for `kanban`). A `kanban/` folder without a `version` (a product's own folder, say) is not a ledger.
- **This project / All projects:** in a cell, **All projects** shows every project's ledger on one board, each card naming its project (title from `config.yaml`, else the folder's name); a project that can't be read (a format it doesn't know, a YAML mistake) is listed as unreadable and the rest show. In the cell's own folder, which is in no project, it opens on All projects.
- **A format it doesn't know is not guessed at:** a ledger without `version: 3` shows why in the red band (format 2 says to move it by hand, `docs/operate.md`).
- **YAML it won't expand:** a `config.yaml` or `issue.yaml` with a YAML alias (`*name`; `kanban` writes none) or over 256 KiB is not read: the issue is listed as unreadable. Ledger files are written by agents, and nested aliases would make a short file grow without bound.
- **A band that never shows old contents as new:** when the contents last changed and when they were last checked; "not checked lately" after 30 s here (60 s on another machine). If the ledger can't be read (a YAML mistake, a missing file), a red band says why and the board keeps what it last read.
- **Board:** a column per status (`config.yaml` `columns`), cards with id, the project (All projects) or the repo (a project with several), parent, what it waits for (`after A-0011`), the agent working on it with its state (Needs you / Working / …) and its profile (`claude`, `codex`, …); a card whose agent needs you and isn't done has a `blocked` border. Click the agent to go to it.
- **Done a day ago or more** ([ADR-0059](../../../docs/adr/0059-board-leaves-old-done-to-history.md) decisions 2–3): a done issue whose `updated` is more than 24 hours ago (or that has no `updated`) leaves Board and List; History has it. Board's Done column holds the rest, newest first, its count the cards in it; under them one line, "565 older · History" (only "Older · History" while the chips or `/` filter), opens History. No line when nothing is old. Flow and the cards are as before: an old issue still names a card's parent and what it waits for, and Flow still shows the done issues of a parent with open ones (decision 5).
- **List:** issues under their parents, in column order; an old done issue stays as the head of what is shown under it.
- **Flow** ([ADR-0042](../../../docs/adr/0042-owner-waits-do-not-stop-the-line.md) decision 13): which issues wait in which order, as a Gantt chart whose axis is the order, not dates. On the left a tree: the project (All projects only), the milestone (only for a project whose `config.yaml` has `phases`, in their order; issues without a `phase` under "No milestone"), then issues under their parents. Every level folds. On the right a bar per issue, colored and named by its status: an issue that waits on no open issue is in stage 1, one that waits on others sits one right of the furthest of them (done ones don't count), so bars in the same stage can go ahead side by side; the filter and chips hide rows but don't move bars. An issue waiting on the owner (merge, decision, a task for you) carries an amber mark; an issue held until a day (`not_before`) says the day. Done issues are left out, except the done issues of a parent that still has open ones (faded). Within a level, what holds an owner wait comes first, then what holds work in progress, then by stage and number. The order among issues that can start is not in the ledger, so it isn't shown; nor are lines between bars: what an issue waits on is in its detail and on its card. Click a row for the detail.
- **History** (ADR-0059 decision 4): every done issue, newest first, one per row: id, when it was done (`updated`), title, the project (All projects) and profile. The newest 200 first; "Load older" adds 200, up to 2,000, then a line says to find older ones with `/`. While `/` is typed it narrows the rows shown; `Enter` looks through every done issue: `andon issue history` looks in the cell (decision 8), or, when the board read every issue with the fixed scripts, through every done issue read. Click a row for the detail.
- **What it reads** (ADR-0059 decisions 6 and 11): at every read, `andon issue board` in the cell gives each project's open issues, those done in the last 24 hours and the issues they point at, with the count of older done ones. When the command can't be used (an older cell, a machine without `andon`, this computer with Windows), the fixed scripts read every issue as before and the board shows the same. Which way it read is not shown.
- **Owner waits** come from `andon waiting --json`, run in the cell in each shown project's folder at every read (it alone counts them; ADR-0042 decision 14). When it is missing, fails or prints a version other than 1, Flow shows no marks and the rest of the board works as before.
- **Chief of Staff:** when `config.yaml` names the project's `chief_of_staff`, a button in the header shows its state and goes to it.
- **Detail** (click a card): fields (Project, Repo, Profile, …), links (web links open in your browser), brief and report rendered as Markdown, the files in `library/`, and a button to go to the agent. Mermaid blocks show as code here (files-plus draws them).
- Chips filter by project (All projects) or by repo (a project with several repos); `/` filters by text.

It reads again as soon as anything under `kanban/` changes on this computer, and every 10 s for another machine.

## Open it

- Add the key to your `keys.json` (also in `keys.json` here), then `ctrl-shift-y` opens and closes it as a large dialog:

  ```json
  { "global": { "ctrl-shift-y": "mod.open:kanban-board" } }
  ```

- Or keep it as a tab: `herdr-app do tab.new --name Board --mod kanban-board`.

Keys while it has the keyboard: `h`/`l` columns, `j`/`k` cards, `Enter` open, `g` go to the agent, `v` board / list / flow / history, `p` this project / all projects, `/` filter, `Esc` close the detail, the filter, then the dialog.

## What it runs

| Call | When |
|---|---|
| `exec("sh", ["-c", <fixed script>, "kanban-board", <max>, <total>, <folder>, <file>…], {on})` printing each `kanban/config.yaml` up the folders (another machine, or not Windows); on this computer with Windows, `exec("powershell", ["-NoProfile", "-NonInteractive", "-Command", <fixed script> followed by the quoted paths])` saying whether each is a reparse point and its length, then `readTextFile`; and `exec("git", ["rev-parse", "--path-format=absolute", "--git-common-dir"], {cwd, on})` | finding the ledger |
| `exec("sh", ["-c", <fixed script>, "kanban-board", <folder>, <max>, <total>], {on})` printing each `projects/*/kanban/config.yaml` (another machine, or not Windows); `fs.readdir(<folder>/projects)` + the PowerShell check + `readTextFile` (this computer, Windows) | finding the project from the repository, finding the cell, reading All projects |
| `exec("andon", ["issue", "board", "--json", "--since", <a day ago>, "--project", <slug>], {cwd: <project>, on, timeout: 2000})`, `--project` only for a project in the cell's `projects/`; not on this computer with Windows | every read, once per project shown: only the issues the board needs ([ADR-0059](../../../docs/adr/0059-board-leaves-old-done-to-history.md) decision 6) |
| `exec("sh", ["-c", <fixed script>, "kanban-board", <root>, <max>, <total>], {on})` printing each `issue.yaml` (another machine, or not Windows); `exec("powershell", …)` listing the issue folders with each `issue.yaml`'s length or that it is a reparse point, then `readTextFile` (this computer, Windows) | reading every issue when `andon issue board` can't be used: missing, failing, printing another version, no `issues` or another `project`, or not back in 2 s (then not called for 5 minutes); always on this computer with Windows (decision 11) |
| `exec("andon", ["issue", "history", "--json", "--limit", <n>, "--match", <text>, "--project", <slug>], {cwd: <project>, on, timeout: 2000})`, `--match` only after `Enter` on `/` text that isn't in the project's name | while History is shown, at every read and when Load older or `Enter` changes what it asks, for each project read through `andon issue board` (decision 8) |
| `exec("andon", ["waiting", "--json"], {cwd: <project>, on})`, not on this computer with Windows | every read, once per project shown (Flow's owner-wait marks) |
| `watchFolder(<root>/kanban)` for each ledger shown | this computer only |
| The same checked read of `brief.md`, `report.md` (1 MiB each); `exec("ls", ["-1", <issue>/library], {on})` | opening a card |
| `run("agent.focus", {terminal})` | going to a card's agent or the Chief of Staff |
| `exec("explorer.exe" \| "open" \| "xdg-open", [url])`, only `http(s)` URLs | clicking a web link |

Every file is checked on the machine it is on before it is read: a file that is a symbolic link (a reparse point on Windows), or whose folder is one (the issue's folder, `kanban/`), is not read and is shown as `symbolic link`; a `config.yaml` or `issue.yaml` over 256 KiB, a brief or report over 1 MiB, and anything past 4 MiB in one read are not read either (the scripts look at the size before `head -c`). Ledger files are written by agents.

Nothing is written (`andon waiting`, `andon issue board` and `andon issue history` only read). No network. The `/` text goes to `andon issue history` as one argument, never through a shell. The root path from another machine only ever goes to programs as a separate argument.

## Origin

Copied from herdr-flightdeck (a private repository that Andon came out of) `herdr-app/mods/work-board` at commit `afc57ac5ec85d3123cbf5c067a9cd4ee3cf5b268`, where it was named `work-board`. Changed for Andon: the ledger's folder `.flightdeck/` → `kanban/` (a folder is a ledger only with a `version`), `coordinator` → `chief_of_staff`, `FLIGHTDECK_ROOT` → `KANBAN_ROOT`, the name `work-board` → `kanban-board`, the checked reads above, and the cell's words. The preview picture was not copied.

## Licence

The mod is under Andon's licence (Apache-2.0, the repository's `LICENSE`). `vendor/js-yaml/js-yaml.mjs` is [js-yaml](https://github.com/nodeca/js-yaml) 5.5.0 (MIT, `vendor/js-yaml/LICENSE`), `dist/js-yaml.mjs` byte-for-byte as published on npm (SHA-256 `213470960d9c2ff1d01f9164e5017749bfc383d8a6ebb8683f35c76e8c6d052b`). `vendor/js-yaml/js-yaml.d.mts` is ours: the part of its API the mod uses.
