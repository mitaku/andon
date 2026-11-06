// A lazy file browser for the focused pane's folder, with bounded read-only previews.
//
// files-plus: a changed copy of Herdr's shipped `files` mod (herdr-app 0.1.10, defaults/mods/files),
// by the herdr-app authors, under the Apache License 2.0 (LICENSE beside this file).
// Changed by herdr-flightdeck: images (png, jpg, gif, webp, bmp, svg) preview as pictures, and
// Mermaid files (.mmd, .mermaid) as diagrams drawn by Pele (pele/, MIT), and Markdown rendered,
// with its Mermaid blocks and image lines drawn, and HTML read as text with a button to open it in
// the browser, on this computer and on other machines.
// Everything else is Herdr's code as shipped.
import { View, div, svg, image } from "gpui-kit";
import { v_flex, h_flex, Editor, EditorState, TextView } from "gpui-base";
import { spaces, panes, focused, exec, readTextFile, watchFolder, theme, settings, window, appHome } from "herdr";
import * as fs from "fs/promises";
import { render as renderMermaid } from "./pele/pele.js";

function location() {
  const space = spaces().find(s => s.active), f = focused();
  const pane = f?.pane ? panes().find(p => p.pane === f.pane) : null;
  const on = f?.pane === null ? (f.terminal?.includes("/") ? f.terminal.split("/")[0] : null) : space?.machine ?? null;
  return { cwd: pane?.cwd || f?.cwd || space?.folder, on, space: space?.id };
}
const key = () => JSON.stringify(location());
const LANG = { ts: "js", tsx: "js", mts: "js", cts: "js", jsx: "js", mjs: "js", toml: "yaml", jsonc: "json", svelte: "html", vue: "html" };
const PER_FOLDER = 500, TOTAL = 5000;

// files-plus: image previews. image() draws only files under the app home, so a picture is
// copied into cache/files-plus/ first. fs.writeFile writes an empty file for bytes (herdr-app
// 0.1.10), so programs move the bytes: PowerShell copies a file here; from another machine,
// base64 travels as text and is decoded here.
const IMAGES = new Set(["png", "jpg", "jpeg", "gif", "webp", "bmp", "svg"]);
const MAX_IMAGE = 5 * 1024 * 1024; // base64 of it must fit exec's 8 MiB output
const CACHE = "cache/files-plus", KEEP = 50, KEEP_MS = 7 * 86400000;
const isImage = (path) => IMAGES.has((path.split(".").pop() ?? "").toLowerCase());
const mib = (n) => `${(n / 1048576).toFixed(1)} MiB`;
const windows = () => window()?.platform === "windows";
// A fixed script with quoted local paths, as directory() above does.
const ps = (script, ...args) =>
  exec("powershell", ["-NoProfile", "-NonInteractive", "-Command", `& { ${script} } ${args.map((a) => `'${a.replaceAll("'", "''")}'`).join(" ")}`]);

// FNV-1a: where, which file, its size and time name the cache file, so an edited picture is fetched again.
function fnv(text) {
  let h = 0x811c9dc5;
  for (let i = 0; i < text.length; i++) h = Math.imul(h ^ text.charCodeAt(i), 0x01000193) >>> 0;
  return h.toString(16).padStart(8, "0");
}

// files-plus: Mermaid. Pele draws SVG text in the mod; Herdr's SVG drawing ignores CSS variables,
// so Pele's are resolved to the theme's colors before the file is written (text writes fine).
const MERMAID = new Set(["mmd", "mermaid"]);
const isMermaid = (path) => MERMAID.has((path.split(".").pop() ?? "").toLowerCase());

function inlineColors(svgText, t) {
  const colors = { "--pele-bg": t.background, "--pele-fg": t.foreground, "--pele-muted": t.muted, "--pele-line": t.foreground,
    "--pele-surface": t.surface, "--pele-surface-alt": t.raised, "--pele-border": t.border, "--pele-accent": t.accent, "--pele-font": "sans-serif" };
  const declared = {};
  for (const [, name, value] of svgText.matchAll(/(--[\w-]+):([^;"]+)/g)) declared[name] = value;
  const resolve = (text, depth = 0) => depth > 5 ? text
    : text.replace(/var\((--[\w-]+)(?:,([^()]*))?\)/g, (_m, name, fallback) => colors[name] ?? (declared[name] ? resolve(declared[name], depth + 1) : fallback ?? ""));
  return resolve(svgText).replace(/--[\w-]+:[^;"]+;?/g, "");
}

// Draws the diagram into the cache at most `room` px wide: image() draws an SVG at the size its
// root says, so the size is written into the file (the viewBox keeps the drawing whole).
// Named by its text, the theme and the width, so an edit, a new theme or a resize draws it again.
async function cacheMermaid(file, on, room) {
  const text = await readTextFile(file, { on });
  if (text === null) throw new Error("This file is no longer available");
  return drawMermaid(text, room);
}

async function drawMermaid(text, room) {
  const t = theme();
  const { svg: drawn, width, height } = renderMermaid(text);
  const scale = Math.min(1, room / width), w = Math.round(width * scale), h = Math.round(height * scale);
  const hash = fnv(`${text}\0${JSON.stringify(t)}\0${w}`);
  const dir = `${appHome()}/${CACHE}`;
  await fs.mkdir(dir, { recursive: true });
  const hit = (await fs.readdir(dir)).find((n) => n.endsWith(`-${hash}.svg`));
  const name = hit ?? `${Date.now()}-${hash}.svg`;
  if (!hit) await fs.writeFile(`${dir}/${name}`, inlineColors(drawn, t).replace(/ width="[\d.]+" height="[\d.]+"/, ` width="${w}" height="${h}"`));
  return { path: `${CACHE}/${name}`, width: w, height: h, room };
}

// files-plus: Markdown. TextView draws Markdown but no images, so the text is cut into blocks:
// Markdown for TextView, ```mermaid fences for Pele, and lines that are only an image for image().
const MARKDOWN = new Set(["md", "markdown"]);
const isMarkdown = (path) => MARKDOWN.has((path.split(".").pop() ?? "").toLowerCase());
// files-plus: HTML. TextView draws HTML without CSS, so the page is read as text here (its head,
// styles and scripts dropped), and "Open in browser" shows it as made: the file itself on this
// computer, a copy in the cache from another machine (files beside it don't come along).
const HTML = new Set(["html", "htm"]);
const isHtml = (path) => HTML.has((path.split(".").pop() ?? "").toLowerCase());
const MAX_HTML = 400000;
const htmlBody = (text) => text.replace(/<!--[\s\S]*?-->/g, "").replace(/<(head|script|style|template|noscript)\b[\s\S]*?<\/\1\s*>/gi, "");
const IMAGE_LINE = /^\s*!\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)\s*$/;

function markdownBlocks(text) {
  /** @type {{ kind: string, text?: string, src?: string, diagram?: { path: string, width: number, height: number }, picture?: string, error?: string }[]} */
  const blocks = [];
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  let md = [], fence = null, mermaid = null;
  const flush = () => { if (md.join("").trim()) blocks.push({ kind: "md", text: md.join("\n") }); md = []; };
  for (const line of lines) {
    if (mermaid) {
      if (/^\s*(```|~~~)\s*$/.test(line)) { blocks.push({ kind: "mermaid", text: mermaid.join("\n") }); mermaid = null; }
      else mermaid.push(line);
      continue;
    }
    const open = line.match(/^\s*(```|~~~)\s*(\S*)/);
    if (fence) { md.push(line); if (open && line.trim().startsWith(fence)) fence = null; continue; }
    if (open && open[2].toLowerCase() === "mermaid") { flush(); mermaid = []; continue; }
    if (open) { fence = open[1]; md.push(line); continue; }
    const pic = !fence && line.match(IMAGE_LINE);
    if (pic && !/^[a-z]+:/i.test(pic[1])) { flush(); blocks.push({ kind: "image", src: decodeURI(pic[1]) }); continue; }
    md.push(line);
  }
  if (mermaid) blocks.push({ kind: "mermaid", text: mermaid.join("\n") });
  flush();
  return blocks;
}

// A link or image path relative to a file in the tree, as a tree path ("docs/a.md"); null if it leaves the tree.
function relative(from, target) {
  const parts = from.split("/").slice(0, -1);
  for (const p of target.replace(/\\/g, "/").split("/")) {
    if (p === "..") { if (!parts.length) return null; parts.pop(); }
    else if (p && p !== ".") parts.push(p);
  }
  return parts.join("/");
}

// Web links open in the person's browser. Only http(s): anything else could name a program.
function openWeb(url) {
  if (!/^https?:\/\/[^\s]+$/i.test(url)) return;
  const platform = window()?.platform;
  exec(platform === "windows" ? "explorer.exe" : platform === "macos" ? "open" : "xdg-open", [url]).catch(() => {});
}

async function fileInfo(file, on) {
  const r = !on && windows()
    ? await ps("param($p) $i = Get-Item -LiteralPath $p -ErrorAction Stop; '{0} {1}' -f $i.Length, $i.LastWriteTimeUtc.Ticks", file)
    : await exec("stat", ["-c", "%s %Y", "--", file], { on });
  if (r.code !== 0) throw new Error(r.stderr.trim() || "Cannot read this file");
  const [size, time] = r.stdout.trim().split(/\s+/).map(Number);
  return { size, time };
}

// Copies a picture (or a page to open) into the cache (named "<ms>-<hash>.<ext>") and returns its app-home path.
async function cacheFile(file, on) {
  const { size, time } = await fileInfo(file, on);
  if (size > MAX_IMAGE) throw new Error(`Too large to copy (${mib(size)})`);
  const ext = file.split(".").pop().toLowerCase(), hash = fnv(`${on ?? ""}\0${file}\0${size}\0${time}`);
  const dir = `${appHome()}/${CACHE}`;
  await fs.mkdir(dir, { recursive: true });
  const hit = (await fs.readdir(dir)).find((n) => n.endsWith(`-${hash}.${ext}`));
  if (hit) return `${CACHE}/${hit}`;
  const name = `${Date.now()}-${hash}.${ext}`, out = `${dir}/${name}`;
  let r;
  if (!on && windows()) r = await ps("param($s, $d) Copy-Item -LiteralPath $s -Destination $d -Force -ErrorAction Stop", file, out);
  else {
    const b64 = await exec("base64", ["-w0", "--", file], { on });
    if (b64.code !== 0) throw new Error(b64.stderr.trim() || "Cannot read this file");
    const text = `${out}.b64`;
    await fs.writeFile(text, b64.stdout.trim());
    r = windows()
      ? await ps("param($s, $d) [IO.File]::WriteAllBytes($d, [Convert]::FromBase64String([IO.File]::ReadAllText($s)))", text, out)
      : await exec("sh", ["-c", 'base64 -d "$1" > "$2"', "files-plus", text, out]);
    await fs.unlink(text).catch(() => {});
  }
  if (r.code !== 0) throw new Error(r.stderr.trim() || "Cannot copy this file");
  return `${CACHE}/${name}`;
}

// Opens a file on this computer with the system's default app (the browser, for a page).
// Only HTML files come here. explorer.exe exits 1 even when it opened the file.
function openLocal(file) {
  if (windows()) return exec("explorer.exe", [file.replace(/\//g, "\\")]);
  return exec(window()?.platform === "macos" ? "open" : "xdg-open", [file]);
}

// Drops cached pictures older than a week, and all but the newest 50.
async function pruneCache() {
  const dir = `${appHome()}/${CACHE}`;
  if (!(await fs.exists(dir))) return;
  const names = (await fs.readdir(dir)).filter((n) => /^\d+-/.test(n)).sort().reverse();
  for (const [i, n] of names.entries()) if (i >= KEEP || Date.now() - Number(n.split("-")[0]) > KEEP_MS) await fs.unlink(`${dir}/${n}`).catch(() => {});
}

async function directory(path, on) {
  if (!on && window()?.platform === "windows") {
    const r = await exec("powershell", ["-NoProfile", "-Command", "& { param($p) @(Get-ChildItem -LiteralPath $p -Force -ErrorAction Stop | Where-Object Name -ne '.git' | Select-Object -First 501 | ForEach-Object { @{ name=$_.Name; dir=($_.PSIsContainer -and -not ($_.Attributes -band [IO.FileAttributes]::ReparsePoint)) } }) | ConvertTo-Json -Compress } '" + path.replaceAll("'", "''") + "'"]);
    if (r.code !== 0) throw new Error(r.stderr || "Cannot list this folder");
    const result = JSON.parse(r.stdout || "[]");
    return Array.isArray(result) ? result : [result];
  }
  const script = 'n=0; for p in "$1"/* "$1"/.[!.]* "$1"/..?*; do [ -e "$p" ] || [ -L "$p" ] || continue; name=${p##*/}; [ "$name" = .git ] && continue; if [ -d "$p" ] && [ ! -L "$p" ]; then kind=d; else kind=f; fi; printf "%s\\0%s\\0" "$kind" "$name"; n=$((n+1)); [ "$n" -gt 500 ] && break; done; test -d "$1"';
  const r = await exec("sh", ["-c", script, "files", path], { on });
  if (r.code !== 0) throw new Error(r.stderr || "Cannot list this folder");
  const fields = r.stdout.split("\0"), out = [];
  for (let i = 0; i + 1 < fields.length; i += 2) out.push({ name: fields[i + 1], dir: fields[i] === "d" });
  return out;
}

export default class Files extends View {
  init(_props, cx) {
    this.saved = new Map();
    this.entries = new Map(); this.folders = new Set(); this.pending = new Set(); this.marks = {};
    this.refresh(cx);
    cx.timer.every(10000, cx => this.refresh(cx));
    cx.spawn(async () => { await pruneCache().catch(() => {}); });
  }

  watch(root, cx) {
    if (this.on || this.watched === root) return;
    this.watched = root;
    const context = this.context;
    cx.spawn(async cx => {
      try { while (this.context === context) { await watchFolder(root); if (this.current(context)) this.refresh(cx); } }
      catch { if (this.context === context) this.watched = null; }
    });
  }

  current(context) {
    return this.context === context && key() === context.key;
  }

  async loadFolder(path, context) {
    const { on, cwd: root } = context;
    const entries = await directory(path ? `${root}/${path}` : root, on);
    if (!this.current(context) || (path && !this.folders.has(path))) return;
    const used = [...this.entries.entries()].filter(([p]) => p !== path).reduce((n, [, e]) => n + e.length, 0);
    this.entries.set(path, entries.slice(0, Math.max(0, Math.min(PER_FOLDER, TOTAL - used))).sort((a, b) => Number(b.dir) - Number(a.dir) || a.name.localeCompare(b.name)));
    if (entries.length > PER_FOLDER || used + entries.length > TOTAL) this.limited = true;
  }

  refresh(cx) {
    const { cwd, on } = location(), next = key();
    if (next !== this.context?.key) {
      if (this.context) this.saved.set(this.context.key, { folders: this.folders, file: this.file });
      const saved = this.saved.get(next);
      this.context = { key: next, cwd, on }; this.root = cwd; this.on = on; this.entries = new Map(); this.folders = new Set(saved?.folders);
      this.pending = new Set(); this.marks = {}; this.show(null); this.watched = null; this.error = null; this.limited = false;
      this.loading = false;
      if (saved?.file) this.show(saved.file, cx);
    }
    if (!cwd || this.loading) return;
    const context = this.context;
    this.loading = true;
    this.limited = false;
    cx.spawn(async cx => {
      try {
        for (const path of ["", ...this.folders]) {
          if (!this.current(context)) return;
          await this.loadFolder(path, context);
        }
        const top = await exec("git", ["rev-parse", "--show-toplevel"], { cwd, on }).catch(() => null);
        const marks = {};
        if (top?.code === 0) {
          // Writes nothing under .git (index lock, fsmonitor cookie): the folder watch would refresh again.
          const status = await exec("git", ["--no-optional-locks", "-c", "core.fsmonitor=false", "status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=normal"], { cwd, on }).catch(() => null);
          if (status?.code === 0) for (const entry of status.stdout.split("\0").filter(Boolean).slice(0, TOTAL)) {
            const xy = entry.slice(0, 2);
            marks[`${top.stdout.trim()}/${entry.slice(3)}`] = xy === "??" || xy.includes("A") ? "A" : xy.includes("U") ? "U" : xy.includes("D") ? "D" : "M";
          }
        }
        if (!this.current(context)) return;
        this.marks = marks; this.error = null; this.watch(cwd, cx);
      } catch (e) { if (this.current(context)) this.error = String(e); }
      finally { if (this.context === context) this.loading = false; cx.notify(); }
    });
  }

  toggle(path, cx) {
    if (this.folders.has(path)) {
      for (const folder of [...this.folders]) if (folder === path || folder.startsWith(path + "/")) {
        this.folders.delete(folder);
        this.entries.delete(folder);
      }
    } else {
      this.folders.add(path);
      if (!this.entries.has(path) && !this.pending.has(path)) {
        const context = this.context;
        this.pending.add(path);
        cx.spawn(async cx => {
          try { await this.loadFolder(path, context); }
          catch (e) { if (this.current(context)) this.error = String(e); }
          finally { if (this.context === context) this.pending.delete(path); cx.notify(); }
        });
      }
    }
    cx.notify();
  }

  show(path, cx) {
    this.code?.release(); this.code = null; this.file = path; this.fileError = null; this.picture = null; this.diagram = null; this.markdown = null; this.html = null; this.browserError = null;
    if (!path || !cx) return;
    const context = this.context, root = this.root, on = this.on;
    if (isMermaid(path)) {
      cx.spawn(async cx => {
        try {
          const diagram = await cacheMermaid(`${root}/${path}`, on, this.room());
          if (this.file === path && this.current(context)) this.diagram = diagram;
          // The panel's width may have come in while it was drawing.
          if (this.diagram === diagram && diagram.room !== this.room()) this.show(path, cx);
        } catch (e) { if (this.file === path && this.current(context)) this.fileError = e instanceof Error ? e.message.split("\n").slice(0, 4).join("\n") : "Cannot draw this diagram"; }
        finally { cx.notify(); }
      });
      return;
    }
    if (isImage(path)) {
      cx.spawn(async cx => {
        try {
          const picture = await cacheFile(`${root}/${path}`, on);
          if (this.file === path && this.current(context)) this.picture = picture;
        } catch (e) { if (this.file === path && this.current(context)) this.fileError = e instanceof Error ? e.message : "Cannot preview this image"; }
        finally { cx.notify(); }
      });
      return;
    }
    cx.spawn(async cx => {
      try {
        const text = await readTextFile(`${root}/${path}`, { on });
        if (this.file !== path || !this.current(context)) return;
        if (text === null) { this.fileError = "This file is no longer available"; return; }
        if (text.includes("\0")) { this.fileError = "Binary file · no text preview"; return; }
        const t = theme(), ext = path.includes(".") ? path.split(".").pop() : "txt";
        this.code = EditorState.new({ language: LANG[ext] ?? ext, readonly: true,
          value: text.length > 400000 ? text.slice(0, 400000) + "\n… (file truncated)" : text,
          font_family: settings().font_family, font_size: 12,
          syntax: { keyword: t.ansi5, string: t.ansi2, comment: t.dim, function: t.ansi4, type: t.ansi3, number: t.ansi1, constant: t.ansi6 } });
        if (isMarkdown(path)) this.renderMarkdown(path, text, context, cx);
        if (isHtml(path) && text.length <= MAX_HTML) this.html = htmlBody(text);
      } catch { if (this.file === path && this.current(context)) this.fileError = "Cannot preview this file (binary, too large, or unreadable)"; }
      finally { cx.notify(); }
    });
  }

  // files-plus: cuts the Markdown into blocks and draws its diagrams and pictures; the source view stays in this.code.
  renderMarkdown(path, text, context, cx) {
    const root = this.root, on = this.on, room = this.room();
    const blocks = markdownBlocks(text);
    this.markdown = { path, text, room, blocks };
    for (const b of blocks) {
      if (b.kind === "md") continue;
      cx.spawn(async cx => {
        try {
          if (b.kind === "mermaid") b.diagram = await drawMermaid(b.text, room);
          else {
            const target = relative(path, b.src);
            if (target === null) throw new Error(`${b.src} is outside this folder`);
            b.picture = await cacheFile(`${root}/${target}`, on);
          }
        } catch (e) { b.error = e instanceof Error ? e.message.split("\n").slice(0, 4).join("\n") : "Cannot draw this"; }
        if (this.markdown?.blocks === blocks && this.current(context)) cx.notify();
      });
    }
  }

  // A clicked link: web links to the browser, links to files in this folder open here.
  followLink(url, cx) {
    if (/^https?:/i.test(url)) return openWeb(url);
    const bare = decodeURI(url.replace(/^file:\/\//i, "").split("#")[0]);
    if (!bare) return;
    const root = (this.root ?? "").replace(/\\/g, "/");
    const inside = bare.replace(/\\/g, "/").startsWith(root + "/") ? bare.replace(/\\/g, "/").slice(root.length + 1) : relative(this.file ?? "", bare);
    if (inside) { this.show(inside, cx); cx.notify(); }
  }

  markdownView(colors) {
    const t = /** @type {Record<string, `#${string}`>} */ (colors);
    const m = this.markdown, d = this.room();
    return v_flex().id("files-markdown").flex_1().min_h(0).p(14).gap(10).overflow_y_scrollbar()
      .children(m.blocks.map((b, i) => b.kind === "md"
        ? TextView.markdown(`files-md-${i}`, b.text).selectable(true).on_link_click((url, cx) => this.followLink(url, cx))
        : b.error ? div().p(8).rounded(6).bg(t.surface).text_size(12).text_color(t.dim).child(b.error)
        : b.diagram ? h_flex().justify_center().child(image(b.diagram.path).w(b.diagram.width).h(b.diagram.height).flex_none())
        : b.picture ? h_flex().justify_center().child(image(b.picture).max_w(d))
        : div().text_size(12).text_color(t.dim).child(b.kind === "mermaid" ? "Drawing diagram…" : "Loading image…")));
  }

  // files-plus: the page as text; how it looks is the browser's.
  htmlView(colors) {
    const t = /** @type {Record<string, `#${string}`>} */ (colors);
    return v_flex().id("files-html").flex_1().min_h(0).overflow_y_scrollbar()
      .child(div().px(14).py(6).flex_none().border_b(1).border_color(t.border).text_size(11.5).text_color(this.browserError ? t.ansi1 : t.dim)
        .child(this.browserError ?? "Styles and scripts are not shown here. Open in browser to see the page as made."))
      .child(div().p(14).child(TextView.html("files-html-page", this.html).selectable(true).on_link_click((url, cx) => this.followLink(url, cx))));
  }

  // files-plus: the HTML file in the browser, as it was made. Only on a click, since its scripts run there.
  openInBrowser(cx) {
    const path = this.file, root = this.root, on = this.on, context = this.context;
    if (!path || !isHtml(path)) return;
    this.browserError = null;
    cx.spawn(async cx => {
      try { await openLocal(on ? `${appHome()}/${await cacheFile(`${root}/${path}`, on)}` : `${root}/${path}`); }
      catch (e) { if (this.file === path && this.current(context)) this.browserError = e instanceof Error ? e.message : "Cannot open this page"; }
      finally { cx.notify(); }
    });
  }

  // files-plus: the width a diagram may take in the panel, in the units sizes are given in:
  // on_size_change reports the panel as shown, and the app's zoom multiplies every size a mod sets.
  room() {
    const zoom = Number(settings().zoom) || 1;
    return Math.max(80, Math.round((this.width ?? 360) / zoom - 24));
  }

  rows(prefix, depth, out, t) {
    if (out.length >= TOTAL || depth > 40) return out;
    for (const entry of this.entries.get(prefix) ?? []) {
      if (out.length >= TOTAL) break;
      const path = prefix ? `${prefix}/${entry.name}` : entry.name, open = this.folders.has(path), mark = this.marks[`${this.root}/${path}`];
      out.push(h_flex().id(`file-${path}`).h(28).flex_none().pl(8 + depth * 18).pr(8).gap(7).items_center().rounded(7).cursor_pointer().hover(s => s.bg(t.hover))
        .on_click((_e, cx) => entry.dir ? this.toggle(path, cx) : this.show(path, cx))
        .child(entry.dir ? svg(`mods/files-plus/chevron-${open ? "down" : "right"}.svg`).size(12).flex_none().text_color(t.dim) : div().w(12).flex_none())
        .child(svg(`mods/files-plus/${entry.dir ? "folder" : "file"}.svg`).size(14).flex_none().text_color(t.muted))
        .child(div().flex_1().min_w(0).truncate().child(entry.name))
        .when(Boolean(mark), el => el.child(div().text_size(11).text_color(mark === "A" ? t.ansi2 : mark === "D" || mark === "U" ? t.ansi1 : t.blocked).child(mark))));
      if (entry.dir && open) this.rows(path, depth + 1, out, t);
    }
    return out;
  }

  render(cx) {
    if (key() !== this.context?.key) this.refresh(cx);
    const t = theme();
    const d = this.diagram, md = this.markdown && !this.source, page = this.html !== null && !this.source, c =/** @type {Record<string, `#${string}`>} */ (t);
    // files-plus: diagrams are drawn to fit the panel; after a resize, draw them again.
    return v_flex().id("files").on_size_change((size, cx) => {
      this.width = size.width;
      if (this.diagram && this.diagram.room !== this.room()) this.show(this.file, cx);
      if (this.markdown && this.markdown.room !== this.room()) this.renderMarkdown(this.markdown.path, this.markdown.text, this.context, cx);
    }).size_full().min_h(0).rounded(10).overflow_hidden().bg(t.background).border_1().border_color(t.border).text_color(t.foreground).text_size(13)
      .child(h_flex().h(42).flex_none().px(14).gap(8).items_center().border_b(1).border_color(t.border)
        .when(Boolean(this.file), el => el.child(div().id("files-back").w(20).cursor_pointer().text_color(t.muted).on_click((_e, cx) => { this.show(null); cx.notify(); }).child("←")))
        .child(div().font_semibold().child("Files"))
        .when(Boolean(this.file && isHtml(this.file)), el => el.child(div().id("files-open-browser").px(6).rounded(5).text_size(11.5).text_color(c.muted).cursor_pointer()
          .hover(s => s.bg(c.hover).text_color(c.foreground))
          .on_click((_e, cx) => this.openInBrowser(cx))
          .child("Open in browser")))
        .when(Boolean(this.markdown) || this.html !== null, el => el.child(div().id("files-md-mode").px(6).rounded(5).text_size(11.5).text_color(c.muted).cursor_pointer()
          .hover(s => s.bg(c.hover).text_color(c.foreground))
          .on_click((_e, cx) => { this.source = !this.source; cx.notify(); })
          .child(this.source ? "Preview" : "Source")))
        .child(div().flex_1().min_w(0).truncate().text_right().text_size(12).text_color(t.dim).child(this.file ?? this.root?.split(/[\\/]/).pop() ?? "")))
      .when(!this.file, el => el.child(v_flex().id("files-tree").flex_1().min_h(0).overflow_y_scrollbar().p(6).children(this.rows("", 0, [], t))
        .when(Boolean(this.error) || !this.entries.get("")?.length, el => el.child(div().p(8).text_size(12).text_color(t.dim).child(this.error || (this.loading ? "Loading files…" : this.root ? "No files" : "No folder"))))
        .when(this.limited, el => el.child(div().p(8).text_size(11).text_color(t.dim).child("File list limited to 500 entries per folder and 5,000 total")))))
      .when(Boolean(this.file), el => el.child(md ? this.markdownView(t)
        : page ? this.htmlView(t)
        : this.code ? Editor.new(this.code).flex_1().min_h(0)
        : d ? v_flex().id("files-diagram").flex_1().min_h(0).p(12).items_center().overflow_y_scrollbar().child(image(d.path).w(d.width).h(d.height).flex_none())
        : this.picture ? v_flex().id("files-picture").flex_1().min_h(0).p(12).items_center().overflow_y_scrollbar().child(image(this.picture).max_w_full())
        : div().p(12).text_color(t.dim).child(this.fileError || "Loading file…")));
  }
}
