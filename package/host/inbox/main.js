// Inbox: what needs you, longest waiting first, and the way to each project's roles.
// Above Needs you: one line per project with merges or decisions waiting on you (ADR-0043), read
// from `andon waiting --json --project <slug>` in the cell; the mod counts nothing itself.
// Needs you: blocked agents (ADR-0015). A role's reply is an unread mark on its Projects row, except
// in a project whose waits can't be read, where it is a Reply in Needs you as before (ADR-0043).
// Projects: the Chief of Staff and Chief Engineers, idle or working (ADR-0015).
// Issue agents that finished, work or idle are not shown (kanban-board and Spaces show the flow).
// The mod only reads: it writes no label.

import { View, div, svg, image } from "gpui-kit";
import { v_flex, h_flex } from "gpui-base";
import { agents, terminals, spaces, panes, machines, focused, run, theme, setAgentOrder, nextCommand, shared, setShared, exec, layout } from "herdr";
import { displayName } from "./name.js";

const RANK = { blocked: 4, done: 3, working: 2, idle: 1 };
const KIND = { permission: "Permission", question: "Question", auth: "Auth" };
// What every copy must judge alike: when each agent last changed state, and the turn of a
// paneless role's reply the person has opened. localStorage is per copy; shared() is not.
const MEMORY = "inbox.memory";
// Before the mod was renamed from focus-queue (2026-11-04).
const OLD_MEMORY = "focus-queue.memory";
const MSG_MAX = 60;
// Andon's lights (ADR-0023), the logo's colors rather than the theme's: amber, answer it and it
// moves on; red, its turn failed and needs looking into. `lamp` fills, `ink` writes on it.
// The andon mod's lantern (../andon/brand.js) lights in the same colors.
const GLOW = {
  amber: { lamp: "#F0A202", ink: "#1F2328" },
  red: { lamp: "#D8453B", ink: "#FFFFFF" },
};
const glowOf = (i) => (i.kind === "Failed" ? "red" : "amber");
// What the lantern at the top of the column shows: red, amber, or null (unlit). Every copy sees it.
const LANTERN = "inbox.lantern";
// The same decision with what it counted (ADR-0043 decision 10): {version, at, light, failed, blocked,
// merge, decision, reply, unread, unreadable} (README "Lights"). The Stream Deck plugin reads it from
// the App's shared.json (../streamdeck/, ADR-0045): not one of Andon's contracts, but change both together.
const LAMP = "inbox.lamp";
const LAMP_VERSION = 1;
// The same value is put again, `at` renewed, this often: how a reader outside the App tells it's still kept.
const LAMP_EVERY = 30000;
// Owner waits per machine and project (ADR-0043 decisions 8 and 9), shared so every copy judges
// alike and only one calls: "<machine>/<project>" -> {at, fails, why, ok: {at, items: [{kind, since}]}}.
const WAITING = "inbox.waiting";
// A project name the mod passes to andon: the shape of a kanban project's slug (schema/project.schema.json,
// internal/spawn SlugRe): a lowercase letter or digit, then lowercase letters, digits, `_` and `-`.
const SLUG = /^[a-z0-9][a-z0-9_-]*$/;
// At most one call per machine and project in this long, whichever copy calls.
const WAIT_EVERY = 10000;
// Unreadable this many times in a row: the project lights by replies again (ADR-0043 decision 9).
const UNREADABLE_AFTER = 3;
// How long one call may run before the App stops it (as the waiting mod does).
const READ_TIMEOUT = 15000;
// A call that hasn't come back in this long no longer keeps this copy from calling again.
const READ_GIVES_UP = 60000;
const ASKS = { merge: "to merge", decision: "to decide" };

// What `andon waiting --json --project <slug>` printed, as [items, null] or [null, why it can't be
// read]. Only `version`, `project` and each item's `kind` and `since` are read.
export function parseWaiting(r, project) {
  if (!r || r.code !== 0) {
    const said = String(r?.stderr ?? "").split("\n").find((l) => l.trim()) ?? "";
    return [null, short(`andon waiting exited ${r?.code ?? "?"}${said ? `: ${said}` : ""}`)];
  }
  let out;
  try {
    out = JSON.parse(String(r.stdout));
  } catch {
    return [null, "andon waiting printed no JSON"];
  }
  if (out?.version !== 1) return [null, short(`andon waiting printed version ${out?.version}`)];
  if (out.project !== project) return [null, short(`andon waiting answered for ${out.project}`)];
  if (!Array.isArray(out.items)) return [null, "andon waiting printed no items"];
  return [out.items.map((x) => ({ kind: String(x?.kind ?? ""), since: String(x?.since ?? "") })), null];
}

// Whether what is put says the same, whenever it was put. Field by field: the App keeps its own order.
function sameLamp(was, lamp) {
  if (!was || typeof was !== "object") return false;
  const names = Object.keys(lamp);
  return Object.keys(was).length === names.length && names.every((k) => k === "at" || was[k] === lamp[k]);
}
// Put this recently, and not ahead of the clock (which may have gone back).
const recent = (was, now) => typeof was?.at === "number" && was.at <= now && now - was.at < LAMP_EVERY;

const machineOf = (addr) => (addr.includes("/") ? addr.split("/")[0] : null);
const LOCAL = "local";
const CHROME = "mods/workspace-chrome";
const machineLabel = (name) => (name === LOCAL ? "This computer" : name);
// The agent in view: the one an agent block shows, else the focused terminal (a tab layout).
const inView = () => focused()?.agent ?? focused()?.terminal ?? null;
// A sub-agent without a pane: error counts as blocked; its done is its parent's news.
const kidStatus = (s) => (s === "error" ? "blocked" : s === "done" ? "idle" : s);
// A terminal's parent as a full address (a child names it inside its own machine: `t3`).
const parentOf = (k) => {
  const machine = machineOf(k.address);
  return machine && k.parent && !k.parent.includes("/") ? `${machine}/${k.parent}` : k.parent ?? null;
};
// Andon's agents carry `project=<slug>` (andon issue start, hc run --label). They are colleagues, not
// helpers: each gets its own row even without a pane. One under a pane is that pane's own agent,
// so its row is the pane's.
// The roles a person starts and talks with (Andon's docs/contract.md §8): a turn they end is a reply.
const TALKS_TO_YOU = new Set(["chief-of-staff", "chief-engineer"]);
// Each of those roles has its own icon (Lucide clipboard-list, drafting-compass); anything else keeps the program's.
export const ROLE_ICON = { "chief-of-staff": "mods/inbox/chief-of-staff.svg", "chief-engineer": "mods/inbox/chief-engineer.svg" };
const ownRow = (k, listed) => Boolean(k.labels?.issue || (k.labels?.project && !listed.has(parentOf(k))));
const urgent = (a, b) => ((RANK[b] ?? 0) > (RANK[a] ?? 0) ? b : a);

// The andon layout's right panel (layouts/andon.json): show the column, then the waiting block of its stack.
export const PANEL = { column: "panel", stack: "panel-body", child: "waiting" };
// Whether this layout has the waiting mod's pane: a layout without it opens the Chief of Staff instead.
function hasWaitingPane() {
  const find = (n) => (n?.id === PANEL.child && n?.content?.mod === "waiting") || (n?.children ?? []).some(find);
  return find(layout());
}
function openWaiting() {
  run("layout.show", { id: PANEL.column });
  run("layout.select", { stack: PANEL.stack, child: PANEL.child });
}

// The program's icon from Herdr's program table: a one-color glyph or a picture.
function icon(p, size, color) {
  if (!p?.icon) return null;
  return p.tint ? svg(p.icon).size(size).flex_none().text_color(color) : image(p.icon).w(size).h(size).flex_none();
}

function duration(ms) {
  const m = Math.floor(ms / 60000);
  if (m < 1) return "<1m";
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  return h < 24 ? `${h}h` : `${Math.floor(h / 24)}d`;
}

function load(key, fallback) {
  try {
    return JSON.parse(localStorage.getItem(key) ?? "null") ?? fallback;
  } catch {
    return fallback;
  }
}

const short = (s) => {
  const one = (s ?? "").replace(/\s+/g, " ").trim();
  return one.length > MSG_MAX ? `${one.slice(0, MSG_MAX - 1)}…` : one;
};

export default class Inbox extends View {
  init(_props, cx) {
    this.handle = cx.focus_handle();
    // address -> { s, at }: the last state seen and when it changed (at null: seen at start).
    // address -> the turn of a paneless role's reply the person opened: no Reply again for that turn.
    // What an older version kept (the focus-queue name) is carried over once.
    const mem = /** @type {any} */ (shared(MEMORY) ?? shared(OLD_MEMORY));
    this.changed = mem?.changed ?? load("changed", {});
    this.seen = mem?.seen ?? {};
    // Machine and project pairs this copy is reading now, with when the call went out.
    this.reading = new Map();
    this.sel = null;
    // What inbox.call opened last, in this copy.
    this.called = null;
    this.filter = "";
    this.typing = false;
    this.gAt = 0;
    // The machines the queue is narrowed to (null: all), and the picker's state.
    this.picked = load("machines", null);
    this.picker = false;
    this.pickerAt = 0;
    // The one timer: wait times move on, and owner waits are read again (ADR-0043 decision 8).
    cx.timer.every(30000, (cx) => {
      this.poll(cx, true);
      cx.notify();
    });
    cx.spawn(async (cx) => {
      for (;;) {
        const c = await nextCommand();
        this.command(c.replace(/^inbox[.:]/, ""), cx);
        cx.notify();
      }
    });
  }

  // Live descendants of `addr` that have no pane (not in agents()) and no row of their own, depth first.
  kids(terms, listed, addr) {
    return terms
      .filter((k) => k.state === "live" && !listed.has(k.address) && parentOf(k) === addr && !ownRow(k, listed))
      .flatMap((k) => [k, ...this.kids(terms, listed, k.address)]);
  }

  // Everything the inbox shows, in order. Also keeps the change times.
  model() {
    const now = Date.now();
    const mem = /** @type {any} */ (shared(MEMORY));
    if (mem) {
      this.changed = mem.changed ?? this.changed;
      this.seen = mem.seen ?? this.seen;
    }
    const list = agents();
    const terms = terminals();
    const byAddr = new Map(terms.map((t) => [t.address, t]));
    const listed = new Set(list.map((a) => a.address));
    const spaceList = spaces();
    const tabs = new Map(spaceList.flatMap((s) => s.tabs).map((t) => [t.id, t]));
    const spaceOf = new Map(spaceList.map((s) => [s.id, s]));
    const branchOf = new Map(panes().filter((p) => p.terminal).map((p) => [p.terminal, p.branch]));
    // Andon's agents without a pane, shaped like agents() rows: shown by peeking, named by hc.
    const paneless = terms
      .filter((k) => k.state === "live" && !listed.has(k.address) && ownRow(k, listed))
      .map((k) => ({
        address: k.address,
        status: k.status === "error" ? "blocked" : k.status ?? "idle",
        program: k.program,
        title: k.name ?? "",
        // "project › issue", or the Chief of Staff's project alone.
        space_name: [k.labels.project, k.labels.issue].filter(Boolean).join(" › "),
        tab: null,
        tab_name: "",
        space: null,
        paneless: true,
      }));
    // One machine and project pair per Chief of Staff terminal: where andon waiting is called.
    const store = /** @type {Record<string, any>} */ (shared(WAITING) ?? {});
    const cosOf = new Map();
    for (const k of terms) {
      if (k.state !== "live" || k.labels?.role !== "chief-of-staff" || !k.labels?.project) continue;
      const machine = machineOf(k.address) ?? LOCAL;
      const key = `${machine}/${k.labels.project}`;
      if (!cosOf.has(key)) cosOf.set(key, { key, machine, project: k.labels.project, cos: k.address, paned: listed.has(k.address), changedAt: 0 });
    }
    // How a project's waits stand: read (the last good output, fewer than 3 failures since),
    // pending (not read yet) or unreadable (with why). Pending and unreadable light by replies.
    const standing = (machine, project) => {
      const key = `${machine}/${project}`;
      if (!project) return { key, mode: "pending" };
      if (!cosOf.has(key)) return { key, machine, project, mode: "unreadable", why: "no Chief of Staff here" };
      if (!SLUG.test(project)) return { key, machine, project, mode: "unreadable", why: "the project's name isn't one andon takes" };
      const e = store[key];
      const fails = e?.fails ?? 0;
      if (fails >= UNREADABLE_AFTER) return { key, machine, project, mode: "unreadable", why: e?.why ?? "unreadable" };
      if (Array.isArray(e?.ok?.items)) return { key, machine, project, mode: "read", items: e.ok.items };
      return { key, mode: "pending" };
    };
    let dirty = false;
    // A failed turn needs the person as a question does. Programs report it as `error`; agents() has
    // no such word, so it's read from the terminal's own status.
    const own = (a) => (byAddr.get(a.address)?.status === "error" ? "blocked" : a.status);
    const items = [...list.map((a) => ({ ...a, status: own(a), paneless: false })), ...paneless].map((a) => {
      const term = byAddr.get(a.address) ?? {};
      const kids = this.kids(terms, listed, a.address);
      const status = kids.reduce((s, k) => urgent(s, kidStatus(k.status)), a.status);
      // A blocked sub-agent opens by itself (peek) unless the parent is blocked too.
      const peek = a.status !== "blocked" ? kids.find((k) => kidStatus(k.status) === "blocked") : null;
      const report = (peek ? peek.report : term.report) ?? {};
      const failed = (peek ?? term).status === "error";
      const turn = term.turn ?? 0;
      // Done and idle are one: looking at a done agent turns it idle.
      const s = status === "done" ? "idle" : status;
      const seen = this.changed[a.address];
      if (seen?.s !== s) {
        this.changed[a.address] = { s, at: seen ? now : null };
        dirty = true;
      }
      // The Chief of Staff and a Chief Engineer talk with the person, so a turn they end is a reply,
      // unseen while Herdr says done (it turns done into idle by itself once looked at). One without
      // a pane stays done: opening it is looking, remembered for that turn.
      const lead = [term, ...kids].find((k) => TALKS_TO_YOU.has(k.labels?.role));
      const talks = Boolean(lead);
      const unseen = talks && status === "done" && !(a.paneless && this.seen[a.address] === turn);
      const project = lead?.labels?.project ?? term.labels?.project ?? "";
      const machine = machineOf(a.address) ?? LOCAL;
      const stand = talks ? standing(machine, project) : null;
      // Where the project's waits are read, a reply is only an unread mark in Projects; where they
      // can't be, it is a Reply in Needs you and lights the lantern as before (ADR-0043 decision 9).
      const reply = unseen && stand?.mode !== "read";
      const unread = unseen && stand?.mode === "read";
      // Anything blocked needs the person; otherwise only the roles show, as the way in.
      const section = status === "blocked" || reply ? "needs" : talks ? "projects" : null;
      const at = this.changed[a.address].at;
      const pair = stand && cosOf.get(stand.key);
      if (pair) pair.changedAt = Math.max(pair.changedAt, at ?? 0);
      const tab = tabs.get(a.tab);
      return {
        address: a.address,
        status,
        section,
        turn,
        talks,
        role: lead?.labels?.role ?? null,
        project,
        issue: lead?.labels?.issue ?? "",
        paneless: a.paneless,
        peek: peek?.address ?? (a.paneless ? a.address : null),
        kind: status === "blocked" ? (failed ? "Failed" : KIND[report.kind] ?? null) : reply ? "Reply" : null,
        reply,
        unread,
        stand,
        msg: short(report.msg),
        // A `title` label (andon issue start puts the issue's title there) names it best.
        name: term.labels?.title || (a.paneless && a.title ? a.title : displayName(a, tab)),
        program: a.program,
        space: a.space_name,
        tab: tab?.name ?? a.tab_name,
        branch: branchOf.get(a.address) ?? spaceOf.get(a.space)?.branch ?? null,
        machine,
        // hc ls can leave status_ms out; the tab's last change is the next best guess.
        since: at ?? term.status_ms ?? tab?.changed_at ?? now,
        approx: at == null,
      };
    });
    if (dirty) {
      setShared(MEMORY, { changed: this.changed, seen: this.seen });
    }
    const f = this.filter.toLowerCase();
    const match = (i) => !f || [i.name, i.space, i.tab, i.machine].some((x) => x.toLowerCase().includes(f));
    // Every machine, with how many wait on it and whether it is connected.
    const hosts = [{ name: LOCAL, away: false }, ...machines().filter((x) => x.name).map((x) => ({ name: x.name, away: x.layout === "unavailable" }))];
    const waits = (i) => i.section === "needs";
    // The projects with merges or decisions waiting on you: one line each, by project then machine.
    // Hand work (`task`) shows only in the waiting pane (ADR-0043 decision 4).
    const owed = [...cosOf.values()].flatMap((p) => {
      const s = standing(p.machine, p.project);
      if (s.mode !== "read") return [];
      const asks = s.items.filter((x) => x.kind in ASKS);
      if (!asks.length) return [];
      const times = asks.map((x) => Date.parse(x.since)).filter(Number.isFinite);
      const count = (kind) => asks.filter((x) => x.kind === kind).length;
      return [{ ...p, merge: count("merge"), decision: count("decision"), since: times.length ? Math.min(...times) : now }];
    }).sort((x, y) => x.project.localeCompare(y.project) || x.machine.localeCompare(y.machine));
    const owedCount = (w) => w.merge + w.decision;
    // The projects whose waits can't be read, each once: they light by replies (ADR-0043 decision 9).
    const lost = [...new Map(items.filter((i) => i.stand?.mode === "unreadable").map((i) => [i.stand.key, i.stand])).values()]
      .sort((x, y) => x.project.localeCompare(y.project) || x.machine.localeCompare(y.machine));
    // The lantern speaks for everything, whatever the pick or filter hides (ADR-0043 decision 10).
    const needing = items.filter(waits);
    const failed = needing.filter((i) => glowOf(i) === "red").length;
    const replies = needing.filter((i) => i.reply).length;
    const lamp = {
      version: LAMP_VERSION,
      at: now,
      light: null,
      failed,
      blocked: needing.length - failed - replies,
      merge: owed.reduce((n, w) => n + w.merge, 0),
      decision: owed.reduce((n, w) => n + w.decision, 0),
      reply: replies,
      unread: items.filter((i) => i.unread).length,
      unreadable: lost.length,
    };
    lamp.light = lamp.failed ? "red" : lamp.blocked + lamp.merge + lamp.decision + lamp.reply ? "amber" : null;
    if ((shared(LANTERN) ?? null) !== lamp.light) setShared(LANTERN, lamp.light);
    const put = /** @type {any} */ (shared(LAMP));
    if (!sameLamp(put, lamp) || !recent(put, now)) setShared(LAMP, lamp);
    const picked = this.picked?.filter((n) => hosts.some((x) => x.name === n)) ?? null;
    const here = (i) => !picked || picked.includes(i.machine);
    const pick = (id) => items.filter((i) => i.section === id && match(i) && here(i));
    const oldest = (x, y) => x.since - y.since;
    const waitingOn = (name) => items.filter((i) => i.machine === name && waits(i)).length
      + owed.filter((w) => w.machine === name).reduce((n, w) => n + owedCount(w), 0);
    const m = {
      now,
      machines: hosts.length > 1 ? hosts.map((x) => ({ ...x, waiting: waitingOn(x.name) })) : [],
      picked,
      // Waiting on machines left out of the pick: never hidden without a count.
      elsewhere: items.filter((i) => waits(i) && !here(i)).length + owed.filter((w) => !here(w)).reduce((n, w) => n + owedCount(w), 0),
      // Not narrowed by the filter: a count line isn't a row to search.
      owed: owed.filter(here),
      lost: lost.filter(here),
      pairs: [...cosOf.values()],
      lamp,
      needs: pick("needs").sort(oldest),
      // By project, then the Chief of Staff before Chief Engineers (by issue).
      projects: pick("projects").sort((x, y) => x.project.localeCompare(y.project)
        || (x.role === "chief-of-staff" ? 0 : 1) - (y.role === "chief-of-staff" ? 0 : 1)
        || x.issue.localeCompare(y.issue) || x.name.localeCompare(y.name)),
    };
    // What next walks: Needs you, then the roles with an unread reply, oldest first (ADR-0043
    // decision 5). What j/k walk: both sections.
    m.queue = [...m.needs, ...m.projects.filter((i) => i.unread).sort(oldest)];
    m.rows = [...m.needs, ...m.projects];
    // What inbox.call walks (ADR-0045 decision 9): what lights the lantern, so neither the pick nor
    // the filter narrows it. Needs you, oldest first, then the count lines, oldest first.
    m.calls = [
      ...[...needing].sort(oldest).map((row) => ({ key: `row:${row.address}`, row })),
      ...[...owed].sort(oldest).map((line) => ({ key: `owed:${line.key}`, line })),
    ];
    return m;
  }

  open(i) {
    if (i.peek) run("terminal.peek", { terminal: i.peek });
    else run("agent.focus", { terminal: i.address });
    // A paneless role's reply stays done in the core: opening it is the person looking.
    if ((i.reply || i.unread) && i.paneless) {
      this.seen[i.address] = i.turn;
      setShared(MEMORY, { changed: this.changed, seen: this.seen });
    }
  }

  // A count line: the waiting pane, or without one, the project's Chief of Staff (ADR-0043 decision 7).
  openOwed(w) {
    if (hasWaitingPane()) openWaiting();
    else run(w.paned ? "agent.focus" : "terminal.peek", { terminal: w.cos });
  }

  // Reads the owner waits of each machine and project that is due: on the timer (`tick`), when a
  // role's state changed since the last read, or never read. At most once per WAIT_EVERY, whichever
  // copy of the mod reads (the time is kept in shared()).
  poll(cx, tick, m = this.model()) {
    if (!cx?.spawn) return;
    const now = Date.now();
    const store = /** @type {Record<string, any>} */ (shared(WAITING) ?? {});
    for (const p of m.pairs) {
      if (!SLUG.test(p.project) || now - (this.reading.get(p.key) ?? -Infinity) < READ_GIVES_UP) continue;
      const e = store[p.key];
      if (e && now - (e.at ?? 0) < WAIT_EVERY) continue;
      if (e && !tick && !(p.changedAt > (e.at ?? 0))) continue;
      this.read(cx, p);
    }
  }

  read(cx, p) {
    const put = (change) => {
      const all = { ...(/** @type {Record<string, any>} */ (shared(WAITING) ?? {})) };
      const was = all[p.key] ?? { at: 0, fails: 0, why: null, ok: null };
      all[p.key] = { ...was, ...change(was) };
      setShared(WAITING, all);
    };
    this.reading.set(p.key, Date.now());
    put(() => ({ at: Date.now() }));
    cx.spawn(async (cx) => {
      let items = null;
      let why = null;
      try {
        const r = await exec("andon", ["waiting", "--json", "--project", p.project], { on: p.machine === LOCAL ? null : p.machine, timeout: READ_TIMEOUT });
        [items, why] = parseWaiting(r, p.project);
      } catch (e) {
        why = short(`can't run andon waiting: ${e?.message ?? e}`);
      }
      this.reading.delete(p.key);
      put((was) => (items ? { fails: 0, why: null, ok: { at: Date.now(), items } } : { fails: (was.fails ?? 0) + 1, why }));
      // The lantern and inbox.lamp now, not at the next paint: a window out of sight may not paint.
      this.model();
      cx.notify();
    });
  }

  // The longest-waiting item that isn't the one in view.
  next(m) {
    const shown = inView();
    const i = m.queue.find((x) => x.address !== shown);
    if (i) this.open(i);
  }

  // The one after what this opened last, else the first: a key outside the App can't see what is in view.
  call(m) {
    if (!m.calls.length) return;
    const one = m.calls[(m.calls.findIndex((x) => x.key === this.called) + 1) % m.calls.length];
    this.called = one.key;
    if (one.row) {
      this.sel = one.row.address;
      this.open(one.row);
    } else this.openOwed(one.line);
  }

  command(c, cx) {
    const m = this.model();
    if (c === "next") this.next(m);
    else if (c === "call") this.call(m);
    else if (c === "focus") {
      this.sel ??= m.rows[0]?.address ?? null;
      this.handle.focus();
    }
  }

  // The second layer of keys: only while the queue has the keyboard.
  key(e, cx) {
    const m = this.model();
    const k = e.key;
    const ch = e.key_char ?? "";
    if (this.typing) {
      if (k === "escape") {
        this.typing = false;
        this.filter = "";
      } else if (k === "enter") this.typing = false;
      else if (k === "backspace") this.filter = this.filter.slice(0, -1);
      else if (ch && !e.modifiers.control && !e.modifiers.alt && !e.modifiers.platform) this.filter += ch;
      else return;
      cx.stop_propagation();
      cx.notify();
      return;
    }
    if (e.modifiers.control || e.modifiers.alt || e.modifiers.platform) return;
    if (this.picker) {
      const n = m.machines.length;
      if (k === "j" || k === "down") this.pickerAt = Math.min(n - 1, this.pickerAt + 1);
      else if (k === "k" || k === "up") this.pickerAt = Math.max(0, this.pickerAt - 1);
      else if (k === "space" || k === "enter") this.toggleMachine(m, m.machines[this.pickerAt]?.name);
      else if (k === "a") this.pickMachines(null);
      else if (k === "m" || k === "escape") this.picker = false;
      else return;
      cx.stop_propagation();
      cx.notify();
      return;
    }
    const rows = m.rows;
    let at = rows.findIndex((r) => r.address === this.sel);
    const pressedG = this.gAt && m.now - this.gAt < 1000;
    this.gAt = 0;
    if (k === "j" || k === "down") at = Math.min(rows.length - 1, at + 1);
    else if (k === "k" || k === "up") at = Math.max(0, at - 1);
    else if (ch === "G" || (k === "g" && e.modifiers.shift)) at = rows.length - 1;
    else if (k === "g") {
      if (pressedG) at = 0;
      else this.gAt = m.now;
    } else if (k === "enter" || k === "l") {
      if (rows[at]) this.open(rows[at]);
    } else if (k === "m") {
      if (m.machines.length) this.picker = true;
    } else if (ch === "/" || k === "/") {
      this.typing = true;
      this.filter = "";
    } else if (k === "escape") {
      if (this.filter) this.filter = "";
      else run("pane.go", { direction: "right" });
    } else return;
    this.sel = rows[at]?.address ?? this.sel;
    cx.stop_propagation();
    cx.notify();
  }

  pickMachines(names) {
    this.picked = names;
    localStorage.setItem("machines", JSON.stringify(names));
  }

  // Checks or unchecks one machine; checking every one is "all" again.
  toggleMachine(m, name) {
    if (!name) return;
    const all = m.machines.map((x) => x.name);
    const now = m.picked ?? all;
    const next = now.includes(name) ? now.filter((n) => n !== name) : [...now, name];
    this.pickMachines(next.length === all.length ? null : all.filter((n) => next.includes(n)));
  }

  // The status mark, drawn as Herdr's own agents list draws it; a row that needs you is lit.
  mark(t, i) {
    if (i.status === "blocked" || i.reply) {
      const g = GLOW[glowOf(i)];
      return div().flex_none().px(6).h(18).rounded(5).bg(g.lamp).text_size(10.5).font_semibold().text_color(g.ink)
        .flex().items_center().child(i.kind ?? "Needs you");
    }
    if (i.status === "working") return svg(`${CHROME}/spinner.svg`).size(12).mx(1).flex_none().text_color(t.working).spin(900);
    // A role's reply you haven't looked at: a small dot in the theme's muted color, never a light
    // (ADR-0043 decision 5; a first shape to look at and change).
    if (i.unread) return div().id(`unread-${i.address}`).size(7).mx(3).flex_none().rounded(4).bg(t.muted).tooltip("Unread reply");
    return div().size(7).mx(3).flex_none().rounded(4).border(1.5).border_color(t.idle ?? t.dim);
  }

  // A button in the column's header line.
  headButton(t, id, icon, tip, act) {
    return div().id(id).size(24).flex_none().flex().items_center().justify_center().rounded(6).cursor_pointer()
      .hover((s) => s.bg(t.hover).text_color(t.foreground))
      .tooltip(tip)
      .on_click((_e, cx) => {
        act(cx);
        cx.notify();
      })
      .child(svg(icon).size(13));
  }

  row(t, m, i, keyboard) {
    const shown = inView() === i.address;
    const picked = keyboard && this.sel === i.address;
    const time = `${duration(m.now - i.since)}${i.approx ? "+" : ""}`;
    const quiet = i.status === "idle";
    // The Chief of Staff runs the same program as the issue agents: its own glyph tells it apart.
    const pic = i.talks ? svg(ROLE_ICON[i.role]).size(16).flex_none().text_color(t.accent) : icon(i.program, 16, t.foreground);
    // Only what's unusual: a named tab, a branch other than main/master, the agent's message.
    const branch = i.branch === "main" || i.branch === "master" ? null : i.branch;
    const tail = [i.tab !== i.name && !/^\d+$/.test(i.tab) ? i.tab : null, branch, i.msg].filter(Boolean).join(" · ");
    // Not when one machine is picked.
    const machine = m.picked?.length !== 1;
    return h_flex()
      .id(`row-${i.address}`)
      .px(8)
      .py(6)
      .gap(9)
      .flex_none()
      .items_start()
      .rounded(8)
      .cursor_pointer()
      .when(shown, (el) => el.bg(t.selected))
      .when(picked, (el) => el.border(1).border_color(t.accent))
      .hover((s) => s.bg(t.hover))
      .on_click((_e, cx) => {
        this.sel = i.address;
        this.open(i);
        cx.notify();
      })
      .child(div().w(16).mt(2).flex_none().when(Boolean(pic), (el) => el.child(pic)))
      .child(v_flex().flex_1().min_w(0).gap(1)
        .child(h_flex().h(20).gap(6).items_center()
          .child(div().flex_1().min_w(0).truncate().text_color(quiet ? t.muted : t.foreground).child(i.name))
          .child(div().flex_none().text_size(11).text_color(t.dim).child(time))
          .child(this.mark(t, i)))
        .child(h_flex().h(16).gap(5).items_center().text_size(11.5).text_color(t.dim)
          .when(machine, (el) => el.child(svg(`${CHROME}/${i.machine === LOCAL ? "monitor" : "server"}.svg`).size(11).flex_none())
            .child(div().flex_none().text_color(t.muted).child(machineLabel(i.machine))).child(div().flex_none().child("·")))
          .child(div().flex_1().min_w(0).truncate().child(i.space)))
        .when(Boolean(tail), (el) => el.child(div().truncate().text_size(11.5).text_color(t.dim).child(tail))));
  }

  // The machine picker: the header's button names the pick; it opens a list of machines in the column.
  pickerButton(t, m) {
    const p = m.picked;
    const label = !p ? "All machines" : p.length === 0 ? "No machines" : p.length <= 2 ? p.map(machineLabel).join(", ") : `${p.length} machines`;
    return h_flex().id("machine-picker").min_w(0).h(24).px(8).gap(5).items_center().rounded(6).cursor_pointer()
      .text_size(11.5).text_color(p ? t.foreground : t.dim)
      .when(this.picker, (el) => el.bg(t.hover))
      .hover((s) => s.bg(t.hover).text_color(t.foreground))
      .on_click((_e, cx) => {
        this.picker = !this.picker;
        cx.notify();
      })
      .child(svg(`${CHROME}/server.svg`).size(12).flex_none())
      .child(div().min_w(0).truncate().child(label))
      .child(svg(`${CHROME}/chevron-${this.picker ? "down" : "right"}.svg`).size(11).flex_none());
  }

  // Under the header while machines are left out or away: how many wait elsewhere, how many are offline.
  pickerNote(t, m) {
    const away = m.machines.filter((x) => x.away).length;
    if (!m.elsewhere && !away) return null;
    return h_flex().px(16).pb(4).gap(8).flex_none().items_center().text_size(11)
      .when(m.elsewhere > 0, (el) => el.child(div().id("machine-elsewhere").flex_none().text_color(t.blocked).cursor_pointer()
        .hover((s) => s.text_color(t.foreground))
        .tooltip("Show all machines")
        .on_click((_e, cx) => {
          this.pickMachines(null);
          cx.notify();
        })
        .child(`+${m.elsewhere} waiting elsewhere`)))
      .when(away > 0, (el) => el.child(div().flex_none().text_color(t.dim).child(`${away} offline`)));
  }

  pickerList(t, m, keyboard) {
    const on = (name) => !m.picked || m.picked.includes(name);
    const rows = m.machines.map((x, n) => h_flex().id(`machine-${x.name}`).h(28).px(8).gap(7).flex_none().items_center().rounded(7)
      .text_size(12).cursor_pointer()
      .when(keyboard && n === this.pickerAt, (el) => el.bg(t.hover))
      .hover((s) => s.bg(t.hover))
      .on_click((_e, cx) => {
        this.pickerAt = n;
        this.toggleMachine(m, x.name);
        cx.notify();
      })
      .child(div().size(14).flex_none().flex().items_center().justify_center().rounded(4)
        .when(on(x.name), (el) => el.bg(t.solid).text_color(t.solid_foreground).child(svg(`${CHROME}/check.svg`).size(11)))
        .when(!on(x.name), (el) => el.border(1.5).border_color(t.input)))
      .child(svg(`${CHROME}/${x.name === LOCAL ? "monitor" : "server"}.svg`).size(13).flex_none().text_color(t.dim))
      .child(div().flex_1().min_w(0).truncate().text_color(x.away ? t.dim : t.foreground).child(machineLabel(x.name)))
      .when(x.away, (el) => el.child(div().flex_none().text_size(11).text_color(t.dim).child("not connected")))
      .when(!x.away && x.waiting > 0, (el) => el.child(div().flex_none().text_size(11).text_color(t.blocked).child(String(x.waiting))))
      .child(div().size(6).rounded(3).flex_none().bg(x.away ? t.dim : t.ansi2)));
    const all = h_flex().id("machine-all").h(28).px(8).flex_none().items_center().rounded(7).text_size(12).text_color(t.dim).cursor_pointer()
      .hover((s) => s.bg(t.hover).text_color(t.foreground))
      .on_click((_e, cx) => {
        this.pickMachines(null);
        cx.notify();
      })
      .child("All machines");
    return v_flex().mx(8).mb(6).p(4).flex_none().rounded(8).bg(t.popover).border(1).border_color(t.border)
      .children(rows).child(all);
  }

  // Above Needs you, one per project with merges or decisions waiting on you (ADR-0043 decision 7):
  // the amber dot, the project, the counts by kind, the oldest wait. No heading.
  owedLine(t, m, w) {
    const counts = Object.entries(ASKS).filter(([k]) => w[k] > 0).map(([k, s]) => `${w[k]} ${s}`).join(" · ");
    return h_flex().id(`owed-${w.key}`).px(8).py(5).gap(8).flex_none().items_center().rounded(8).cursor_pointer()
      .hover((s) => s.bg(t.hover))
      .tooltip("Waiting on you")
      .on_click((_e, cx) => {
        this.openOwed(w);
        cx.notify();
      })
      .child(div().size(7).flex_none().rounded(4).bg(GLOW.amber.lamp))
      .child(div().flex_none().text_color(t.foreground).child(w.project))
      .child(div().flex_1().min_w(0).truncate().text_size(11.5).text_color(t.muted).child(counts))
      .child(div().flex_none().text_size(11).text_color(t.dim).child(duration(m.now - w.since)));
  }

  // A project whose waits can't be read: why, in one line (ADR-0043 decision 9).
  lostLine(t, s) {
    return div().id(`lost-${s.key}`).px(8).py(3).flex_none().truncate().text_size(11).text_color(t.dim)
      .child(`${s.project}: replies light the lantern (${s.why})`);
  }

  // A section's heading; `lamp` puts a lit dot before it.
  heading(t, label, count, color, lamp = null) {
    return h_flex().px(8).pt(8).h(30).gap(6).flex_none().items_center().text_size(11).font_medium()
      .when(Boolean(lamp), (el) => el.child(div().size(7).flex_none().rounded(4).bg(lamp)))
      .child(div().text_color(color).child(label))
      .child(div().text_color(t.dim).child(String(count)));
  }

  render(cx) {
    // Theme colors are "#rrggbb" strings; say so, so element colors type-check.
    const t = /** @type {Record<string, `#${string}`>} */ (theme());
    const m = this.model();
    this.poll(cx, false, m);
    setAgentOrder(m.rows.map((i) => i.address));
    const keyboard = this.handle.is_focused();
    if (!m.rows.some((r) => r.address === this.sel)) this.sel = m.rows[0]?.address ?? null;
    const body = [];
    if (this.typing || this.filter) {
      body.push(h_flex().h(28).px(8).gap(7).flex_none().items_center().text_size(12).text_color(this.typing ? t.foreground : t.muted)
        .child(svg(`${CHROME}/search.svg`).size(12).flex_none().text_color(t.dim))
        .child(div().child(`${this.filter}${this.typing ? "▏" : ""}`)));
    }
    body.push(...m.owed.map((w) => this.owedLine(t, m, w)), ...m.lost.map((s) => this.lostLine(t, s)));
    if (m.needs.length) body.push(this.heading(t, "Needs you", m.needs.length, t.foreground, GLOW.amber.lamp), ...m.needs.map((i) => this.row(t, m, i, keyboard)));
    else if (this.filter || !m.owed.length) body.push(div().px(8).py(12).text_size(12).text_color(t.dim).child(this.filter ? "Nothing matches." : "Nothing needs you."));
    if (m.projects.length) body.push(this.heading(t, "Projects", m.projects.length, t.dim), ...m.projects.map((i) => this.row(t, m, i, keyboard)));
    const note = m.machines.length ? this.pickerNote(t, m) : null;
    return v_flex()
      .id("inbox")
      .track_focus(this.handle)
      .on_key_down((e, cx) => this.key(e, cx))
      .size_full()
      .min_h(0)
      .bg(t.chrome)
      .text_size(13)
      .child(h_flex().h(31).pl(16).pr(8).gap(8).flex_none().items_center().justify_between().text_size(11.5).text_color(t.dim)
        .child(div().flex_none().child("Inbox"))
        .child(h_flex().min_w(0).gap(2).items_center()
          .when(m.machines.length > 0, (el) => el.child(this.pickerButton(t, m)))
          // Every key command has a button too; the tooltip names the key (keys get forgotten).
          .child(this.headButton(t, "fq-next", "mods/inbox/next.svg", "Next waiting (ctrl-shift-j)", () => this.next(this.model())))
          .child(this.headButton(t, "fq-kanban-board", "mods/inbox/kanban-board.svg", "Kanban board (ctrl-shift-y)", () => run("mod.open", { name: "kanban-board" })))
          // The waiting mod's screen in the right panel (ADR-0042 decision 12). No count here: Needs you stays terminal states.
          .child(this.headButton(t, "fq-waiting", "mods/inbox/waiting.svg", "Waiting on you (right panel)", () => openWaiting()))))
      .when(Boolean(note), (el) => el.child(note))
      .when(this.picker && m.machines.length > 0, (el) => el.child(this.pickerList(t, m, keyboard)))
      .child(v_flex().id("fq-list").flex_1().min_h(0).px(8).pb(14).overflow_y_scrollbar().mode("always").children(body))
      .when(keyboard, (el) => el.child(div().h(24).px(16).flex_none().flex().items_center().text_size(10.5).text_color(t.dim)
        .child(this.picker ? "j/k · Space check · a all · m close" : "j/k · Enter open · m machines · / filter · Esc")));
  }
}
