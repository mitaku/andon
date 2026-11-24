# Kanban board

A Jira-like board over a `kanban/` work ledger (one folder per issue, written by the `kanban` CLI): columns of cards, a list by parent, the flow of issues in order (Flow), the done issues newest first (History), and a detail panel, read live and never showing old contents as new.

## What you see

On the theme's `background`, as a dialog (92% × 88%) or a pane:

- Header, 44px: "Kanban" (14px semibold), the project's title or folder name (`dim`; "<cell> · all projects" for all), Board / List / Flow / History and This project / All projects (24px pills, `selected` when chosen; the second pair only in a cell), chips (22px, radius 11, `raised`/`selected`) for the projects (all) or the repos (a project with more than one), the `/` filter text, a Chief of Staff button (status mark + "Chief of Staff", or "Chief of Staff t2 (not running)" in `dim`) when the config names one (`chief_of_staff`), and a close `x` in a dialog.
- Band, 26px, 11.5px: "Changed …" and "Checked …" (`dim`; `blocked` with "— not checked lately" after 30 s here, 60 s remote); the machine name on the right for a remote ledger. A read failure turns it `blocked` on a 12% `blocked` background: "Can't read the ledger: <first line>" and "showing what was read <ago>".
- Board: 12px padding, columns 270px wide on `surface` (radius 10, 8px padding) with an upper-case label and count, cards scrolling inside. A card (radius 8, 1px `border`, 10px padding): `id` · project (all) or repo (several repos) · `· parent title` (11px `dim`), the title (13px), `after A-11` (11px `muted`) while those aren't done, and the agent row (status mark + `@agent`, or `(not running)`, then the profile as a 16px `raised` pill). Border: `blocked` when its agent is blocked and the issue isn't done; `accent` for the keyboard selection; `selected` background when open. The Done column holds only issues done within 24 hours, newest first; under its cards a 22px line (11.5px `muted`, `foreground` on hover) "<n> older · History", or "Older · History" while filtered, when any done issue is older.
- List: 30px rows, children indented 18px under parents: id, status label, title, project or repo, profile, agent. No done issue older than 24 hours, except one heading rows under it.
- History: 30px rows, newest first: id (60px, `dim`), when done (110px, 11px `muted`, local `YYYY-MM-DD HH:MM`), title (13px), the project (all), profile pill; "Nothing done yet." when empty. Under the rows "Load older" (12px `accent`) while more are held, or at 2,000 "Showing the newest 2000. Find older ones with /" (`dim`).
- Flow: a 22px axis row ("Issue", then the stage numbers 1…n, 128px each), then 28px rows. Left, 400px: a fold mark (▾/▸, 14px) indented 16px per level, then a project (semibold `foreground`) or milestone (semibold `muted`) name, or the issue's id (60px, `dim`) and title (13px). Right: a 120px bar (18px, radius 5, 1px border in the status color on its 18% fill: `working` for doing, `accent` for review, `done` for done, `dim` otherwise) after (stage − 1) × 128px, naming the status; an amber (#F0A202) pill "Merge" / "Decision" / "Your task" for an owner wait; "from <day>" (`muted`) for `not_before`. Done rows at half opacity; the open row `selected`.
- Detail: 440px on `surface` with a left border: id, a status pill, the title (15px semibold), fields (Project when all, Repo, Profile, Parent, After, Phase, Branch, Worktree, Updated), a "Go to @agent" button, links, then Brief and Report as Markdown (`TextView.markdown`), then the `library/` file names.

## How it behaves

- Root: up from the focused pane's folder for `kanban/config.yaml` with a whole-number `version` (at most 12 folders); else from `git rev-parse --git-common-dir`'s parent (the main checkout) up, at each folder the ledgers `projects/*/kanban/config.yaml` (with a `version`) whose `repos` entry (`host/owner/repo`) ends that path (case-insensitive on Windows), stopping at the first folder with any; two or more is an error. The cell: the nearest folder above with such a ledger. In a cell but in no project, All projects. Re-found when the focused folder or machine changes.
- Format: `version: 3` or the ledger isn't read (the band says so; for all projects, it is listed as unreadable).
- All projects: each ledger read on its own; issues are keyed `<slug>/<id>` (ids are per ledger), `after` and `parent` look inside their own ledger, and an issue in its ledger's last column counts as done. The columns are the ledgers' own when they all agree, else the default five.
- Reading: config, then every `issues/*/issue.yaml`; an issue that doesn't parse is listed as unreadable above the board and the rest show. A changed signature updates "Changed"; every good read updates "Checked"; a failed read keeps the last contents.
- Flow: stage 1 for an issue waiting on no open issue in its ledger; else one more than the furthest open issue it waits on; every issue in a cycle is stage 1. Stages count every issue in the ledger, so the filter and chips hide rows without moving bars. Shown: open issues, and done issues with an open issue under them or under their parent. Order within a level: holds an owner wait, holds `doing`/`review`, then stage, then number; milestones in `phases` order, then "No milestone"; a parent's issues follow it whatever their own `phase`. Folding is kept across reads.
- Old done issues (ADR-0059): an issue in its ledger's last column whose `updated` (a quoted time, or the Date js-yaml makes of an unquoted one) is 24 hours or more before `Date.now()`, or doesn't parse, is old. Board and List leave old issues out; the Done column sorts by `updated`, newest first (ties by id); the line's count is every old issue of the shown projects, whatever the filter. Flow and the cards look at every issue read. History: every done issue by `updated`, newest first, `historyLimit` of them (200, +200 per "Load older", at most 2,000; back to 200 on a new place or scope); while typing `/` the filter narrows those rows, after `Enter` it applies to every done issue before the limit. `v` turns Board → List → Flow → History.
- Owner waits: `andon waiting --json` in each project's folder (`exec` with `cwd`, `on`) after each good read, on its own (`cx.spawn`, one at a time) so a command that hangs stops only the marks; `version` must be 1 and each item's `kind` one of merge / decision / task, else no marks. The marks don't move "Changed".
- Agents: an issue's `agent` is the terminal's address on the ledger's machine; prefixed with the machine for a remote ledger and matched against `agents()`, else by a terminal name or tab name.
- Keys: as in the README; the dialog takes the keyboard 30 ms after it opens (`cx.timer.after`: focus can't move during a render).

## Built from

- `mod.json`: `{ "place": "none", "title": "Kanban board", "focusable": true, "overlay": { "kind": "dialog", "width": 0.92, "height": 0.88 } }`; `mod.open:kanban-board` toggles it.
- `herdr`: `focused()`, `panes()`, `spaces()`, `agents()`, `terminals()`, `readTextFile`, `exec`, `watchFolder`, `run`, `props()`, `closeOverlay`, `theme()`; `fs/promises` `readdir`/`exists`; `TextView.markdown`.
- js-yaml 5.5.0 (bundled) `load`.
- `andon waiting --json` (the cell's Andon; [contract.md](../../../docs/contract.md) §25) for the owner-wait marks.

## Watch out

- A `.d.ts` beside a `.mjs` is not used for its types: name it `.d.mts`.
- A scrolling column needs a definite height: give columns `h_full()` and the card list `flex_1().min_h(0)`, or the cards collapse to nothing.
- `fs/promises` reads only under home; another machine needs `exec`/`readTextFile` with `on`.

## Done when

- `herdr-app check` passes.
- A ledger made with `kanban` shows its columns and cards; `kanban done` on an issue moves it within seconds, and frees what waited on it.
- Breaking `config.yaml` shows the red band over the last contents; fixing it clears the band.
- Flow shows issues cut from one design document under it, in their stages; an issue whose PR waits for the owner's merge carries "Merge".
- An issue done more than a day ago is gone from Board and List, counted under the Done column, and found in History (newest first, and with `/` then `Enter`).
