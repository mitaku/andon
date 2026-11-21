// Waiting on you: what waits on the owner in the focused pane's project, for the right panel.
// The list is `andon waiting --json` run in the cell (Andon's docs/contract.md §25): the command holds
// the counting rule, and the mod only draws its output in its order (ADR-0042 decisions 12 and 14).
// Its one way to write is `andon waiting answer` on a decision the output marks answerable, run only when
// the person picks an option and then presses the answer button (decisions 17 and 18). The command checks
// the question and writes the answer; the mod holds no rule of what may be answered.

import { View, div, svg } from "gpui-kit";
import { v_flex, h_flex } from "gpui-base";
import { spaces, panes, focused, terminals, exec, run, theme, window } from "herdr";

const CHROME = "mods/workspace-chrome";
export const VERSION = 1;
const POLL_MS = 10000;
const TIMEOUT_MS = 15000;
const KIND = { merge: "Merge", decision: "Decide", task: "Do" };
// A question as the output names it (schema/waiting.schema.json): it starts with a letter or digit, so it is never a flag.
const QUESTION = /^[A-Za-z0-9][A-Za-z0-9._-]*#[1-9][0-9]*$/;

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

// What one run of the command gives: { data } to draw, or { reason } in one line.
export function readOutput(r) {
  if (!r) return { reason: "andon waiting did not run" };
  // 127: the shell found no such command (a cell older than andon waiting).
  if (r.code === 127) return { reason: "This cell has no andon: update the cell's Andon" };
  // 2: andon's usage error. The arguments are fixed, so it is an andon older than `andon waiting`.
  if (r.code === 2) return { reason: "This cell's andon has no waiting command: update the cell's Andon" };
  if (r.code !== 0) return { reason: `andon waiting failed: ${firstLine(r.stderr) || `exit ${r.code}`}` };
  let data;
  try { data = JSON.parse(r.stdout); } catch { return { reason: "andon waiting printed something that is not JSON" }; }
  if (!data || typeof data !== "object") return { reason: "andon waiting printed something that is not JSON" };
  if (data.version !== VERSION) return { reason: `andon waiting speaks version ${data.version}; this pane reads version ${VERSION}` };
  if (!data.slots || !Array.isArray(data.items)) return { reason: "andon waiting printed no slots or items" };
  return { data };
}

// What one run of `andon waiting answer` gives: null when it wrote, else the reason in one line.
export function answerFailure(r) {
  if (!r) return "andon waiting answer did not run";
  if (r.code === 0) return null;
  if (r.code === 127) return "This cell has no andon: update the cell's Andon";
  // 2: andon's usage error. The arguments are fixed, so it is an andon older than `andon waiting answer`.
  if (r.code === 2) return "This cell's andon can't answer from the pane: update the cell's Andon";
  return `Not answered: ${firstLine(r.stderr) || `exit ${r.code}`}`;
}

// Web links open in the person's browser. Only http(s): anything else could name a program.
function openWeb(url) {
  if (!/^https?:\/\/[^\s]+$/i.test(url)) return;
  const platform = window()?.platform;
  exec(platform === "windows" ? "explorer.exe" : platform === "macos" ? "open" : "xdg-open", [url]).catch(() => {});
}

export default class Waiting extends View {
  init(_props, cx) {
    this.where = null;
    this.data = null;
    this.reason = null;
    this.reading = false;
    this.open = new Set();
    this.forget();
    this.refresh(cx);
    cx.timer.every(POLL_MS, (cx) => this.refresh(cx));
  }

  // Runs the command in the focused pane's folder on its machine; a new place drops what was read.
  refresh(cx) {
    const { cwd, on } = location();
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
        got = readOutput(await exec("andon", ["waiting", "--json"], { cwd, on, timeout: TIMEOUT_MS }));
      } catch (e) {
        got = { reason: `Can't run andon waiting: ${firstLine(e?.message ?? e)}` };
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

  // Whether this row takes an answer here: the output says so, and the question and its options are there.
  answerable(i) {
    return i.kind === "decision" && i.answerable === true && QUESTION.test(String(i.question ?? ""))
      && Array.isArray(i.ask?.options) && i.ask.options.length > 0 && !this.answered.has(i.question);
  }

  // The answer button: `andon waiting answer <id>#<n> --option <n>` in the folder and machine the row was read from.
  answer(i, n, cx) {
    const q = i.question, label = i.ask?.options?.[n]?.label, where = this.where;
    if (!this.answerable(i) || !label || !where?.cwd || this.answering.has(q)) return;
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
    const text = `Slots ${s.working}/${s.limit} working · ${s.waiting} waiting on you${s.waiting_full ? " · full" : ""}`;
    return div().id("waiting-slots").px(16).h(26).flex_none().flex().items_center().text_size(11.5)
      .text_color(s.waiting_full ? t.blocked : t.muted).child(text);
  }

  link(t, id, text, act) {
    return div().id(id).px(8).h(22).flex_none().flex().items_center().rounded(6).bg(t.raised).text_size(11.5).text_color(t.foreground)
      .cursor_pointer().hover((s) => s.bg(t.hover))
      .on_click((_e, cx) => { cx.stop_propagation(); act(cx); })
      .child(text);
  }

  // What an opened row shows: the question, facts, reason and options, what waits behind, and the ways out.
  detail(t, i, chief) {
    const ask = i.ask;
    const label = (text) => div().text_size(10.5).font_medium().text_color(t.dim).child(text);
    const line = (text, color = t.foreground) => div().text_size(12).text_color(color).child(String(text));
    const parts = [];
    if (ask) {
      parts.push(div().id(`waiting-question-${i.id}`).text_size(12.5).text_color(t.foreground).child(String(ask.question)));
      if (ask.facts?.length) parts.push(label("Facts"), ...ask.facts.map((f) => line(`· ${f}`, t.muted)));
      if (ask.reason) parts.push(label("Why"), line(ask.reason, t.muted));
      const can = this.answerable(i), picked = this.picked.get(i.question);
      parts.push(label("Options"), ...ask.options.map((o, n) => div().id(`waiting-option-${i.id}-${n}`).text_size(12)
        .text_color(o.pick || picked === n ? t.foreground : t.muted)
        .when(can, (el) => el.px(6).py(2).rounded(6).cursor_pointer().hover((s) => s.bg(t.hover))
          .when(picked === n, (el) => el.bg(t.raised))
          // Picking only marks the option: nothing is written until the answer button.
          .on_click((_e, cx) => { cx.stop_propagation(); this.picked.set(i.question, n); this.failed.delete(i.question); cx.notify(); }))
        .child(`${n + 1}. ${o.label}${o.pick ? " (recommended)" : ""}`)));
    } else if (i.reason) parts.push(line(i.reason, t.muted));
    if (i.question) parts.push(line(`Question ${i.question}`, t.dim));
    // A merge is merged on GitHub. A decision the output doesn't mark answerable (no options, or irreversible),
    // and any answer that isn't one of the options, goes to the Chief of Staff.
    const answered = i.question ? this.answered.get(i.question) : undefined;
    const failed = i.question ? this.failed.get(i.question) : undefined;
    if (answered !== undefined) parts.push(div().id(`waiting-answered-note-${i.id}`).text_size(12).text_color(t.foreground).child(`Answered: ${answered}`));
    else if (this.answerable(i)) parts.push(line("Pick an option, then answer. Anything else, tell the Chief of Staff.", t.dim));
    else if (i.kind === "decision") parts.push(line("Answer it to the Chief of Staff.", t.dim));
    if (failed) parts.push(div().id(`waiting-answer-failed-${i.id}`).text_size(12).text_color(t.blocked).child(failed));
    if (i.holds?.length) parts.push(label("Waiting behind it"), line(i.holds.join(", "), t.muted));
    const ways = [];
    const picked = this.answerable(i) ? this.picked.get(i.question) : undefined;
    if (picked !== undefined && i.ask.options[picked]) {
      const busy = this.answering.has(i.question);
      ways.push(this.link(t, `waiting-answer-${i.id}`, busy ? "Answering…" : `Answer "${i.ask.options[picked].label}"`,
        (cx) => this.answer(i, picked, cx)));
    }
    if (i.kind === "merge" && i.pr) ways.push(this.link(t, `waiting-pr-${i.id}`, "Open PR", () => openWeb(i.pr)));
    if (chief) ways.push(this.link(t, `waiting-cos-${i.id}`, "Go to Chief of Staff", () => run("agent.focus", { terminal: chief.address })));
    return v_flex().id(`waiting-detail-${i.id}`).pl(24).pr(4).pt(4).pb(6).gap(4).children(parts)
      .when(ways.length > 0, (el) => el.child(h_flex().pt(4).gap(6).children(ways)));
  }

  row(t, i, now, chief) {
    const opened = this.open.has(i.id);
    const since = Date.parse(i.since);
    const held = i.holds?.length ?? 0;
    const answered = i.question ? this.answered.get(i.question) : undefined;
    return v_flex().id(`waiting-row-${i.id}`).flex_none().rounded(8).when(opened, (el) => el.bg(t.selected))
      .child(h_flex().id(`waiting-head-${i.id}`).px(8).py(6).gap(8).items_center().rounded(8).cursor_pointer()
        .hover((s) => s.bg(t.hover))
        .on_click((_e, cx) => { this.toggle(i.id); cx.notify(); })
        .child(svg(`${CHROME}/chevron-${opened ? "down" : "right"}.svg`).size(11).flex_none().text_color(t.dim))
        .child(div().id(`waiting-kind-${i.id}`).flex_none().px(6).h(18).flex().items_center().rounded(5).bg(t.raised)
          .text_size(10.5).font_semibold().text_color(t.foreground).child(KIND[i.kind] ?? String(i.kind)))
        .child(v_flex().flex_1().min_w(0)
          .child(div().truncate().text_size(12.5).text_color(t.foreground).child(`${i.id} ${i.title ?? ""}`))
          .child(h_flex().gap(6).text_size(11).text_color(t.dim)
            .child(div().flex_none().child(Number.isNaN(since) ? "" : duration(now - since)))
            .when(held > 0, (el) => el.child(div().flex_none().child(`· ${held} waiting behind`)))
            .when(answered !== undefined, (el) => el.child(div().id(`waiting-answered-${i.id}`).flex_none().text_color(t.foreground)
              .child(`· Answered: ${answered}`))))))
      .when(opened, (el) => el.child(this.detail(t, i, chief)));
  }

  render(cx) {
    const t = /** @type {Record<string, `#${string}`>} */ (theme());
    const { cwd, on } = location();
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
        body.push(v_flex().id("waiting-list").flex_1().min_h(0).px(8).pb(14).gap(2).overflow_y_scrollbar()
          .children(d.items.map((i) => this.row(t, i, now, chief))));
      }
    }
    return v_flex().id("waiting").size_full().min_h(0).bg(t.background).text_size(13)
      .child(h_flex().h(31).pl(16).pr(8).gap(8).flex_none().items_center().justify_between().text_size(11.5).text_color(t.dim)
        .child(div().flex_none().child("Waiting on you"))
        .when(Boolean(d?.project), (el) => el.child(div().min_w(0).truncate().child(String(d.project)))))
      .children(body);
  }
}
