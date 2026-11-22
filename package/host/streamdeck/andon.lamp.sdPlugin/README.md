# Andon for Stream Deck

One action, **Lamp**: Andon's lantern on a key (ADR-0045). It lights when you are called, says how much waits, and a press opens the next thing that waits in Herdr App.

## The key

| Key | When |
|---|---|
| red, white text | a terminal's turn failed |
| amber, dark text | you are called: answer or merge |
| dark, the lantern unlit | nothing waits for you |
| dark, the lantern and `?` | can't tell: the App is closed, the inbox isn't on screen or is older than this plugin, or it hasn't put its value for 90 seconds |

A lit key has the total in large type and, under it, up to two kinds, the most pressing first: `failed`, `blocked`, `decide`, `merge`, `reply`. Every Lamp key shows the same.

The plugin decides no light. The inbox mod (`../../inbox/`, "Lights" in its README) decides it.

## Place it

Cell side: `STREAMDECK_PLUGINS=<Stream Deck's plugins folder> bin/bootstrap host-install` (Andon's docs/operate.md §4 has the folders, WSL, updating and taking it out). Or copy this folder there by hand, named `andon.lamp.sdPlugin`. Quit Stream Deck before you copy and start it afterwards, then put **Andon › Lamp** on a key.

It needs Stream Deck 7.1 or later (which brings Node.js 26), and Herdr App on the same machine, with the layout `andon` and an inbox mod at least as new as this plugin. There is nothing to install or build.

## What it reads

`shared.json` in the App's settings folder, every 2 seconds while a Lamp key is in view, and from it only `inbox.lamp` (the fields are in the inbox's README). The folder is found as the App finds it:

1. `HERDR_APP_HOME`
2. `$XDG_CONFIG_HOME/herdr-app`
3. `~/.config/herdr-app`

A read that fails keeps the last value until it is 90 seconds old. It writes nothing.

## What it runs

| Call | When |
|---|---|
| `herdr-app do inbox.call` | a press: the inbox opens the next thing that waits |
| `herdr-app` (no arguments, detached) | after that ended 0 or 1: the running App comes to the front |

Both run without a shell and without a window, in your home folder, with the environment Stream Deck gave the plugin. If the first doesn't end 0 or 1 within 5 seconds, or can't start, nothing else runs and the key shows Stream Deck's alert. An end of 1 (the inbox isn't on screen) raises the App and shows the alert. A press during another is dropped. The App is never started by a press: no window, no second call.

The program is `HERDR_APP_BIN`, else `herdr-app` on PATH, else on Windows `%LOCALAPPDATA%\Programs\herdr-app\herdr-app.exe`. Programs started from the macOS Dock get a short PATH: if the key only alerts there, set `HERDR_APP_BIN` where Stream Deck can see it and start Stream Deck again.

The only connection is the WebSocket to Stream Deck on 127.0.0.1. No network, no other file, no other command.

## Files

`plugin.js` is what Stream Deck starts; `lamp.js` listens and reads, `key.js` judges the value's age and draws, `app.js` runs Herdr App. `tests/host/streamdeck.mjs` in Andon's repository runs them.

## Origin

Written for Andon, without Elgato's SDK. The images are Andon's logo (`docs/assets/logo/` in Andon's repository). Under Andon's licence (Apache-2.0).
