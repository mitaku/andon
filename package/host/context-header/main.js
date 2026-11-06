// Context header: a pane's header that says which work this is and what it waits for.
//   { "tab": "active", "pane": { "header": { "content": { "mod": "context-header" }, "height": 46 } } }
// Line 1: the program's icon, the name (a named tab, else the agent's own title), the status,
// how long it has been in it, and the settled mark. Line 2: machine · space · tab · branch · folder,
// or, while blocked, what the agent asks. Hovering shows split, zoom and close for this pane.
// Props from the app: { pane, terminal }. Click: focus the pane. Nothing is written.

import { View, div, svg, image } from "gpui-kit";
import { v_flex, h_flex } from "gpui-base";
import { panes, terminals, spaces, props, run, theme, home, hc, notify } from "herdr";
import { displayName } from "./name.js";

const CHROME = "mods/workspace-chrome";
const KIND = { permission: "Permission", question: "Question", auth: "Auth" };
const MSG_MAX = 120;

// The program's icon from Herdr's program table: a one-color glyph or a picture.
function icon(p, size, color) {
  if (!p?.icon) return null;
  return p.tint ? svg(p.icon).size(size).flex_none().text_color(color) : image(p.icon).w(size).h(size).flex_none();
}

// Andon's task terminals are named from their labels (Andon's docs/contract.md §8), as the andon
// mod names their Space and tab (ADR-0002, ADR-0003): "<id> <title>", "CE <id> <title>" for a Chief
// Engineer. A reviewer shares its worker's tab, so the tab's name would be the worker's.
function andonName(labels) {
  const title = typeof labels?.title === "string" ? labels.title.replace(/[\s\p{Cc}]+/gu, " ").trim() : "";
  const task = typeof labels?.task === "string" ? labels.task : "";
  if (!labels?.project || !task || !title) return null;
  return labels.role === "chief-engineer" ? `CE ${task} ${title}` : labels.role ? null : `${task} ${title}`;
}

function folder(cwd) {
  if (!cwd) return "";
  const h = home();
  return cwd === h ? "~" : cwd.startsWith(h + "/") || cwd.startsWith(h + "\\") ? "~" + cwd.slice(h.length) : cwd;
}

function duration(ms) {
  const m = Math.floor(ms / 60000);
  if (m < 1) return "<1m";
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  return h < 24 ? `${h}h` : `${Math.floor(h / 24)}d`;
}

const short = (s) => {
  const one = (s ?? "").replace(/\s+/g, " ").trim();
  return one.length > MSG_MAX ? `${one.slice(0, MSG_MAX - 1)}…` : one;
};
const machineOf = (addr) => (addr?.includes("/") ? addr.split("/")[0] : null);
// A theme color ("#rrggbb") with an alpha byte added.
const alpha = (color, aa) => /** @type {`#${string}`} */ (`${color}${aa}`);

export default class ContextHeader extends View {
  init(_props, cx) {
    this.hovered = false;
    // The wait time moves on even when nothing else changes.
    cx.timer.every(30000, (cx) => cx.notify());
  }

  // Everything the header says about this pane.
  facts() {
    const p = panes().find((x) => x.pane === props()?.pane);
    if (!p) return null;
    const terms = terminals();
    const term = terms.find((k) => k.address === p.terminal) ?? {};
    const space = spaces().find((s) => s.tabs.some((tb) => tb.id === p.tab));
    const tab = space?.tabs.find((tb) => tb.id === p.tab);
    const machine = machineOf(p.terminal) ?? (space?.machine || null);
    // Live sub-agents without a pane; their parent is a bare address on their own machine.
    const kids = terms.filter((k) => k.state === "live" && k.parent && (machine && !k.parent.includes("/") ? `${machine}/${k.parent}` : k.parent) === p.terminal && !panes().some((x) => x.terminal === k.address));
    const agent = Boolean(p.status);
    const report = term.report ?? {};
    const labelled = agent ? andonName(term.labels) : null;
    const name = labelled ?? (agent ? displayName({ title: p.title, program: p.program, address: p.terminal }, tab) : p.title || p.program?.name || "shell");
    // Line 2 doesn't say the name again; an Andon task's tab is named after its id, which line 1 has.
    const tabName = tab && !/^\d+$/.test(tab.name) && !labelled ? tab.name : null;
    return {
      p,
      agent,
      name,
      status: p.status,
      // The core's time when it has one (hc ls can leave it out), else when the tab last changed state.
      since: term.status_ms ?? tab?.changed_at ?? null,
      kind: p.status === "blocked" ? KIND[report.kind] ?? null : null,
      msg: p.status === "blocked" ? short(report.msg) : "",
      settled: Boolean(term.labels?.settled),
      turn: term.turn ?? 0,
      kids: kids.length,
      where: [machine ?? "This computer", space?.name, tabName].filter((x) => x && x !== name),
      branch: p.branch && p.branch !== "main" && p.branch !== "master" ? p.branch : null,
      folder: folder(p.cwd),
    };
  }

  button(t, id, name, command, args) {
    return div().id(id).p(3).rounded(4).text_color(t.dim)
      .hover((s) => s.bg(t.hover).text_color(t.foreground))
      .on_click((_e, cx) => {
        cx.stop_propagation();
        run(command, args);
      })
      .child(svg(`mods/pane/${name}.svg`).size(14));
  }

  // Settles this pane's agent: the same `settled=<ms>:<turn>` label inbox writes
  // (ctrl-shift-d there also opens the next one; this button only settles).
  settleButton(t, f) {
    return div().id(`ch-s-${f.p.pane}`).p(3).rounded(4).text_color(t.dim)
      .hover((s) => s.bg(t.hover).text_color(t.foreground))
      .tooltip("Settle (ctrl-shift-d)")
      .on_click((_e, cx) => {
        cx.stop_propagation();
        cx.spawn(async () => {
          const r = await hc(["label", f.p.terminal, `settled=${Date.now()}:${f.turn}`]).catch((e) => ({ code: 1, stdout: "", stderr: String(e) }));
          if (r.code !== 0) notify({ title: "Couldn't settle", body: `${f.p.terminal}: ${(r.stderr || r.stdout || "").trim()}`, level: "error" });
        });
      })
      .child(svg(`${CHROME}/check.svg`).size(14));
  }

  mark(t, f) {
    if (f.status === "blocked") {
      return div().flex_none().px(6).h(18).flex().items_center().rounded(5).bg(alpha(t.blocked, "26")).text_size(10.5).font_semibold().text_color(t.blocked)
        .child(f.kind ?? "Needs you");
    }
    if (f.status === "working") return svg(`${CHROME}/spinner.svg`).size(12).flex_none().text_color(t.working).spin(900);
    if (f.status === "done") return div().size(7).flex_none().rounded(4).bg(t.done);
    if (f.status === "idle") return div().size(7).flex_none().rounded(4).border(1.5).border_color(t.idle ?? t.dim);
    return null;
  }

  render(cx) {
    const t = /** @type {Record<string, `#${string}`>} */ (theme());
    const f = this.facts();
    if (!f) return div();
    const pane = f.p.pane;
    const pic = icon(f.p.program, 14, t.dim);
    const mark = this.mark(t, f);
    const second = f.msg || [...f.where, f.branch, f.folder].filter(Boolean).join(" · ");
    return v_flex()
      .id(`ctx-head-${pane}`)
      .size_full()
      .px(10)
      .justify_center()
      .gap(1)
      .bg(t.background)
      .border_b(1)
      .border_color(t.border)
      .on_hover((hovered, cx) => {
        this.hovered = hovered;
        cx.notify();
      })
      .on_click(() => run("pane.focus", { pane }))
      .child(h_flex().h(20).gap(8).items_center().text_size(12.5)
        .when(Boolean(pic), (el) => el.child(pic))
        .child(div().flex_1().min_w(0).truncate().font_medium().text_color(t.foreground).child(f.name))
        .when(f.kids > 0, (el) => el.child(div().flex_none().text_size(11).text_color(t.dim).child(`${f.kids} sub-agent${f.kids > 1 ? "s" : ""}`)))
        .when(f.settled && f.status !== "blocked" && f.status !== "working", (el) => el.child(div().flex_none().text_size(11).text_color(t.dim).child("settled")))
        .when(Boolean(f.since) && f.agent, (el) => el.child(div().flex_none().text_size(11).text_color(t.dim).child(duration(Date.now() - f.since))))
        .when(Boolean(mark), (el) => el.child(mark))
        .when(this.hovered && f.agent && !f.settled, (el) => el.child(this.settleButton(t, f)))
        .when(this.hovered, (el) => el
          .child(this.button(t, `ch-r-${pane}`, "split-right", "pane.split", { direction: "right", pane }))
          .child(this.button(t, `ch-d-${pane}`, "split-down", "pane.split", { direction: "down", pane }))
          .child(this.button(t, `ch-z-${pane}`, f.p.zoomed ? "unzoom" : "zoom", "pane.zoom", { pane }))
          .child(this.button(t, `ch-x-${pane}`, "close", "pane.close", { pane }))))
      .child(div().h(16).truncate().text_size(11.5).text_color(f.msg ? t.blocked : t.dim).child(second));
  }
}
