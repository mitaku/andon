# The `kanban/` ledger format

Format version **2** (2026-11-02). This is the contract between the ledger's CLI (`kanban`, in [`cmd/kanban/`](../cmd/kanban/); `deck` is another name for it for one version) and anything else that reads or writes a ledger — Andon's commands, skills and gate, and boards that read the files directly. Since 2026-11-05 this file is the format's reference; it was copied from herdr-flightdeck's `docs/ledger-format.md` at tag `deck-v0.4.0` ([decisions.md](decisions.md) §7, §12). Where it says *yard*, Andon says *plant* (建屋). Andon's reserved keys in `task.yaml` are in [contract.md](contract.md) §2.

JSON Schemas for editors and checks: [config.v2.schema.json](../schema/ledger/config.v2.schema.json) (`config.yaml`), [task.v2.schema.json](../schema/ledger/task.v2.schema.json) (`task.yaml`), [questions.v2.schema.json](../schema/ledger/questions.v2.schema.json) (`questions.yaml`) and [routine.v2.schema.json](../schema/ledger/routine.v2.schema.json) (`routines/<name>.yaml`). They are derived from this file; where they differ, this file is right.

**A ledger belongs to one project.** On a yard, a project is `<yard-root>/projects/<slug>/`: a trust boundary with one Chief of Staff, and one or more repositories cloned with ghq. Its ledger is `projects/<slug>/kanban/`, kept out of git. Version 1 put one ledger at the yard's root; see [Changes from version 1](#changes-from-version-1).

## Compatibility

- **Readers** must ignore keys they don't know, and must keep working when one task can't be read (show it as unreadable, show the rest).
- **Writers** must keep keys they don't know, and should keep comments and key order (the Go `kanban` does).
- Adding an optional key or a file is not a format change. Removing or renaming a key, changing a key's meaning or type, or changing the layout is: it raises `version` in `config.yaml` and is announced in this file.
- A reader or writer that finds a `version` it doesn't know (or none, which means 1) must say so and stop, rather than guess.

### Added in version 2 (no version change)

| Date | Added | Where |
|---|---|---|
| 2026-11-02 | `routines/<name>.yaml`, `state/routines.json`, `config.yaml` `timezone`, `task.yaml` `routine` | [Routines](#routines) |
| 2026-11-03 | `task.yaml` `who` (`person`: a person's task) | [`task.yaml`](#tasksidtaskyaml) |
| 2026-11-03 | `coordinator.md`, a reserved file name (moved out of the ledger on 2026-11-06, below) | — |
| 2026-11-03 | `log.jsonl` lines `{"config": …}`, written by `kanban config set` | [`log.jsonl`](#logjsonl) |
| 2026-11-03 | `tasks/<id>/questions.yaml`, the ledger's `library/`, `log.jsonl` lines `{"question": …}` | [Questions](#questions) |

### Moved without a version change (2026-11-06)

The folder, one key and the ids changed at once, for the one ledger there was; `kanban migrate` moves a ledger, and nothing reads the old names ([Moving from `.flightdeck/`](#moving-from-flightdeck)).

| Was | Now |
|---|---|
| `projects/<slug>/.flightdeck/`, `$FLIGHTDECK_ROOT` | `projects/<slug>/kanban/`, `$KANBAN_ROOT`; a folder is a ledger only when `kanban/config.yaml` has a `version` (see [Finding the root](#finding-the-root)) |
| `config.yaml` `coordinator` | `config.yaml` `chief_of_staff` |
| `coordinator.md` in the ledger | the Chief of Staff section of `projects/<slug>/AGENTS.md`, outside the ledger ([ADR-0004](adr/0004-project-layout.md)) |
| ids `<prefix>-<n>` (`A-5`) | ids `<prefix>-<n>` with `n` of four digits (`A-0005`); `A-5` is read as `A-0005` (see [Writing](#writing)) |

## Layout

```
<yard-root>/
  projects/<slug>/               the project (its folder name is the slug)
    kanban/
      config.yaml                the ledger's settings
      tasks/<id>/
        task.yaml                the task's state            (required for a task)
        brief.md                 what to do                  (optional)
        report.md                what was done, what's next  (optional)
        questions.yaml           what it asks the person     (optional; see Questions)
        library/                 what it produced            (optional)
      routines/<name>.yaml       a kind of work run again and again (optional; see Routines)
      state/routines.json        when each routine last fired, or was first seen (written by kanban)
      library/                   the Chief of Staff's documents for the person (optional)
      log.jsonl                  who changed what, when      (written by kanban)
      .lock                      writers' lock               (an empty file; see Writing)
      .gitignore                 `*`: the ledger stays out of git
  .ghq/<host>/<owner>/<repo>/    the project's repositories (ghq's layout)
```

The ledger's **root** is the folder that holds `kanban/`: the project's folder. Outside a yard, any folder can be a root.

The ledger is not kept in git. The yard keeps the storage across container rebuilds (that is the yard's job, not the ledger's).

### Finding the root

From a working folder, in order:

1. `$KANBAN_ROOT`, if set (it must hold `kanban/config.yaml`). The Chief of Staff sets it for each agent it starts.
2. The nearest folder at or above the working folder that holds `kanban/config.yaml` with a whole-number `version`. `kanban/` is a plain name, so a `kanban/` folder without one (in a product's repository, say) is not a ledger and the walk goes on.
3. **From the repository.** Take the main checkout: the parent of `git rev-parse --path-format=absolute --git-common-dir`, so a worktree anywhere (`~/.herdr/worktrees/…`) leads to its repository. Walk up from the main checkout. At each folder, look at every `projects/*/kanban/config.yaml` (with a `version`, as in 2) and keep the ledgers whose `repos` match the main checkout (see `repos` below). Stop at the first folder where any match.
   - One match: that ledger.
   - More than one: an error that names them; set `$KANBAN_ROOT`.
   - None, up to the top: there is no ledger.

## `config.yaml`

```yaml
version: 2                      # required
title: sample site             # optional; the slug when absent
prefix: FD                      # task ids are <prefix>-<n> (FD-0001)
columns: [todo, queued, doing, review, done]
phases:                         # optional, in order
  - { id: P1, title: Attention queue }
repos:                          # the project's repositories, ghq style
  - github.com/<owner>/sample-site
chief_of_staff: t2              # optional; the Chief of Staff's terminal
```

| Key | Type | Default | Meaning |
|---|---|---|---|
| `version` | integer | — | Format version. **Required**: `2`. |
| `title` | string | the slug | A name to show. |
| `prefix` | string | `W` | Prefix of new task ids. Ids are unique within a ledger only. |
| `columns` | list of strings | `[todo, queued, doing, review, done]` | The statuses, in board order. **The last one means done.** |
| `phases` | list of `{id, title}` | `[]` | Ordered stages, for timeline views. |
| `repos` | list of strings | `[]` | The project's repositories as `<host>/<owner>/<repo>`, the path ghq gives them. A checkout matches an entry when the path of its main checkout ends with `/<host>/<owner>/<repo>` (compared with `/` separators; without regard to case on Windows). |
| `chief_of_staff` | string | — | The terminal of the project's Chief of Staff: its herdr-core address on the ledger's own machine (`t2`). The address changes each time the Chief of Staff is started again; it sets it with `kanban config set chief_of_staff=<address>`, which writes under the lock and keeps comments and key order. |

## `tasks/<id>/task.yaml`

**The folder's name is the task's id.** An `id` key inside, if present, must match it.

| Key | Type | Meaning |
|---|---|---|
| `id` | string | Same as the folder name. |
| `title` | string | **Required** to be a useful task; a task without one is listed with no title. |
| `status` | string | One of `columns`. A value not in `columns` is shown in the first column. |
| `repo` | string | One of `repos`: the repository the task works in. |
| `parent` | string | Id of the task this is part of (an epic). |
| `phase` | string | Id from `phases`. |
| `after` | list of strings | Ids in this ledger that must be done first. |
| `profile` | string | The yard's profile for the agent (`opus`, `codex`, `reviewer`, …). The yard defines profiles; the ledger only keeps the name. |
| `who` | string | `person`: a person's task (a decision to make, or work only a person can do). Absent: an agent's task. Views may show a person's tasks apart. |
| `agent` | string | The terminal working on it: its herdr-core address **on the ledger's own machine** (`t3`, not `vps/t3`), the way herdr-core records a `parent`. A tab or terminal name is also accepted by readers. A machine-qualified address (`<machine>/t7`, as herdr-core prints it for a terminal on another machine) may also appear; readers (work-board) already read it that way. |
| `branch` | string | The task's git branch. |
| `worktree` | string | The task's worktree folder (`~` allowed). |
| `links` | list of strings | `http(s)` URLs, or paths relative to the root. |
| `routine` | string | The name of the routine that added it (see Routines). |
| `created`, `updated` | string | UTC time, `YYYY-MM-DDTHH:MM:SSZ`. Written as strings (quoted in YAML when needed). |

Any other key is allowed and kept.

**Keys other tools use.** These names are taken by other tools that read and write `task.yaml` (for example Andon, and the gate that checks a pull request); `kanban` keeps them like any other key and gives them no meaning. Don't use them for anything else.

| Key | Used by |
|---|---|
| `review`, `reviewer`, `review_rounds` | Whether a task gets an independent review, by which profile, and how many rounds. |
| `must_fix` | The lightest severity of finding the agent must fix. |
| `merge` | Who merges. |
| `machine` | The machine the task's agent runs on. |
| `pr` | The URL of the task's pull request; the one place the gate reads it from. |

### Status rules

- **Done** is the last column (`done` by default).
- **Waiting:** a task waits on each id in `after` that is missing, unreadable, or not done.
- **Ready:** status is `todo` or `queued` and it waits on nothing. (`kanban ready` lists these.)
- `queued` means "added with something to wait for". When the last thing it waits for is done, `kanban done` moves it to the first column.
- **Needs you is not a status.** It comes from the agent's live state in herdr-core (blocked), never from the ledger. A task's open questions (see Questions) are in the ledger; they are not a status either.
- Who may change what (for example, that only the Chief of Staff moves a task to `done`) is not part of the format. The yard's skills decide it.

## `brief.md`, `report.md`, `library/`

- `brief.md`: the task as given — what to do, limits, how to check. Markdown.
- `report.md`: what was done and what's next, written by whoever works on it (`kanban report`). Markdown; a `## Next` section is the convention for the next step.
- `library/`: files the task produced (images, diagrams, documents). Viewers list and open them; nothing else is assumed.

## Routines

A **routine** is a kind of work defined once and run again and again: look at untriaged tasks every morning, check the architecture every night, review what reviews keep finding. Each time its trigger fires, `kanban` adds one task to the ledger from it. The task is an ordinary task: the Chief of Staff starts an agent for it, and the board shows it. Adding routines is a file, not a format change: format 2 readers that don't know them ignore them.

People write routines (or the Chief of Staff, on a person's word). `kanban` reads them; nothing else acts on them.

### `routines/<name>.yaml`

The file's name is the routine's name (one plain name, as for ids).

```yaml
title: Look at untriaged tasks     # the added task's title
brief: |                           # the added task's brief.md
  List the tasks in todo with no brief. For each, ...
profile: sonnet                    # optional: the added task's profile
repo: github.com/x/app             # optional: one of repos
schedule: "0 9 * * 1-5"            # a time trigger: cron's five fields
on: [done]                         # an event trigger: statuses a task moves into
open: 1                            # at most this many of its tasks not done; default 1
paused: false                      # true: never fires
```

| Key | Type | Meaning |
|---|---|---|
| `title` | string | **Required.** The title of each task it adds. |
| `brief` | string | The `brief.md` of each task it adds. |
| `profile`, `repo` | string | Copied into each task it adds. `repo` must be one of `repos`. |
| `schedule` | string | Fires at these times: cron's five fields (minute, hour, day of month, month, day of week), in the time zone of `config.yaml` `timezone`, else UTC. |
| `on` | list of strings | Fires when any task moves into one of these statuses, except a task this routine added. |
| `open` | integer | Fires only while fewer than this many of the tasks it added are not done. Default 1. This is what keeps a slow routine from piling up tasks, and a routine from feeding itself. |
| `paused` | boolean | `true`: never fires. |

A routine needs `schedule`, `on`, or both. Any other key is allowed and kept.

### Firing

- **Time:** `kanban tick` fires each routine whose `schedule` had a time after its last firing and up to now. Times missed while nothing ran `kanban tick` fire once, not once each. The yard runs `kanban tick` every 5 minutes (a cron entry: the yard's job).
- **The first time** `kanban` sees a routine with a command that writes the state (`kanban tick`, `kanban routine list`), it records that time in `state/routines.json` as if the routine had fired then, and doesn't fire it. The schedule counts from that time. The routine file's creation and change times are not used.
- **Events:** a `kanban` command that moves a task into a status (`start`, `review`, `done`, `set status=`) fires, in the same lock, each routine whose `on` lists that status.
- **Each firing**, under the lock: skip it if `paused`, or if `open` of its tasks are not done (log the skip). Else add a task with `title`, `brief`, `profile`, `repo`, `routine: <name>` and status `todo` (the first column), as `kanban add` would, and log it. A skipped firing is still a firing: its time is recorded, so a schedule time that was skipped doesn't fire later.
- `kanban routine run <name>` fires a routine now, by the same rules.
- `state/routines.json` keeps `{ "<name>": "<UTC time of the last firing, or of when kanban first saw it>" }`, written like any file (atomically, under the lock).

### Config

| Key | Type | Default | Meaning |
|---|---|---|---|
| `timezone` | string | `UTC` | An IANA time zone (`Asia/Tokyo`) for `schedule`. |

## Questions

A task agent that needs the person to decide something writes the question into the ledger and stops, instead of asking in its terminal. The Chief of Staff gathers the open questions of every task into one document for the person, writes the answers back, and passes each answer to its agent. The person sees one Needs you, the Chief of Staff's, not one per agent.

The ledger, not the Chief of Staff's memory, is where a question lives: a question is closed only when its agent has received the answer, and anything that drops one (a Chief of Staff's context compacted or restarted, a question missed while gathering, a half-answered document) leaves it open, to be found by the next `kanban questions`.

Adding questions is a file, not a format change: format 2 readers that don't know them ignore them.

### `tasks/<id>/questions.yaml`

A list, oldest first. Each question is named `<id>#<n>` (`FD-0012#2`).

```yaml
- n: 1
  text: |                         # the question, for the person
    Ship the export as CSV only, or CSV and Excel?
  kind: business                  # optional: which kind of decision point
  asked: "2026-11-03T01:20:00Z"
  askedBy: terminal:t3
  status: received                # open → answered → received
  answer: |
    CSV only for now.
  answered: "2026-11-03T02:05:00Z"
  answeredBy: terminal:t2
  received: "2026-11-03T02:06:00Z"
```

| Key | Type | Meaning |
|---|---|---|
| `n` | integer | **Required.** 1, 2, … within the task; never reused. |
| `text` | string | **Required.** The question, written for the person: what, why it matters, the options, and the agent's pick. |
| `kind` | string | Optional. `business`, `users`, `promise` or `irreversible`: the four kinds of decision point (the decision points of [design.md](design.md) §1.3); or `line`: a change to the line itself (the gates, roles and rules the work runs on, not the product). |
| `status` | string | `open` (asked), `answered` (the answer is in the ledger), `received` (the agent has it). |
| `asked`, `answered`, `received` | string | UTC times, as in `task.yaml`. |
| `askedBy`, `answeredBy` | string | As `by` in `log.jsonl`. |
| `answer` | string | The answer, as the person gave it. |

Any other key is allowed and kept.

### The questions' lifecycle

- `kanban ask <id> "<text>"` (task agent): adds an `open` question. The agent then ends its turn and waits.
- `kanban questions` (Chief of Staff): lists the `open` and `answered` questions of the ledger, oldest first; `--open` only the open ones, `--json` for programs. This is where the Chief of Staff starts every time, not from what it remembers.
- `kanban answer <id>#<n> "<answer>"` (Chief of Staff): writes the answer; `answered`. The Chief of Staff then passes it to the task's agent.
- `kanban received <id>#<n>` (task agent): `received`. The question is closed.
- `kanban done` refuses a task with an `open` or `answered` question (it says which). `--force` closes it anyway and logs that.

### What the Chief of Staff does with them

How the Chief of Staff gathers and asks is the yard's skill, not the format. The format makes these checks possible:

- **Every question asked is in the document.** The Chief of Staff's document for the person (a page in the ledger's `library/`) lists the `<id>#<n>` of every question it holds, and the time `kanban questions` was read. A question that is merged with another lists all of their names; its answer is written to each.
- **Nothing is left behind.** After the person answers, `kanban questions --open` lists what is still open; it goes into the next document.
- **Every answer arrives.** A question `answered` but not `received` is one the agent never got.
- **Every stopped agent has asked.** A task whose agent is stopped (Needs you, or idle with the task not in review or done) but has no `open` question asked outside the ledger. Viewers that see both herdr-core and the ledger (work-board, inbox) show it.

## `log.jsonl`

One JSON object per line, appended:

```json
{"at":"2026-11-02T10:03:00Z","by":"terminal:t3","id":"FD-0012","change":{"status":"doing","agent":"t3"}}
```

`by` is `terminal:<$HC_TERMINAL_ID>` when run in a herdr-core terminal, else the user name. `change` holds the keys that changed (`null` for a removed key), or `{"add": "<title>"}` for a new task, or `{"report": "write" | "append"}`. A routine's firing logs `{"routine": "<name>", "fired": "<id added>"}` or `{"routine": "<name>", "skipped": "open" | "paused"}` (with no `id`), and `by` is `routine:<name>`. Questions log `{"question": <n>, "status": "open" | "answered" | "received"}` with the task's `id` (the text and answer are in `questions.yaml`, not in the log); `kanban done --force` past one logs `{"status": "<last column>", "forced": [<n>, …]}`. A task moved out of `queued` by `kanban done` logs `{"status": "<first column>", "freedBy": "<id>"}`. `kanban config set` logs `{"config": {"chief_of_staff": "<address>" | null}}` (with no `id`; lines from before 2026-11-06 say `coordinator`). `kanban migrate` logs `{"migrate": {"from": ".flightdeck", "ids": {…}}}` (with no `id`).

## Writing

- **Take the lock** for any write: an OS lock on `kanban/.lock` (`flock` on Linux and macOS, `LockFileEx` on Windows), held while you read the task again, change it and log the change. The file stays (empty). Readers don't lock.
- **Write atomically:** to a temporary file beside the target (`<file>.tmp-…`), then rename it over the target.
- **Ids:** under the lock, the next id is `<prefix>-<n+1>` with the number zero-padded to four digits (`W-0001`, `W-0012`), where `n` is the highest number among the existing task folders with that prefix. Padded ids sort by name in number order (`ls tasks/`, boards); past `W-9999` the number only grows (`W-10000`) and that order breaks. An id and the `prefix` are each one plain name: no `/`, `\`, `:`, and not `.` or `..`.
- **Reading ids:** an id typed or found as `<prefix>-<digits>` names the task with that number, however many digits: `W-5`, `W-05` and `W-0005` are the same task. `kanban` takes them so in its arguments (`<id>`, `<id>#<n>`, `--parent`, `--after`, `parent=`, `after=`) and in `log.jsonl`, whose lines written before the move to four digits keep `W-5`. It always shows and writes `W-0005`.
- **Stay inside:** don't follow symbolic links inside `kanban/`; `kanban` refuses a task folder, `task.yaml`, `report.md` or the lock that is a link, so nothing is written outside the ledger.
- Prefer `kanban` to writing files by hand; it does all of the above. Editing a `task.yaml` in an editor is fine when no `kanban` is writing the same task.

## `.gitignore`

`kanban init` writes `kanban/.gitignore` with one line, `*`, so the ledger stays out of whatever repository holds the project's folder.

## Moving from `.flightdeck/`

`kanban migrate [--root dir]` (root as for `kanban init`) moves a ledger from `<root>/.flightdeck/` to `<root>/kanban/`, once. It stops, and changes nothing, when `kanban/` is already there, when `config.yaml` isn't version 2, when a `task.yaml` can't be read or a task folder is a symbolic link, when the dispatcher state can't be read as the dispatcher's state, when the `project.json` it would draft (below) wouldn't pass Andon's reader, or when a task is `doing` or `review` (a worktree, branch, terminal and Space carry its id; finish those first). Otherwise it:

1. renames, in each `task.yaml`, `id`, `parent` and `after` to four-digit ids, then each `tasks/<id>/` folder (with its `library/`);
2. renames `config.yaml` `coordinator` to `chief_of_staff` (value, comments and place kept);
3. moves the folder to `kanban/` (`.gitignore` stays `*`);
4. appends `coordinator.md`, if there is one, to `<root>/AGENTS.md` under `## Chief of Staff` and removes it;
5. renames the moved ids in Andon's dispatcher state for the project (`$XDG_STATE_HOME/andon/dispatcher/<slug>.json`, else `~/.local/state/…`): keys and strings that are an id, or an id followed by `#`, `@` or `|`;
6. when `<root>/project.json` is missing, writes a draft of it for the person to check and commit: `version` 1, `project` (`config.yaml` `title`, else the folder's name, and `prefix`), `repos` (`config.yaml` `repos`) and `delegation` (`<root>/delegation.json` as it is, which stays). The shape is Andon's [project.schema.json](../schema/project.schema.json); kanban commits nothing;
7. logs `{"at", "by", "migrate": {"from": ".flightdeck", "ids": {old: new}}}`.

`log.jsonl`, `brief.md`, `report.md` and the questions keep the ids they were written with (the readers take `A-5` as `A-0005`). Routines name no task ids. Branch and worktree names are not changed.

## Reading a whole yard

A view of every project (the yard project's) reads each `<yard-root>/projects/*/kanban/` as its own ledger. Ids are only unique within a ledger, so such a view names a task as `<slug>/<id>`. `after` and `parent` don't reach across ledgers.

## Changes from version 1

| Version 1 | Version 2 |
|---|---|
| One ledger at the yard's root, for every project | One ledger per project, at `projects/<slug>/kanban/` |
| `config.yaml` `projects`: name → folder | Removed. The ledger is the project; `repos` lists its repositories |
| `task.yaml` `project` | Removed. `repo` names the repository |
| — | `config.yaml` `title`, `coordinator` (`chief_of_staff` since 2026-11-06); `task.yaml` `profile` |
| `version` optional | `version: 2` required |
| `.gitignore`: `.lock`, `*.tmp-*` | `.gitignore`: `*` (out of git) |
| Root: env, up, up from the main checkout | Root: env, up, then the project whose `repos` match the main checkout |

There is no migration: a version 1 ledger is refused with a message. Move its tasks by hand, one project at a time.
