// Inbox: what needs you, longest waiting first, and the way to each project's roles.
// Needs you: blocked agents, and a turn the Chief of Staff or a Chief Engineer ended (a reply).
// Projects: the Chief of Staff and Chief Engineers, idle or working (ADR-0015).
// Issue agents that finished, work or idle are not shown (kanban-board and Spaces show the flow).
// The mod only reads: it writes no label.

import { View, div, svg, image } from "gpui-kit";
import { v_flex, h_flex } from "gpui-base";
import { agents, terminals, spaces, panes, machines, focused, run, theme, setAgentOrder, nextCommand, shared, setShared } from "herdr";
import { displayName } from "./name.js";

const RANK = { blocked: 4, done: 3, working: 2, idle: 1 };
const KIND = { permission: "Permission", question: "Question", auth: "Auth" };
// What every copy must judge alike: when each agent last changed state, and the turn of a
// paneless role's reply the person has opened. localStorage is per copy; shared() is not.
const MEMORY = "inbox.memory";
// Before the mod was renamed from focus-queue (2026-11-04).
const OLD_MEMORY = "focus-queue.memory";
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
// Each of those roles has its own icon (Lucide clipboard-list, drafting-compass); anything else keeps the program's.
export const ROLE_ICON = { "chief-of-staff": "mods/inbox/chief-of-staff.svg", "chief-engineer": "mods/inbox/chief-engineer.svg" };
const ownRow = (k, listed) => Boolean(k.labels?.issue || (k.labels?.project && !listed.has(parentOf(k))));
const urgent = (a, b) => ((RANK[b] ?? 0) > (RANK[a] ?? 0) ? b : a);

// A theme color ("#rrggbb") with an alpha byte added.
const alpha = (color, aa) => /** @type {`#${string}`} */ (`${color}${aa}`);

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
    this.sel = null;
    this.filter = "";
    this.typing = false;
    this.gAt = 0;
    // The machines the queue is narrowed to (null: all), and the picker's state.
    this.picked = load("machines", null);
    this.picker = false;
    this.pickerAt = 0;
    cx.timer.every(30000, (cx) => cx.notify());
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
      // The Chief of Staff and a Chief Engineer talk with the person, so a turn they end is a reply:
      // Needs you while unseen (Herdr's done), Projects once looked at (Herdr turns done into idle by
      // itself). One without a pane stays done: opening it is looking, remembered for that turn.
      const lead = [term, ...kids].find((k) => TALKS_TO_YOU.has(k.labels?.role));
      const talks = Boolean(lead);
      const reply = talks && status === "done" && !(a.paneless && this.seen[a.address] === turn);
      // Anything blocked needs the person; otherwise only the roles show, as the way in.
      const section = status === "blocked" || reply ? "needs" : talks ? "projects" : null;
      const project = lead?.labels?.project ?? term.labels?.project ?? "";
      const at = this.changed[a.address].at;
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
      setShared(MEMORY, { changed: this.changed, seen: this.seen });
    }
    const f = this.filter.toLowerCase();
    const match = (i) => !f || [i.name, i.space, i.tab, i.machine].some((x) => x.toLowerCase().includes(f));
    // Every machine, with how many wait on it and whether it is connected.
    const hosts = [{ name: LOCAL, away: false }, ...machines().filter((x) => x.name).map((x) => ({ name: x.name, away: x.layout === "unavailable" }))];
    const waits = (i) => i.section === "needs";
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
      // By project, then the Chief of Staff before Chief Engineers (by issue).
      projects: pick("projects").sort((x, y) => x.project.localeCompare(y.project)
        || (x.role === "chief-of-staff" ? 0 : 1) - (y.role === "chief-of-staff" ? 0 : 1)
        || x.issue.localeCompare(y.issue) || x.name.localeCompare(y.name)),
    };
    // What next walks: Needs you. What j/k walk: both sections.
    m.queue = m.needs;
    m.rows = [...m.needs, ...m.projects];
    return m;
  }

  open(i) {
    if (i.peek) run("terminal.peek", { terminal: i.peek });
    else run("agent.focus", { terminal: i.address });
    // A paneless role's reply stays done in the core: opening it is the person looking.
    if (i.reply && i.paneless) {
      this.seen[i.address] = i.turn;
      setShared(MEMORY, { changed: this.changed, seen: this.seen });
    }
  }

  // The longest-waiting item that isn't the one in view.
  next(m) {
    const shown = inView();
    const i = m.queue.find((x) => x.address !== shown);
    if (i) this.open(i);
  }

  command(c, cx) {
    const m = this.model();
    if (c === "next") this.next(m);
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

  // The status mark, drawn as Herdr's own agents list draws it.
  mark(t, i) {
    if (i.status === "blocked" || i.reply) {
      return div().flex_none().px(6).h(18).rounded(5).bg(alpha(t.blocked, "26")).text_size(10.5).font_semibold().text_color(t.blocked)
        .flex().items_center().child(i.kind ?? "Needs you");
    }
    if (i.status === "working") return svg(`${CHROME}/spinner.svg`).size(12).mx(1).flex_none().text_color(t.working).spin(900);
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

  heading(t, label, count, color) {
    return h_flex().px(8).pt(8).h(30).gap(6).flex_none().items_center().text_size(11).font_medium()
      .child(div().text_color(color).child(label))
      .child(div().text_color(t.dim).child(String(count)));
  }

  render(cx) {
    // Theme colors are "#rrggbb" strings; say so, so element colors type-check.
    const t = /** @type {Record<string, `#${string}`>} */ (theme());
    const m = this.model();
    setAgentOrder(m.rows.map((i) => i.address));
    const keyboard = this.handle.is_focused();
    if (!m.rows.some((r) => r.address === this.sel)) this.sel = m.rows[0]?.address ?? null;
    const body = [];
    if (this.typing || this.filter) {
      body.push(h_flex().h(28).px(8).gap(7).flex_none().items_center().text_size(12).text_color(this.typing ? t.foreground : t.muted)
        .child(svg(`${CHROME}/search.svg`).size(12).flex_none().text_color(t.dim))
        .child(div().child(`${this.filter}${this.typing ? "▏" : ""}`)));
    }
    if (m.needs.length) body.push(this.heading(t, "Needs you", m.needs.length, t.blocked), ...m.needs.map((i) => this.row(t, m, i, keyboard)));
    else body.push(div().px(8).py(12).text_size(12).text_color(t.dim).child(this.filter ? "Nothing matches." : "Nothing needs you."));
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
          .child(this.headButton(t, "fq-kanban-board", "mods/inbox/kanban-board.svg", "Kanban board (ctrl-shift-y)", () => run("mod.open", { name: "kanban-board" })))))
      .when(Boolean(note), (el) => el.child(note))
      .when(this.picker && m.machines.length > 0, (el) => el.child(this.pickerList(t, m, keyboard)))
      .child(v_flex().id("fq-list").flex_1().min_h(0).px(8).pb(14).overflow_y_scrollbar().mode("always").children(body))
      .when(keyboard, (el) => el.child(div().h(24).px(16).flex_none().flex().items_center().text_size(10.5).text_color(t.dim)
        .child(this.picker ? "j/k · Space check · a all · m close" : "j/k · Enter open · m machines · / filter · Esc")));
  }
}
