// Kanban board: the work ledger in kanban/ (Andon's docs/ledger.md, format 3) as a Jira-like board.
// Reads only: people and agents change the ledger with the `kanban` CLI. Shows the ledger of the
// project the focused pane's folder belongs to, or every project's ledger in the cell, here or on
// another machine, and never shows old contents as new: a band says when it last changed and was
// last checked, and a failed read keeps the last good contents under a red band. Opens as a dialog
// (`mod.open:kanban-board`) or as a tab's pane.

import { View, div, svg } from "gpui-kit";
import { v_flex, h_flex, TextView } from "gpui-base";
import { agents, terminals, panes, spaces, focused, exec, readTextFile, watchFolder, run, theme, props, closeOverlay, window } from "herdr";
import * as fs from "fs/promises";
import { load } from "./vendor/js-yaml/js-yaml.mjs";

const CHROME = "mods/workspace-chrome";
const FORMAT = 3;
const DEFAULT_COLUMNS = ["todo", "queued", "doing", "review", "done"];
const LABEL = { todo: "To do", queued: "Queued", doing: "Doing", review: "Review", done: "Done" };
const KIND = { permission: "Permission", question: "Question", auth: "Auth" };
const POLL_MS = 10000;
const STALE_MS = { here: 30000, remote: 60000 };
const MAX_UP = 12;
// Ledger files are read on the machine they are on, and refused before they are read when they are
// a symbolic link (a reparse point on Windows) or their folder is one, or when they are too large:
// the ledger is written by agents (changed by Andon).
const MAX_TEXT = 256 * 1024;        // one config.yaml or issue.yaml
const MAX_DOC = 1024 * 1024;        // a brief.md or report.md
const MAX_TOTAL = 4 * 1024 * 1024;  // all that one read prints (exec's output stops at 8 MiB)
// emit <dir> <file>: \x1f and why, or the file's text; t counts the bytes printed against $total.
const SH_EMIT = 'emit() { if [ -L "$1" ] || [ -L "$2" ]; then printf "\\037symbolic link\\n"; return; fi; ' +
  '[ -f "$2" ] || { printf "\\037not a file\\n"; return; }; s=$(wc -c < "$2" | tr -d " "); ' +
  'if [ "$s" -gt "$max" ]; then printf "\\037larger than %s bytes\\n" "$max"; return; fi; ' +
  't=$((t + s)); if [ "$t" -gt "$total" ]; then printf "\\037over %s bytes in all\\n" "$total"; return; fi; ' +
  'head -c "$max" "$2"; printf "\\n"; }; ';
// Each issue folder's name after \x1e, then its issue.yaml (or \x1f and why). A fixed script: root, max, total.
export const LIST_ISSUES = SH_EMIT + 'max=$2; total=$3; t=0; [ -L "$1/kanban" ] || [ -L "$1/kanban/issues" ] && exit 0; ' +
  'for d in "$1"/kanban/issues/*/; do n=${d%/}; [ -e "$n/issue.yaml" ] || [ -L "$n/issue.yaml" ] || continue; ' +
  'printf "\\036%s\\n" "${n##*/}"; emit "$n" "$n/issue.yaml"; done';
// Each project's slug after \x1e, then its config.yaml (or \x1f and why). A fixed script: folder, max, total.
export const LIST_PROJECTS = SH_EMIT + 'max=$2; total=$3; t=0; for c in "$1"/projects/*/kanban/config.yaml; do ' +
  '[ -e "$c" ] || [ -L "$c" ] || continue; p=${c%/kanban/config.yaml}; printf "\\036%s\\n" "${p##*/}"; emit "$p/kanban" "$c"; done';
// Pairs of <folder> <file> after max and total: each pair's number after \x1e, then the file (or \x1f and why).
export const READ_FILES = SH_EMIT + 'max=$1; total=$2; t=0; i=0; shift 2; while [ $# -ge 2 ]; do ' +
  'printf "\\036%s\\n" "$i"; emit "$1" "$2"; i=$((i + 1)); shift 2; done';
// Windows: for each path, "none", "link" (a reparse point), "dir" or its length. Paths are arguments.
const PS_ITEMS = "foreach ($p in $args) { $i = Get-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue; " +
  "if (-not $i) { 'none' } elseif ($i.Attributes -band [IO.FileAttributes]::ReparsePoint) { 'link' } elseif ($i.PSIsContainer) { 'dir' } else { [string]$i.Length } }";
// Windows: each issue folder's name, a tab, then "link" or issue.yaml's length. The root is the argument.
const PS_ISSUES = "param($r) $d = Join-Path $r 'kanban\\issues'; $k = Get-Item -LiteralPath (Join-Path $r 'kanban') -Force -ErrorAction SilentlyContinue; " +
  "$t = Get-Item -LiteralPath $d -Force -ErrorAction SilentlyContinue; if (-not $t -or (($k.Attributes -bor $t.Attributes) -band [IO.FileAttributes]::ReparsePoint)) { return }; " +
  "Get-ChildItem -LiteralPath $d -Force -Directory | ForEach-Object { $f = Get-Item -LiteralPath (Join-Path $_.FullName 'issue.yaml') -Force -ErrorAction SilentlyContinue; " +
  "if ($f) { $s = if (($_.Attributes -bor $f.Attributes) -band [IO.FileAttributes]::ReparsePoint) { 'link' } elseif ($f.PSIsContainer) { 'dir' } else { [string]$f.Length }; \"{0}`t{1}\" -f $_.Name, $s } }";
// A PowerShell single-quoted string (', and ‘ ’ ‚ ‛, which also end one, doubled).
const psQuote = (a) => `'${a.replace(/['‘’‚‛]/g, (m) => m + m)}'`;
const ps = (script, ...args) =>
  exec("powershell", ["-NoProfile", "-NonInteractive", "-Command", `& { ${script} } ${args.map(psQuote).join(" ")}`]);
const REPO = /^[^/]+\/[^/]+\/[^/]+$/;
// Flow (ADR-0042 decision 13): owner waits come from `andon waiting --json` (decision 14), which alone
// counts them; the board draws its mark and nothing else. Another version is not guessed at.
const WAITING_VERSION = 1;
const WAIT_LABEL = { merge: "Merge", decision: "Decision", task: "Your task" };
const AMBER = "#F0A202";            // the logo's amber, as the inbox lights what waits on you
const TREE_W = 400;
const STAGE_W = 128;
// Old done issues (ADR-0059 decisions 2–4): a done issue whose `updated` is more than a day ago
// leaves Board's Done column and List for History. Flow keeps what it needs (decision 5).
const RECENT_MS = 24 * 60 * 60 * 1000;
const HISTORY_STEP = 200;           // History shows this many at first and adds this many at a time
const HISTORY_MAX = 2000;           // past this, find older ones with /
const VIEWS = ["board", "list", "flow", "history"];
// What the board reads (ADR-0059 decisions 6, 8, 11): `andon issue board` in the cell gives only the
// issues it needs, `andon issue history` History's (docs/contract.md §29). When one is missing, fails,
// doesn't come back or prints another shape, the fixed scripts read every issue as before.
const BOARD_VERSION = 1;
const BOARD_TIMEOUT = 2000;          // a call not back in this long is given up on, on the mod's clock
const BOARD_RESTS = 5 * 60 * 1000;   // after one that didn't come back, read with the fixed scripts this long
const SLUG = /^[a-z0-9][a-z0-9_-]*$/; // a project name andon takes (schema/project.schema.json)
const ID = /^(?!\.\.?$)[^/\\\x00-\x1f]+$/; // an issue id that names one folder
const HUNG = Symbol("hung");

const alpha = (color, aa) => /** @type {`#${string}`} */ (`${color}${aa}`);
const label = (column) => LABEL[column] ?? column;
const num = (id) => Number(String(id).split("-").pop()) || 0;
const byId = (a, b) => String(a.slug).localeCompare(String(b.slug)) || num(a.id) - num(b.id) || String(a.id).localeCompare(String(b.id));
const short = (repo) => String(repo).split("/").pop();
const slugOf = (root) => root.replace(/\/+$/, "").split("/").pop();
const local = (on) => !on && window()?.platform === "windows";
// An issue's `updated` as milliseconds: kanban writes a quoted time; js-yaml makes an unquoted one a
// Date. NaN when it has none or it doesn't parse: such a done issue counts as old.
const timeOf = (x) => (typeof x?.updated?.getTime === "function" ? x.updated.getTime() : Date.parse(String(x?.updated ?? "")));
// Newest first; issues without a time last.
const newest = (a, b) => (Number.isFinite(timeOf(b)) ? timeOf(b) : -Infinity) - (Number.isFinite(timeOf(a)) ? timeOf(a) : -Infinity) || byId(b, a);
function when(x) {
  const t = timeOf(x);
  if (!Number.isFinite(t)) return "";
  const d = new Date(t), p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

function ago(ms) {
  if (ms == null) return "never";
  const s = Math.floor((Date.now() - ms) / 1000);
  if (s < 10) return "just now";
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  return h < 24 ? `${h}h ago` : `${Math.floor(h / 24)}d ago`;
}

// The folders from `dir` up to the top, at most MAX_UP of them, with "/" separators.
function ancestors(dir) {
  const parts = dir.replace(/\\/g, "/").replace(/\/+$/, "").split("/");
  const out = [];
  for (let i = parts.length; i > 0 && out.length < MAX_UP; i--) {
    const d = parts.slice(0, i).join("/");
    if (d && !/^[A-Za-z]:$/.test(d)) out.push(d);
    else if (/^[A-Za-z]:$/.test(d)) out.push(`${d}/`);
  }
  return out;
}
const join = (dir, name) => `${dir.replace(/\/$/, "")}/${name}`;

// Where the person is: the focused pane's folder and machine (null here).
function location() {
  const f = focused();
  const pane = f?.pane ? panes().find((p) => p.pane === f.pane) : null;
  const space = spaces().find((s) => s.active);
  const term = pane?.terminal ?? f?.terminal ?? null;
  const on = term?.includes("/") ? term.split("/")[0] : space?.machine ?? null;
  return { cwd: pane?.cwd || f?.cwd || space?.folder || null, on };
}

/** @returns {Record<string, any>} */
// Ledger files are written by agents: a short YAML whose aliases nest (`b: [*a, *a]`, again and
// again) would grow without bound when the board copies it. `kanban` writes no aliases, so a file
// with one is refused (shown as unreadable), and so is an oversized file.
const YAML_MAX_BYTES = 256 * 1024;
const YAML_MAX_ALIASES = 0;
function parseYaml(text) {
  if (text.length > YAML_MAX_BYTES) throw new Error(`larger than ${YAML_MAX_BYTES / 1024} KiB`);
  const value = /** @type {any} */ (load(text.replace(/^﻿/, ""), { maxAliases: YAML_MAX_ALIASES }));
  if (value != null && (typeof value !== "object" || Array.isArray(value))) throw new Error("not a mapping");
  return value ?? {};
}

// A config.yaml as the board uses it. Readers stop at a format they don't know (ledger.md).
function readConfig(text) {
  const c = parseYaml(text);
  if (c.version === 2) throw new Error("config.yaml is format 2; move it by hand (docs/operate.md)");
  if (c.version !== FORMAT) throw new Error(c.version == null ? "this ledger is format 1; the board reads format 3 (docs/ledger.md)" : `format ${c.version} is unknown; the board reads format 3`);
  return {
    columns: Array.isArray(c.columns) && c.columns.length ? c.columns.map(String) : DEFAULT_COLUMNS,
    repos: Array.isArray(c.repos) ? c.repos.map(String) : [],
    title: c.title == null ? null : String(c.title),
    chiefOfStaff: c.chief_of_staff == null ? null : String(c.chief_of_staff),
    // Milestones, in order (ledger.md: a list of {id, title}).
    phases: Array.isArray(c.phases) ? c.phases.filter((p) => p && typeof p === "object" && p.id != null)
      .map((p) => ({ id: String(p.id), title: String(p.title ?? p.id) })) : [],
  };
}

// What `andon waiting --json` says about one project, run in the cell with the project's folder as
// the working folder: issue id → kind of owner wait. null when the command is missing, fails or
// prints another version: Flow then shows no marks and the board works as before (decision 14).
export async function readWaiting(root, on) {
  if (local(on)) return null;
  const r = await exec("andon", ["waiting", "--json"], { cwd: root, on }).catch(() => null);
  if (r?.code !== 0) return null;
  try {
    const out = JSON.parse(r.stdout);
    if (out?.version !== WAITING_VERSION || !Array.isArray(out.items)) return null;
    const marks = new Map();
    for (const it of out.items) {
      if (it && typeof it.id === "string" && WAIT_LABEL[it.kind] && !marks.has(it.id)) marks.set(it.id, it.kind);
    }
    return marks;
  } catch { return null; }
}

// The project a folder is in the cell (projects/<slug>/), as andon's --project: with KANBAN_ROOT set in
// the cell, andon would read another project without it (ADR-0059 decision 6). [] elsewhere.
const projectArgs = (root) => {
  const parts = root.replace(/\/+$/, "").split("/");
  return parts.at(-2) === "projects" && SLUG.test(parts.at(-1)) ? ["--project", parts.at(-1)] : [];
};

// A call raced against the mod's own clock: HUNG when it isn't back in BOARD_TIMEOUT (the App is
// also told to stop it then), null when it fails to start.
function within(timer, call) {
  return Promise.race([Promise.resolve(call).catch(() => null), new Promise((resolve) => timer?.after?.(BOARD_TIMEOUT, () => resolve(HUNG)))]);
}

// What `andon issue board --json` or `andon issue history --json` printed for the project at `root`
// (docs/contract.md §29), or null when it can't be used: it failed, isn't JSON, has another version,
// no issues or another project (ADR-0059 decision 11). Unknown keys are left alone.
export function readBoardOutput(r, root) {
  if (!r || r === HUNG || r.code !== 0) return null;
  try {
    const out = JSON.parse(String(r.stdout));
    if (out?.version !== BOARD_VERSION || !Array.isArray(out.issues) || out.project !== slugOf(root)) return null;
    return out;
  } catch { return null; }
}

// The issues a command printed, as the board holds them: an unreadable one keeps its id and why.
// An entry without an id that names one folder is dropped.
function fromBoard(root, issues) {
  const slug = slugOf(root);
  return issues.filter((x) => x && typeof x === "object" && !Array.isArray(x) && typeof x.id === "string" && ID.test(x.id)).map((x) => {
    const at = { id: x.id, slug, root, key: `${slug}/${x.id}` };
    return x.broken != null ? { ...at, broken: String(x.broken).split("\n")[0] } : { ...x, ...at };
  });
}

// fn(x) once per issue key; a call that comes back to an issue still being worked out gets false
// (a cycle), whichever way the result goes.
function memoOf(fn) {
  const m = new Map();
  return (x) => {
    if (!m.has(x.key)) { m.set(x.key, false); m.set(x.key, fn(x)); }
    return m.get(x.key);
  };
}

// Flow's rows (ADR-0042 decision 13): a tree of project (all projects only), milestone (a ledger
// with phases), parent and issue; each issue has its stage on the order axis. Stage 1 waits on
// nothing open; an issue waiting on others sits one right of the furthest of them. Done issues are
// left out, except a done issue whose parent still has open issues under it (shown faded). Within a
// level, what holds an owner wait comes first, then what holds work in progress, then by stage and
// number. `waits` maps "<slug>/<id>" to the kind of owner wait. `visible` picks the issues a
// filter leaves in the tree; stages are counted over all of `tasks`, so a hidden issue still holds
// back what waits on it.
export function flowRows(ledgers, tasks, waits, { all = false, folded = new Set(), visible = () => true } = {}) {
  const lastCol = new Map(ledgers.map((l) => [l.root, l.columns.at(-1) ?? "done"]));
  const done = (x) => x.status === (lastCol.get(x.root) ?? "done");
  const everyKey = new Map(tasks.map((x) => [x.key, x]));
  const waitsOn = (x) => (done(x) ? [] : [].concat(x.after ?? []).map((a) => everyKey.get(`${x.slug}/${a}`)).filter((w) => w && !done(w)));
  // An issue that waits on itself through others is in a cycle: every issue in it is stage 1.
  const inCycle = memoOf((x) => {
    const seen = new Set(), todo = [...waitsOn(x)];
    while (todo.length) {
      const w = todo.pop();
      if (w.key === x.key) return true;
      if (!seen.has(w.key)) { seen.add(w.key); todo.push(...waitsOn(w)); }
    }
    return false;
  });
  const stage = memoOf((x) => (inCycle(x) ? 1 : Math.max(1, ...waitsOn(x).map((w) => stage(w) + 1))));
  const shownTasks = tasks.filter(visible);
  const byKey = new Map(shownTasks.map((x) => [x.key, x]));
  const parentOf = (x) => (x.parent == null ? null : byKey.get(`${x.slug}/${x.parent}`) ?? null);
  const kids = new Map();
  for (const x of shownTasks) {
    const p = parentOf(x);
    if (p && p !== x) kids.set(p.key, [...(kids.get(p.key) ?? []), x]);
  }
  const memo = memoOf;
  const openBelow = memo((x) => (kids.get(x.key) ?? []).some((k) => !done(k) || openBelow(k)));
  const shown = (x) => !done(x) || openBelow(x) || Boolean(parentOf(x) && openBelow(parentOf(x)));
  const holdsWait = memo((x) => waits.has(x.key) || (kids.get(x.key) ?? []).some((k) => shown(k) && holdsWait(k)));
  const holdsWork = memo((x) => (!done(x) && (x.status === "doing" || x.status === "review")) || (kids.get(x.key) ?? []).some((k) => shown(k) && holdsWork(k)));
  const rank = (xs) => (xs.some(holdsWait) ? 0 : xs.some(holdsWork) ? 1 : 2);
  const order = (a, b) => rank([a]) - rank([b]) || stage(a) - stage(b) || num(a.id) - num(b.id) || String(a.id).localeCompare(String(b.id));
  const rows = [];
  const addTask = (x, depth) => {
    const under = (kids.get(x.key) ?? []).filter(shown).sort(order);
    const key = `task:${x.key}`;
    rows.push({ key, kind: "task", depth, task: x, stage: stage(x), faded: done(x), wait: waits.get(x.key) ?? null, foldable: under.length > 0, folded: folded.has(key) });
    if (!folded.has(key)) for (const k of under) addTask(k, depth + 1);
  };
  const group = (key, kind, label, depth, roots) => {
    rows.push({ key, kind, depth, label, foldable: true, folded: folded.has(key) });
    if (!folded.has(key)) for (const x of [...roots].sort(order)) addTask(x, depth + 1);
  };
  const projects = ledgers.map((l) => ({ l, roots: shownTasks.filter((x) => x.root === l.root && !parentOf(x) && shown(x)) }))
    .filter((p) => p.roots.length)
    .sort((a, b) => rank(a.roots) - rank(b.roots) || String(a.l.slug).localeCompare(String(b.l.slug)));
  for (const { l, roots } of projects) {
    let depth = 0;
    if (all) {
      const key = `project:${l.slug}`;
      rows.push({ key, kind: "project", depth, label: l.name, foldable: true, folded: folded.has(key) });
      if (folded.has(key)) continue;
      depth = 1;
    }
    if (!l.phases.length) { for (const x of [...roots].sort(order)) addTask(x, depth); continue; }
    const ids = new Set(l.phases.map((p) => p.id));
    for (const p of l.phases) {
      const inPhase = roots.filter((x) => String(x.phase ?? "") === p.id);
      if (inPhase.length) group(`milestone:${l.slug}/${p.id}`, "milestone", p.title, depth, inPhase);
    }
    const none = roots.filter((x) => !ids.has(String(x.phase ?? "")));
    if (none.length) group(`milestone:${l.slug}/`, "milestone", "No milestone", depth, none);
  }
  return rows;
}

// A kanban/ folder is a ledger only when its config.yaml has a whole-number version (ledger.md,
// "Finding the root"): a product's repository may have a kanban/ folder of its own.
function isLedger(text) {
  try { return Number.isInteger(parseYaml(text).version); } catch { return false; }
}

// Web links open in the person's browser. Only http(s): anything else could name a program.
function openWeb(url) {
  if (!/^https?:\/\/[^\s]+$/i.test(url)) return;
  const platform = window()?.platform;
  exec(platform === "windows" ? "explorer.exe" : platform === "macos" ? "open" : "xdg-open", [url]).catch(() => {});
}

// Every ledger `projects/*/kanban/config.yaml` under a folder, as { root, slug, text }.
async function projectsUnder(dir, on) {
  const out = [];
  if (!local(on)) {
    const r = await exec("sh", ["-c", LIST_PROJECTS, "kanban-board", dir, String(MAX_TEXT), String(MAX_TOTAL)], { on }).catch(() => null);
    for (const [slug, v] of r?.code === 0 ? chunks(r.stdout) : []) {
      if (v.text !== undefined && isLedger(v.text)) out.push({ root: join(join(dir, "projects"), slug), slug, text: v.text });
    }
    return out;
  }
  const base = join(dir, "projects");
  const slugs = (await fs.readdir(base).catch(() => [])).sort();
  if (!slugs.length) return out;
  const got = await readChecked(slugs.map((slug) => [`${base}/${slug}/kanban`, `${base}/${slug}/kanban/config.yaml`]), MAX_TEXT, on).catch(() => []);
  slugs.forEach((slug, i) => {
    const text = got[i]?.text;
    if (text !== undefined && isLedger(text)) out.push({ root: `${base}/${slug}`, slug, text });
  });
  return out;
}

// What the fixed scripts print: [key, { text } | { error }] for each \x1e chunk.
function chunks(stdout) {
  return stdout.split("\x1e").slice(1).map((chunk) => {
    const nl = chunk.indexOf("\n"), body = chunk.slice(nl + 1);
    return [chunk.slice(0, nl), body.startsWith("\x1f") ? { error: body.slice(1).trim() } : { text: body }];
  });
}

// Windows: a file's state as PS_ITEMS gives it, turned into what to do. Lengths count against `left`.
function verdict(dir, file, max, left) {
  if (dir === "link" || file === "link") return { error: "symbolic link" };
  if (file === "none") return null;
  if (file === "dir" || !/^\d+$/.test(file ?? "")) return { error: "not a file" };
  const size = Number(file);
  if (size > max) return { error: `larger than ${max} bytes` };
  if (size > left.bytes) return { error: `over ${MAX_TOTAL} bytes in all` };
  left.bytes -= size;
  return { read: true };
}

// Reads files as [folder, file] pairs on their machine, each refused before it is read when it or its
// folder is a symbolic link or it is over `max` bytes. Gives { text } | { error } | null (missing) per pair.
async function readChecked(pairs, max, on) {
  if (!local(on)) {
    const r = await exec("sh", ["-c", READ_FILES, "kanban-board", String(max), String(MAX_TOTAL), ...pairs.flat()], { on });
    if (r.code !== 0) throw new Error(r.stderr.trim() || "Cannot read the ledger");
    const got = new Map(chunks(r.stdout));
    return pairs.map((_, i) => {
      const v = got.get(String(i));
      return v?.error === "not a file" ? null : v ?? null;
    });
  }
  const r = await ps(PS_ITEMS, ...pairs.flat());
  if (r.code !== 0) throw new Error(r.stderr.trim() || "Cannot read the ledger");
  const states = r.stdout.split(/\r?\n/).map((x) => x.trim()).filter(Boolean);
  const left = { bytes: MAX_TOTAL };
  const out = [];
  for (let i = 0; i < pairs.length; i++) {
    const v = verdict(states[2 * i], states[2 * i + 1], max, left);
    if (v?.read) {
      const text = await readTextFile(pairs[i][1]).catch(() => null);
      out.push(text === null ? null : { text });
    } else out.push(v);
  }
  return out;
}

// Whether a repos entry names this main checkout: its path ends with /<host>/<owner>/<repo>.
function names(entry, checkout, on) {
  if (!REPO.test(entry)) return false;
  const fold = (s) => (local(on) ? s.toLowerCase() : s);
  return fold(checkout.replace(/\\/g, "/").replace(/\/+$/, "")).endsWith(fold(`/${entry}`));
}

export default class KanbanBoard extends View {
  init(_props, cx) {
    this.handle = cx.focus_handle();
    this.timer = cx.timer;   // the mod's clock for calls to andon
    // As a dialog, take the keyboard when it opens (focus can't move during a render).
    if (props()?.overlay) cx.timer.after(30, () => this.handle.focus());
    this.view = "board";     // "board", "list", "flow" or "history"
    this.historyLimit = HISTORY_STEP; // how many of the newest done issues History shows
    this.folded = new Set(); // Flow's folded rows, by key
    this.filter = "";
    this.typing = false;
    this.pick = null;       // a repo (one project) or a project's slug (the cell) to show alone
    this.openKey = null;
    this.detail = null;
    this.selCol = 0;
    this.selRow = 0;
    this.reset();
    cx.timer.every(POLL_MS, (cx) => this.refresh(cx));
    cx.timer.every(15000, (cx) => cx.notify()); // the band's "ago" moves on
  }

  reset() {
    this.where = null;      // { key, cwd, on }
    this.root = null;       // the project's folder (holding kanban/), or null
    this.plant = null;       // the folder holding projects/*/kanban/, or null
    this.all = false;       // every project in the cell, not only this one
    this.lookError = null;  // finding the ledger failed (two projects claim the repo)
    this.searched = false;
    this.data = null;       // { ledgers, tasks, columns, unreadable } as last read well
    this.signature = null;
    this.changedAt = null;
    this.checkedAt = null;
    this.failure = null;    // { message, at }
    this.reading = false;
    this.waits = new Map(); // Flow's owner-wait marks: "<slug>/<id>" → merge | decision | task
    this.waitReading = false;
    this.watched = new Set();
    this.hist = null;       // History from andon issue history: { key, match, got: root → { tasks, more } }
    this.histReading = false;
    this.hung ??= new Map(); // JSON [on, root] → when `andon issue board` last didn't come back (kept across places)
    this.scope = 0;         // counts switches between this project and all, so a late read is dropped
  }

  // The project's ledger, the way kanban finds it: up from the folder, else the project whose repos
  // name the main checkout. Then the cell: the nearest folder above with projects/*/kanban/.
  async find(cwd, on) {
    for (const d of ancestors(cwd)) {
      const [got] = await readChecked([[join(d, "kanban"), join(d, "kanban/config.yaml")]], MAX_TEXT, on).catch(() => [null]);
      const text = got?.text;
      if (text !== undefined && isLedger(text)) { this.root = d.replace(/\/$/, ""); break; }
    }
    if (!this.root) {
      const r = await exec("git", ["rev-parse", "--path-format=absolute", "--git-common-dir"], { cwd, on }).catch(() => null);
      const common = r?.code === 0 ? r.stdout.trim().replace(/\\/g, "/") : "";
      const main = common.replace(/\/[^/]*$/, "");
      if (main) for (const d of ancestors(main)) {
        const claim = (await projectsUnder(d, on)).filter((p) => {
          try { return parseYaml(p.text).repos?.some?.((e) => names(String(e), main, on)) ?? false; } catch { return false; }
        });
        if (claim.length > 1) { this.lookError = `${claim.map((p) => `projects/${p.slug}`).join(", ")} all list this repository; set KANBAN_ROOT`; return; }
        if (claim.length === 1) { this.root = claim[0].root; this.plant = d.replace(/\/$/, ""); break; }
      }
    }
    if (!this.plant) for (const d of ancestors(this.root ?? cwd)) {
      if ((await projectsUnder(d, on)).length) { this.plant = d.replace(/\/$/, ""); break; }
    }
    // In the cell but in no project (the cell's own folder): show every project.
    if (!this.root && this.plant) this.all = true;
  }

  // Every issue's text, by folder name.
  async readIssues(root, on) {
    const texts = new Map();
    if (!local(on)) {
      const r = await exec("sh", ["-c", LIST_ISSUES, "kanban-board", root, String(MAX_TEXT), String(MAX_TOTAL)], { on });
      if (r.code !== 0) throw new Error(r.stderr.trim() || "Cannot list kanban/issues");
      for (const [id, v] of chunks(r.stdout)) texts.set(id, v);
      return texts;
    }
    const r = await ps(PS_ISSUES, root);
    if (r.code !== 0) throw new Error(r.stderr.trim() || "Cannot list kanban/issues");
    const left = { bytes: MAX_TOTAL };
    for (const line of r.stdout.split(/\r?\n/).filter((x) => x.includes("\t"))) {
      const [id, state] = line.trim().split("\t");
      const v = verdict("dir", state, MAX_TEXT, left);
      if (!v) continue;
      if (!v.read) { texts.set(id, v); continue; }
      const text = await readTextFile(`${root}/kanban/issues/${id}/issue.yaml`).catch(() => null);
      if (text !== null) texts.set(id, { text });
    }
    return texts;
  }

  // The issues the board needs, from `andon issue board` run in the cell in the project's folder
  // (ADR-0059 decisions 6, 11), or null: then the fixed scripts read every issue. Not on this computer
  // with Windows, and not for BOARD_RESTS after a call that didn't come back. `--since` is a day ago.
  async readBoard(root, on, timer) {
    if (local(on)) return null;
    const key = JSON.stringify([on ?? null, root]);
    if (Date.now() - (this.hung.get(key) ?? -Infinity) < BOARD_RESTS) return null;
    const since = new Date(Date.now() - RECENT_MS).toISOString().replace(/\.\d+Z$/, "Z");
    const started = Date.now();
    const r = await within(timer, exec("andon", ["issue", "board", "--json", "--since", since, ...projectArgs(root)], { cwd: root, on, timeout: BOARD_TIMEOUT }));
    if (r === HUNG || Date.now() - started >= BOARD_TIMEOUT) { this.hung.set(key, Date.now()); return null; }
    this.hung.delete(key);
    return readBoardOutput(r, root);
  }

  // One ledger: its config and tasks. Throws when its config can't be read. `via` says how its issues
  // were read; `older` is how many old done issues andon counted (null: count those held).
  async readLedger(root, on, timer) {
    const [got] = await readChecked([[`${root}/kanban`, `${root}/kanban/config.yaml`]], MAX_TEXT, on);
    if (!got) throw new Error("kanban/config.yaml is gone");
    if (got.error) throw new Error(`kanban/config.yaml: ${got.error}`);
    const config = readConfig(got.text);
    const slug = slugOf(root);
    const board = await this.readBoard(root, on, timer);
    const older = Number.isInteger(board?.older) && board.older >= 0 ? board.older : null;
    const ledger = { root, slug, name: config.title ?? slug, ...config, via: board ? "andon" : "files", older };
    if (board) return { ledger, tasks: fromBoard(root, board.issues) };
    /** @type {Record<string, any>[]} */
    const tasks = [];
    for (const [id, t] of await this.readIssues(root, on)) {
      const at = { id, slug, root, key: `${slug}/${id}` };
      if (t.error) { tasks.push({ ...at, broken: t.error }); continue; }
      try { tasks.push({ ...parseYaml(t.text), ...at }); }
      catch (e) { tasks.push({ ...at, broken: String(e instanceof Error ? e.message : e).split("\n")[0] }); }
    }
    return { ledger, tasks };
  }

  refresh(cx) {
    const { cwd, on } = location();
    const key = JSON.stringify([cwd, on]);
    if (this.where?.key !== key) {
      this.reset();
      this.where = { key, cwd, on };
      this.openKey = null;
      this.detail = null;
      this.pick = null;
      this.historyLimit = HISTORY_STEP;
    }
    if (!cwd || this.reading) return;
    const where = this.where, scope = this.scope;
    this.reading = true;
    cx.spawn(async (cx) => {
      try {
        if (!this.searched) {
          await this.find(cwd, on);
          this.searched = true;
        }
        if (this.where !== where || (!this.root && !this.plant)) return;
        const all = this.all && this.plant;
        const roots = all ? (await projectsUnder(this.plant, on)).map((p) => p.root) : [this.root];
        const ledgers = [], tasks = [], unreadable = [];
        for (const root of roots) {
          try {
            const got = await this.readLedger(root, on, cx.timer ?? this.timer);
            ledgers.push(got.ledger);
            tasks.push(...got.tasks);
          } catch (e) {
            // One project's mistake doesn't hide the others; alone, it is the band's failure.
            if (!all) throw e;
            unreadable.push(`${slugOf(root)} (${String(e instanceof Error ? e.message : e).split("\n")[0]})`);
          }
        }
        if (!on) for (const root of roots) this.watch(root, cx);
        tasks.sort(byId);
        if (this.where !== where || this.scope !== scope) return;
        const same = ledgers.every((l) => JSON.stringify(l.columns) === JSON.stringify(ledgers[0].columns));
        const columns = ledgers.length && same ? ledgers[0].columns : DEFAULT_COLUMNS;
        const signature = JSON.stringify([ledgers, tasks, unreadable]);
        if (signature !== this.signature) {
          this.signature = signature;
          this.changedAt = Date.now();
          this.data = { ledgers, tasks, columns, unreadable };
        }
        this.checkedAt = Date.now();
        this.failure = null;
        this.readWaits(ledgers, on, cx);
        if (this.view === "history") this.readHistory(cx, true);
      } catch (e) {
        if (this.where === where && this.scope === scope) this.failure = { message: String(e instanceof Error ? e.message : e).split("\n")[0], at: Date.now() };
      } finally {
        this.reading = false;
        // Switched between this project and all of them while reading: read the other now.
        if (this.where === where && this.scope !== scope) this.refresh(cx);
        cx.notify();
      }
    });
  }

  // Flow's owner-wait marks: one `andon waiting --json` per project shown (decision 14), after each
  // good read. Apart from reading the ledger, so a command that hangs stops only the marks.
  readWaits(ledgers, on, cx) {
    if (this.waitReading) return;
    const where = this.where, scope = this.scope;
    this.waitReading = true;
    cx.spawn(async (cx) => {
      try {
        const waits = new Map();
        for (const l of ledgers) {
          for (const [id, kind] of (await readWaiting(l.root, on)) ?? []) waits.set(`${l.slug}/${id}`, kind);
        }
        if (this.where === where && this.scope === scope) this.waits = waits;
      } finally {
        this.waitReading = false;
        cx.notify();
      }
    });
  }

  // What History asks andon for (ADR-0059 decision 8): the projects read through `andon issue board`,
  // the number shown and the `/` text after Enter (while it is typed, the last one asked). null when
  // History isn't shown or every project was read with the fixed scripts.
  historyKey() {
    const roots = (this.data?.ledgers ?? []).filter((l) => l.via === "andon" && (!this.all || !this.pick || l.slug === this.pick)).map((l) => l.root);
    if (this.view !== "history" || !roots.length) return null;
    return JSON.stringify([this.scope, roots, this.historyLimit, this.typing ? this.hist?.match ?? "" : this.filter]);
  }

  // History's rows from `andon issue history`, once per project, when what it asks changed (or `again`,
  // at each read of the ledger while it is shown). The text goes as one argument, never through a shell;
  // a project whose name holds it is asked without it, as every issue of it matches. A project whose
  // call can't be used shows the done issues held.
  readHistory(cx, again = false) {
    const key = this.historyKey();
    if (!key || !cx?.spawn || this.histReading || (!again && this.hist?.key === key)) return;
    const [scope, roots, limit, match] = JSON.parse(key);
    const where = this.where, on = where?.on, timer = cx.timer ?? this.timer;
    this.histReading = true;
    cx.spawn(async (cx) => {
      try {
        const got = new Map();
        for (const root of roots) {
          const text = match && !slugOf(root).toLowerCase().includes(match.toLowerCase()) ? ["--match", match] : [];
          const r = await within(timer, exec("andon", ["issue", "history", "--json", "--limit", String(limit), ...text, ...projectArgs(root)], { cwd: root, on, timeout: BOARD_TIMEOUT }));
          const out = readBoardOutput(r, root);
          if (out) got.set(root, { tasks: fromBoard(root, out.issues).filter((t) => !t.broken), more: Number.isInteger(out.more) && out.more > 0 ? out.more : 0 });
        }
        if (this.where === where && this.scope === scope) this.hist = { key, match, got };
      } finally {
        this.histReading = false;
        // What History asks changed while it was read (Load older, Enter): ask again.
        if (this.where === where && this.scope === scope) this.readHistory(cx);
        cx.notify();
      }
    });
  }

  // On this computer, read again as soon as anything under a ledger changes.
  watch(root, cx) {
    if (this.watched.has(root)) return;
    this.watched.add(root);
    const where = this.where;
    cx.spawn(async (cx) => {
      try {
        while (this.where === where) {
          await watchFolder(`${root}/kanban`);
          if (this.where === where) this.refresh(cx);
        }
      } catch {}
    });
  }

  // Between this project and every project in the cell.
  setAll(all, cx) {
    if (!this.plant || (!all && !this.root) || this.all === all) return;
    this.all = all;
    this.scope++;
    this.pick = null; this.openKey = null; this.detail = null; this.historyLimit = HISTORY_STEP;
    this.data = null; this.signature = null; this.changedAt = null; this.checkedAt = null; this.failure = null;
    this.waits = new Map(); this.hist = null;
    this.refresh(cx);
  }

  // brief.md and report.md, read when a card opens.
  openTask(task, cx) {
    this.openKey = task.key;
    this.detail = { key: task.key, brief: null, report: null, library: [], loading: true };
    const on = this.where?.on, detail = this.detail;
    cx.spawn(async (cx) => {
      const base = `${task.root}/kanban/issues/${task.id}`;
      const [brief, report] = await readChecked([[base, `${base}/brief.md`], [base, `${base}/report.md`]], MAX_DOC, on).catch(() => [null, null]);
      const shown = (v) => (v?.error ? `(not shown: ${v.error})` : v?.text ?? null);
      detail.brief = shown(brief);
      detail.report = shown(report);
      const ls = await exec("ls", ["-1", `${base}/library`], { on }).catch(() => null);
      if (ls?.code === 0) detail.library = ls.stdout.split("\n").filter(Boolean);
      else if (!on && (await fs.exists(`${base}/library`).catch(() => false))) detail.library = (await fs.readdir(`${base}/library`)).sort();
      detail.loading = false;
      cx.notify();
    });
  }

  // The agent a task names, as Herdr lists it: its address on the ledger's machine, or a name.
  agentOf(name, list, terms) {
    if (!name) return null;
    name = String(name);
    const on = this.where?.on;
    const address = on && !name.includes("/") ? `${on}/${name}` : name;
    const named = terms.find((k) => k.name === name && (on ? k.address.startsWith(`${on}/`) : !k.address.includes("/")));
    return list.find((a) => a.address === address) ?? list.find((a) => a.address === named?.address)
      ?? list.find((a) => a.tab_name === name) ?? { address, status: null, missing: true };
  }

  // What to tell cards apart by: the project in the cell, the repo when a project has several.
  placeOf(task) {
    if (this.all) return this.ledgerOf(task)?.name ?? task.slug;
    return (this.data?.ledgers[0]?.repos.length ?? 0) > 1 && task.repo ? short(task.repo) : null;
  }

  ledgerOf(task) {
    return this.data?.ledgers.find((l) => l.root === task.root) ?? null;
  }

  // The task's own done: the last column of its ledger.
  done(task) {
    return task.status === (this.ledgerOf(task)?.columns.at(-1) ?? "done");
  }

  // What the chips and the `/` filter leave.
  matches(t) {
    const f = this.filter.toLowerCase();
    return (!this.pick || (this.all ? t.slug === this.pick : t.repo === this.pick))
      && (!f || [t.id, t.title, t.repo, t.slug, t.profile, t.agent].some((x) => String(x ?? "").toLowerCase().includes(f)));
  }

  // A done issue whose `updated` is a day ago or more (ADR-0059 decision 2): History's, not Board's or List's.
  old(task) {
    if (!this.done(task)) return false;
    const t = timeOf(task);
    return !Number.isFinite(t) || t < Date.now() - RECENT_MS;
  }

  // Board's columns and List's issues: old done issues are left out, and the Done column runs newest
  // first (ADR-0059 decision 3). `old` is what the filter leaves of them (List keeps those that head a
  // tree), `older` how many old done issues the shown projects hold, filter or not. `all` is every
  // issue the filter leaves, old or not: Flow's (decision 5).
  columns() {
    const cols = this.data?.columns ?? DEFAULT_COLUMNS;
    const tasks = this.data?.tasks ?? [];
    const shown = tasks.filter((t) => this.matches(t));
    const broken = shown.filter((t) => t.broken);
    const place = (t, c, i) => this.done(t) ? i === cols.length - 1 : t.status === c || (i === 0 && !cols.includes(t.status));
    const recent = shown.filter((t) => !t.broken && !this.old(t));
    return {
      cols: cols.map((c, i) => {
        const here = recent.filter((t) => place(t, c, i));
        return { id: c, tasks: i === cols.length - 1 ? here.sort(newest) : here };
      }),
      broken,
      all: shown,
      old: shown.filter((t) => !t.broken && this.old(t)),
      // andon counted a project's old done issues (ADR-0059 decision 7); read with the fixed scripts, count them here.
      older: (this.data?.ledgers ?? []).reduce((n, l) => n + (l.via === "andon" && Number.isInteger(l.older) ? l.older
        : tasks.filter((t) => t.root === l.root && !t.broken && this.old(t)).length), 0),
    };
  }

  // History (ADR-0059 decision 4): every done issue, newest first, up to historyLimit. While `/` is
  // being typed it narrows the rows shown; after Enter it looks through every done issue: what
  // `andon issue history` found for a project read through andon (decision 8), else every one held.
  history() {
    const tasks = this.data?.tasks ?? [];
    const done = [];
    let further = 0;
    for (const l of this.data?.ledgers ?? []) {
      const h = l.via === "andon" ? this.hist?.got.get(l.root) : null;
      if (h) { done.push(...h.tasks.filter((t) => this.done(t))); further += h.more; }
      else done.push(...tasks.filter((t) => t.root === l.root && !t.broken && this.done(t)));
    }
    done.sort(newest);
    const picked = done.filter((t) => !this.pick || (this.all ? t.slug === this.pick : t.repo === this.pick));
    const found = this.typing ? picked : picked.filter((t) => this.matches(t));
    const rows = found.slice(0, this.historyLimit);
    return { rows: this.typing ? rows.filter((t) => this.matches(t)) : rows, more: found.length - rows.length + further };
  }

  close() {
    const overlay = props()?.overlay;
    if (overlay) closeOverlay(overlay);
  }

  key(e, cx) {
    const k = e.key, ch = e.key_char ?? "";
    if (this.typing) {
      if (k === "escape") { this.typing = false; this.filter = ""; }
      else if (k === "enter") this.typing = false;
      else if (k === "backspace") this.filter = this.filter.slice(0, -1);
      else if (ch && !e.modifiers.control && !e.modifiers.alt && !e.modifiers.platform) this.filter += ch;
      else return;
      cx.stop_propagation(); cx.notify();
      return;
    }
    if (e.modifiers.control || e.modifiers.alt || e.modifiers.platform) return;
    const { cols } = this.columns();
    const col = cols[this.selCol];
    const card = col?.tasks[this.selRow];
    if (k === "h" || k === "left") { this.selCol = Math.max(0, this.selCol - 1); this.selRow = 0; }
    else if (k === "l" || k === "right") { this.selCol = Math.min(cols.length - 1, this.selCol + 1); this.selRow = 0; }
    else if (k === "j" || k === "down") this.selRow = Math.min((col?.tasks.length ?? 1) - 1, this.selRow + 1);
    else if (k === "k" || k === "up") this.selRow = Math.max(0, this.selRow - 1);
    else if (k === "enter") { if (card) this.openTask(card, cx); }
    else if (k === "g") {
      const a = card && this.agentOf(card.agent, agents(), terminals());
      if (a && !a.missing) run("agent.focus", { terminal: a.address });
    } else if (k === "v") this.view = VIEWS[(VIEWS.indexOf(this.view) + 1) % VIEWS.length];
    else if (k === "p") this.setAll(!this.all, cx);
    else if (ch === "/" || k === "/") { this.typing = true; this.filter = ""; }
    else if (k === "escape") { if (this.openKey) { this.openKey = null; this.detail = null; } else if (this.filter) this.filter = ""; else this.close(); }
    else return;
    cx.stop_propagation(); cx.notify();
  }

  mark(t, a) {
    if (!a || a.missing || !a.status) return null;
    if (a.status === "blocked") {
      const kind = KIND[terminals().find((k) => k.address === a.address)?.report?.kind];
      return div().flex_none().px(5).h(16).flex().items_center().rounded(4).bg(alpha(t.blocked, "26")).text_size(10).font_semibold().text_color(t.blocked).child(kind ?? "Needs you");
    }
    if (a.status === "working") return svg(`${CHROME}/spinner.svg`).size(11).flex_none().text_color(t.working).spin(900);
    if (a.status === "done") return div().size(7).flex_none().rounded(4).bg(t.done);
    return div().size(7).flex_none().rounded(4).border(1.5).border_color(t.idle ?? t.dim);
  }

  pill(t, text) {
    return div().flex_none().px(5).h(16).flex().items_center().rounded(4).bg(t.raised).text_size(10.5).text_color(t.muted).child(text);
  }

  card(t, task, picked, list, terms, tasksById) {
    const a = this.agentOf(task.agent, list, terms);
    const waiting = (task.after ?? []).map(String).filter((x) => { const w = tasksById.get(`${task.slug}/${x}`); return !w || !this.done(w); });
    const parent = task.parent ? tasksById.get(`${task.slug}/${task.parent}`)?.title ?? task.parent : null;
    const open = this.openKey === task.key, place = this.placeOf(task);
    return v_flex()
      .id(`wb-card-${task.key}`)
      .p(10).gap(4).flex_none().rounded(8)
      .bg(open ? t.selected : t.background)
      .border(1).border_color(a?.status === "blocked" && !this.done(task) ? t.blocked : picked ? t.accent : t.border)
      .cursor_pointer()
      .hover((s) => s.bg(t.hover))
      .on_click((_e, cx) => { this.openTask(task, cx); cx.notify(); })
      .child(h_flex().gap(6).items_center().text_size(11).text_color(t.dim)
        .child(div().flex_none().font_medium().child(String(task.id)))
        .when(Boolean(place), (el) => el.child(div().flex_none().child(String(place))))
        .when(Boolean(parent), (el) => el.child(div().flex_1().min_w(0).truncate().child(`· ${parent}`))))
      .child(div().text_size(13).text_color(t.foreground).child(String(task.title ?? "(no title)")))
      .when(waiting.length > 0, (el) => el.child(div().text_size(11).text_color(t.muted).child(`after ${waiting.join(", ")}`)))
      .when(Boolean(a) || Boolean(task.profile), (el) => el.child(h_flex().id(`wb-agent-${task.key}`).gap(6).items_center().text_size(11).text_color(a?.missing ? t.dim : t.muted)
        .when(Boolean(a) && !a.missing, (r) => r.cursor_pointer().hover((s) => s.text_color(t.foreground)).on_click((_e, cx) => { cx.stop_propagation(); run("agent.focus", { terminal: a.address }); }))
        .when(Boolean(a && this.mark(t, a)), (r) => r.child(this.mark(t, a)))
        .when(Boolean(a), (r) => r.child(div().min_w(0).truncate().child(a.missing ? `${task.agent} (not running)` : `@${task.agent}`)))
        .when(Boolean(task.profile), (r) => r.child(this.pill(t, String(task.profile))))));
  }

  band(t) {
    const stale = this.checkedAt && Date.now() - this.checkedAt > (this.where?.on ? STALE_MS.remote : STALE_MS.here);
    const base = h_flex().h(26).px(14).gap(10).flex_none().items_center().text_size(11.5).border_b(1).border_color(t.border);
    if (this.failure) {
      return base.bg(alpha(t.blocked, "1f")).text_color(t.blocked)
        .child(div().flex_1().min_w(0).truncate().child(`Can't read the ledger: ${this.failure.message}`))
        .child(div().flex_none().child(this.data ? `showing what was read ${ago(this.checkedAt)}` : "nothing read yet"));
    }
    return base.text_color(stale ? t.blocked : t.dim)
      .child(div().child(`Changed ${ago(this.changedAt)}`))
      .child(div().child(`Checked ${ago(this.checkedAt)}${stale ? " — not checked lately" : ""}`))
      .child(div().flex_1())
      .when(Boolean(this.where?.on), (el) => el.child(div().child(this.where.on)));
  }

  detailView(t, list, terms, tasksById) {
    const task = tasksById.get(this.openKey);
    if (!task) return null;
    const d = this.detail;
    const a = this.agentOf(task.agent, list, terms);
    const field = (k, v) => v == null || (Array.isArray(v) && !v.length) ? null
      : h_flex().gap(8).text_size(12).child(div().w(70).flex_none().text_color(t.dim).child(k)).child(div().flex_1().min_w(0).truncate().text_color(t.foreground).child(Array.isArray(v) ? v.join(", ") : String(v)));
    const md = (id, text) => TextView.markdown(id, text).selectable(true).on_link_click((url) => openWeb(url));
    return v_flex().id("wb-detail").w(440).flex_none().h_full().min_h(0).border_l(1).border_color(t.border).bg(t.surface).overflow_y_scrollbar().p(14).gap(10)
      .child(h_flex().gap(8).items_center()
        .child(div().flex_none().text_size(12).text_color(t.dim).child(String(task.id)))
        .child(div().flex_none().px(6).h(18).flex().items_center().rounded(5).bg(t.raised).text_size(11).text_color(t.muted).child(label(task.status)))
        .child(div().flex_1())
        .child(div().id("wb-detail-close").p(3).rounded(4).text_color(t.dim).cursor_pointer().hover((s) => s.bg(t.hover).text_color(t.foreground))
          .on_click((_e, cx) => { this.openKey = null; this.detail = null; cx.notify(); }).child(svg(`${CHROME}/x.svg`).size(13))))
      .child(div().text_size(15).font_semibold().text_color(t.foreground).child(String(task.title ?? "")))
      .children([field("Project", this.all ? this.ledgerOf(task)?.name ?? task.slug : null), field("Repo", task.repo), field("Profile", task.profile),
        field("Parent", task.parent && `${task.parent} ${tasksById.get(`${task.slug}/${task.parent}`)?.title ?? ""}`),
        field("After", task.after), field("Phase", task.phase), field("Branch", task.branch), field("Worktree", task.worktree),
        field("Updated", task.updated)].filter(Boolean))
      .when(Boolean(a) && !a.missing, (el) => el.child(h_flex().id("wb-detail-agent").gap(6).items_center().px(8).h(26).rounded(6).bg(t.raised).text_size(12).cursor_pointer()
        .hover((s) => s.bg(t.hover)).on_click(() => run("agent.focus", { terminal: a.address }))
        .when(Boolean(this.mark(t, a)), (r) => r.child(this.mark(t, a))).child(div().child(`Go to @${task.agent}`))))
      .when(Boolean(task.links?.length), (el) => el.child(v_flex().gap(2).children(task.links.map((l, i) => div().id(`wb-link-${i}`).text_size(12)
        .text_color(/^https?:/.test(l) ? t.accent : t.muted).when(/^https?:/.test(l), (x) => x.cursor_pointer().on_click(() => openWeb(l))).child(String(l))))))
      .when(Boolean(d?.loading), (el) => el.child(div().text_size(12).text_color(t.dim).child("Loading…")))
      .when(Boolean(d?.brief), (el) => el.child(div().text_size(11).font_medium().text_color(t.dim).child("Brief")).child(md("wb-brief", d.brief)))
      .when(Boolean(d?.report), (el) => el.child(div().text_size(11).font_medium().text_color(t.dim).child("Report")).child(md("wb-report", d.report)))
      .when(Boolean(d?.library?.length), (el) => el.child(div().text_size(11).font_medium().text_color(t.dim).child(`Library (${d.library.length})`))
        .children(d.library.map((n) => div().text_size(12).text_color(t.muted).child(n))));
  }

  listView(t, cols, old, list, terms, tasksById) {
    const order = new Map(cols.map((c, i) => [c.id, i]));
    const all = cols.flatMap((c) => c.tasks);
    // An old done issue stays as the head of a tree while something under it is shown (ADR-0059 decision 3).
    const oldByKey = new Map(old.map((x) => [x.key, x]));
    const seen = new Set(all.map((x) => x.key));
    for (const x of [...all]) {
      for (let p = x; p.parent != null;) {
        p = tasksById.get(`${p.slug}/${p.parent}`);
        if (!p || seen.has(p.key)) break;
        seen.add(p.key);
        if (oldByKey.has(p.key)) all.push(p);
      }
    }
    const roots = all.filter((x) => !x.parent || !tasksById.has(`${x.slug}/${x.parent}`));
    const kids = (task) => all.filter((x) => x.slug === task.slug && String(x.parent) === String(task.id));
    const rank = (x) => this.done(x) ? cols.length : order.get(x.status) ?? 0;
    const rows = [];
    const add = (task, depth) => {
      const a = this.agentOf(task.agent, list, terms), place = this.placeOf(task);
      rows.push(h_flex().id(`wb-row-${task.key}`).h(30).px(10).pl(10 + depth * 18).gap(10).items_center().rounded(6).flex_none().cursor_pointer()
        .bg(this.openKey === task.key ? t.selected : alpha(t.background, "00")).hover((s) => s.bg(t.hover))
        .on_click((_e, cx) => { this.openTask(task, cx); cx.notify(); })
        .child(div().w(60).flex_none().text_size(11.5).text_color(t.dim).child(String(task.id)))
        .child(div().w(70).flex_none().text_size(11).text_color(t.muted).child(label(task.status)))
        .child(div().flex_1().min_w(0).truncate().text_size(13).text_color(t.foreground).child(String(task.title ?? "")))
        .when(Boolean(place), (el) => el.child(div().flex_none().text_size(11).text_color(t.dim).child(String(place))))
        .when(Boolean(task.profile), (el) => el.child(this.pill(t, String(task.profile))))
        .when(Boolean(a), (el) => el.when(Boolean(this.mark(t, a)), (r) => r.child(this.mark(t, a))).child(div().flex_none().text_size(11).text_color(t.dim).child(`@${task.agent}`))));
      for (const k of kids(task).sort((x, y) => rank(x) - rank(y) || byId(x, y))) add(k, depth + 1);
    };
    for (const r of roots.sort((x, y) => rank(x) - rank(y) || byId(x, y))) add(r, 0);
    return v_flex().id("wb-list").flex_1().min_w(0).min_h(0).p(10).overflow_y_scrollbar().children(rows);
  }

  // History (ADR-0059 decision 4): one row per done issue, newest first; Load older adds 200 up to 2,000.
  historyView(t) {
    const { rows, more } = this.history();
    const line = (task) => h_flex().id(`wb-hist-${task.key}`).h(30).px(10).gap(10).items_center().rounded(6).flex_none().cursor_pointer()
      .bg(this.openKey === task.key ? t.selected : alpha(t.background, "00")).hover((s) => s.bg(t.hover))
      .on_click((_e, cx) => { this.openTask(task, cx); cx.notify(); })
      .child(div().w(60).flex_none().text_size(11.5).text_color(t.dim).child(String(task.id)))
      .child(div().w(110).flex_none().text_size(11).text_color(t.muted).child(when(task)))
      .child(div().flex_1().min_w(0).truncate().text_size(13).text_color(t.foreground).child(String(task.title ?? "(no title)")))
      .when(this.all, (el) => el.child(div().flex_none().text_size(11).text_color(t.dim).child(String(this.ledgerOf(task)?.name ?? task.slug))))
      .when(Boolean(task.profile), (el) => el.child(this.pill(t, String(task.profile))));
    const end = more <= 0 ? null : this.historyLimit < HISTORY_MAX
      ? div().id("wb-load-older").px(10).h(30).flex_none().flex().items_center().rounded(6).text_size(12).text_color(t.accent).cursor_pointer()
        .hover((s) => s.bg(t.hover)).on_click((_e, cx) => { this.historyLimit = Math.min(HISTORY_MAX, this.historyLimit + HISTORY_STEP); cx.notify(); }).child("Load older")
      : div().id("wb-history-end").px(10).h(30).flex_none().flex().items_center().text_size(12).text_color(t.dim)
        .child(`Showing the newest ${HISTORY_MAX}. Find older ones with /`);
    return v_flex().id("wb-history").flex_1().min_w(0).min_h(0).p(10).overflow_y_scrollbar()
      .when(rows.length === 0, (el) => el.child(div().p(10).text_size(13).text_color(t.dim).child(this.filter ? "Nothing done matches." : "Nothing done yet.")))
      .children(rows.map(line))
      .when(Boolean(end), (el) => el.child(end));
  }

  // Flow (ADR-0042 decision 13): the tree on the left, a bar per issue on the order axis on the right.
  flowView(t) {
    const shown = new Set(this.columns().all.map((x) => x.key));
    const tasks = (this.data?.tasks ?? []).filter((x) => !x.broken);
    const rows = flowRows(this.data?.ledgers ?? [], tasks, this.waits, { all: this.all, folded: this.folded, visible: (x) => shown.has(x.key) });
    const stages = Math.max(1, ...rows.map((r) => r.stage ?? 1));
    const tone = { doing: t.working, review: t.accent, done: t.done ?? t.dim };
    const fold = (r) => div().id(`wb-fold-${r.key}`).w(14).flex_none().text_size(10).text_color(t.dim)
      .when(r.foldable, (el) => el.cursor_pointer().hover((s) => s.text_color(t.foreground))
        .on_click((_e, cx) => { cx.stop_propagation(); if (this.folded.has(r.key)) this.folded.delete(r.key); else this.folded.add(r.key); cx.notify(); })
        .child(r.folded ? "▸" : "▾"));
    const bar = (r) => {
      const x = r.task, c = r.faded ? t.dim : tone[x.status] ?? t.dim;
      return h_flex().flex_none().gap(6).items_center()
        .child(div().w((r.stage - 1) * STAGE_W).flex_none())
        .child(div().id(`wb-bar-${x.key}`).w(STAGE_W - 8).h(18).flex_none().flex().items_center().px(6).rounded(5)
          .bg(alpha(c, "2e")).border(1).border_color(c).text_size(10.5).text_color(t.foreground).child(div().min_w(0).truncate().child(label(x.status))))
        .when(Boolean(r.wait), (el) => el.child(div().id(`wb-wait-${x.key}`).flex_none().px(5).h(16).flex().items_center().rounded(4)
          .bg(AMBER).text_size(10).font_semibold().text_color("#000000").child(WAIT_LABEL[r.wait])))
        .when(Boolean(x.not_before) && !r.faded, (el) => el.child(div().flex_none().text_size(10.5).text_color(t.muted).child(`from ${x.not_before}`)));
    };
    const line = (r) => {
      const head = r.kind !== "task";
      const left = h_flex().w(TREE_W).flex_none().min_w(0).pl(10 + r.depth * 16).gap(6).items_center()
        .child(fold(r))
        .when(head, (el) => el.child(div().min_w(0).truncate().text_size(12).font_semibold().text_color(r.kind === "project" ? t.foreground : t.muted).child(String(r.label))))
        .when(!head, (el) => el.child(div().w(60).flex_none().text_size(11.5).text_color(t.dim).child(String(r.task.id)))
          .child(div().flex_1().min_w(0).truncate().text_size(13).text_color(t.foreground).child(String(r.task.title ?? "(no title)"))));
      return h_flex().id(`wb-flow-${r.key}`).h(28).flex_none().items_center().rounded(6)
        .when(!head, (el) => el.cursor_pointer().hover((s) => s.bg(t.hover)).bg(this.openKey === r.task.key ? t.selected : alpha(t.background, "00"))
          .on_click((_e, cx) => { this.openTask(r.task, cx); cx.notify(); }))
        .when(Boolean(r.faded), (el) => el.opacity(0.5))
        .child(left)
        .when(!head, (el) => el.child(bar(r)));
    };
    const axis = h_flex().h(22).flex_none().items_center().text_size(10.5).text_color(t.dim)
      .child(div().w(TREE_W).flex_none().pl(10).child("Issue"))
      .children(Array.from({ length: stages }, (_, i) => div().w(STAGE_W).flex_none().child(String(i + 1))));
    return v_flex().id("wb-flow").flex_1().min_w(0).min_h(0).p(10).overflow_y_scrollbar()
      .child(axis)
      .when(rows.length === 0, (el) => el.child(div().p(10).text_size(13).text_color(t.dim).child("Nothing open.")))
      .children(rows.map(line));
  }

  render(cx) {
    const t = /** @type {Record<string, `#${string}`>} */ (theme());
    const { cwd, on } = location();
    if (this.where?.key !== JSON.stringify([cwd, on])) this.refresh(cx);
    const list = agents(), terms = terminals();
    const { cols, broken, old, older } = this.columns();
    if (this.view === "history") this.readHistory(cx);
    // History's rows from andon too, so a row opens; what the ledger read holds wins.
    const held = [...(this.hist?.got.values() ?? [])].flatMap((h) => h.tasks);
    const tasksById = new Map([...held, ...(this.data?.tasks ?? [])].map((x) => [x.key, x]));
    const keyboard = this.handle.is_focused();
    const ledgers = this.data?.ledgers ?? [];
    const one = !this.all ? ledgers[0] : null;
    // Chips: the projects in the cell, or the repos of a project that has more than one.
    const picks = this.all ? ledgers.map((l) => [l.slug, l.name]) : (one?.repos.length ?? 0) > 1 ? one.repos.map((r) => [r, short(r)]) : [];
    const chief = one?.chiefOfStaff ? this.agentOf(one.chiefOfStaff, list, terms) : null;
    const button = (id, text, active, onClick) => div().id(id).px(8).h(24).flex().items_center().rounded(6).text_size(12).cursor_pointer()
      .bg(active ? t.selected : alpha(t.background, "00")).text_color(active ? t.foreground : t.muted)
      .hover((s) => s.bg(t.hover)).on_click(onClick).child(text);
    const chip = (value, text) => div().id(`wb-pick-${value ?? "all"}`).px(8).h(22).flex().items_center().rounded(11).text_size(11.5).cursor_pointer()
      .bg(this.pick === value ? t.selected : t.raised).text_color(this.pick === value ? t.foreground : t.muted)
      .hover((s) => s.bg(t.hover)).on_click((_e, cx) => { this.pick = value; cx.notify(); }).child(text);
    const header = h_flex().h(44).px(14).gap(10).flex_none().items_center().border_b(1).border_color(t.border)
      .child(div().flex_none().text_size(14).font_semibold().text_color(t.foreground).child("Kanban"))
      .child(div().flex_none().text_size(12).text_color(t.dim).child(this.all ? `${slugOf(this.plant ?? "")} · all projects` : one?.name ?? (this.root ? slugOf(this.root) : "")))
      .child(h_flex().gap(4)
        .child(button("wb-view-board", "Board", this.view === "board", (_e, cx) => { this.view = "board"; cx.notify(); }))
        .child(button("wb-view-list", "List", this.view === "list", (_e, cx) => { this.view = "list"; cx.notify(); }))
        .child(button("wb-view-flow", "Flow", this.view === "flow", (_e, cx) => { this.view = "flow"; cx.notify(); }))
        .child(button("wb-view-history", "History", this.view === "history", (_e, cx) => { this.view = "history"; cx.notify(); })))
      .when(Boolean(this.plant && this.root), (el) => el.child(h_flex().gap(4)
        .child(button("wb-scope-project", "This project", !this.all, (_e, cx) => { this.setAll(false, cx); cx.notify(); }))
        .child(button("wb-scope-all", "All projects", this.all, (_e, cx) => { this.setAll(true, cx); cx.notify(); }))))
      .when(picks.length > 1, (el) => el.child(h_flex().gap(4).child(chip(null, "All")).children(picks.map(([v, text]) => chip(v, text)))))
      .child(div().flex_1())
      .when(this.typing || Boolean(this.filter), (el) => el.child(div().text_size(12).text_color(this.typing ? t.foreground : t.muted).child(`/${this.filter}${this.typing ? "▏" : ""}`)))
      .when(Boolean(chief), (el) => el.child(h_flex().id("wb-chief-of-staff").gap(6).px(8).h(24).items_center().rounded(6).text_size(12)
        .text_color(chief.missing ? t.dim : t.muted)
        .when(!chief.missing, (r) => r.cursor_pointer().hover((s) => s.bg(t.hover).text_color(t.foreground)).on_click(() => run("agent.focus", { terminal: chief.address })))
        .when(Boolean(this.mark(t, chief)), (r) => r.child(this.mark(t, chief)))
        .child(div().child(chief.missing ? `Chief of Staff ${one.chiefOfStaff} (not running)` : "Chief of Staff"))))
      .when(Boolean(props()?.overlay), (el) => el.child(div().id("wb-close").p(4).rounded(4).text_color(t.dim).cursor_pointer()
        .hover((s) => s.bg(t.hover).text_color(t.foreground)).on_click(() => this.close()).child(svg(`${CHROME}/x.svg`).size(14))));
    const empty = (text) => div().p(20).text_size(13).text_color(t.dim).child(text);
    let body;
    if (!cwd) body = empty("No folder in view.");
    else if (!this.searched) body = empty("Looking for kanban/…");
    else if (this.lookError) body = empty(`Can't tell which project this is: ${this.lookError}.`);
    else if (!this.root && !this.plant) body = empty("No project ledger here. Make one in the project's folder with `kanban init --repo <host>/<owner>/<repo>`.");
    else if (!this.data) body = empty(this.failure ? "" : "Reading…");
    else if (this.view === "list") body = this.listView(t, cols, old, list, terms, tasksById);
    else if (this.view === "flow") body = this.flowView(t);
    else if (this.view === "history") body = this.historyView(t);
    else {
      // Under the Done column: how many old done issues there are and the way to History (ADR-0059
      // decision 3). Filtered, the count isn't shown: it is the whole project's.
      const filtered = Boolean(this.pick || this.filter);
      const olderRow = older > 0 ? div().id("wb-older").flex_none().px(4).h(22).flex().items_center().text_size(11.5).text_color(t.muted).cursor_pointer()
        .hover((s) => s.text_color(t.foreground)).on_click((_e, cx) => { this.view = "history"; cx.notify(); })
        .child(filtered ? "Older · History" : `${older} older · History`) : null;
      body = h_flex().id("wb-board").flex_1().min_w(0).h_full().min_h(0).p(12).gap(12).overflow_x_scrollbar()
        .children(cols.map((c, ci) => v_flex().id(`wb-col-${c.id}`).w(270).flex_none().h_full().min_h(0).gap(8).p(8).rounded(10).bg(t.surface)
          .child(h_flex().h(22).gap(6).items_center().text_size(11.5).font_medium().text_color(t.muted)
            .child(div().child(label(c.id).toUpperCase())).child(div().text_color(t.dim).child(String(c.tasks.length))))
          .child(v_flex().id(`wb-cards-${c.id}`).flex_1().gap(8).min_h(0).overflow_y_scrollbar()
            .children(c.tasks.map((task, ri) => this.card(t, task, keyboard && ci === this.selCol && ri === this.selRow, list, terms, tasksById))))
          .when(ci === cols.length - 1 && Boolean(olderRow), (el) => el.child(olderRow))));
    }
    const unreadable = [...broken.map((b) => `${this.all ? `${b.slug}/` : ""}${b.id} (${b.broken})`), ...(this.data?.unreadable ?? [])];
    return v_flex()
      .id("kanban-board")
      .track_focus(this.handle)
      .on_key_down((e, cx) => this.key(e, cx))
      .size_full().min_h(0).bg(t.background).text_color(t.foreground)
      .child(header)
      .when(Boolean(this.root || this.plant) && !this.lookError, (el) => el.child(this.band(t)))
      .when(unreadable.length > 0, (el) => el.child(div().px(14).py(4).text_size(11.5).text_color(t.blocked).child(`Unreadable: ${unreadable.join(", ")}`)))
      .child(h_flex().flex_1().min_h(0).child(body).when(Boolean(this.openKey), (el) => el.child(this.detailView(t, list, terms, tasksById))))
      .when(keyboard, (el) => el.child(div().h(22).px(14).flex_none().flex().items_center().text_size(10.5).text_color(t.dim)
        .child(`h/l columns · j/k cards · Enter open · g go to agent · v board/list/flow/history${this.plant && this.root ? " · p this project/all" : ""} · / filter · Esc close`)));
  }
}
