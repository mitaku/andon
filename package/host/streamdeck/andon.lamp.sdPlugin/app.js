// Herdr App, from outside (ADR-0045 decisions 8, 13 and 15): where it keeps shared.json, and what a press runs.
import path from "node:path";

const CALL = ["do", "inbox.call"];
const STATUS = ["status"];
// How long `herdr-app do` or `herdr-app status` may take.
const GIVES_UP = 5000;
// herdr-app do's exit codes when the App is running: 0 the inbox got the command, 1 the App answered an error.
// 2 is no window. Only 0 and 1 may be followed by a bare herdr-app: without a window, that one becomes the App.
const DELIVERED = 0;
const REFUSED = 1;
const NOT_FOUND = "not found";

// The App's own order (herdr-app src/mods.rs app_dir); an empty variable counts as unset.
export function appHome(env, home) {
  return env.HERDR_APP_HOME || path.join(env.XDG_CONFIG_HOME || path.join(home, ".config"), "herdr-app");
}

// Runs `herdr-app <first>`, and a bare detached `herdr-app` to bring the App to the front when `raises(code)`.
// Returns whether it all went well: `fine(code)` and, if it raised, the raise started.
async function press({ env, platform, home, spawn, timers }, first, raises, fine) {
  // One working folder for both runs, so one name finds one program. No shell, no window.
  const options = { env, cwd: home, stdio: "ignore", shell: false, windowsHide: true };

  // How `herdr-app <first>` ended: its exit code, NOT_FOUND, or null (couldn't run, killed, too slow).
  const run = (bin) => new Promise((resolve) => {
    let child;
    try {
      child = spawn(bin, first, options);
    } catch {
      return resolve(null); // spawn throws for some failures, as a .cmd without a shell
    }
    const giveUp = timers.setTimeout(() => {
      child.kill();
      resolve(null);
    }, GIVES_UP);
    const ended = (how) => {
      timers.clearTimeout(giveUp);
      resolve(how);
    };
    child.on("error", (e) => ended(e.code === "ENOENT" ? NOT_FOUND : null));
    child.on("exit", ended);
  });

  // Detached, so that Stream Deck can't take the App down with it, should this run become the App.
  const raise = (bin) => new Promise((resolve) => {
    let child;
    try {
      child = spawn(bin, [], { ...options, detached: true });
    } catch {
      return resolve(false);
    }
    child.on("error", () => resolve(false));
    child.on("spawn", () => resolve(true));
    child.unref();
  });

  let bin = env.HERDR_APP_BIN || "herdr-app";
  let code = await run(bin);
  if (code === NOT_FOUND && !env.HERDR_APP_BIN && platform === "win32" && env.LOCALAPPDATA) {
    bin = path.win32.join(env.LOCALAPPDATA, "Programs", "herdr-app", "herdr-app.exe");
    code = await run(bin);
  }
  if (!raises(code)) return false;
  return (await raise(bin)) && fine(code);
}

// The Lamp key: opens what waits and brings the App to the front. True when both went well.
export function openWaiting(deps) {
  return press(deps, CALL, (code) => code === DELIVERED || code === REFUSED, (code) => code === DELIVERED);
}

// The Working key: brings the App to the front when `herdr-app status` says it runs. True when it did.
export function raiseApp(deps) {
  return press(deps, STATUS, (code) => code === 0, () => true);
}
