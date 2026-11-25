// How many of Andon's role terminals are working (ADR-0045 decisions 15 and 16), from herdr-core's own events:
// `hc events --json` sends the terminals here and on connected machines, then every change, one JSON per line.
// It is a fact the API returns, not a light: the inbox mod still decides the lantern alone.

// Three of the 15-second heartbeats.
export const SILENT = 45000;
// How long it waits before starting `hc events` again after it ended or went silent.
export const AGAIN = 30000;

// A terminal Andon started for a role: the Chief of Staff, the Chief Engineer and the dispatcher carry the
// label `role`, a worker and a reviewer the label `issue` (decision 16).
const isRole = (labels) => [labels?.role, labels?.issue].some((v) => typeof v === "string" && v !== "");

// The number shown: role terminals that haven't finished and whose status is working.
export function countWorking(terminals) {
  let n = 0;
  for (const t of terminals.values()) {
    if (t?.state !== "finished" && t?.status?.state === "working" && isRole(t.labels)) n++;
  }
  return n;
}

// Follows `hc events --json` while started. `show(n)` gets the count, or null when it can't tell ("?").
export function follower({ env, home, spawn, timers, now, show }) {
  const options = { env, cwd: home, stdio: ["ignore", "pipe", "ignore"], shell: false, windowsHide: true };
  let wanted = false;
  let run = 0; // which child the callbacks belong to; a stale one is ignored
  let child = null;
  let silence = null;
  let again = null;
  let last = 0;

  function ended(which) {
    if (which !== run || !wanted) return;
    run++;
    timers.clearTimeout(silence);
    silence = null;
    child?.kill();
    child = null;
    show(null);
    again ??= timers.setTimeout(() => {
      again = null;
      begin();
    }, AGAIN);
  }

  function watchSilence(which) {
    silence = timers.setTimeout(() => {
      if (which !== run) return;
      const left = SILENT - (now() - last);
      if (left > 0) watchSilence(which);
      else ended(which);
    }, Math.max(SILENT - (now() - last), 1));
  }

  function begin() {
    if (!wanted || child) return;
    const which = ++run;
    const terminals = new Map();
    let snapshot = -1; // the seq of the last list; events at or below it are already in it
    let known = false;
    let rest = "";

    function take(line) {
      let m;
      try {
        m = JSON.parse(line);
      } catch {
        return;
      }
      const list = Array.isArray(m?.terminals) ? m : Array.isArray(m?.params?.terminals) ? m.params : null;
      if (list) {
        terminals.clear();
        for (const t of list.terminals) if (typeof t?.address === "string") terminals.set(t.address, t);
        snapshot = Number.isSafeInteger(list.seq) ? list.seq : -1;
        known = true;
      } else if (m?.method === "event" && known) {
        const { terminal: address, type, data, seq } = m.params ?? {};
        if (typeof address !== "string" || (Number.isSafeInteger(seq) && seq <= snapshot)) return;
        const t = terminals.get(address);
        if (type === "created" && data) terminals.set(address, { ...data, address });
        else if (type === "removed") terminals.delete(address);
        else if (!t) return;
        else if (type === "status") t.status = data?.status ?? null;
        else if (type === "labels") t.labels = data?.labels ?? {};
        else if (type === "finished") t.state = "finished";
        else if (type === "lost") t.status = null; // its program is gone; a new one says its own status

        else return;
      } else if (m?.method === "missed") {
        known = false; // until the fresh list
        show(null);
        return;
      } else return;
      if (known) show(countWorking(terminals));
    }

    try {
      child = spawn(env.HC_BIN || "hc", ["events", "--json"], options);
    } catch {
      child = null;
      return ended(which);
    }
    last = now();
    watchSilence(which);
    child.on("error", () => ended(which));
    child.on("exit", () => ended(which));
    child.stdout?.setEncoding?.("utf8");
    child.stdout?.on("data", (chunk) => {
      if (which !== run) return;
      last = now();
      const lines = (rest + chunk).split("\n");
      rest = lines.pop();
      lines.forEach(take);
    });
  }

  return {
    start() {
      wanted = true;
      begin();
    },
    // Ends the child; nothing is shown after this.
    stop() {
      wanted = false;
      run++;
      timers.clearTimeout(silence);
      timers.clearTimeout(again);
      silence = again = null;
      child?.kill();
      child = null;
    },
  };
}
