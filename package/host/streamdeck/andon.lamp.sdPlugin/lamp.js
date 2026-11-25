// Andon's lantern on a Stream Deck key (ADR-0045): reads what the inbox mod decided from Herdr App's
// shared.json, draws it, and on a press has the inbox open what waits. It decides no light itself.
// A second key, Working, shows how many role terminals herdr-core says are working (decision 15).
// No dependency: Stream Deck's WebSocket messages are spoken directly.
import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";

import { appHome, openWaiting, raiseApp } from "./app.js";
import { draw, drawWorking, trusted } from "./key.js";
import { follower } from "./working.js";

const LIGHT = "andon.lamp.light";
const WORKING = "andon.lamp.working";
const LAMP = "inbox.lamp";
const READ_EVERY = 2000;

// The arguments Stream Deck starts a plugin with, as {name: value}.
function parseArgs(argv) {
  const out = {};
  for (let i = 0; i + 1 < argv.length; i += 2) out[argv[i]] = argv[i + 1];
  return out;
}

// Runs the plugin. The tests pass their own socket, clock, timers, file and spawn.
export function start({
  argv = process.argv.slice(2),
  env = process.env,
  platform = process.platform,
  home = os.homedir(),
  WebSocket = globalThis.WebSocket,
  spawn: run = spawn,
  // Synchronous on purpose: the file is open only for a moment while the App replaces it, and reads can't overlap.
  readFile = readFileSync,
  now = Date.now,
  timers = globalThis,
  exit = (code) => process.exit(code),
} = {}) {
  const args = parseArgs(argv);
  const port = args["-port"];
  const uuid = args["-pluginUUID"];
  const registerEvent = args["-registerEvent"];
  if (!/^\d{1,5}$/.test(port ?? "") || !uuid || !registerEvent) {
    console.error("andon.lamp: Stream Deck starts this, with -port, -pluginUUID, -registerEvent and -info");
    return exit(1);
  }

  const file = path.join(appHome(env, home), "shared.json");
  const keys = new Set(); // the contexts of the Lamp keys in view
  const workKeys = new Set(); // and of the Working keys
  let timer = null;
  let value; // inbox.lamp at the last read that worked
  let image = null;
  let pressing = false;
  let working = drawWorking(null);

  const socket = new WebSocket(`ws://127.0.0.1:${port}`);
  const send = (message) => socket.send(JSON.stringify(message));
  const setImage = (context) => send({ event: "setImage", context, payload: { image } });

  // Reads, judges by the clock, and sends every key the image if it changed. Returns whether it did.
  function refresh() {
    try {
      value = JSON.parse(readFile(file, "utf8"))?.[LAMP];
    } catch {
      // Being replaced, or gone: the last value stands until it is too old to trust.
    }
    const next = draw(trusted(value, now()));
    if (next === image) return false;
    image = next;
    keys.forEach(setImage);
    return true;
  }

  function appear(context) {
    keys.add(context);
    timer ??= timers.setInterval(refresh, READ_EVERY);
    // A key that comes into view always gets the image, changed or not (also after the PC slept).
    if (!refresh()) setImage(context);
  }

  function disappear(context) {
    keys.delete(context);
    if (keys.size || timer === null) return;
    timers.clearInterval(timer);
    timer = null;
  }

  // Only while a Working key is in view does `hc events` run.
  const follow = follower({
    env, home, spawn: run, timers, now,
    show(n) {
      const next = drawWorking(n);
      if (next === working) return;
      working = next;
      workKeys.forEach((context) => send({ event: "setImage", context, payload: { image: working } }));
    },
  });

  function appearWorking(context) {
    workKeys.add(context);
    send({ event: "setImage", context, payload: { image: working } });
    follow.start();
  }

  function disappearWorking(context) {
    workKeys.delete(context);
    if (workKeys.size) return;
    follow.stop();
    working = drawWorking(null);
  }

  // One press at a time, of either key.
  async function press(context, open) {
    if (pressing) return;
    pressing = true;
    const ok = await open({ env, platform, home, spawn: run, timers });
    pressing = false;
    if (!ok) send({ event: "showAlert", context });
  }

  socket.addEventListener("open", () => send({ event: registerEvent, uuid }));
  socket.addEventListener("message", ({ data }) => {
    let message;
    try {
      message = JSON.parse(data);
    } catch {
      return;
    }
    if (typeof message?.context !== "string") return;
    if (message.action === LIGHT) {
      if (message.event === "willAppear") appear(message.context);
      else if (message.event === "willDisappear") disappear(message.context);
      else if (message.event === "keyDown") press(message.context, openWaiting);
    } else if (message.action === WORKING) {
      if (message.event === "willAppear") appearWorking(message.context);
      else if (message.event === "willDisappear") disappearWorking(message.context);
      else if (message.event === "keyDown") press(message.context, raiseApp);
    }
  });
  // No reconnecting (ADR-0045 decision 10): Stream Deck starts the plugin again.
  socket.addEventListener("error", () => exit(1));
  socket.addEventListener("close", () => exit(0));
}
