// model/conform.mjs: モデルの状態の図と、課題の状態の定義（process/states/issue.yaml）を、対応表（issue-map.txt）で
// 2 方向に照らす（ADR-0025 決めたこと 6・受け入れ条件 4・5）。model/check が打つ。Node の標準だけを使う。
//   node conform.mjs <検査器の -dump dot,actionlabels の出力> <issue-map.txt> <issue.yaml>
// 食い違いを全部書いて終了コード 1。合えば 1 行書いて 0。
import { readFileSync } from "node:fs";

const [dotPath, mapPath, yamlPath] = process.argv.slice(2);
const errors = [];
const key = (from, to, cmd) => `${from} → ${to}（${cmd}）`;

// 「andon issue a・b」は a と b の両方（internal/issue/transition.go froms と同じ読み方）
function expand(cmd) {
  if (cmd.startsWith("andon issue ")) {
    return cmd.slice("andon issue ".length).split("・").map((s) => `andon issue ${s.trim()}`);
  }
  return cmd.split("・").map((s) => s.trim());
}

// 状態の図: 状態ごとの status と、移り方（行動の名前つき）。初めの状態は style = filled
const status = new Map();
const initial = [];
const edges = [];
for (const line of readFileSync(dotPath, "utf8").split("\n")) {
  let m = line.match(/^(-?\d+) -> (-?\d+) \[label="([^"]+)"/);
  if (m) {
    edges.push({ from: m[1], to: m[2], action: m[3] });
    continue;
  }
  m = line.match(/^(-?\d+) \[label="(.*)"/);
  if (m) {
    const s = m[2].match(/status = \\"([^\\]+)\\"/);
    if (!s) errors.push(`状態の図の状態 ${m[1]} に status が無い`);
    status.set(m[1], s ? s[1] : "?");
    if (/,style = filled\]$/.test(line)) initial.push(m[1]);
  }
}
if (initial.length === 0 || edges.length === 0) {
  console.error(`FAIL 状態の図が読めない（初めの状態 ${initial.length}・移り方 ${edges.length}）: ${dotPath}`);
  process.exit(1);
}

// 対応表
const actions = new Map();
const outside = new Map();
for (const [i, raw] of readFileSync(mapPath, "utf8").split("\n").entries()) {
  const line = raw.trim();
  if (line === "" || line.startsWith("#")) continue;
  const where = `issue-map.txt:${i + 1}`;
  const f = line.split(/\s+/);
  if (f[0] === "action" && f.length >= 3) {
    const cmd = f.slice(2).join(" ");
    actions.set(f[1], cmd === "-" ? [] : expand(cmd));
  } else if (f[0] === "outside" && f.length >= 4) {
    const [body, reason] = f.slice(3).join(" ").split(" — ");
    if (!reason || !reason.trim()) errors.push(`${where}: outside の行に理由（ — のあと）が無い`);
    outside.set(key(f[1], f[2], body.trim()), where);
  } else {
    errors.push(`${where}: action <行動> <コマンド> か outside <from> <to> <コマンド> — <理由> の形でない`);
  }
}

// issue.yaml の transitions（1 行 1 件の { } の形だけを読む。ほかの形の行があれば落とす）
const allowed = new Set();
let inTransitions = false;
for (const [i, line] of readFileSync(yamlPath, "utf8").split("\n").entries()) {
  if (/^\S/.test(line)) inTransitions = line.startsWith("transitions:");
  if (!inTransitions || !/^\s+-/.test(line)) continue;
  const m = line.match(/^\s+- \{(.*)\}\s*$/);
  if (!m) {
    errors.push(`issue.yaml:${i + 1}: transitions の行が 1 行 1 件の { } の形でない（conform.mjs が読めない）`);
    continue;
  }
  const row = {};
  for (const [, k, q, bare] of m[1].matchAll(/(\w+):\s*(?:"([^"]*)"|([^,]*))\s*(?:,|$)/g)) {
    row[k] = q ?? bare.trim();
  }
  if (!row.from || !row.to) {
    errors.push(`issue.yaml:${i + 1}: from・to が読めない`);
    continue;
  }
  for (const c of row.command ? expand(row.command) : ["-"]) allowed.add(key(row.from, row.to, c));
}

// モデル → issue.yaml
const happens = new Set();
const seen = new Set();
const moves = [
  ...initial.map((s) => ({ action: "Init", from: "[*]", to: status.get(s) })),
  ...edges.map((e) => ({ action: e.action, from: status.get(e.from), to: status.get(e.to) })),
];
for (const mv of moves) {
  seen.add(mv.action);
  const cmds = actions.get(mv.action);
  if (!cmds) {
    if (!errors.some((e) => e.includes(`行動 ${mv.action} が`))) errors.push(`行動 ${mv.action} が対応表（issue-map.txt）に無い`);
    continue;
  }
  if (cmds.length === 0) {
    if (mv.from !== mv.to) errors.push(`モデル → issue.yaml: 行動 ${mv.action} は status を ${mv.from} → ${mv.to} へ動かすのに、対応表のコマンドが -`);
    continue;
  }
  const ok = cmds.filter((c) => allowed.has(key(mv.from, mv.to, c)));
  for (const c of ok) happens.add(key(mv.from, mv.to, c));
  if (ok.length === 0) {
    const m = `モデル → issue.yaml: 行動 ${mv.action}（${cmds.join("・")}）が status を ${mv.from} → ${mv.to} へ動かすが、issue.yaml にその行が無い`;
    if (!errors.includes(m)) errors.push(m);
  }
}
for (const a of actions.keys()) {
  if (!seen.has(a)) errors.push(`対応表の行動 ${a} がモデルで一度も起きない（名前違いか、消えた行動）`);
}

// issue.yaml → モデル
for (const k of allowed) {
  if (happens.has(k) && outside.has(k)) errors.push(`issue.yaml → モデル: ${k} はモデルで起きるのに、対応表で「モデルの外」と書いている（${outside.get(k)}）`);
  if (!happens.has(k) && !outside.has(k)) errors.push(`issue.yaml → モデル: 行 ${k} はモデルで起きず、対応表の「モデルの外」にも無い`);
}
for (const [k, where] of outside) {
  if (!allowed.has(k)) errors.push(`対応表の「モデルの外」${k} が issue.yaml に無い（${where}）`);
}

if (errors.length > 0) {
  for (const e of errors) console.error(`FAIL ${e}`);
  process.exit(1);
}
console.log(`ok   issue.yaml と照らした（モデル → issue.yaml: 行動 ${actions.size} 個の移り方が全部ある。issue.yaml → モデル: 行 ${allowed.size} 件のうち、モデルで起きる ${happens.size}・モデルの外 ${outside.size}）`);
