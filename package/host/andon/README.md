# andon

Two jobs in one mod, one per block of the `andon` layout (`../layouts/andon.json`):

- **Naming** (the `andon-names` block, 1px, no props): names Andon's Spaces and tabs from the terminals' labels and where their panes are. It draws nothing. What it names, and the rules: `docs/operate.md` ("Herdr App の mod とレイアウト").
- **The top of the sidebar** (the `brand` block, props `{ "brand": true }`): Andon's lantern and name, then Herdr's search and sidebar buttons (ADR-0023). It names nothing.

## The lantern

The logo's "A" lantern (`docs/assets/logo/andon-mark.svg`), 20px, beside "Andon". Its window is the sidebar's light:

| Window | When |
|---|---|
| unlit (the theme's `dim`) | nothing needs you |
| amber `#F0A202` / `#FFD36B` | rows wait for your answer (Permission, Question, Auth, a role's Reply) |
| red `#D8453B` / `#F29A92` | a row's turn failed (Failed) |

The inbox decides which and keeps it in `shared("inbox.lantern")`; this block only reads it, so the two never disagree. A change fades over 0.9 s (`.transition("opacity")`), then the light stays still: no blinking, no timer, no sound. There is no green: the working spinner in the rows says what works.

`svg()` paints one color, so the logo is drawn in layers: `lantern-frame.svg` (the theme's `foreground`), `lantern-window.svg` unlit, then each light (`lantern-window.svg` and `lantern-glow.svg`) shown or faded out.

On the rail (the sidebar folded under 160px) only the button to unfold it stays, as before; the lantern isn't shown there. The inbox's rows still light.

Colors, sizes and the fade are first choices to look at and change (ADR-0023 decision 5).

## Place it

The `andon` layout already has both blocks. In your own layout, put the naming block in a column or row that is always shown, and, for the lantern, replace the `workspace-chrome` brand block with:

```json
{ "id": "brand", "type": "block", "size": { "px": 46 }, "window_drag": true, "content": { "mod": "andon", "props": { "brand": true } } }
```

Keep the `workspace-chrome` footer block (`"props": { "footer": true }`): this mod doesn't draw the footer. The lantern lights only with the inbox in the same layout.

## What it runs

| Call | When |
|---|---|
| `run("space.rename", {id, name})`, `run("tab.rename", {id, name})` | naming, only when a name differs |
| `run("mod.open", {name: "picker"})` | the search button |
| `run("layout.resize", {id: "sidebar", px})` and `setShared("sidebar.width" \| "sidebar.collapsed")` | the sidebar button (Herdr's own) |

Reads `terminals()`, `panes()`, `spaces()`, `props()`, `theme()`, `layout()`, `window()`, `shared("inbox.lantern")`. No `hc`, no `exec`, no files, no network.

## Origin

Made for Andon. `brand.js` is a changed copy of the brand in Herdr's shipped `workspace-chrome` mod (`defaults/mods/workspace-chrome/main.js` in [herdrdev/herdr-app](https://github.com/herdrdev/herdr-app) at commit `903dc810da0c839ba865d6b190f3aeeceb3a636b`): Andon's lantern and name in place of Herdr's logo and name; the footer left to workspace-chrome. The search and sidebar icons are still read from `mods/workspace-chrome/`. App updates to that brand aren't followed by themselves.

## Licence

`brand.js` is a changed copy of herdr-app code by the herdr-app authors, under the Apache License 2.0: see [LICENSE](LICENSE); the change is marked at its top. The rest, and the lantern pictures (the Andon logo), are Andon's, under Andon's licence (Apache-2.0).
