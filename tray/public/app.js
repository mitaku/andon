// 一覧の面の最小の画面（ADR-0064 切る案 2）。画面は切る案 4 で作り直す。
// /            未読の一覧。?view=recent で 30 日の全部、?view=old で古い報告
// /r/<id>      報告。一覧の面の API が既読を書いて署名付き URL を返し、別オリジンの報告の面を枠に出す
const bar = document.getElementById("bar");
const main = document.getElementById("main");

function el(tag, attrs = {}, ...children) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  e.append(...children);
  return e;
}

async function api(path, init) {
  const res = await fetch(path, { ...init, headers: { Accept: "application/json" } });
  const body = await res.json();
  if (!res.ok) throw new Error(body.message || res.status);
  return body;
}

async function list(view) {
  const views = [["unread", "未読"], ["recent", "30日"], ["old", "古い報告"]];
  bar.replaceChildren(...views.map(([v, label]) => el("a", v === view ? { href: `/?view=${v}`, "aria-current": "page" } : { href: `/?view=${v}` }, label)));
  const { reports } = await api(`/api/reports?view=${view}`);
  if (reports.length === 0) {
    main.replaceChildren(el("p", {}, "報告は無い"));
    return;
  }
  const out = [];
  let group = "";
  let ul;
  for (const r of reports) {
    const g = `${r.cell} / ${r.project}`;
    if (g !== group) {
      group = g;
      ul = el("ul");
      out.push(el("h2", {}, g), ul);
    }
    const when = new Date(r.updated_at).toLocaleString("ja-JP");
    ul.append(el("li", r.unread ? { class: "unread" } : {}, el("a", { href: `/r/${r.id}` }, r.title || r.name, el("div", { class: "meta" }, when))));
  }
  main.replaceChildren(...out);
}

async function report(id) {
  document.body.classList.add("report");
  bar.replaceChildren(el("a", { href: "/" }, "← 一覧"));
  const r = await api(`/api/reports/${encodeURIComponent(id)}/open`, { method: "POST" });
  bar.append(el("span", {}, r.title));
  if ((r.content_type || "").startsWith("application/pdf")) {
    main.replaceChildren(el("p", {}, el("a", { href: r.url, target: "_blank", rel: "noopener" }, "PDF を新しいタブで開く")));
    return;
  }
  // allow-same-origin と allow-top-navigation は付けない（報告のスクリプトが枠を外のページへ移しても sandbox の中に居る）
  const frame = el("iframe", { sandbox: "allow-scripts allow-popups allow-popups-to-escape-sandbox", src: r.url, title: r.title });
  main.replaceChildren(frame);
}

const m = /^\/r\/([0-9a-f]{32})$/.exec(location.pathname);
(m ? report(m[1]) : list(new URLSearchParams(location.search).get("view") || "unread")).catch((e) => {
  main.replaceChildren(el("p", {}, `読めない: ${e.message}`));
});
