// Inbox: the agents waiting for you, longest waiting first.
// Needs you (blocked) before Done (finished and not settled yet). Settle, then next.
// Working, Idle and Settled wait in folded shelves.
// Opened as a dialog (mod.open:inbox), the same queue is drawn as a Kanban board:
// a column per section and a swimlane per machine.

import { View, div, svg, image } from "gpui-kit";
import { v_flex, h_flex } from "gpui-base";
import { agents, terminals, spaces, panes, machines, focused, run, hc, notify, theme, setAgentOrder, nextCommand, shared, setShared, props, closeOverlay } from "herdr";
import { displayName } from "./name.js";

const RANK = { blocked: 4, done: 3, working: 2, idle: 1 };
const KIND = { permission: "Permission", question: "Question", auth: "Auth" };
const SHELVES = [
  { id: "working", label: "Working" },
  { id: "idle", label: "Idle" },
  { id: "settled", label: "Settled" },
];
const BOARD_COLUMNS = [
  { id: "needs", label: "Needs you" },
  { id: "done", label: "Done" },
  { id: "working", label: "Working" },
  { id: "idle", label: "Idle" },
  { id: "settled", label: "Settled" },
];
// What every copy (the column, the board dialog) must judge alike: when each agent last
// changed state, and the turn it had when first seen. localStorage is per copy; shared() is not.
const MEMORY = "inbox.memory";
// Before the mod was renamed from focus-queue (2026-11-04).
const OLD_MEMORY = "focus-queue.memory";
const SETTLED_FIRST = 10;
const SETTLED_MORE = 25;
const MSG_MAX = 60;

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
const ownRow = (k, listed) => Boolean(k.labels?.issue || (k.labels?.project && !listed.has(parentOf(k))));
const urgent = (a, b) => ((RANK[b] ?? 0) > (RANK[a] ?? 0) ? b : a);

// A theme color ("#rrggbb") with an alpha byte added.
const alpha = (color, aa) => /** @type {`#${string}`} */ (`${color}${aa}`);

// The program's icon from Herdr's program table: a one-color glyph or a picture.
function icon(p, size, color) {
  if (!p?.icon) return null;
  return p.tint ? svg(p.icon).size(size).flex_none().text_color(color) : image(p.icon).w(size).h(size).flex_none();
}

function parseSettled(v) {
  if (!v) return null;
  const [ms, turn] = String(v).split(":").map(Number);
  return Number.isFinite(ms) ? { ms, turn: Number.isFinite(turn) ? turn : 0 } : null;
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
    // The board dialog is another copy of this mod; it draws the queue as a board.
    this.board = Boolean(props()?.overlay);
    // Swimlanes per machine: off by default (a board reads by state; cards name their machine).
    this.lanes = load("lanes", false);
    // The board takes the keyboard when it opens (focus can't move during a render).
    if (this.board) cx.timer.after(30, () => this.handle.focus());
    // address -> { s, at }: the last state seen and when it changed (at null: seen at start);
    // address -> the turn when this mod first saw it, a later turn is news. Moved from
    // localStorage to shared(), then renamed; what an older version kept is carried over once.
    const mem = /** @type {any} */ (shared(MEMORY) ?? shared(OLD_MEMORY));
    this.changed = mem?.changed ?? load("changed", {});
    this.base = mem?.base ?? load("base", {});
    this.shelves = load("shelves", {});
    // address -> the settled value written but not back in terminals() yet ("" = removed).
    this.pending = new Map();
    // Tabs this copy has named: each at most once, so a name the person clears stays cleared.
    this.settledLimit = SETTLED_FIRST;
    this.sel = null;
    this.filter = "";
    this.typing = false;
    this.gAt = 0;
    this.lastSettled = null;
    this.hovered = null;
    // The machines the queue is narrowed to (null: all), and the picker's state.
    this.picked = load("machines", null);
    this.picker = false;
    this.pickerAt = 0;
    cx.timer.every(30000, (cx) => cx.notify());
    // Key commands go to every shown copy; only the column acts on them.
    if (!this.board) cx.spawn(async (cx) => {
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

  settledOf(addr, term) {
    return parseSettled(this.pending.has(addr) ? this.pending.get(addr) : term?.labels?.settled);
  }

  // Everything the queue shows, in order. Also keeps the change times and unsettles agents
  // that have new work.
  model(cx) {
    const now = Date.now();
    const mem = /** @type {any} */ (shared(MEMORY));
    if (mem) {
      this.changed = mem.changed ?? this.changed;
      this.base = mem.base ?? this.base;
    }
    const list = agents();
    const terms = terminals();
    const byAddr = new Map(terms.map((t) => [t.address, t]));
    const listed = new Set(list.map((a) => a.address));
    const spaceList = spaces();
    const tabs = new Map(spaceList.flatMap((s) => s.tabs).map((t) => [t.id, t]));
    const spaceOf = new Map(spaceList.map((s) => [s.id, s]));
    const branchOf = new Map(panes().filter((p) => p.terminal).map((p) => [p.terminal, p.branch]));
    for (const [addr, v] of this.pending) {
      if ((byAddr.get(addr)?.labels?.settled ?? "") === v) this.pending.delete(addr);
    }
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
      if (!(a.address in this.base)) {
        this.base[a.address] = turn;
        dirty = true;
      }
      // Done and idle are one: looking at a done agent turns it idle.
      const s = status === "done" ? "idle" : status;
      const seen = this.changed[a.address];
      if (seen?.s !== s) {
        this.changed[a.address] = { s, at: seen ? now : null };
        dirty = true;
      }
      let settled = this.settledOf(a.address, term);
      // Busy counts only when it became busy after the settle, so a blocked agent can be settled.
      const changedAt = Math.max(term.status_ms ?? 0, ...kids.map((k) => k.status_ms ?? 0));
      const busy = (status === "working" || status === "blocked") && changedAt > settled?.ms;
      if (settled && (busy || turn > settled.turn)) {
        if (this.pending.get(a.address) !== "") this.write(a.address, "", cx);
        settled = null;
      }
      // The Chief of Staff and a Chief Engineer talk with the person, so a turn they end is a reply:
      // Needs you while unseen (Herdr's done), Idle once looked at (Herdr turns done into idle by itself).
      const lead = [term, ...kids].find((k) => TALKS_TO_YOU.has(k.labels?.role));
      const talks = Boolean(lead);
      const reply = talks && status === "done";
      const finished = !talks && (status === "done" || (status === "idle" && turn > this.base[a.address]));
      const section = settled ? "settled" : status === "blocked" || reply ? "needs" : finished ? "done" : status === "working" ? "working" : "idle";
      const at = this.changed[a.address].at;
      const tab = tabs.get(a.tab);
      return {
        address: a.address,
        status,
        section,
        settled,
        turn,
        talks,
        peek: peek?.address ?? (a.paneless ? a.address : null),
        kind: status === "blocked" ? (failed ? "Failed" : KIND[report.kind] ?? null) : reply ? "Reply" : null,
        reply,
        msg: short(report.msg),
        // A `title` label (andon issue start puts the issue's title there) names it best.
        name: term.labels?.title || (a.paneless && a.title ? a.title : displayName(a, tab)),
        program: a.program,
        space: a.space_name,
        tab: tab?.name ?? a.tab_name,
        branch: branchOf.get(a.address) ?? spaceOf.get(a.space)?.branch ?? null,
        machine: machineOf(a.address) ?? LOCAL,
        // hc ls can leave status_ms out; the tab's last change is the next best guess.
        since: at ?? term.status_ms ?? tab?.changed_at ?? now,
        approx: at == null,
      };
    });
    if (dirty) {
      setShared(MEMORY, { changed: this.changed, base: this.base });
    }
    const f = this.filter.toLowerCase();
    const match = (i) => !f || [i.name, i.space, i.tab, i.machine].some((x) => x.toLowerCase().includes(f));
    // Every machine, with how many wait on it and whether it is connected.
    const hosts = [{ name: LOCAL, away: false }, ...machines().filter((x) => x.name).map((x) => ({ name: x.name, away: x.layout === "unavailable" }))];
    const waits = (i) => i.section === "needs" || i.section === "done";
    const picked = this.picked?.filter((n) => hosts.some((x) => x.name === n)) ?? null;
    const here = (i) => !picked || picked.includes(i.machine);
    const pick = (id) => items.filter((i) => i.section === id && match(i) && here(i));
    const oldest = (x, y) => x.since - y.since;
    const m = {
      now,
      machines: hosts.length > 1 ? hosts.map((x) => ({ ...x, waiting: items.filter((i) => i.machine === x.name && waits(i)).length })) : [],
      picked,
      // Waiting on machines left out of the pick: never hidden without a count.
      elsewhere: items.filter((i) => waits(i) && !here(i)).length,
      needs: pick("needs").sort(oldest),
      done: pick("done").sort(oldest),
      working: pick("working").sort(oldest),
      idle: pick("idle").sort((x, y) => x.name.localeCompare(y.name)),
      settled: pick("settled").sort((x, y) => y.settled.ms - x.settled.ms),
    };
    m.queue = [...m.needs, ...m.done];
    // What j/k walk: the queue, then the open shelves.
    m.rows = [...m.queue];
    for (const s of SHELVES) {
      if (!this.shelves[s.id]) continue;
      m.rows.push(...(s.id === "settled" ? m.settled.slice(0, this.settledLimit) : m[s.id]));
    }
    return m;
  }

  open(i) {
    if (i.peek) run("terminal.peek", { terminal: i.peek });
    else run("agent.focus", { terminal: i.address });
    // From the board, show the agent: the dialog goes.
    if (this.board) closeOverlay(props().overlay);
  }

  // The longest-waiting item that isn't the one in view.
  next(m) {
    const shown = inView();
    const i = m.queue.find((x) => x.address !== shown);
    if (i) this.open(i);
  }

  // Writes the settled label; the row moves at once, and back if hc fails.
  write(addr, value, cx) {
    this.pending.set(addr, value);
    cx.spawn(async (cx) => {
      let error = null;
      try {
        const r = await hc(["label", addr, `settled=${value}`]);
        if (r.code !== 0) error = (r.stderr || r.stdout).trim() || `hc exited ${r.code}`;
      } catch (e) {
        error = String(e instanceof Error ? e.message : e);
      }
      if (!error) return;
      this.pending.delete(addr);
      notify({ title: value ? "Couldn't settle" : "Couldn't unsettle", body: `${addr}: ${error}`, level: "error" });
      cx.notify();
    });
  }

  settle(i, m, cx) {
    const shown = inView();
    this.write(i.address, `${Date.now()}:${i.turn}`, cx);
    this.lastSettled = i.address;
    this.hovered = null;
    if (i.address === shown) this.next(m);
  }

  unsettle(addr, cx) {
    this.write(addr, "", cx);
    this.hovered = null;
  }

  command(c, cx) {
    const m = this.model(cx);
    if (c === "next") this.next(m);
    else if (c === "settle") {
      const shown = inView();
      const i = shown && m.queue.find((x) => x.address === shown);
      if (i) this.settle(i, m, cx);
      else this.next(m);
    } else if (c === "focus") {
      this.sel ??= m.rows[0]?.address ?? null;
      this.handle.focus();
    }
  }

  // The second layer of keys: only while the queue has the keyboard.
  key(e, cx) {
    if (this.board) return this.boardKey(e, cx);
    const m = this.model(cx);
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
    } else if (k === "d") {
      const i = rows[at];
      if (i && i.section !== "settled") {
        this.write(i.address, `${Date.now()}:${i.turn}`, cx);
        this.lastSettled = i.address;
        at = Math.min(at + 1, rows.length - 1);
        if (rows[at] === i) at -= 1;
      }
    } else if (k === "u") {
      if (this.lastSettled) this.unsettle(this.lastSettled, cx);
      this.lastSettled = null;
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

  // The status mark, drawn as Herdr's own agents list draws it.
  mark(t, i) {
    if (i.status === "blocked" || i.reply) {
      return div().flex_none().px(6).h(18).rounded(5).bg(alpha(t.blocked, "26")).text_size(10.5).font_semibold().text_color(t.blocked)
        .flex().items_center().child(i.kind ?? "Needs you");
    }
    if (i.status === "working") return svg(`${CHROME}/spinner.svg`).size(12).mx(1).flex_none().text_color(t.working).spin(900);
    if (i.section === "done") return div().size(7).mx(3).flex_none().rounded(4).bg(t.done);
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

  // A small icon button, as Herdr's rows have them (pin, fold).
  iconButton(t, id, icon, tip, act) {
    return div().id(id).flex_none().p(2).rounded(4).text_color(t.dim).cursor_pointer()
      .hover((s) => s.bg(t.hover).text_color(t.foreground))
      .tooltip(tip)
      .on_click((_e, cx) => {
        cx.stop_propagation();
        act(cx);
        cx.notify();
      })
      .child(svg(`${CHROME}/${icon}.svg`).size(13));
  }

  row(t, m, i, keyboard) {
    const shown = inView() === i.address;
    const picked = keyboard && this.sel === i.address;
    const hover = this.hovered === i.address;
    const button = i.section === "settled"
      ? this.iconButton(t, `unsettle-${i.address}`, "chevron-left", "Unsettle", (cx) => this.unsettle(i.address, cx))
      : this.iconButton(t, `settle-${i.address}`, "check", "Settle", (cx) => this.settle(i, this.model(cx), cx));
    const when = i.section === "settled" ? i.settled.ms : i.since;
    const time = `${duration(m.now - when)}${i.approx && i.section !== "settled" ? "+" : ""}`;
    const quiet = i.section === "idle" || i.section === "settled";
    // The Chief of Staff runs the same program as the issue agents: its own glyph tells it apart.
    const pic = i.talks ? svg("mods/inbox/chief-of-staff.svg").size(16).flex_none().text_color(t.accent) : icon(i.program, 16, t.foreground);
    // Only what's unusual: a named tab, a branch other than main/master, the agent's message.
    const branch = i.branch === "main" || i.branch === "master" ? null : i.branch;
    const tail = [i.tab !== i.name && !/^\d+$/.test(i.tab) ? i.tab : null, branch, i.msg].filter(Boolean).join(" · ");
    // Not when one machine is picked, nor in a board lane that already names it.
    const machine = m.picked?.length !== 1 && !this.inLane;
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
      .on_hover((on, cx) => {
        if (on) this.hovered = i.address;
        else if (this.hovered === i.address) this.hovered = null;
        cx.notify();
      })
      .on_click((_e, cx) => {
        this.sel = i.address;
        this.open(i);
        cx.notify();
      })
      .child(div().w(16).mt(2).flex_none().when(Boolean(pic), (el) => el.child(pic)))
      .child(v_flex().flex_1().min_w(0).gap(1)
        .child(h_flex().h(20).gap(6).items_center()
          .child(div().flex_1().min_w(0).truncate().text_color(quiet ? t.muted : t.foreground).child(i.name))
          .when(hover, (el) => el.child(button))
          .when(!hover, (el) => el.child(div().flex_none().text_size(11).text_color(t.dim).child(time)))
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

  heading(t, label, count, color) {
    return h_flex().px(8).pt(8).h(30).gap(6).flex_none().items_center().text_size(11).font_medium()
      .child(div().text_color(color).child(label))
      .child(div().text_color(t.dim).child(String(count)));
  }

  shelf(t, m, s, keyboard) {
    const all = m[s.id];
    const open = Boolean(this.shelves[s.id]);
    const shown = s.id === "settled" ? all.slice(0, this.settledLimit) : all;
    const head = h_flex()
      .id(`shelf-${s.id}`)
      .px(8)
      .pt(8)
      .h(30)
      .gap(6)
      .flex_none()
      .items_center()
      .text_size(11)
      .font_medium()
      .text_color(t.dim)
      .cursor_pointer()
      .hover((el) => el.text_color(t.foreground))
      .on_click((_e, cx) => {
        this.shelves[s.id] = !open;
        if (s.id === "settled" && open) this.settledLimit = SETTLED_FIRST;
        localStorage.setItem("shelves", JSON.stringify(this.shelves));
        cx.notify();
      })
      .child(div().child(s.label))
      .child(div().child(String(all.length)))
      .child(div().flex_1())
      .child(svg(`${CHROME}/chevron-${open ? "down" : "right"}.svg`).size(11).flex_none());
    if (!open) return [head];
    const rows = shown.map((i) => this.row(t, m, i, keyboard));
    if (s.id === "settled" && all.length > shown.length) {
      rows.push(div().id("settled-more").px(8).h(26).flex_none().text_size(12).text_color(t.dim).cursor_pointer()
        .hover((el) => el.text_color(t.foreground))
        .on_click((_e, cx) => {
          this.settledLimit += SETTLED_MORE;
          cx.notify();
        })
        .child(`Show ${Math.min(SETTLED_MORE, all.length - shown.length)} more`));
    }
    return [head, ...rows];
  }

  // The board's lanes: one per machine (unless turned off), each with the five columns.
  lanesOf(m) {
    const cols = (items) => BOARD_COLUMNS.map((c) => ({ ...c, items: c.id === "settled" ? items.settled.slice(0, SETTLED_MORE) : items[c.id] }));
    const pick = (fn) => ({ needs: m.needs.filter(fn), done: m.done.filter(fn), working: m.working.filter(fn), idle: m.idle.filter(fn), settled: m.settled.filter(fn) });
    if (!this.lanes || m.machines.length < 2) return [{ name: null, away: false, cols: cols(pick(() => true)) }];
    return m.machines.filter((x) => !m.picked || m.picked.includes(x.name))
      .map((x) => ({ name: x.name, away: x.away, cols: cols(pick((i) => i.machine === x.name)) }));
  }

  boardKey(e, cx) {
    if (e.modifiers.control || e.modifiers.alt || e.modifiers.platform) return;
    const m = this.model(cx), lanes = this.lanesOf(m), k = e.key;
    let li = 0, ci = 0, ri = 0;
    lanes.forEach((l, a) => l.cols.forEach((c, b) => c.items.forEach((i, r) => { if (i.address === this.sel) { li = a; ci = b; ri = r; } })));
    const col = (a, b) => lanes[a]?.cols[b]?.items ?? [];
    if (k === "h" || k === "left") { ci = Math.max(0, ci - 1); ri = Math.min(ri, Math.max(0, col(li, ci).length - 1)); }
    else if (k === "l" || k === "right") { ci = Math.min(BOARD_COLUMNS.length - 1, ci + 1); ri = Math.min(ri, Math.max(0, col(li, ci).length - 1)); }
    else if (k === "j" || k === "down") { if (ri + 1 < col(li, ci).length) ri++; else if (li + 1 < lanes.length) { li++; ri = 0; } }
    else if (k === "k" || k === "up") { if (ri > 0) ri--; else if (li > 0) { li--; ri = Math.max(0, col(li, ci).length - 1); } }
    else if (k === "enter") { const i = col(li, ci)[ri]; if (i) this.open(i); }
    else if (k === "d") {
      const i = col(li, ci)[ri];
      if (i && i.section !== "settled") { this.write(i.address, `${Date.now()}:${i.turn}`, cx); this.lastSettled = i.address; }
    } else if (k === "u") { if (this.lastSettled) this.unsettle(this.lastSettled, cx); this.lastSettled = null; }
    else if (k === "s") { this.lanes = !this.lanes; localStorage.setItem("lanes", JSON.stringify(this.lanes)); }
    else if (k === "escape") return closeOverlay(props().overlay);
    else return;
    this.sel = col(li, ci)[ri]?.address ?? this.sel;
    cx.stop_propagation();
    cx.notify();
  }

  renderBoard(t, m) {
    const keyboard = this.handle.is_focused();
    const lanes = this.lanesOf(m);
    this.inLane = lanes.length > 1 || Boolean(lanes[0]?.name);
    const color = { needs: t.blocked, done: t.done ?? t.foreground, working: t.working, idle: t.dim, settled: t.dim };
    const header = h_flex().h(44).px(16).gap(10).flex_none().items_center().border_b(1).border_color(t.border)
      .child(div().text_size(14).font_semibold().text_color(t.foreground).child("Agents"))
      .child(div().text_size(12).text_color(t.dim).child(`${m.needs.length} need you · ${m.done.length} done`))
      .child(div().flex_1())
      .when(m.machines.length > 1, (el) => el.child(div().id("fq-lanes").px(8).h(24).flex().items_center().rounded(6).text_size(12).cursor_pointer()
        .bg(this.lanes ? t.selected : t.raised).text_color(this.lanes ? t.foreground : t.muted).hover((s) => s.bg(t.hover))
        .on_click((_e, cx) => { this.lanes = !this.lanes; localStorage.setItem("lanes", JSON.stringify(this.lanes)); cx.notify(); })
        .child("By machine")))
      .child(div().id("fq-board-close").p(4).rounded(4).text_color(t.dim).cursor_pointer().hover((s) => s.bg(t.hover).text_color(t.foreground))
        .on_click(() => closeOverlay(props().overlay)).child(svg(`${CHROME}/x.svg`).size(14)));
    const titles = h_flex().px(16).pt(10).pb(4).gap(10).flex_none()
      .children(BOARD_COLUMNS.map((c) => h_flex().flex_1().min_w(0).gap(6).items_center().text_size(11.5).font_medium()
        .child(div().text_color(color[c.id]).child(c.label))
        .child(div().text_color(t.dim).child(String(c.id === "settled" ? m.settled.length : m[c.id].length)))));
    const lane = (l) => v_flex().gap(6).flex_none()
      .when(Boolean(l.name), (el) => el.child(h_flex().px(16).pt(10).gap(7).items_center().text_size(12).text_color(t.muted)
        .child(svg(`${CHROME}/${l.name === LOCAL ? "monitor" : "server"}.svg`).size(13).flex_none())
        .child(div().font_medium().child(machineLabel(l.name)))
        .when(l.away, (x) => x.child(div().text_color(t.dim).child("not connected")))))
      .child(h_flex().px(16).gap(10).items_start()
        .children(l.cols.map((c) => v_flex().id(`fq-board-${l.name ?? "all"}-${c.id}`).flex_1().min_w(0).gap(6).p(6).rounded(8).bg(t.surface).border(1).border_color(t.border).min_h(44)
          .children(c.items.map((i) => div().rounded(8).border(1).bg(t.background)
            .border_color(i.section === "needs" ? t.blocked : keyboard && this.sel === i.address ? t.accent : t.border)
            .child(this.row(t, m, i, false)))))));
    return v_flex()
      .id("focus-board")
      .track_focus(this.handle)
      .on_key_down((e, cx) => this.key(e, cx))
      .size_full().min_h(0).bg(t.background).text_size(13)
      .child(header)
      .child(titles)
      .child(v_flex().id("fq-board-lanes").flex_1().min_h(0).pb(16).overflow_y_scrollbar().children(lanes.map(lane)))
      .when(keyboard, (el) => el.child(div().h(22).px(16).flex_none().flex().items_center().text_size(10.5).text_color(t.dim)
        .child("h/l columns · j/k cards · Enter open · d settle · u undo · s by machine · Esc close")));
  }

  render(cx) {
    // Theme colors are "#rrggbb" strings; say so, so element colors type-check.
    const t = /** @type {Record<string, `#${string}`>} */ (theme());
    const m = this.model(cx);
    if (this.board) return this.renderBoard(t, m);
    setAgentOrder([...m.needs, ...m.done, ...m.working].map((i) => i.address));
    const keyboard = this.handle.is_focused();
    if (!m.rows.some((r) => r.address === this.sel)) this.sel = m.rows[0]?.address ?? null;
    const body = [];
    if (this.typing || this.filter) {
      body.push(h_flex().h(28).px(8).gap(7).flex_none().items_center().text_size(12).text_color(this.typing ? t.foreground : t.muted)
        .child(svg(`${CHROME}/search.svg`).size(12).flex_none().text_color(t.dim))
        .child(div().child(`${this.filter}${this.typing ? "▏" : ""}`)));
    }
    if (m.needs.length) body.push(this.heading(t, "Needs you", m.needs.length, t.blocked), ...m.needs.map((i) => this.row(t, m, i, keyboard)));
    if (m.done.length) body.push(this.heading(t, "Done", m.done.length, t.done ?? t.foreground), ...m.done.map((i) => this.row(t, m, i, keyboard)));
    if (!m.queue.length) body.push(div().px(8).py(12).text_size(12).text_color(t.dim).child(this.filter ? "Nothing matches." : "Nothing needs you."));
    body.push(...SHELVES.flatMap((s) => this.shelf(t, m, s, keyboard)));
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
          .child(this.headButton(t, "fq-next", "mods/inbox/next.svg", "Next waiting (ctrl-shift-j)", (cx) => this.next(this.model(cx))))
          // The same queue as a Kanban board (this mod again, as a dialog).
          .child(this.headButton(t, "fq-open-board", `${CHROME}/layers.svg`, "Board (ctrl-shift-u)", () => run("mod.open", { name: "inbox" })))
          .child(this.headButton(t, "fq-kanban-board", "mods/inbox/kanban-board.svg", "Kanban board (ctrl-shift-y)", () => run("mod.open", { name: "kanban-board" })))))
      .when(Boolean(note), (el) => el.child(note))
      .when(this.picker && m.machines.length > 0, (el) => el.child(this.pickerList(t, m, keyboard)))
      .child(v_flex().id("fq-list").flex_1().min_h(0).px(8).pb(14).overflow_y_scrollbar().mode("always").children(body))
      .when(keyboard, (el) => el.child(div().h(24).px(16).flex_none().flex().items_center().text_size(10.5).text_color(t.dim)
        .child(this.picker ? "j/k · Space check · a all · m close" : "j/k · Enter open · d settle · u undo · m machines · / filter · Esc")));
  }
}
