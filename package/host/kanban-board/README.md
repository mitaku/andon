# work-board

The work ledger in `.flightdeck/` ([format 2](../../../docs/ledger-format.md): one ledger per project) as a Jira-like board: columns of cards, a list grouped by parent, and a detail panel with the task's brief and report. It reads only; people and agents change the ledger with `kanban` ([mitaku/andon](https://github.com/mitaku/andon), `cmd/kanban`; it was `deck` in this repo until deck-v0.4.0).

![work-board](preview.png)

## What it shows

- **The ledger of the project you are in:** found the way `deck` finds it: up from the focused pane's folder, else the project in the yard (`projects/*/.flightdeck/`) whose `repos` name the repository's main checkout, so a worktree anywhere finds it. On this computer or on another machine. Two projects claiming the same repository is shown as such (set `FLIGHTDECK_ROOT` for `deck`).
- **This project / All projects:** in a yard, **All projects** shows every project's ledger on one board, each card naming its project (title from `config.yaml`, else the folder's name); a project that can't be read (a format 1 ledger, a YAML mistake) is listed as unreadable and the rest show. In the yard's own folder, which is in no project, it opens on All projects.
- **A format it doesn't know is not guessed at:** a ledger without `version: 2` shows why in the red band (format 1 is the old one-ledger-per-yard shape).
- **A band that never shows old contents as new:** when the contents last changed and when they were last checked; "not checked lately" after 30 s here (60 s on another machine). If the ledger can't be read (a YAML mistake, a missing file), a red band says why and the board keeps what it last read.
- **Board:** a column per status (`config.yaml` `columns`), cards with id, the project (All projects) or the repo (a project with several), parent, what it waits for (`after A-11`), the agent working on it with its state (Needs you / Working / …) and its profile (`opus`, `codex`, …); a card whose agent needs you and isn't done has a `blocked` border. Click the agent to go to it.
- **List:** tasks under their parents, in column order.
- **Coordinator:** when `config.yaml` names the project's `coordinator`, a button in the header shows its state and goes to it.
- **Detail** (click a card): fields (Project, Repo, Profile, …), links (web links open in your browser), brief and report rendered as Markdown, the files in `library/`, and a button to go to the agent. Mermaid blocks show as code here (files-plus draws them).
- Chips filter by project (All projects) or by repo (a project with several repos); `/` filters by text.

It reads again as soon as anything under `.flightdeck/` changes on this computer, and every 10 s for another machine.

## Open it

- Add the key to your `keys.json` (also in `keys.json` here), then `ctrl-shift-y` opens and closes it as a large dialog:

  ```json
  { "global": { "ctrl-shift-y": "mod.open:work-board" } }
  ```

- Or keep it as a tab: `herdr-app do tab.new --name Board --mod work-board`.

Keys while it has the keyboard: `h`/`l` columns, `j`/`k` cards, `Enter` open, `g` go to the agent, `v` board / list, `p` this project / all projects, `/` filter, `Esc` close the detail, the filter, then the dialog.

## What it runs

| Call | When |
|---|---|
| `readTextFile(<folder>/.flightdeck/config.yaml, {on})` up the folders, and `exec("git", ["rev-parse", "--path-format=absolute", "--git-common-dir"], {cwd, on})` | finding the ledger |
| `fs.readdir(<folder>/projects)` + `readTextFile` of each `projects/*/.flightdeck/config.yaml` (this computer, Windows); `exec("sh", ["-c", <fixed script>, "work-board", <folder>], {on})` printing each of them (another machine, or not Windows) | finding the project from the repository, finding the yard, reading All projects |
| `fs.readdir` + `readTextFile` of each `task.yaml` (this computer, Windows); `exec("sh", ["-c", <fixed script>, "work-board", <root>], {on})` printing each `task.yaml` (another machine, or not Windows) | reading the tasks |
| `watchFolder(<root>/.flightdeck)` for each ledger shown | this computer only |
| `readTextFile` of `brief.md`, `report.md`; `exec("ls", ["-1", <task>/library], {on})` | opening a card |
| `run("agent.focus", {terminal})` | going to a card's agent or the coordinator |
| `exec("explorer.exe" \| "open" \| "xdg-open", [url])`, only `http(s)` URLs | clicking a web link |

Nothing is written. No network. The root path from another machine only ever goes to programs as a separate argument.

## Licence

`vendor/js-yaml/js-yaml.mjs` is [js-yaml](https://github.com/nodeca/js-yaml) 5.5.0 (MIT, `vendor/js-yaml/LICENSE`), `dist/js-yaml.mjs` byte-for-byte as published on npm (the CLI used the same file until it moved to Go; SHA-256 `213470960d9c2ff1d01f9164e5017749bfc383d8a6ebb8683f35c76e8c6d052b`). `vendor/js-yaml/js-yaml.d.mts` is ours: the part of its API the mod uses.
