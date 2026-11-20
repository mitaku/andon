// model/reach.mjs: 状態の図を読んで、「行ける」の形の規則を確かめる（ADR-0032 決めたこと 9）。model/check が打つ。Node の標準だけを使う。
//   node reach.mjs <検査器の -dump dot,actionlabels の出力> <規則の名前> <変数> <値> [除く行動…]
// 規則: 初めの状態から、除く行動を使わずに届くどの状態からも、除く行動を使わずに「<変数> = <値>」の状態まで行ける。
// 「どの状態からも行ける道がある」は状態 1 つの式（INVARIANT）では書けず、時相の性質（PROPERTY）は「どの道でも」の形なので、図を読む。
// 破れていれば、行けない状態の 1 つ（初めから一番近いもの）と、そこまでの行動の並びを書いて終了コード 1。守っていれば 1 行書いて 0。
// 状態の図は strict digraph で、同じ 2 つの状態の間の移り方は 1 本にまとまる（ラベルは 1 つ）。除く行動は、ほかの行動と
// 同じ移り方にならないもの（ほかの行動が動かさない変数を動かすもの）を選ぶ。
import { readFileSync } from "node:fs";

const [dotPath, rule, name, value, ...except] = process.argv.slice(2);
if (!dotPath || !rule || !name || value === undefined) {
  console.error("使い方: node reach.mjs <状態の図> <規則> <変数> <値> [除く行動…]");
  process.exit(2);
}
const goalLine = `/\\ ${name} = ${value}`;
const skip = new Set(except);

const label = new Map();
const initial = [];
const out = new Map();
const into = new Map();
const actions = new Set();
const unescape = (s) => s.replace(/\\n/g, "\n").replace(/\\"/g, '"').replace(/\\\\/g, "\\");
for (const line of readFileSync(dotPath, "utf8").split("\n")) {
  let m = line.match(/^(-?\d+) -> (-?\d+) \[label="([^"]+)"/);
  if (m) {
    actions.add(m[3]);
    if (skip.has(m[3])) continue;
    if (!out.has(m[1])) out.set(m[1], []);
    out.get(m[1]).push({ to: m[2], action: m[3] });
    if (!into.has(m[2])) into.set(m[2], []);
    into.get(m[2]).push(m[1]);
    continue;
  }
  m = line.match(/^(-?\d+) \[label="((?:[^"\\]|\\.)*)"/);
  if (m) {
    label.set(m[1], unescape(m[2]));
    if (/,style = filled\]$/.test(line)) initial.push(m[1]);
  }
}
if (initial.length === 0 || actions.size === 0) {
  console.error(`FAIL ${rule}: 状態の図が読めない（初めの状態 ${initial.length}・移り方 ${actions.size}）: ${dotPath}`);
  process.exit(1);
}
for (const a of skip) {
  if (!actions.has(a)) {
    console.error(`FAIL ${rule}: 除く行動 ${a} が状態の図に一度も無い（名前違いか、消えた行動）`);
    process.exit(1);
  }
}
const isGoal = (s) => label.get(s).split("\n").includes(goalLine);
const goals = [...label.keys()].filter(isGoal);
if (goals.length === 0) {
  console.error(`FAIL ${rule}: 「${name} = ${value}」の状態が状態の図に 1 つも無い（変数か値の書き違い）`);
  process.exit(1);
}

// 初めの状態から、除く行動を使わずに届く状態（幅優先。どの行動で来たかを残す）
const from = new Map(initial.map((s) => [s, null]));
const order = [...initial];
for (let i = 0; i < order.length; i++) {
  for (const e of out.get(order[i]) ?? []) {
    if (from.has(e.to)) continue;
    from.set(e.to, { prev: order[i], action: e.action });
    order.push(e.to);
  }
}
// ゴールまで、除く行動を使わずに行ける状態（ゴールから逆向きに）
const back = new Set(goals);
const queue = [...goals];
while (queue.length > 0) {
  for (const p of into.get(queue.pop()) ?? []) {
    if (!back.has(p)) {
      back.add(p);
      queue.push(p);
    }
  }
}

const stuck = order.filter((s) => !back.has(s));
if (stuck.length > 0) {
  const s = stuck[0];
  const path = [];
  for (let x = s; from.get(x); x = from.get(x).prev) path.unshift(from.get(x).action);
  console.error(`FAIL ${rule}: 「${name} = ${value}」まで行けない状態が ${stuck.length} 個ある（除く行動: ${[...skip].join("・") || "なし"}）。初めから一番近いもの:`);
  console.error(`     行動の並び: ${path.join(" → ") || "（初めの状態）"}`);
  for (const l of label.get(s).split("\n")) console.error(`     ${l}`);
  process.exit(1);
}
console.log(`ok   ${rule}: 除く行動（${[...skip].join("・") || "なし"}）を使わずに届く ${order.length} 状態のどれからも、「${name} = ${value}」まで行ける`);
