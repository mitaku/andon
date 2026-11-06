# context-header

A pane header that says which work you're looking at and what it waits for, so you don't have to read back through the terminal.

![context-header](preview.png)

- **Line 1:** the program's icon, the name (a tab you named, else the agent's own title, else the program), sub-agents if any, `settled` if you settled it, how long it has been in its state, and the status mark (Permission / Question / Auth while blocked, a spinner while working, a dot when done, a ring when idle).
- **Line 2:** machine · space · tab (when named) · branch (other than main/master) · folder. While the agent is blocked and says why, its message instead.
- Hover: `✓` settle (an agent not settled yet; the same `settled` label inbox writes, tooltip `ctrl-shift-d`, but it doesn't open the next one), then split right, split down, zoom and close, as Herdr's own header has. Click: focus the pane.

For a plain shell it shows the terminal's title and folder, like Herdr's header.

## Place it

The `flightdeck` layout uses it for every pane. In your own layout, set the tab block's pane header to it, 46px high:

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
