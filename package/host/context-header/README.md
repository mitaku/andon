# context-header

A pane header that says which work you're looking at and what it waits for, so you don't have to read back through the terminal.

- **Line 1:** the program's icon, the name (for an Andon issue's agent or reviewer, `<id> <title>` from its labels, `CE <id> <title>` for a Chief Engineer; else a tab you named, else the agent's own title, else the program), sub-agents if any, `settled` if you settled it, how long it has been in its state, and the status mark (Permission / Question / Auth while blocked, a spinner while working, a dot when done, a ring when idle).
- **Line 2:** machine · space · tab (when named, and not for an Andon issue, whose tab is its id) · branch (other than main/master) · folder. While the agent is blocked and says why, its message instead.
- Hover: `✓` settle (an agent not settled yet; the same `settled` label inbox writes, tooltip `ctrl-shift-d`, but it doesn't open the next one), then split right, split down, zoom and close, as Herdr's own header has. Click: focus the pane.

For a plain shell it shows the terminal's title and folder, like Herdr's header.

## Place it

The `andon` layout (`../layouts/andon.json`) uses it for every pane. In your own layout, set the tab block's pane header to it, 46px high:

```json
"header": { "content": { "mod": "context-header" }, "height": 46, "position": "top", "reveal": "always" }
```

## What it runs

| Call | When |
|---|---|
| `run("pane.focus", {pane})` | clicking the header |
| `run("pane.split", {direction, pane})`, `run("pane.zoom", {pane})`, `run("pane.close", {pane})` | the hover buttons |
| `hc(["label", <address>, "settled=<ms>:<turn>"])` | the hover `✓` (settle) |
| `notify({level: "error"})` | only when that settle fails |

It reads `panes()`, `terminals()` and `spaces()`. No `exec`, no file reads, no network. The only thing it writes is the `settled` label, when you press `✓`. Icons are Herdr's (`mods/pane/`, `mods/workspace-chrome/`), referenced, not copied.

## Origin

Copied from herdr-flightdeck (a private repository that Andon came out of) `herdr-app/mods/context-header` at commit `afc57ac5ec85d3123cbf5c067a9cd4ee3cf5b268`. Changed for Andon: an Andon issue's terminal is named from its `project`, `role`, `issue` and `title` labels (the names the andon mod gives Spaces and tabs, ADR-0002 and ADR-0003), and line 2 doesn't repeat the name. The preview picture was not copied. Under Andon's licence (Apache-2.0).
