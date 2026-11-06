# kanban-board

The work ledger in `kanban/` ([format 2](../../../docs/ledger.md): one ledger per project) as a Jira-like board: columns of cards, a list grouped by parent, and a detail panel with the task's brief and report. It reads only; people and agents change the ledger with `kanban` (`cmd/kanban`).

## What it shows

- **The ledger of the project you are in:** found the way `kanban` finds it ([ledger.md](../../../docs/ledger.md), "Finding the root"): up from the focused pane's folder to a `kanban/config.yaml` with a whole-number `version`, else the project in the plant (`projects/*/kanban/`) whose `repos` name the repository's main checkout, so a worktree anywhere finds it. On this computer or on another machine. Two projects claiming the same repository is shown as such (set `KANBAN_ROOT` for `kanban`). A `kanban/` folder without a `version` (a product's own folder, say) is not a ledger.
- **This project / All projects:** in a plant, **All projects** shows every project's ledger on one board, each card naming its project (title from `config.yaml`, else the folder's name); a project that can't be read (a format it doesn't know, a YAML mistake) is listed as unreadable and the rest show. In the plant's own folder, which is in no project, it opens on All projects.
- **A format it doesn't know is not guessed at:** a ledger without `version: 2` shows why in the red band.
- **YAML it won't expand:** a `config.yaml` or `task.yaml` with a YAML alias (`*name`; `kanban` writes none) or over 256 KiB is not read: the task is listed as unreadable. Ledger files are written by agents, and nested aliases would make a short file grow without bound.
- **A band that never shows old contents as new:** when the contents last changed and when they were last checked; "not checked lately" after 30 s here (60 s on another machine). If the ledger can't be read (a YAML mistake, a missing file), a red band says why and the board keeps what it last read.
- **Board:** a column per status (`config.yaml` `columns`), cards with id, the project (All projects) or the repo (a project with several), parent, what it waits for (`after A-0011`), the agent working on it with its state (Needs you / Working / …) and its profile (`claude`, `codex`, …); a card whose agent needs you and isn't done has a `blocked` border. Click the agent to go to it.
- **List:** tasks under their parents, in column order.
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

Keys while it has the keyboard: `h`/`l` columns, `j`/`k` cards, `Enter` open, `g` go to the agent, `v` board / list, `p` this project / all projects, `/` filter, `Esc` close the detail, the filter, then the dialog.

## What it runs

| Call | When |
|---|---|
| `readTextFile(<folder>/kanban/config.yaml, {on})` up the folders, and `exec("git", ["rev-parse", "--path-format=absolute", "--git-common-dir"], {cwd, on})` | finding the ledger |
| `fs.readdir(<folder>/projects)` + `readTextFile` of each `projects/*/kanban/config.yaml` (this computer, Windows); `exec("sh", ["-c", <fixed script>, "kanban-board", <folder>], {on})` printing each of them (another machine, or not Windows) | finding the project from the repository, finding the plant, reading All projects |
| `fs.readdir` + `readTextFile` of each `task.yaml` (this computer, Windows); `exec("sh", ["-c", <fixed script>, "kanban-board", <root>], {on})` printing each `task.yaml` (another machine, or not Windows) | reading the tasks |
| `watchFolder(<root>/kanban)` for each ledger shown | this computer only |
| `readTextFile` of `brief.md`, `report.md`; `exec("ls", ["-1", <task>/library], {on})` | opening a card |
| `run("agent.focus", {terminal})` | going to a card's agent or the Chief of Staff |
| `exec("explorer.exe" \| "open" \| "xdg-open", [url])`, only `http(s)` URLs | clicking a web link |

Nothing is written. No network. The root path from another machine only ever goes to programs as a separate argument.

## Origin

Copied from herdr-flightdeck (a private repository that Andon came out of) `herdr-app/mods/work-board` at commit `afc57ac5ec85d3123cbf5c067a9cd4ee3cf5b268`, where it was named `work-board`. Changed for Andon: the ledger's folder `.flightdeck/` → `kanban/` (a folder is a ledger only with a `version`), `coordinator` → `chief_of_staff`, `FLIGHTDECK_ROOT` → `KANBAN_ROOT`, the name `work-board` → `kanban-board`, and the plant's words. The preview picture was not copied.

## Licence

The mod is under Andon's licence (Apache-2.0, the repository's `LICENSE`). `vendor/js-yaml/js-yaml.mjs` is [js-yaml](https://github.com/nodeca/js-yaml) 5.5.0 (MIT, `vendor/js-yaml/LICENSE`), `dist/js-yaml.mjs` byte-for-byte as published on npm (SHA-256 `213470960d9c2ff1d01f9164e5017749bfc383d8a6ebb8683f35c76e8c6d052b`). `vendor/js-yaml/js-yaml.d.mts` is ours: the part of its API the mod uses.
