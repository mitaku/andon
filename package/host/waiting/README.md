# waiting

What waits on you in the project you are in, as a screen of the right panel beside Files and Changes: pull requests waiting for your merge, decisions a question waits on, and owner tasks ([ADR-0042](../../../docs/adr/0042-owner-waits-do-not-stop-the-line.md) decision 12). You can answer a decision's question here by picking one of its options (decisions 17 and 18); everything else it only shows.

The list is not counted here. The mod runs `andon waiting --json` in the cell, in the focused pane's folder, and draws its output in its order ([contract.md](../../../docs/contract.md) §25). The command holds the rule of what waits on you and how the parallel slots count, so the Chief of Staff and this pane never disagree (decision 14).

## What it shows

- **Slots** on the first line: `Slots 3/4 working · 2 waiting on you` (`working`, `limit` and `waiting` of the output's `slots`), with `· full` in `blocked` when the waits reached the limit (`waiting_full`: no new issue starts).
- **One row per wait**, in the output's order (most issues waiting behind it first, then the oldest): the kind (`Merge`, `Decide`, `Do`), the issue's id and title, how long it has waited (`since`), and how many issues wait behind it (`holds`).
- **Later**: the waits the output leaves for later (`call` false: the Chief of Staff marked them, and nothing behind them is in a hurry) come after the rest, under a `Later` heading with their number ([ADR-0062](../../../docs/adr/0062-later-owner-waits.md) decision 8). They don't light the lantern, but they are rows like the others: open one to read it, and answer a decision the same way. Opened, it also says why it was left (`later`) and, when there is one, the day it calls again (`later_until`). With only such rows, `Nothing calls you now` comes before the heading. An output without `call` (an andon older than the field) has no heading: every row is drawn as before. To leave a wait for later or call it now, tell the Chief of Staff; the pane has no button for it (decision 17).
- **Click a row to open it:** the question, its facts, why it is asked and its options (the recommended one marked), or the reason a merge waits; on a task, the steps from its brief (the brief's text, line by line) and the brief's links, each opening in your browser; the question's number; the issues waiting behind it by id. A merge row has **Open PR** (your browser). Every row has **Go to Chief of Staff** while the project's Chief of Staff (`role=chief-of-staff`, `project=<slug>`) runs on that machine.
- **Answer a decision:** on a decision row the output marks `answerable` (it has options and isn't irreversible), click an option to pick it, then press **Answer "<option>"**. Picking alone writes nothing. The button runs `andon waiting answer <id>#<n> --option <n>` in the folder and machine the row was read from; the command checks the question again and writes the option's `label` as the answer. The row then shows `Answered: <option>` and leaves the list at the next read. If the command refuses (say the question was answered in the conversation meanwhile), the row shows its reason in one line in `blocked` and keeps your pick. A decision without `answerable: true` (an irreversible one, one without options, or an andon older than the field) shows `Answer it to the Chief of Staff.` and no button; so does any answer that isn't one of the options.
- **Nothing waiting:** the slots line and `Nothing is waiting on you`.
- **When it can't tell:** one line in `blocked` with the reason, and nothing else: the command failed (its first line of error, such as no ledger in this folder), its output isn't JSON, its `version` isn't 1, or the cell's `andon` is missing or older than `andon waiting` (update the cell's Andon).

It is not called "Needs you": the inbox's Needs you is terminal states, and a question on the ledger is not a state. Merges stay on GitHub: this pane opens the pull request and never merges.

It runs the command again every 10 s, and when the focused folder or machine changes (one run took about 0.1 s on a ledger of 500 issues).

## Place it

The `andon` layout (`../layouts/andon.json`) has it as the third block of the right panel's stack, with the id `waiting`:

```json
{ "id": "waiting", "type": "block", "content": { "mod": "waiting" } }
```

The inbox's header has a button (`waiting.svg`) that shows the panel and selects this block (`layout.show` `panel`, then `layout.select` `panel-body` / `waiting`). The tab bar's Files and Changes buttons open their own blocks as before.

To close it, press the close button at the right of its header, or Esc while the pane has the keyboard. That hides the whole right panel (`layout.hide` `panel`), Files and Changes with it, and the shown tab takes the width. What you opened or picked is kept while you stay in the same folder: show it again from the inbox and it is as you left it. Moving to another folder meanwhile starts over there, as it always does. The button and Esc are there only when the block sits in the layout's `panel`.

## What it runs

| Call | When |
|---|---|
| `exec("andon", ["waiting", "--json"], {cwd, on})` | every 10 s and when the focused folder or machine changes; `cwd` is the focused pane's folder, `on` its machine |
| `exec("andon", ["waiting", "answer", question, "--option", n], {cwd, on})` | **Answer** on an answerable decision row, once per press; `question` is the item's `question` from the output (`<id>#<n>`), `n` the picked option's number from 1, `cwd` and `on` where the row was read |
| `exec("xdg-open" \| "open" \| "explorer.exe", [url])` | **Open PR** on a merge row, and a link under a task row's steps (only `http(s)` links) |
| `run("agent.focus", {terminal})` | **Go to Chief of Staff** |
| `run("pane.go", {direction: "left"})`, then `run("layout.hide", {id: "panel"})` | the close button, or Esc while the pane has the keyboard (`pane.go` only then) |

The arguments are fixed: no path, slug or text from the screen goes into a command. The only write is `andon waiting answer`, and the command, not the pane, decides what may be answered. No `kanban`, no file reads, no `fetch`, no notifications.

## Icon

The inbox's `waiting.svg` is [Lucide](https://lucide.dev)'s `hourglass` (lucide 1.77.0), ISC (`../inbox/LICENSE-Lucide.txt`), with `stroke-width` 1.5 to match the other icons.

## Origin

Written for Andon (ADR-0042). The Later heading (ADR-0062). Under Andon's licence (Apache-2.0).
