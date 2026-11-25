# Andon for Stream Deck

Two actions (ADR-0045):

- **Lamp**: Andon's lantern on a key. It lights when you are called, says how much waits, and a press opens the next thing that waits in Herdr App.
- **Working**: how many of Andon's role terminals are working, from herdr-core. When the lantern is out, it tells a running line from an empty one. A press brings Herdr App to the front.

## The Lamp key

| Key | When |
|---|---|
| red, white text | a terminal's turn failed |
| amber, dark text | you are called: answer, merge or do your task |
| dark, the lantern unlit | nothing waits for you |
| dark, the lantern and `?` | can't tell: the App is closed, the inbox isn't on screen or is older than this plugin, or it hasn't put its value for 90 seconds |

A lit key has the total in large type and, under it, up to two kinds, the most pressing first: `failed`, `blocked`, `decide`, `merge`, `task` (hand work for you), `reply`. What is left for later doesn't light the key and isn't counted (ADR-0062 decision 7). Every Lamp key shows the same.

The plugin decides no light. The inbox mod (`../../inbox/`, "Lights" in its README) decides it.

## The Working key

| Key | When |
|---|---|
| dark, a number and `working` | that many role terminals are working |
| dark, `idle` dimmed | none is working: time to put in the next piece of work |
| dark, `?` alone | can't tell: `hc` isn't found, or `hc events` ended or sent nothing for 45 seconds |

A role terminal is one Andon started for a role: it carries the label `role` (Chief of Staff, Chief Engineer) or `issue` (a worker, a reviewer). It counts those that haven't finished and whose status is `working`, here and on connected machines. Never amber or red: those are the lantern's. Every Working key shows the same. It shows with the App closed too.

## Place it

Cell side: `STREAMDECK_PLUGINS=<Stream Deck's plugins folder> bin/bootstrap host-install` (Andon's docs/operate.md §4 has the folders, WSL, updating and taking it out). Or copy this folder there by hand, named `andon.lamp.sdPlugin`. Quit Stream Deck before you copy and start it afterwards, then put **Andon › Lamp** on a key, and **Andon › Working** on another if you want it.

It needs Stream Deck 7.1 or later (which brings Node.js 26), and Herdr App on the same machine, with the layout `andon` and an inbox mod at least as new as this plugin. The Working key needs herdr-core's `hc` on the same machine. There is nothing to install or build.

## What it reads

`shared.json` in the App's settings folder, every 2 seconds while a Lamp key is in view, and from it only `inbox.lamp` (the fields are in the inbox's README). It reads version 2 only; the value of an older inbox (version 1) shows `?` until you put the inbox and this plugin in place together. The folder is found as the App finds it:

1. `HERDR_APP_HOME`
2. `$XDG_CONFIG_HOME/herdr-app`
3. `~/.config/herdr-app`

A read that fails keeps the last value until it is 90 seconds old. It writes nothing.

For the Working key, while one is in view, it runs `hc events --json` (not detached: it ends with the plugin), counts from the first list of terminals, and corrects the count with each change after it. After `missed`, it counts again from the fresh list. When no Working key is left in view, it ends `hc events`. If that ends, or sends no line for 45 seconds (three heartbeats), the key shows `?` and it starts `hc events` again 30 seconds later. The program is `HC_BIN`, else `hc` on PATH.

## What it runs

| Call | When |
|---|---|
| `herdr-app do inbox.call` | a press: the inbox opens the next thing that waits |
| `herdr-app` (no arguments, detached) | after that ended 0 or 1: the running App comes to the front |
| `hc events --json` | while a Working key is in view (above) |
| `herdr-app status` | a press of Working; if it ends 0, the bare `herdr-app` above brings the App to the front, else the key alerts |

All run without a shell and without a window, in your home folder, with the environment Stream Deck gave the plugin. For a Lamp press: if the first doesn't end 0 or 1 within 5 seconds, or can't start, nothing else runs and the key shows Stream Deck's alert. An end of 1 (the inbox isn't on screen) raises the App and shows the alert. `herdr-app status` has the same 5 seconds. A press during another, of either key, is dropped. A press doesn't start a closed App: without a window, the second call isn't made.

The program is `HERDR_APP_BIN`, else `herdr-app` on PATH, else on Windows `%LOCALAPPDATA%\Programs\herdr-app\herdr-app.exe`. Programs started from the macOS Dock get a short PATH: if the key only alerts there, set `HERDR_APP_BIN` where Stream Deck can see it and start Stream Deck again.

The only connection is the WebSocket to Stream Deck on 127.0.0.1. No network, no other file, no other command. The Lamp key never runs `hc`: without it, only the Working key shows `?`.

## Files

`plugin.js` is what Stream Deck starts; `lamp.js` listens and reads, `key.js` judges the value's age and draws both keys, `app.js` runs Herdr App, `working.js` follows `hc events` and counts. `tests/host/streamdeck.mjs` in Andon's repository runs them.

## Origin

Written for Andon, without Elgato's SDK. The images are Andon's logo (`docs/assets/logo/` in Andon's repository). Under Andon's licence (Apache-2.0).
