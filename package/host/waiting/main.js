// Waiting on you: what waits on the owner in the focused pane's project, for the right panel.
// The list is `andon waiting list --json` run in the cell (Andon's docs/contract.md §25): the command holds
// the counting rule, and the mod only draws its output in its order (ADR-0042 decisions 12 and 14). What the
// output leaves for later (`call` false) goes under a "Later" heading, answered the same way (ADR-0062 decision 8).
// Its one way to write is `andon waiting answer` on a decision the output marks answerable, run only when
// the person picks an option and then presses the answer button (decisions 17 and 18). The command checks
// the question and writes the answer; the mod holds no rule of what may be answered.

import { View, div, svg } from "gpui-kit";
import { v_flex, h_flex } from "gpui-base";
import { spaces, panes, focused, terminals, exec, run, theme, window, layout, nextCommand } from "herdr";

const CHROME = "mods/workspace-chrome";
export const VERSION = 1;
const POLL_MS = 10000;
const TIMEOUT_MS = 15000;
const KIND = { merge: "Merge", decision: "Decide", task: "Do" };
// A question as the output names it (schema/waiting.schema.json): it starts with a letter or digit, so it is never a flag.
const QUESTION = /^[A-Za-z0-9][A-Za-z0-9._-]*#[1-9][0-9]*$/;
// The andon layout's right panel, which the inbox shows this block in (layout.show) and this pane hides.
export const PANEL = "panel";

// Whether a layout tree has this mod inside the panel: only then can the pane close it.
function holds(node, under) {
  const here = under || node?.id === PANEL;
  return (here && node?.content?.mod === "waiting") || (Array.isArray(node?.children) && node.children.some((n) => holds(n, here)));
}
export const inPanel = () => holds(layout(), false);

// Where the person is: the focused pane's folder and machine, as Herdr's Files panel finds them.
function location() {
  const space = spaces().find((s) => s.active), f = focused();
  const pane = f?.pane ? panes().find((p) => p.pane === f.pane) : null;
  const on = f?.pane === null ? (f.terminal?.includes("/") ? f.terminal.split("/")[0] : null) : space?.machine ?? null;
  return { cwd: pane?.cwd || f?.cwd || space?.folder || null, on };
}

function duration(ms) {
  const m = Math.floor(ms / 60000);
  if (m < 1) return "<1m";
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  return h < 24 ? `${h}h` : `${Math.floor(h / 24)}d`;
}

const firstLine = (s) => String(s ?? "").trim().split("\n")[0].trim();

// Whether the output left the row for later. A row without `call` (an andon older than the field) calls.
export const isLater = (i) => i?.call === false;

// andon's refusal of a name it doesn't have (an older andon: "知らない", a newer one: "名前が変わった"). The
// arguments are fixed, so the cell's andon and these host parts are from different versions, and reinstalling
// the parts puts the cell's copy here (ADR-0053 decision 9).
const MISMATCH = /知らない|名前が変わった/;
const VERSIONS_DIFFER = "This cell's andon and this pane are different versions: reinstall the host parts";

// What one run of the command gives: { data } to draw, or { reason } in one line.
export function readOutput(r) {
  if (!r) return { reason: "andon waiting list did not run" };
  // 127: the shell found no such command (a cell without andon).
  if (r.code === 127) return { reason: "This cell has no andon: update the cell's Andon" };
  if (r.code === 2 && MISMATCH.test(String(r.stderr ?? ""))) return { reason: VERSIONS_DIFFER };
  if (r.code !== 0) return { reason: `andon waiting list failed: ${firstLine(r.stderr) || `exit ${r.code}`}` };
  let data;
  try { data = JSON.parse(r.stdout); } catch { return { reason: "andon waiting list printed something that is not JSON" }; }
  if (!data || typeof data !== "object") return { reason: "andon waiting list printed something that is not JSON" };
  if (data.version !== VERSION) return { reason: `andon waiting list speaks version ${data.version}; this pane reads version ${VERSION}` };
  if (!data.slots || !Array.isArray(data.items)) return { reason: "andon waiting list printed no slots or items" };
  return { data };
}

// What one run of `andon waiting answer` gives: null when it wrote, else the reason in one line.
export function answerFailure(r) {
  if (!r) return "andon waiting answer did not run";
  if (r.code === 0) return null;
  if (r.code === 127) return "This cell has no andon: update the cell's Andon";
  if (r.code === 2 && MISMATCH.test(String(r.stderr ?? ""))) return VERSIONS_DIFFER;
  return `Not answered: ${firstLine(r.stderr) || `exit ${r.code}`}`;
}

// A web link: http(s) only, anything else could name a program.
const WEB = /^https?:\/\/[^\s]+$/i;

// Web links open in the person's browser.
function openWeb(url) {
  if (!WEB.test(url)) return;
  const platform = window()?.platform;
  exec(platform === "windows" ? "explorer.exe" : platform === "macos" ? "open" : "xdg-open", [url]).catch(() => {});
}

export default class Waiting extends View {
  init(_props, cx) {
    this.handle = cx.focus_handle();
    this.where = null;
    this.data = null;
    this.reason = null;
    this.reading = false;
    this.open = new Set();
    this.forget();
    this.refresh(cx);
    cx.timer.every(POLL_MS, (cx) => this.refresh(cx));
    // pane.go hands the keyboard over with "focus" (mod.json: focusable).
    cx.spawn(async (cx) => {
      for (;;) {
        const c = await nextCommand();
        if (c === "focus" || c === "waiting.focus") this.handle.focus();
        cx.notify();
      }
    });
  }

  // Hides the right panel. The copy stays alive while hidden, so what was read, the open rows, the picks and an
  // answer in flight are there when the inbox shows the panel again. Holding the keyboard, it is first handed to
  // the left, as the inbox's Esc hands it to the right, so it doesn't stay on a hidden pane.
  // Not close(): Herdr calls a View's close(cx) as its cleanup when the block goes and on quit.
  closePanel() {
    if (!inPanel()) return false;
    if (this.handle.is_focused()) run("pane.go", { direction: "left" });
    run("layout.hide", { id: PANEL });
    return true;
  }

  // Keys while the pane holds the keyboard: Esc closes the panel. (key(i) is a row's key.)
  onKey(e, cx) {
    if (e.modifiers?.control || e.modifiers?.alt || e.modifiers?.platform) return;
    if (e.key !== "escape" || !this.closePanel()) return;
    cx.stop_propagation();
    cx.notify();
  }

  // Where the rows are read from: the focused pane's place, except while this pane holds the keyboard. Then the
  // focused pane may be reported otherwise (clicking a row takes the keyboard), and the place stays as it was.
  place() {
    if (this.where && this.handle.is_focused()) return { cwd: this.where.cwd, on: this.where.on };
    return location();
  }

  // Runs the command in the focused pane's folder on its machine; a new place drops what was read.
  refresh(cx) {
    const { cwd, on } = this.place();
    const key = JSON.stringify([cwd, on]);
    if (this.where?.key !== key) {
      this.where = { key, cwd, on };
      this.data = null;
      this.reason = null;
      this.open = new Set();
      this.forget();
    }
    if (!cwd || this.reading) return;
    const where = this.where;
    this.reading = true;
    cx.spawn(async (cx) => {
      let got;
      try {
        // The command and its arguments are fixed: no path and no slug from the screen goes into them.
        got = readOutput(await exec("andon", ["waiting", "list", "--json"], { cwd, on, timeout: TIMEOUT_MS }));
      } catch (e) {
        got = { reason: `Can't run andon waiting list: ${firstLine(e?.message ?? e)}` };
      } finally {
        this.reading = false;
      }
      // The place changed while reading: read the new one now, not at the next tick.
      if (this.where !== where) { this.refresh(cx); return; }
      this.data = got.data ?? null;
      this.reason = got.reason ?? null;
      // An answered question leaves the list at the next read; until then its row stays answered.
      const listed = new Set((this.data?.items ?? []).map((i) => i.question));
      for (const q of this.answered.keys()) if (!listed.has(q)) this.answered.delete(q);
      cx.notify();
    });
  }

  // What the person did on the rows of one place: the picked option, answers in flight, answered, failed.
  forget() {
    this.picked = new Map();
    this.answering = new Set();
    this.answered = new Map();
    this.failed = new Map();
  }

  // A row's key for its element ids and for opening it: a decision by its question (an issue can have two),
  // a merge or a task by its issue (one each).
  key(i) {
    return i.kind === "decision" && QUESTION.test(String(i.question ?? "")) ? i.question.replace("#", "-") : i.id;
  }

  // Whether the row is in what is on screen now, read from where the focus is now: a row drawn from a place
  // the focus has since left is not, even before the pane reads the new place.
  current(i) {
    const { cwd, on } = this.place();
    return this.where?.key === JSON.stringify([cwd, on]) && Array.isArray(this.data?.items) && this.data.items.includes(i);
  }

  // Whether this row takes an answer here: the output says so, and the question and its options are there.
  answerable(i) {
    return i.kind === "decision" && i.answerable === true && QUESTION.test(String(i.question ?? ""))
      && Array.isArray(i.ask?.options) && i.ask.options.length > 0 && !this.answered.has(i.question);
  }

  // The answer button: `andon waiting answer <id>#<n> --option <n>` in the folder and machine the row was read from.
  answer(i, n, cx) {
    const q = i.question, label = i.ask?.options?.[n]?.label, where = this.where;
    if (!this.current(i) || !this.answerable(i) || !label || !where?.cwd || this.answering.has(q)) return;
    this.answering.add(q);
    this.failed.delete(q);
    cx.notify();
    cx.spawn(async (cx) => {
      let reason;
      try {
        // The command and its arguments are fixed: the question comes from the output, the option is a number.
        reason = answerFailure(await exec("andon", ["waiting", "answer", q, "--option", String(n + 1)], { cwd: where.cwd, on: where.on, timeout: TIMEOUT_MS }));
      } catch (e) {
        reason = `Can't run andon waiting answer: ${firstLine(e?.message ?? e)}`;
      } finally {
        this.answering.delete(q);
      }
      // The place changed while answering: what was done there is dropped with it.
      if (this.where !== where) return;
      if (reason) this.failed.set(q, reason);
      else { this.answered.set(q, label); this.picked.delete(q); }
      cx.notify();
    });
  }

  // The project's Chief of Staff, as Herdr lists it: labels role and project, on the ledger's machine.
  chiefOfStaff(project) {
    const on = this.where?.on;
    return terminals().find((k) => k.state === "live" && k.labels?.role === "chief-of-staff" && k.labels?.project === project
      && (on ? k.address.startsWith(`${on}/`) : !k.address.includes("/"))) ?? null;
  }

  toggle(id) {
    if (this.open.has(id)) this.open.delete(id);
    else this.open.add(id);
  }

  slotsLine(t, s) {
    const text = `Slots ${s.working}/${s.limit} working · ${s.waiting} waiting on you`;
    return div().id("waiting-slots").px(16).h(26).flex_none().flex().items_center().text_size(11.5)
      .text_color(t.muted).child(text);
  }

  link(t, id, text, act) {
    return div().id(id).px(8).h(22).flex_none().flex().items_center().rounded(6).bg(t.raised).text_size(11.5).text_color(t.foreground)
      .cursor_pointer().hover((s) => s.bg(t.hover))
      .on_click((_e, cx) => { cx.stop_propagation(); act(cx); })
      .child(text);
  }

  // What an opened row shows: the question, facts, reason and options, what waits behind, and the ways out.
  detail(t, i, chief) {
    const ask = i.ask, k = this.key(i);
    const label = (text) => div().text_size(10.5).font_medium().text_color(t.dim).child(text);
    const line = (text, color = t.foreground) => div().text_size(12).text_color(color).child(String(text));
    const parts = [];
    if (ask) {
      parts.push(div().id(`waiting-question-${k}`).text_size(12.5).text_color(t.foreground).child(String(ask.question)));
      if (ask.facts?.length) parts.push(label("Facts"), ...ask.facts.map((f) => line(`· ${f}`, t.muted)));
      if (ask.reason) parts.push(label("Why"), line(ask.reason, t.muted));
      const can = this.answerable(i), picked = this.picked.get(i.question);
      parts.push(label("Options"), ...ask.options.map((o, n) => div().id(`waiting-option-${k}-${n}`).text_size(12)
        .text_color(o.pick || picked === n ? t.foreground : t.muted)
        .when(can, (el) => el.px(6).py(2).rounded(6).cursor_pointer().hover((s) => s.bg(t.hover))
          .when(picked === n, (el) => el.bg(t.raised))
          // Picking only marks the option: nothing is written until the answer button.
          .on_click((_e, cx) => {
            cx.stop_propagation();
            if (!this.current(i)) return;
            this.picked.set(i.question, n);
            this.failed.delete(i.question);
            cx.notify();
          }))
        .child(`${n + 1}. ${o.label}${o.pick ? " (recommended)" : ""}`)));
    } else if (i.reason) parts.push(line(i.reason, t.muted));
    // A task: the brief's text as the steps, line by line, and the brief's web links, each opening in the browser.
    if (i.kind === "task") {
      const steps = typeof i.steps === "string" ? i.steps.trim() : "";
      if (steps) parts.push(label("Steps"), v_flex().id(`waiting-steps-${k}`).gap(2)
        .children(steps.split("\n").map((l) => line(l.trim() ? l : " "))));
      const links = (Array.isArray(i.links) ? i.links : []).filter((u) => typeof u === "string" && WEB.test(u));
      if (links.length) parts.push(label("Links"), ...links.map((u, n) => div().id(`waiting-link-${k}-${n}`).text_size(12)
        .text_color(t.accent).px(6).py(2).rounded(6).cursor_pointer().hover((s) => s.bg(t.hover))
        .on_click((_e, cx) => { cx.stop_propagation(); openWeb(u); })
        .child(u)));
    }
    // Left for later: why, and the day it calls again when there is one (ADR-0062 decision 8).
    if (isLater(i)) {
      const why = firstLine(i.later), until = typeof i.later_until === "string" ? i.later_until : "";
      parts.push(label("Later"), div().id(`waiting-later-${k}`).text_size(12).text_color(t.muted)
        .child(`${why || "Left for later"}${until ? ` (until ${until})` : ""}`));
    }
    if (i.question) parts.push(line(`Question ${i.question}`, t.dim));
    // A merge is merged on GitHub. A decision the output doesn't mark answerable (no options, or irreversible),
    // and any answer that isn't one of the options, goes to the Chief of Staff.
    const answered = i.question ? this.answered.get(i.question) : undefined;
    const failed = i.question ? this.failed.get(i.question) : undefined;
    if (answered !== undefined) parts.push(div().id(`waiting-answered-note-${k}`).text_size(12).text_color(t.foreground).child(`Answered: ${answered}`));
    else if (this.answerable(i)) parts.push(line("Pick an option, then answer. Anything else, tell the Chief of Staff.", t.dim));
    else if (i.kind === "decision") parts.push(line("Answer it to the Chief of Staff.", t.dim));
    if (failed) parts.push(div().id(`waiting-answer-failed-${k}`).text_size(12).text_color(t.blocked).child(failed));
    if (i.holds?.length) parts.push(label("Waiting behind it"), line(i.holds.join(", "), t.muted));
    const ways = [];
    const picked = this.answerable(i) ? this.picked.get(i.question) : undefined;
    if (picked !== undefined && i.ask.options[picked]) {
      const busy = this.answering.has(i.question);
      ways.push(this.link(t, `waiting-answer-${k}`, busy ? "Answering…" : `Answer "${i.ask.options[picked].label}"`,
        (cx) => this.answer(i, picked, cx)));
    }
    if (i.kind === "merge" && i.pr) ways.push(this.link(t, `waiting-pr-${k}`, "Open PR", () => openWeb(i.pr)));
    if (chief) ways.push(this.link(t, `waiting-cos-${k}`, "Go to Chief of Staff", () => run("agent.focus", { terminal: chief.address })));
    return v_flex().id(`waiting-detail-${k}`).pl(24).pr(4).pt(4).pb(6).gap(4).children(parts)
      .when(ways.length > 0, (el) => el.child(h_flex().pt(4).gap(6).children(ways)));
  }

  row(t, i, now, chief) {
    const k = this.key(i);
    const opened = this.open.has(k);
    const since = Date.parse(i.since);
    const held = i.holds?.length ?? 0;
    const answered = i.question ? this.answered.get(i.question) : undefined;
    return v_flex().id(`waiting-row-${k}`).flex_none().rounded(8).when(opened, (el) => el.bg(t.selected))
      .child(h_flex().id(`waiting-head-${k}`).px(8).py(6).gap(8).items_center().rounded(8).cursor_pointer()
        .hover((s) => s.bg(t.hover))
        .on_click((_e, cx) => { this.toggle(k); cx.notify(); })
        .child(svg(`${CHROME}/chevron-${opened ? "down" : "right"}.svg`).size(11).flex_none().text_color(t.dim))
        .child(div().id(`waiting-kind-${k}`).flex_none().px(6).h(18).flex().items_center().rounded(5).bg(t.raised)
          .text_size(10.5).font_semibold().text_color(t.foreground).child(KIND[i.kind] ?? String(i.kind)))
        .child(v_flex().flex_1().min_w(0)
          .child(div().truncate().text_size(12.5).text_color(t.foreground).child(`${i.id} ${i.title ?? ""}`))
          .child(h_flex().gap(6).text_size(11).text_color(t.dim)
            .child(div().flex_none().child(Number.isNaN(since) ? "" : duration(now - since)))
            .when(held > 0, (el) => el.child(div().flex_none().child(`· ${held} waiting behind`)))
            .when(answered !== undefined, (el) => el.child(div().id(`waiting-answered-${k}`).flex_none().text_color(t.foreground)
              .child(`· Answered: ${answered}`))))))
      .when(opened, (el) => el.child(this.detail(t, i, chief)));
  }

  render(cx) {
    const t = /** @type {Record<string, `#${string}`>} */ (theme());
    const { cwd, on } = this.place();
    if (this.where?.key !== JSON.stringify([cwd, on])) this.refresh(cx);
    const d = this.data;
    const body = [];
    const note = (id, text, color = t.dim) => div().id(id).px(16).py(10).text_size(12).text_color(color).child(text);
    if (!cwd) body.push(note("waiting-note", "No folder in view."));
    else if (this.reason) body.push(note("waiting-reason", this.reason, t.blocked));
    else if (!d) body.push(note("waiting-note", "Reading…"));
    else {
      body.push(this.slotsLine(t, d.slots));
      const chief = this.chiefOfStaff(d.project);
      if (!d.items.length) body.push(note("waiting-empty", "Nothing is waiting on you"));
      else {
        const now = Date.now();
        // What calls you first, in the output's order; what is left for later under its heading, in its order.
        const calling = d.items.filter((i) => !isLater(i)), later = d.items.filter(isLater);
        const rows = calling.length ? calling.map((i) => this.row(t, i, now, chief))
          : [div().id("waiting-none-calling").px(8).py(6).text_size(12).text_color(t.dim).child("Nothing calls you now")];
        if (later.length) rows.push(h_flex().id("waiting-later-heading").px(8).pt(10).pb(2).gap(6).flex_none().items_center()
          .text_size(11).font_medium().text_color(t.dim).child(div().child("Later")).child(div().child(String(later.length))),
        ...later.map((i) => this.row(t, i, now, chief)));
        body.push(v_flex().id("waiting-list").flex_1().min_h(0).px(8).pb(14).gap(2).overflow_y_scrollbar().children(rows));
      }
    }
    const closable = inPanel(), keyboard = this.handle.is_focused();
    return v_flex().id("waiting").track_focus(this.handle).on_key_down((e, cx) => this.onKey(e, cx))
      .size_full().min_h(0).bg(t.background).text_size(13)
      .child(h_flex().h(31).pl(16).pr(8).gap(8).flex_none().items_center().justify_between().text_size(11.5).text_color(t.dim)
        .child(div().flex_none().child("Waiting on you"))
        .child(h_flex().min_w(0).gap(4).items_center()
          .when(Boolean(d?.project), (el) => el.child(div().min_w(0).truncate().child(String(d.project))))
          // The inbox's waiting button shows the panel again.
          .when(closable, (el) => el.child(div().id("waiting-close").size(24).flex_none().flex().items_center().justify_center()
            .rounded(6).cursor_pointer().hover((s) => s.bg(t.hover).text_color(t.foreground)).tooltip("Close (Esc)")
            .on_click((_e, cx) => { cx.stop_propagation(); this.closePanel(); cx.notify(); })
            .child(svg(`${CHROME}/x.svg`).size(13))))))
      .children(body)
      .when(keyboard && closable, (el) => el.child(div().id("waiting-keys").h(24).px(16).flex_none().flex().items_center()
        .text_size(10.5).text_color(t.dim).child("Esc close")));
  }
}
