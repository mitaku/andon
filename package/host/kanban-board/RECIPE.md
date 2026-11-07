# Kanban board

A Jira-like board over a `kanban/` work ledger (one folder per task, written by the `kanban` CLI): columns of cards, a list by parent, and a detail panel, read live and never showing old contents as new.

## What you see

On the theme's `background`, as a dialog (92% × 88%) or a pane:

- Header, 44px: "Kanban" (14px semibold), the project's title or folder name (`dim`; "<cell> · all projects" for all), Board / List and This project / All projects (24px pills, `selected` when chosen; the second pair only in a cell), chips (22px, radius 11, `raised`/`selected`) for the projects (all) or the repos (a project with more than one), the `/` filter text, a Chief of Staff button (status mark + "Chief of Staff", or "Chief of Staff t2 (not running)" in `dim`) when the config names one (`chief_of_staff`), and a close `x` in a dialog.
- Band, 26px, 11.5px: "Changed …" and "Checked …" (`dim`; `blocked` with "— not checked lately" after 30 s here, 60 s remote); the machine name on the right for a remote ledger. A read failure turns it `blocked` on a 12% `blocked` background: "Can't read the ledger: <first line>" and "showing what was read <ago>".
- Board: 12px padding, columns 270px wide on `surface` (radius 10, 8px padding) with an upper-case label and count, cards scrolling inside. A card (radius 8, 1px `border`, 10px padding): `id` · project (all) or repo (several repos) · `· parent title` (11px `dim`), the title (13px), `after A-11` (11px `muted`) while those aren't done, and the agent row (status mark + `@agent`, or `(not running)`, then the profile as a 16px `raised` pill). Border: `blocked` when its agent is blocked and the task isn't done; `accent` for the keyboard selection; `selected` background when open.
- List: 30px rows, children indented 18px under parents: id, status label, title, project or repo, profile, agent.
- Detail: 440px on `surface` with a left border: id, a status pill, the title (15px semibold), fields (Project when all, Repo, Profile, Parent, After, Phase, Branch, Worktree, Updated), a "Go to @agent" button, links, then Brief and Report as Markdown (`TextView.markdown`), then the `library/` file names.

## How it behaves

- Root: up from the focused pane's folder for `kanban/config.yaml` with a whole-number `version` (at most 12 folders); else from `git rev-parse --git-common-dir`'s parent (the main checkout) up, at each folder the ledgers `projects/*/kanban/config.yaml` (with a `version`) whose `repos` entry (`host/owner/repo`) ends that path (case-insensitive on Windows), stopping at the first folder with any; two or more is an error. The cell: the nearest folder above with such a ledger. In a cell but in no project, All projects. Re-found when the focused folder or machine changes.
- Format: `version: 2` or the ledger isn't read (the band says so; for all projects, it is listed as unreadable).
- All projects: each ledger read on its own; tasks are keyed `<slug>/<id>` (ids are per ledger), `after` and `parent` look inside their own ledger, and a task in its ledger's last column counts as done. The columns are the ledgers' own when they all agree, else the default five.
- Reading: config, then every `tasks/*/task.yaml`; a task that doesn't parse is listed as unreadable above the board and the rest show. A changed signature updates "Changed"; every good read updates "Checked"; a failed read keeps the last contents.
- Agents: a task's `agent` is the terminal's address on the ledger's machine; prefixed with the machine for a remote ledger and matched against `agents()`, else by a terminal name or tab name.
- Keys: as in the README; the dialog takes the keyboard 30 ms after it opens (`cx.timer.after`: focus can't move during a render).

## Built from

- `mod.json`: `{ "place": "none", "title": "Kanban board", "focusable": true, "overlay": { "kind": "dialog", "width": 0.92, "height": 0.88 } }`; `mod.open:kanban-board` toggles it.
- `herdr`: `focused()`, `panes()`, `spaces()`, `agents()`, `terminals()`, `readTextFile`, `exec`, `watchFolder`, `run`, `props()`, `closeOverlay`, `theme()`; `fs/promises` `readdir`/`exists`; `TextView.markdown`.
- js-yaml 5.5.0 (bundled) `load`.

## Watch out

- A `.d.ts` beside a `.mjs` is not used for its types: name it `.d.mts`.
- A scrolling column needs a definite height: give columns `h_full()` and the card list `flex_1().min_h(0)`, or the cards collapse to nothing.
- `fs/promises` reads only under home; another machine needs `exec`/`readTextFile` with `on`.

## Done when

- `herdr-app check` passes.
- A ledger made with `kanban` shows its columns and cards; `kanban done` on a task moves it within seconds, and frees what waited on it.
- Breaking `config.yaml` shows the red band over the last contents; fixing it clears the band.
