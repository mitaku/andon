// The App mods Andon ships for the screen (inbox, kanban-board, context-header, files-plus, waiting) and the
// andon layout: their JavaScript parses, the layout carries the naming block, and the two places
// changed for Andon behave (context-header's name, kanban-board's ledger lookup) against a fake API.
import assert from 'node:assert/strict';
import { readFile, readdir, mkdtemp, mkdir, writeFile, symlink, rm } from 'node:fs/promises';
import os from 'node:os';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import vm from 'node:vm';

const HOST = 'package/host';
const MODS = ['andon', 'inbox', 'kanban-board', 'context-header', 'files-plus', 'waiting'];
let failed = 0;
async function check(name, fn) {
  try { await fn(); console.log(`ok   ${name}`); }
  catch (e) { failed++; console.log(`FAIL ${name}: ${e.message}`); }
}

async function files(dir) {
  const out = [];
  for (const e of await readdir(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) out.push(...await files(p)); else out.push(p);
  }
  return out;
}

await check('画面の mod の JavaScript は構文が通る（node --check）', async () => {
  for (const mod of MODS) {
    const js = (await files(`${HOST}/${mod}`)).filter(f => /\.m?js$/.test(f));
    assert.ok(js.some(f => f.endsWith('/main.js')), `${mod} に main.js が無い`);
    for (const f of js) {
      const r = spawnSync(process.execPath, ['--check', f], { encoding: 'utf8' });
      assert.equal(r.status, 0, `${f}: ${r.stderr}`);
    }
  }
});

await check('画面の mod の mod.json・keys.json は JSON', async () => {
  for (const mod of MODS) {
    for (const f of (await files(`${HOST}/${mod}`)).filter(f => /\/(mod|keys)\.json$/.test(f))) {
      JSON.parse(await readFile(f, 'utf8'));
    }
  }
});

await check('レイアウト andon は名前付けの block（andon-names・mod andon）を持ち、同梱の mod を指す', async () => {
  const layout = JSON.parse(await readFile(`${HOST}/layouts/andon.json`, 'utf8'));
  const blocks = [];
  const walk = n => { if (n.type === 'block') blocks.push(n); (n.children ?? []).forEach(walk); };
  walk(layout);
  const names = blocks.filter(b => b.id === 'andon-names');
  assert.equal(names.length, 1, 'andon-names の block が1つでない');
  assert.equal(names[0].content?.mod, 'andon');
  const used = JSON.stringify(layout);
  for (const mod of ['inbox', 'context-header', 'files-plus', 'waiting']) assert.ok(used.includes(`"mod":"${mod}"`), `${mod} を指していない`);
  assert.equal(layout.preview, 'layouts/andon.svg');
});

// Every herdr export the mods import, doing nothing; a test overrides what it looks at.
const FAKE_HERDR = Object.fromEntries(['settings', 'appHome', 'agents', 'terminals', 'panes', 'spaces', 'focused', 'exec', 'readTextFile', 'watchFolder',
  'run', 'theme', 'props', 'closeOverlay', 'window', 'home', 'hc', 'notify', 'shared', 'setShared', 'layout'].map((k) => [k, () => null]));

const el = new Proxy(function () {}, { get: (_t, k) => (k === 'then' ? undefined : () => el), apply: () => el });
const NO_UI = { View: class {}, div: () => el, svg: () => el, image: () => el, v_flex: () => el, h_flex: () => el, TextView: {} };

// Elements that remember what was called on them: { kind, file, calls: [[method, args]], kids }.
// `when` runs its branch when the condition holds, as the App does.
function recordingUi() {
  const make = (kind, file) => {
    const node = { kind, file, calls: [], kids: [] };
    const p = new Proxy(node, { get: (t, k) => {
      if (k in t) return t[k];
      if (k === 'then') return undefined;
      return (...a) => {
        t.calls.push([k, a]);
        if (k === 'child' && a[0] && typeof a[0] === 'object') t.kids.push(a[0]);
        if (k === 'children') t.kids.push(...a[0]);
        if (k === 'when' && a[0]) a[1](p);
        return p;
      };
    } });
    return p;
  };
  return { View: class {}, div: () => make('div'), svg: (f) => make('svg', f), image: (f) => make('image', f), v_flex: () => make('flex'), h_flex: () => make('flex'), TextView: {} };
}
const walk = (n, out = []) => { out.push(n); n.kids.forEach((k) => walk(k, out)); return out; };
const args = (n, method) => n.calls.filter(([k]) => k === method).map(([, a]) => a);
const last = (n, method) => args(n, method).at(-1)?.[0];
const byId = (root, id) => walk(root).find((n) => last(n, 'id') === id);

// Loads a mod's main.js in a VM with fake herdr, gpui and fs modules. Relative imports are loaded too.
async function load(dir, herdr, ui = NO_UI, globals = {}) {
  const context = vm.createContext({ console, ...globals });
  const synthetic = (exports) => new vm.SyntheticModule(Object.keys(exports), function () {
    for (const [k, v] of Object.entries(exports)) this.setExport(k, v);
  }, { context });
  const known = {
    herdr: synthetic({ ...FAKE_HERDR, ...herdr }),
    'gpui-kit': synthetic({ View: ui.View, div: ui.div, svg: ui.svg, image: ui.image }),
    'gpui-base': synthetic({ v_flex: ui.v_flex, h_flex: ui.h_flex, TextView: ui.TextView, Editor: class {}, EditorState: class {} }),
    'fs/promises': synthetic({ readdir: async () => [], exists: async () => false }),
  };
  const cache = new Map();
  const source = async (file) => {
    if (!cache.has(file)) cache.set(file, new vm.SourceTextModule(await readFile(file, 'utf8'), { context, identifier: file }));
    return cache.get(file);
  };
  const main = await source(`${dir}/main.js`);
  await main.link(async (spec, ref) => known[spec] ?? source(path.join(path.dirname(ref.identifier), spec)));
  await main.evaluate();
  return main.namespace;
}

await check('context-header: Andon の担当・レビュアー・Chief Engineer はラベル issue から名付け（task は読まない）、2行目で名前を繰り返さない', async () => {
  let paneList = [], termList = [], spaceList = [], current = null;
  const { default: Header } = await load(`${HOST}/context-header`, {
    panes: () => paneList, terminals: () => termList, spaces: () => spaceList, props: () => ({ pane: current }),
    run: () => {}, theme: () => ({}), home: () => '/home/sample', hc: async () => ({ code: 0 }), notify: () => {},
  });
  const program = { name: 'Claude Code', key: 'claude' };
  spaceList = [
    { id: 'andon-sample-site-A-0002', name: 'A-0002 Fix login', tabs: [{ id: 'andon-sample-site-A-0002', name: 'A-0002', named: true }] },
    { id: 'andon-sample-site', name: 'sample-site', tabs: [
      { id: 'andon-sample-site-cos', name: 'CoS', named: true },
      { id: 'andon-sample-site-A-0001', name: 'CE A-0001 Billing design', named: true },
    ] },
  ];
  const at = (pane, terminal, tab, labels) => {
    paneList.push({ pane, terminal, tab, status: 'idle', title: '✳ something', program, cwd: '/home/sample/x' });
    termList.push({ address: terminal, state: 'live', labels });
  };
  at('p1', 't1', 'andon-sample-site-A-0002', { project: 'sample-site', issue: 'A-0002', title: 'Fix login' });
  at('p2', 't2', 'andon-sample-site-A-0002', { project: 'sample-site', issue: 'A-0003', title: 'Review  of\tA-0002' });
  at('p3', 't3', 'andon-sample-site-A-0001', { project: 'sample-site', role: 'chief-engineer', issue: 'A-0001', title: 'Billing design' });
  at('p4', 't4', 'andon-sample-site-cos', { project: 'sample-site', role: 'chief-of-staff' });
  // The old label task= is not read (ADR-0008 decision 7).
  at('p5', 't5', 'andon-sample-site-A-0009', { project: 'sample-site', task: 'A-0009', title: 'Old label' });
  const facts = (pane) => { current = pane; const h = new Header(); return h.facts(); };
  let f = facts('p1');
  assert.equal(f.name, 'A-0002 Fix login');
  assert.deepEqual([...f.where], ['This computer']);
  f = facts('p2');
  assert.equal(f.name, 'A-0003 Review of A-0002', 'レビュアーは担当の tab の名前で呼ばない');
  assert.deepEqual([...f.where], ['This computer', 'A-0002 Fix login']);
  f = facts('p3');
  assert.equal(f.name, 'CE A-0001 Billing design');
  assert.deepEqual([...f.where], ['This computer', 'sample-site']);
  f = facts('p4');
  assert.equal(f.name, 'CoS', 'Chief of Staff は andon mod が付けた tab の名前');
  assert.deepEqual([...f.where], ['This computer', 'sample-site']);
  f = facts('p5');
  assert.ok(!String(f.name).includes('A-0009'), `ラベル task= から名付けた: ${f.name}`);
});

await check('kanban-board: version のある kanban/ だけを kanban とみなし、symlink・大きすぎるファイル・alias の入れ子を読む前に拒む（固定 script を本物の sh で）', async () => {
  const tmp = await mkdtemp(path.join(os.tmpdir(), 'kanban-board-'));
  try {
    const put = async (p, text) => { await mkdir(path.dirname(p), { recursive: true }); await writeFile(p, text); };
    const plant = `${tmp}/plant`, ledger = `${plant}/projects/sample-site/kanban`;
    await put(`${tmp}/secret.txt`, 'SECRET-OUTSIDE\n');
    await put(`${tmp}/outside/issue.yaml`, 'title: SECRET-OUTSIDE\n');
    await put(`${ledger}/config.yaml`, 'version: 3\nchief_of_staff: t2\nrepos: [github.com/sample/repo]\n');
    await put(`${ledger}/issues/A-0001/issue.yaml`, 'title: Fine\nstatus: todo\n');
    await put(`${ledger}/issues/A-0001/report.md`, '# Report\n');
    await symlink(`${tmp}/secret.txt`, `${ledger}/issues/A-0001/brief.md`);
    // A short issue.yaml whose aliases nest 22 deep: 4M copies if expanded.
    let bomb = 'a0: &a0 [x]\n';
    for (let i = 1; i <= 22; i++) bomb += `a${i}: &a${i} [*a${i - 1}, *a${i - 1}]\n`;
    await put(`${ledger}/issues/A-0002/issue.yaml`, bomb);
    await mkdir(`${ledger}/issues/A-0003`);
    await symlink(`${tmp}/secret.txt`, `${ledger}/issues/A-0003/issue.yaml`);
    await symlink(`${tmp}/outside`, `${ledger}/issues/A-0004`);
    await put(`${ledger}/issues/A-0005/issue.yaml`, `title: big\nnote: ${'x'.repeat(300 * 1024)}\n`);
    await put(`${plant}/.ghq/github.com/sample/repo/kanban/config.yaml`, 'title: not a ledger\n');
    await mkdir(`${tmp}/linked`);
    await symlink(ledger, `${tmp}/linked/kanban`);
    let pending = null;
    const { default: Board } = await load(`${HOST}/kanban-board`, {
      agents: () => [], terminals: () => [], panes: () => [], spaces: () => [], focused: () => null,
      readTextFile: async () => { throw new Error('the board read a file without the fixed script'); },
      watchFolder: async () => {}, run: () => {}, theme: () => ({}), props: () => ({}), closeOverlay: () => {},
      window: () => ({ platform: 'linux' }),
      exec: async (cmd, args, opts = {}) => {
        if (cmd === 'git') return String(opts.cwd).startsWith(`${plant}/.ghq/`) ? { code: 0, stdout: `${plant}/.ghq/github.com/sample/repo/.git\n`, stderr: '' } : { code: 128, stdout: '', stderr: 'not a git repository' };
        const r = spawnSync(cmd, args, { cwd: opts.cwd, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
        return { code: r.status, stdout: r.stdout, stderr: r.stderr };
      },
    });
    const b = new Board();
    b.reset();
    await b.find(`${plant}/.ghq/github.com/sample/repo/src`, null);
    assert.equal(b.root, `${plant}/projects/sample-site`, 'version の無い kanban/ を越えて kanban を見つけていない');
    assert.equal(b.plant, plant);
    const viaLink = new Board();
    viaLink.reset();
    await viaLink.find(`${tmp}/linked`, null);
    assert.equal(viaLink.root, null, 'symlink の kanban/ を kanban とみなした');
    // The old path tasks/<id>/task.yaml is not read (ADR-0008 decision 7).
    await put(`${ledger}/tasks/A-0009/task.yaml`, 'title: Old path\nstatus: todo\n');
    await put(`${ledger}/issues/A-0006/task.yaml`, 'title: Old file name\nstatus: todo\n');
    const started = Date.now();
    const { ledger: config, tasks } = await b.readLedger(b.root, null);
    const shown = JSON.stringify(tasks);
    assert.ok(Date.now() - started < 2000, '読むのに時間がかかった');
    assert.equal(config.chiefOfStaff, 't2');
    const byId = Object.fromEntries(tasks.map((t) => [t.id, t]));
    assert.equal(byId['A-0001'].title, 'Fine');
    assert.ok(!byId['A-0009'] && !byId['A-0006'], `古い path（tasks/・task.yaml）を読んだ: ${Object.keys(byId)}`);
    assert.match(String(byId['A-0002'].broken), /maxAliases/, 'alias の入れ子を読めない課題にしていない');
    assert.match(String(byId['A-0003'].broken), /symbolic link/, 'symlink の issue.yaml を読んだ');
    assert.match(String(byId['A-0004'].broken), /symbolic link/, 'symlink の課題のフォルダを読んだ');
    assert.match(String(byId['A-0005'].broken), /larger than/, '大きすぎる issue.yaml を読んだ');
    assert.ok(!shown.includes('SECRET-OUTSIDE') && !shown.includes('xxxxxxxxxx'), '拒んだファイルの中身が出た');
    b.where = { on: null };
    b.openTask(byId['A-0001'], { spawn: (fn) => (pending = fn({ notify: () => {} })), notify: () => {} });
    await pending;
    assert.equal(b.detail.report, '# Report\n\n');
    assert.match(String(b.detail.brief), /not shown: symbolic link/, 'symlink の brief.md を読んだ');
  } finally {
    await rm(tmp, { recursive: true, force: true });
  }
});

await check('kanban-board: 形式 2 の kanban は読まず、手で移す 1 行を出す（ADR-0008 決めたこと 7）', async () => {
  const tmp = await mkdtemp(path.join(os.tmpdir(), 'kanban-v2-'));
  try {
    await mkdir(`${tmp}/kanban/tasks/A-0001`, { recursive: true });
    await writeFile(`${tmp}/kanban/config.yaml`, 'version: 2\n');
    await writeFile(`${tmp}/kanban/tasks/A-0001/task.yaml`, 'title: Old\nstatus: todo\n');
    const { default: Board } = await load(`${HOST}/kanban-board`, {
      window: () => ({ platform: 'linux' }), readTextFile: async () => { throw new Error('read without the fixed script'); },
      exec: async (cmd, args, opts = {}) => {
        const r = spawnSync(cmd, args, { cwd: opts.cwd, encoding: 'utf8' });
        return { code: r.status, stdout: r.stdout, stderr: r.stderr };
      },
    });
    await assert.rejects(new Board().readLedger(tmp, null), /^Error: config\.yaml is format 2; move it by hand \(docs\/operate\.md\)$/);
  } finally {
    await rm(tmp, { recursive: true, force: true });
  }
});

await check('App の mod は課題を issue の名前で読み、古い名前（ラベル task・kanban/tasks・task.yaml）を読まない', async () => {
  for (const mod of ['andon', 'inbox', 'context-header', 'kanban-board']) {
    const src = await readFile(`${HOST}/${mod}/main.js`, 'utf8');
    assert.doesNotMatch(src, /labels\??\.task\b|\btask=/, `${mod} がラベル task を読む`);
    assert.doesNotMatch(src, /kanban[\/\\]+tasks|task\.yaml/, `${mod} が古い kanban の path を読む`);
  }
  for (const mod of ['andon', 'inbox', 'context-header']) {
    assert.match(await readFile(`${HOST}/${mod}/main.js`, 'utf8'), /labels\??\.issue\b/, `${mod} がラベル issue を読まない`);
  }
});

await check('inbox: Chief of Staff と Chief Engineer は役ごとに別のアイコンを選ぶ（ADR-0002 決めたこと 18）', async () => {
  const ns = await load(`${HOST}/inbox`, { machines: () => [], setAgentOrder: () => {}, nextCommand: () => null, shared: () => ({}), setShared: () => {} });
  const icons = ns.ROLE_ICON;
  assert.ok(icons, 'ROLE_ICON が読めない');
  assert.deepEqual(Object.keys(icons).sort(), ['chief-engineer', 'chief-of-staff']);
  assert.notEqual(icons['chief-of-staff'], icons['chief-engineer'], '役が同じアイコンを使っている');
  for (const file of Object.values(icons)) {
    const svgFile = await readFile(`${HOST}/inbox/${file.replace(/^mods\/inbox\//, '')}`, 'utf8');
    assert.match(svgFile, /stroke-width="1\.5"/, `${file} の stroke-width が 1.5 でない`);
  }
  assert.ok(new Set(await Promise.all(Object.values(icons).map((f) => readFile(`${HOST}/inbox/${f.replace(/^mods\/inbox\//, '')}`, 'utf8')))).size === 2, '2 つの svg の中身が同じ');
});

// The inbox against a fake API: its sections (ADR-0015 acceptance 1).
async function inbox(state, ui = NO_UI, theme = {}) {
  const store = {};
  const calls = { hc: 0, run: [] };
  const ns = await load(`${HOST}/inbox`, {
    agents: () => state.agents ?? [], terminals: () => state.terms ?? [], spaces: () => [], panes: () => [], machines: () => [],
    focused: () => null, theme: () => theme, setAgentOrder: () => {}, nextCommand: () => new Promise(() => {}),
    shared: (k) => store[k] ?? null, setShared: (k, v) => { store[k] = v; },
    run: (cmd, args) => calls.run.push([cmd, args]),
    hc: () => { calls.hc++; throw new Error('the inbox called hc'); },
    notify: () => { throw new Error('the inbox posted a notification'); },
  }, ui);
  const v = new ns.default();
  v.init({}, { focus_handle: () => ({ focus() {}, is_focused: () => false }), timer: { every() {}, after() {} }, spawn: () => {} });
  return { v, calls, store };
}

await check('inbox: 節は Needs you と Projects の 2 つ。担当は blocked のときだけ、役は常に出る（ADR-0015 決めたこと 1〜4）', async () => {
  const program = { name: 'Claude Code', key: 'claude' };
  const term = (address, status, labels, extra = {}) => ({ address, state: 'live', status, labels, program, turn: 1, ...extra });
  const state = { terms: [
    term('t1', 'idle', { project: 'sample-site', role: 'chief-of-staff' }),
    term('t2', 'idle', { project: 'sample-site', role: 'chief-engineer', issue: 'A-0002', title: 'Billing design' }),
    term('t3', 'blocked', { project: 'sample-site', issue: 'A-0003', title: 'Blocked' }),
    term('t4', 'done', { project: 'sample-site', issue: 'A-0004', title: 'Done' }),
    term('t5', 'working', { project: 'sample-site', issue: 'A-0005', title: 'Working' }),
    term('t6', 'idle', { project: 'sample-site', issue: 'A-0006', title: 'Idle' }, { turn: 9 }),
    term('t7', 'idle', { role: 'dispatcher' }),
    term('t8', 'error', { role: 'dispatcher' }),
    term('t9', 'working', { project: 'other-site', role: 'chief-of-staff' }),
    term('t10', 'idle', { project: 'other-site', role: 'chief-engineer', issue: 'B-0001' }),
    term('t11', 'idle', { project: 'other-site', role: 'chief-engineer', issue: 'B-0000' }),
  ], agents: [
    // A pane's agent with a paneless sub-agent that failed: the parent needs you, by peeking the sub-agent.
    { address: 't20', status: 'working', program, title: 'helper', space_name: 'sample' },
    // dispatcher in a pane.
    { address: 't7', status: 'idle', program, title: 'dispatcher', space_name: 'plant' },
    { address: 't8', status: 'idle', program, title: 'dispatcher', space_name: 'plant' },
  ] };
  state.terms.push(term('t20', 'working', {}), term('t21', 'error', {}, { parent: 't20' }));
  const { v, calls } = await inbox(state);
  let m = v.model();
  for (const k of ['done', 'working', 'idle', 'settled']) assert.ok(!(k in m), `m に ${k} がある`);
  const ids = (rows) => [...rows].map((i) => i.address);
  assert.deepEqual(ids(m.projects), ['t9', 't11', 't10', 't1', 't2'], 'プロジェクトの名前順 → CoS → CE（題の順）でない');
  assert.deepEqual(ids(m.needs).sort(), ['t20', 't3', 't8'].sort());
  assert.equal(m.needs.find((i) => i.address === 't20').peek, 't21', 'sub-agent の error で親を peek しない');
  for (const a of ['t4', 't5', 't6', 't7']) assert.ok(!ids(m.rows).includes(a), `${a} が出た`);
  assert.deepEqual(ids(m.rows), [...ids(m.needs), ...ids(m.projects)]);

  // The Chief of Staff: working stays in Projects, done (unseen) is a Reply, blocked needs you.
  state.terms[0].status = 'working';
  assert.ok(ids(v.model().projects).includes('t1'));
  state.terms[0].status = 'done';
  m = v.model();
  assert.equal(m.needs.find((i) => i.address === 't1')?.kind, 'Reply');
  assert.ok(!ids(m.projects).includes('t1'), '役の行が 2 つある');
  // Decision 7 (provisional): opening a paneless role's reply hides it for that turn.
  v.open(m.needs.find((i) => i.address === 't1'));
  assert.deepEqual(JSON.parse(JSON.stringify(calls.run.at(-1))), ['terminal.peek', { terminal: 't1' }]);
  assert.ok(ids(v.model().projects).includes('t1'), '開いたあとも Needs you にいる');
  state.terms[0].turn = 2;
  assert.ok(ids(v.model().needs).includes('t1'), 'turn が進んだのに Needs you に出ない');
  state.terms[0].status = 'blocked';
  assert.ok(ids(v.model().needs).includes('t1'));
  v.command('next', { notify() {} });
  assert.equal(calls.hc, 0, 'hc を呼んだ');
});

await check('inbox: settle とボードのダイアログが無い（mod.json・keys.json・main.js。ADR-0015 決めたこと 5・6）', async () => {
  const mod = JSON.parse(await readFile(`${HOST}/inbox/mod.json`, 'utf8'));
  assert.ok(!('overlay' in mod), 'mod.json に overlay がある');
  const keys = Object.values(JSON.parse(await readFile(`${HOST}/inbox/keys.json`, 'utf8')).global);
  for (const k of ['inbox.settle', 'mod.open:inbox']) assert.ok(!keys.includes(k), `keys.json に ${k} がある`);
  for (const k of ['inbox.next', 'inbox.focus']) assert.ok(keys.includes(k), `keys.json に ${k} が無い`);
  const src = await readFile(`${HOST}/inbox/main.js`, 'utf8');
  assert.doesNotMatch(src, /\bhc\(|settled|mod\.open", \{ name: "inbox"/, 'main.js に settle か hc かボードが残る');
  for (const doc of ['README.md', 'RECIPE.md']) {
    const text = await readFile(`${HOST}/inbox/${doc}`, 'utf8');
    assert.match(text, /\*\*Projects\*\*/, `${doc} に Projects の節が無い`);
    assert.doesNotMatch(text, /\*\*Done\*\*|\bSettled\b|^## The board|\| `d` \|/m, `${doc} に Done・棚・settle・ボードが残る`);
  }
});

// The logo's colors (docs/assets/logo/README.md): amber and red, each with its highlight.
const AMBER = '#F0A202', AMBER_HI = '#FFD36B', RED = '#D8453B', RED_HI = '#F29A92';
const THEME = { blocked: '#123456', foreground: '#202020', dim: '#808080', muted: '#606060', chrome: '#f0f0f0', accent: '#3366cc', working: '#2288aa', idle: '#999999',
  hover: '#e0e0e0', selected: '#dddddd' };
const colors = (root) => walk(root).flatMap((n) => [...args(n, 'bg'), ...args(n, 'text_color')].map((a) => a[0]));
const sampleProgram = { name: 'Claude Code', key: 'claude' };
const sampleTerm = (address, status, labels, extra = {}) => ({ address, state: 'live', status, labels, program: sampleProgram, turn: 1, ...extra });

await check('inbox: Needs you の灯りは、答えれば進む行（Permission・Question・Auth・種類なし・Reply）が琥珀、Failed が赤。テーマの blocked 色を使わない（ADR-0023 決めたこと 1）', async () => {
  const state = { terms: [
    sampleTerm('t1', 'done', { project: 'sample-site', role: 'chief-of-staff' }),
    sampleTerm('t2', 'blocked', { project: 'sample-site', issue: 'A-0002', title: 'Permission' }, { report: { kind: 'permission' } }),
    sampleTerm('t3', 'blocked', { project: 'sample-site', issue: 'A-0003', title: 'Question' }, { report: { kind: 'question' } }),
    sampleTerm('t4', 'blocked', { project: 'sample-site', issue: 'A-0004', title: 'Auth' }, { report: { kind: 'auth' } }),
    sampleTerm('t5', 'blocked', { project: 'sample-site', issue: 'A-0005', title: 'No kind' }),
    sampleTerm('t6', 'error', { project: 'sample-site', issue: 'A-0006', title: 'Failed' }),
    sampleTerm('t7', 'idle', { project: 'sample-site', role: 'chief-engineer', issue: 'A-0007', title: 'Design' }),
  ] };
  const { v } = await inbox(state, recordingUi(), THEME);
  const m = v.model();
  const kinds = Object.fromEntries(m.needs.map((i) => [i.address, i.kind]));
  assert.deepEqual(kinds, { t1: 'Reply', t2: 'Permission', t3: 'Question', t4: 'Auth', t5: null, t6: 'Failed' });
  for (const i of m.needs) {
    const fill = last(v.mark(THEME, i), 'bg');
    assert.equal(fill, i.address === 't6' ? RED : AMBER, `${i.address}（${i.kind}）の灯りの色`);
  }
  const tree = v.render({});
  assert.ok(!colors(tree).includes(THEME.blocked), 'テーマの blocked 色で塗った');
  assert.ok(colors(tree).includes(AMBER), 'Needs you の見出しに琥珀が無い');
  // A row that only lives in Projects is not lit.
  const projects = m.projects.map((i) => colors(v.mark(THEME, i))).flat();
  assert.ok(!projects.includes(AMBER) && !projects.includes(RED), 'Projects の行が灯った');
});

await check('inbox: 行燈の状態は Needs you から決まる。空なら消灯・答え待ちだけなら琥珀・Failed があれば赤。開いた役の返事では灯らない（ADR-0023 決めたこと 3）', async () => {
  const state = { terms: [sampleTerm('t1', 'idle', { project: 'sample-site', role: 'chief-of-staff' })] };
  const { v, store } = await inbox(state);
  const lantern = () => { const m = v.model(); return [store['inbox.lantern'] ?? null, m.needs.length]; };
  assert.deepEqual(lantern(), [null, 0], '呼ぶ行が無いのに灯った');
  state.terms[0].status = 'done';
  assert.deepEqual(lantern(), ['amber', 1], '役の返事で琥珀にならない');
  state.terms.push(sampleTerm('t2', 'error', { project: 'sample-site', issue: 'A-0002', title: 'Failed' }));
  assert.deepEqual(lantern(), ['red', 2], 'Failed があるのに赤でない');
  state.terms.pop();
  assert.deepEqual(lantern(), ['amber', 1]);
  // Opening a paneless role's reply is looking (ADR-0015 decision 7): the inbox drops it, so does the lantern.
  v.open(v.model().needs[0]);
  assert.deepEqual(lantern(), [null, 0], '見た返事で灯ったまま');
  // A filter hides rows from the column, not from the lantern.
  state.terms[0].turn = 2;
  v.filter = 'nothing matches this';
  assert.deepEqual(lantern(), ['amber', 0], '絞り込みで行燈が消えた');
});

// The andon mod's copy that draws the top of the sidebar (props {brand: true}).
async function brand({ lit = null, px = 300 } = {}) {
  const store = { 'inbox.lantern': lit };
  const calls = [];
  const ns = await load(`${HOST}/andon`, {
    props: () => ({ brand: true }), theme: () => THEME, shared: (k) => store[k] ?? null, setShared: (k, val) => { store[k] = val; },
    layout: () => ({ id: 'root', children: [{ id: 'sidebar', size: { px } }] }), window: () => ({ platform: 'linux' }),
    run: (cmd, a) => calls.push([cmd, a]),
    terminals: () => { throw new Error('the brand read terminals'); }, panes: () => [], spaces: () => [],
  }, recordingUi());
  return { tree: new ns.default().render(), calls };
}

await check('andon（brand）: 一番上の行燈は琥珀・赤・消灯をロゴの色で灯し、色が変わるときだけ不透明度で移る。名前付けはしない（ADR-0023 決めたこと 2〜4）', async () => {
  const shown = (tree) => Object.fromEntries(['amber', 'red'].map((n) => [n, last(byId(tree, `lantern-${n}`), 'opacity')]));
  for (const [lit, want] of [[null, { amber: 0, red: 0 }], ['amber', { amber: 1, red: 0 }], ['red', { amber: 0, red: 1 }]]) {
    const { tree, calls } = await brand({ lit });
    assert.deepEqual(shown(tree), want, `${lit} の灯り方`);
    assert.equal(calls.length, 0, `名前付けの command を出した: ${JSON.stringify(calls)}`);
  }
  const { tree } = await brand({ lit: 'red' });
  for (const [n, lamp, hi] of [['amber', AMBER, AMBER_HI], ['red', RED, RED_HI]]) {
    const light = byId(tree, `lantern-${n}`);
    assert.deepEqual(colors(light), [lamp, hi], `${n} の窓の色`);
    const [prop, policy] = args(light, 'transition')[0] ?? [];
    assert.equal(prop, 'opacity', `${n} が不透明度で移らない`);
    assert.ok(policy.duration >= 300, `${n} の移り変わりが速すぎる`);
    assert.equal(args(light, 'loop').length + args(light, 'frames').length, 0, `${n} が明滅する`);
  }
  const text = walk(tree).flatMap((n) => args(n, 'child').map((a) => a[0])).filter((c) => typeof c === 'string');
  assert.ok(text.includes('Andon') && !text.includes('Herdr'), `記号の横の字: ${text}`);
  assert.ok(!walk(tree).some((n) => n.kind === 'image'), 'Herdr のロゴの絵が残る');
  for (const id of ['search', 'sidebar-toggle']) assert.ok(byId(tree, id), `${id} のボタンが無い`);
  assert.ok(!colors(tree).includes(THEME.blocked), 'テーマの blocked 色で塗った');
  // A folded sidebar (the rail) has no lantern; the button to unfold it stays.
  const rail = (await brand({ lit: 'red', px: 44 })).tree;
  assert.ok(!byId(rail, 'lantern') && byId(rail, 'sidebar-toggle'), '畳んだ列に行燈が出たか、開くボタンが無い');
  for (const file of ['lantern-frame.svg', 'lantern-window.svg', 'lantern-glow.svg']) await readFile(`${HOST}/andon/${file}`, 'utf8');
});

await check('inbox と andon（brand）は描き直しのタイマーを足していない（移り変わりは App の transition。ADR-0023 決めたこと 4）', async () => {
  for (const f of ['andon/main.js', 'andon/brand.js']) assert.doesNotMatch(await readFile(`${HOST}/${f}`, 'utf8'), /\btimer\b/, `${f} にタイマーがある`);
  const timers = (await readFile(`${HOST}/inbox/main.js`, 'utf8')).match(/timer\.\w+\(/g) ?? [];
  assert.deepEqual(timers, ['timer.every('], `inbox のタイマー: ${timers}`);
});

await check('レイアウト andon: brand の block は andon mod（props brand）を指し、根に theme・colors が無く、名前付けとフッターが残る（ADR-0023 受け入れ条件 1）', async () => {
  const layout = JSON.parse(await readFile(`${HOST}/layouts/andon.json`, 'utf8'));
  const blocks = [];
  const walkLayout = (n) => { if (n.type === 'block') blocks.push(n); (n.children ?? []).forEach(walkLayout); };
  walkLayout(layout);
  const find = (id) => blocks.find((b) => b.id === id);
  assert.deepEqual(find('brand')?.content, { mod: 'andon', props: { brand: true } });
  assert.ok(!('theme' in layout) && !('colors' in layout), '根に theme か colors がある');
  assert.deepEqual(find('andon-names')?.content, { mod: 'andon' });
  assert.deepEqual(find('sidebar-footer')?.content, { mod: 'workspace-chrome', props: { footer: true } });
  const picture = await readFile(`${HOST}/layouts/andon.svg`, 'utf8');
  for (const word of ['Needs you', 'Projects', 'Andon']) assert.ok(picture.includes(`>${word}`), `見本の絵に ${word} が無い`);
  for (const word of ['Focus', 'Done', 'Working', 'Settled']) assert.ok(!picture.includes(`>${word}`), `見本の絵に古い ${word} が残る`);
});

await check('kanban-board: 1 回の読みで出す量に上限がある（固定 script）', async () => {
  const tmp = await mkdtemp(path.join(os.tmpdir(), 'kanban-total-'));
  try {
    const ns = await load(`${HOST}/kanban-board`, { exec: async () => ({}), readTextFile: async () => null, window: () => ({}) });
    const script = ns.LIST_ISSUES;
    assert.ok(script, 'LIST_ISSUES が読めない');
    for (let i = 1; i <= 20; i++) {
      await mkdir(`${tmp}/kanban/issues/A-${String(i).padStart(4, '0')}`, { recursive: true });
      await writeFile(`${tmp}/kanban/issues/A-${String(i).padStart(4, '0')}/issue.yaml`, `note: ${'y'.repeat(250 * 1024)}\n`);
    }
    const r = spawnSync('sh', ['-c', script, 'kanban-board', tmp, String(256 * 1024), String(4 * 1024 * 1024)], { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
    assert.equal(r.status, 0, r.stderr);
    assert.ok(r.stdout.length <= 4 * 1024 * 1024 + 4096, `出力が ${r.stdout.length} バイト`);
    assert.match(r.stdout, /\x1fover \d+ bytes in all/);
  } finally {
    await rm(tmp, { recursive: true, force: true });
  }
});

await check('files-plus: 自分で読むもの（画像・図・Markdown の画像行）は、途中のフォルダかファイルが symlink なら読む前に拒む（固定 script を本物の sh で）', async () => {
  const tmp = await mkdtemp(path.join(os.tmpdir(), 'files-plus-'));
  try {
    const { NO_LINKS_SH } = await load(`${HOST}/files-plus`, {
      spaces: () => [], panes: () => [], focused: () => null, settings: () => ({}), appHome: () => tmp,
    });
    assert.ok(NO_LINKS_SH, 'NO_LINKS_SH が読めない');
    await mkdir(`${tmp}/repo/docs`, { recursive: true });
    await mkdir(`${tmp}/outside`);
    await writeFile(`${tmp}/outside/secret.png`, 'SECRET');
    await writeFile(`${tmp}/repo/docs/shot.png`, 'PNG');
    await symlink(`${tmp}/outside/secret.png`, `${tmp}/repo/docs/link.png`);
    await symlink(`${tmp}/outside`, `${tmp}/repo/img`);
    const said = (...parts) => spawnSync('sh', ['-c', NO_LINKS_SH, 'files-plus', `${tmp}/repo`, ...parts], { encoding: 'utf8' }).stdout.trim();
    assert.equal(said('docs', 'shot.png'), 'ok');
    assert.equal(said('docs', 'link.png'), 'link', 'symlink のファイルを通した');
    assert.equal(said('img', 'secret.png'), 'link', 'symlink のフォルダを通した');
    const src = await readFile(`${HOST}/files-plus/main.js`, 'utf8');
    for (const call of ['cacheFile(root, path, on)', 'cacheFile(root, target, on)', 'cacheMermaid(root, path, on,']) assert.ok(src.includes(call), `${call} が無い`);
    assert.match(src, /async function cacheFile\(root, path, on\) \{\n  const file = await unlinked\(root, path, on\);/);
    assert.match(src, /async function cacheMermaid\(root, path, on, room\) \{\n  const file = await unlinked\(root, path, on\);/);
  } finally {
    await rm(tmp, { recursive: true, force: true });
  }
});

// The waiting mod against a fake exec: what `andon waiting --json` printed, drawn (ADR-0042 acceptance 10).
const textOf = (root) => walk(root).flatMap((n) => args(n, 'child').map((a) => a[0])).filter((c) => typeof c === 'string');
const SAMPLE_WAITING = {
  version: 1, at: '2026-11-21T10:00:00Z', project: 'sample-site',
  slots: { limit: 4, working: 3, waiting: 2, free: 1, waiting_full: false },
  items: [
    { kind: 'merge', id: 'S-0021', title: '名指しの口を足す', since: '2026-11-21T04:43:35Z', question: 'S-0021#1',
      pr: 'https://github.com/example/sample-site/pull/41', reason: 'delegation: merge=B < tier C', holds: ['S-0022', 'S-0023'] },
    { kind: 'decision', id: 'S-0030', title: '書き出しの形を決める', since: '2026-11-21T02:12:13Z', question: 'S-0030#1',
      ask: { kind: 'users', question: '書き出しは CSV だけにするか', facts: ['依頼書の「やること」'], reason: '利用者に見える',
        options: [{ label: 'CSV だけ', pick: true }, { label: 'Excel も' }] }, answerable: true, holds: [] },
    { kind: 'task', id: 'S-0031', title: 'image を焼き直す', since: '2026-11-21T08:55:43Z', holds: [] },
  ],
};
async function waitingPane(reply, { cwd = '/srv/cell/projects/sample-site', terms = [] } = {}) {
  const calls = { exec: [], run: [] };
  const ns = await load(`${HOST}/waiting`, {
    spaces: () => [{ id: 's1', active: true, machine: null, folder: cwd }], panes: () => [{ pane: 'p1', cwd }],
    focused: () => ({ pane: 'p1' }), terminals: () => terms, theme: () => THEME, window: () => ({ platform: 'linux' }),
    run: (cmd, a) => calls.run.push([cmd, JSON.parse(JSON.stringify(a))]),
    exec: async (cmd, a, opts = {}) => {
      calls.exec.push([cmd, [...a], { cwd: opts.cwd, on: opts.on ?? null }]);
      if (cmd !== 'andon') return { code: 0, stdout: '', stderr: '' };
      if (reply instanceof Error) throw reply;
      return typeof reply === 'function' ? reply() : reply;
    },
  }, recordingUi());
  const v = new ns.default();
  let pending = [];
  const cx = { spawn: (fn) => pending.push(fn(cx)), notify: () => {}, timer: { every() {}, after() {} }, stop_propagation() {} };
  v.init({}, cx);
  await Promise.all(pending);
  return { v, calls, cx, ns, draw: () => v.render(cx) };
}
const json = (o) => ({ code: 0, stdout: JSON.stringify(o), stderr: '' });

await check('waiting: andon waiting --json をフォーカスした pane のフォルダで呼び、枠の行とオーナー待ちの行を出力の並びのまま描く（ADR-0042 受け入れ条件 10）', async () => {
  const { calls, draw } = await waitingPane(json(SAMPLE_WAITING));
  assert.deepEqual(calls.exec, [['andon', ['waiting', '--json'], { cwd: '/srv/cell/projects/sample-site', on: null }]]);
  const tree = draw();
  assert.deepEqual(textOf(byId(tree, 'waiting-slots')), ['Slots 3/4 working · 2 waiting on you']);
  const rows = walk(tree).map((n) => last(n, 'id')).filter((id) => /^waiting-row-/.test(id ?? ''));
  assert.deepEqual(rows, ['waiting-row-S-0021', 'waiting-row-S-0030', 'waiting-row-S-0031'], '出力の並びのままでない');
  assert.deepEqual(['S-0021', 'S-0030', 'S-0031'].map((id) => textOf(byId(tree, `waiting-kind-${id}`))[0]), ['Merge', 'Decide', 'Do']);
  const all = textOf(tree);
  assert.ok(all.includes('S-0021 名指しの口を足す') && all.includes('· 2 waiting behind'), `行の中身: ${all}`);
  assert.ok(!all.includes('Needs you'), '見出しを Needs you にした');
  // Nothing waiting: the slots line and one line.
  const empty = (await waitingPane(json({ ...SAMPLE_WAITING, items: [], slots: { ...SAMPLE_WAITING.slots, waiting: 0 } }))).draw();
  assert.ok(byId(empty, 'waiting-slots') && textOf(byId(empty, 'waiting-empty'))[0] === 'Nothing is waiting on you', '待ちが無いときの形');
  // waiting_full shows on the slots line.
  const full = (await waitingPane(json({ ...SAMPLE_WAITING, slots: { limit: 2, working: 2, waiting: 2, free: 0, waiting_full: true } }))).draw();
  assert.deepEqual(textOf(byId(full, 'waiting-slots')), ['Slots 2/2 working · 2 waiting on you · full']);
});

await check('waiting: 行を開くと問いの文・事実・選択肢・後ろで待つ課題が出て、マージの依頼は PR を開け、Chief of Staff へ行ける。答えるボタンは無い（ADR-0042 受け入れ条件 10）', async () => {
  const cos = { address: 't1', state: 'live', labels: { project: 'sample-site', role: 'chief-of-staff' } };
  const { v, calls, draw, cx } = await waitingPane(json(SAMPLE_WAITING), { terms: [cos, { address: 't2', state: 'live', labels: { project: 'other', role: 'chief-of-staff' } }] });
  let tree = draw();
  assert.ok(!byId(tree, 'waiting-detail-S-0030'), '開く前から中身が出た');
  const click = (node) => { const [fn] = args(node, 'on_click').at(-1); fn({}, cx); };
  click(byId(tree, 'waiting-head-S-0030'));
  tree = draw();
  const detail = byId(tree, 'waiting-detail-S-0030');
  assert.ok(detail, '開いた行の中身が無い');
  const said = textOf(detail);
  for (const want of ['書き出しは CSV だけにするか', '· 依頼書の「やること」', '利用者に見える', '1. CSV だけ (recommended)', '2. Excel も']) assert.ok(said.includes(want), `${want} が無い: ${said}`);
  assert.ok(!walk(detail).some((n) => /answer|confirm/i.test(String(last(n, 'id') ?? ''))), '答えるボタンがある');
  click(byId(tree, 'waiting-cos-S-0030'));
  assert.deepEqual(calls.run.at(-1), ['agent.focus', { terminal: 't1' }], '別のプロジェクトの Chief of Staff へ行った');
  click(byId(tree, 'waiting-head-S-0021'));
  tree = draw();
  assert.ok(textOf(byId(tree, 'waiting-detail-S-0021')).includes('S-0022, S-0023'), '後ろで待つ課題が出ない');
  click(byId(tree, 'waiting-pr-S-0021'));
  assert.deepEqual(calls.exec.at(-1), ['xdg-open', ['https://github.com/example/sample-site/pull/41'], { cwd: undefined, on: null }]);
  assert.ok(!byId(tree, 'waiting-pr-S-0030') && !byId(tree, 'waiting-pr-S-0031'), 'マージの依頼でない行に PR のボタンがある');
  v.toggle('S-0030');
  assert.ok(!byId(draw(), 'waiting-detail-S-0030'), 'もう一度押しても閉じない');
});

await check('waiting: 出力が読めない・version が 1 でない・command が無い・失敗したときは理由を 1 行で出し、ほかを描かない（ADR-0042 受け入れ条件 10）', async () => {
  const cases = [
    [{ code: 0, stdout: 'not json', stderr: '' }, /not JSON/],
    [json({ ...SAMPLE_WAITING, version: 2 }), /version 2; this pane reads version 1/],
    [{ code: 127, stdout: '', stderr: 'sh: andon: not found' }, /^This cell has no andon: update/],
    // An andon older than the waiting command: its usage error (exit 2).
    [{ code: 2, stdout: '', stderr: 'andon: 知らない sub-command: waiting\nusage: andon <sub-command>' }, /^This cell's andon has no waiting command: update/],
    [new Error('spawn andon ENOENT'), /^Can't run andon waiting: spawn andon ENOENT$/],
    [{ code: 1, stdout: '', stderr: 'andon waiting: kanban が見つからない（projects/<slug>/ か担当の worktree で打つ）\nmore' }, /^andon waiting failed: andon waiting: kanban が見つからない/],
    [json({ version: 1, project: 'sample-site' }), /no slots or items/],
  ];
  for (const [reply, want] of cases) {
    const tree = (await waitingPane(reply)).draw();
    const said = textOf(byId(tree, 'waiting-reason') ?? { kids: [], calls: [] });
    assert.equal(said.length, 1, `理由が 1 行でない: ${said}`);
    assert.match(said[0], want);
    assert.ok(!said[0].includes('\n'), '理由が 2 行ある');
    assert.ok(!byId(tree, 'waiting-slots') && !byId(tree, 'waiting-list'), '理由と一緒に枠か行を描いた');
  }
});

// What the two mods run (ADR-0042 acceptance 12): the first argument of every exec( call, in order.
function execTargets(src) {
  const out = [];
  const re = /(?<![.\w])exec\(/g;
  for (let m; (m = re.exec(src));) {
    let i = m.index + 5, depth = 0, quote = null, s = '';
    for (; i < src.length; i++) {
      const c = src[i];
      if (quote) { s += c; if (c === '\\') s += src[++i]; else if (c === quote) quote = null; continue; }
      if (c === '"' || c === "'" || c === '`') { quote = c; s += c; continue; }
      if ('([{'.includes(c)) depth++;
      if (')]}'.includes(c)) { if (depth === 0) break; depth--; }
      if (c === ',' && depth === 0) break;
      s += c;
    }
    out.push(s.trim());
  }
  return out;
}
const OPENER = 'platform === "windows" ? "explorer.exe" : platform === "macos" ? "open" : "xdg-open"';

await check('kanban-board と waiting が呼ぶ command は決まった一覧のまま。kanban を書く command とネットワークが無く、waiting が呼ぶのは andon waiting --json と PR を開くものだけ（ADR-0042 受け入れ条件 12）', async () => {
  const board = await readFile(`${HOST}/kanban-board/main.js`, 'utf8');
  const waiting = await readFile(`${HOST}/waiting/main.js`, 'utf8');
  assert.deepEqual(execTargets(board), ['"powershell"', '"andon"', OPENER, '"sh"', '"sh"', '"git"', '"sh"', '"ls"'], 'kanban-board の exec の呼び先が変わった');
  assert.deepEqual(execTargets(waiting), [OPENER, '"andon"'], 'waiting の exec の呼び先が変わった');
  assert.equal(waiting.split('exec("andon", ["waiting", "--json"],').length, 2, 'andon waiting の引数が決まった形でない');
  assert.equal(board.split('exec("andon", ["waiting", "--json"],').length, 2, 'kanban-board の andon waiting の引数が決まった形でない');
  // The extraction finds a call it should: a new exec in a copy changes the list.
  assert.deepEqual(execTargets(`${waiting}\nexec("kanban", ["done"]);`).at(-1), '"kanban"', '呼び先を拾う式が何も見ていない');
  for (const [name, src] of [['kanban-board', board], ['waiting', waiting]]) {
    assert.doesNotMatch(src, /\bfetch\(|WebSocket|XMLHttpRequest|writeTextFile|\bhc\(/, `${name} がネットワークか書く口を使う`);
  }
  assert.match('x = fetch(url)', /\bfetch\(|WebSocket/, '否定の式が当たらない');
  const readme = await readFile(`${HOST}/waiting/README.md`, 'utf8');
  assert.match(readme, /^## What it runs$/m, 'README に What it runs の表が無い');
  assert.match(readme, /\| `exec\("andon", \["waiting", "--json"\], \{cwd, on\}\)` \|/, 'README の表に andon waiting の行が無い');
});

await check('レイアウト andon: 右のパネルの stack に waiting の block があり、inbox の見出しのボタンがパネルを出して waiting を選ぶ（ADR-0042 決めたこと 12）', async () => {
  const layout = JSON.parse(await readFile(`${HOST}/layouts/andon.json`, 'utf8'));
  const nodes = [];
  const walkLayout = (n) => { nodes.push(n); (n.children ?? []).forEach(walkLayout); };
  walkLayout(layout);
  const node = (id) => nodes.find((n) => n.id === id);
  const stack = node('panel-body');
  assert.equal(stack?.type, 'stack');
  assert.deepEqual(stack.children.map((c) => c.id), ['files', 'changes', 'waiting']);
  assert.deepEqual(node('waiting').content, { mod: 'waiting' });
  assert.equal(stack.active, 'files', '開いたときの画面を変えた');
  const { v, calls } = await inbox({ terms: [] }, recordingUi(), THEME);
  const tree = v.render({});
  const button = byId(tree, 'fq-waiting');
  assert.ok(button, 'inbox の見出しに waiting のボタンが無い');
  args(button, 'on_click').at(-1)[0]({}, { notify() {} });
  const ns = await load(`${HOST}/inbox`, { machines: () => [], setAgentOrder: () => {}, nextCommand: () => null, shared: () => ({}), setShared: () => {} });
  assert.deepEqual(calls.run.map((c) => JSON.parse(JSON.stringify(c))), [['layout.show', { id: 'panel' }], ['layout.select', { stack: 'panel-body', child: 'waiting' }]]);
  assert.deepEqual(JSON.parse(JSON.stringify(ns.PANEL)), { column: node('panel').id, stack: stack.id, child: node('waiting').id }, 'ボタンの向き先がレイアウトに無い');
  assert.ok(!textOf(button).some((x) => /\d/.test(x)), 'ボタンに件数を出した');
});

// Flow (ADR-0042 decision 13, acceptance 11): sample ledgers as the board reads them.
const ledgerOf = (slug, extra = {}) => ({ root: `/cell/projects/${slug}`, slug, name: slug, columns: ['todo', 'queued', 'doing', 'review', 'done'], repos: [], phases: [], ...extra });
const issueOf = (slug, id, fields) => ({ id, slug, root: `/cell/projects/${slug}`, key: `${slug}/${id}`, title: id, ...fields });
// Ten issues: a parent with five under it, a review waiting on the owner, what waits behind it, and
// done issues with nothing open near them.
const TEN = [
  issueOf('sample-site', 'S-0001', { status: 'todo' }),
  issueOf('sample-site', 'S-0002', { status: 'done', parent: 'S-0001' }),
  issueOf('sample-site', 'S-0003', { status: 'doing', parent: 'S-0001', after: ['S-0002'] }),
  issueOf('sample-site', 'S-0004', { status: 'todo', parent: 'S-0001', after: ['S-0003'] }),
  issueOf('sample-site', 'S-0005', { status: 'queued', parent: 'S-0001', after: ['S-0004', 'S-0003'] }),
  issueOf('sample-site', 'S-0006', { status: 'todo', parent: 'S-0001', after: ['S-0005', 'S-0002'] }),
  issueOf('sample-site', 'S-0007', { status: 'done' }),
  issueOf('sample-site', 'S-0008', { status: 'review' }),
  issueOf('sample-site', 'S-0009', { status: 'todo', after: ['S-0008'], not_before: '2026-12-01' }),
  issueOf('sample-site', 'S-0010', { status: 'done', parent: 'S-0007' }),
];
const flowShape = (rows) => Array.from(rows, (r) => r.kind === 'task' ? `${'  '.repeat(r.depth)}${r.task.id}@${r.stage}${r.faded ? ' faded' : ''}${r.wait ? ` ${r.wait}` : ''}` : `${'  '.repeat(r.depth)}[${r.kind}] ${r.label}`);

await check('kanban-board: Flow は親の課題・課題の木を作り、待つ相手のいない課題を 1 段目に、前の課題を 2 つ待つ課題を後ろのほうの相手の 1 つ右に置く。済んだ相手は数えない（ADR-0042 受け入れ条件 11。課題 10 件の見本）', async () => {
  const { flowRows } = await load(`${HOST}/kanban-board`, { window: () => ({ platform: 'linux' }) });
  const waits = new Map([['sample-site/S-0008', 'merge']]);
  const rows = flowRows([ledgerOf('sample-site')], TEN, waits, { all: false });
  assert.deepEqual(flowShape(rows), [
    'S-0008@1 merge',        // holds an owner wait: first
    'S-0001@1',              // holds work in progress (S-0003)
    '  S-0003@1',            // waits only on S-0002, which is done
    '  S-0002@1 faded',      // done, shown faded while its parent has open issues
    '  S-0004@2',
    '  S-0005@3',            // waits on S-0004 (stage 2) and S-0003 (stage 1): one right of S-0004
    '  S-0006@4',
    'S-0009@2',              // behind S-0008
  ], 'Flow の木と段が違う（S-0007・S-0010 は済んで近くに開いた課題が無いので出ない）');
  assert.ok(!rows.some((r) => r.kind === 'milestone'), 'phases の無いプロジェクトにマイルストンの段が出た');
  // Folding a parent hides what is under it.
  const folded = flowRows([ledgerOf('sample-site')], TEN, waits, { folded: new Set(['task:sample-site/S-0001']) });
  assert.deepEqual(flowShape(folded), ['S-0008@1 merge', 'S-0001@1', 'S-0009@2']);
  // A filter that hides what an issue waits on doesn't move it to stage 1.
  const only = flowRows([ledgerOf('sample-site'), ledgerOf('x')], TEN, waits, { visible: (x) => ['S-0005', 'S-0009'].includes(x.id) });
  assert.deepEqual(flowShape(only), ['S-0009@2', 'S-0005@3'], '絞り込みで隠れた待つ相手を段に数えていない');
  // Every issue in a cycle is stage 1; what waits on the cycle sits one right of it.
  const cycle = [
    issueOf('sample-site', 'S-0001', { status: 'todo', after: ['S-0002'] }),
    issueOf('sample-site', 'S-0002', { status: 'todo', after: ['S-0001'] }),
    issueOf('sample-site', 'S-0003', { status: 'todo', after: ['S-0005'] }),
    issueOf('sample-site', 'S-0004', { status: 'todo', after: ['S-0003'] }),
    issueOf('sample-site', 'S-0005', { status: 'todo', after: ['S-0004'] }),
    issueOf('sample-site', 'S-0006', { status: 'todo', after: ['S-0002', 'S-0004'] }),
  ];
  assert.deepEqual(flowShape(flowRows([ledgerOf('sample-site')], cycle, new Map())).sort(),
    ['S-0001@1', 'S-0002@1', 'S-0003@1', 'S-0004@1', 'S-0005@1', 'S-0006@2'], '循環の課題が段 1 でない');
});

await check('kanban-board: Flow は全部のプロジェクトでプロジェクトの段を、phases のあるプロジェクトだけマイルストンの段を phases の順に出し、phase の無い課題を No milestone にまとめる（ADR-0042 受け入れ条件 11。phases のある見本）', async () => {
  const { flowRows } = await load(`${HOST}/kanban-board`, { window: () => ({ platform: 'linux' }) });
  const road = ledgerOf('road', { name: 'Road map', phases: [{ id: 'm1', title: 'Beta' }, { id: 'm2', title: 'Launch' }] });
  const tasks = [
    issueOf('road', 'R-0001', { status: 'todo', phase: 'm2' }),
    issueOf('road', 'R-0002', { status: 'doing', phase: 'm1' }),
    issueOf('road', 'R-0003', { status: 'todo', phase: 'm1', after: ['R-0002'] }),
    issueOf('road', 'R-0004', { status: 'todo' }),
    issueOf('sample-site', 'S-0001', { status: 'todo' }),
  ];
  const rows = flowRows([ledgerOf('sample-site'), road], tasks, new Map(), { all: true });
  assert.deepEqual(flowShape(rows), [
    '[project] Road map',    // holds work in progress: above sample-site
    '  [milestone] Beta',
    '    R-0002@1',
    '    R-0003@2',
    '  [milestone] Launch',
    '    R-0001@1',
    '  [milestone] No milestone',
    '    R-0004@1',
    '[project] sample-site',
    '  S-0001@1',
  ]);
  const one = flowRows([road], tasks.filter((x) => x.slug === 'road'), new Map(), { all: false });
  assert.ok(!one.some((r) => r.kind === 'project'), 'ひとつのプロジェクトの表示にプロジェクトの段が出た');
});

await check('kanban-board: Flow の印は andon waiting --json（version 1）をプロジェクトのフォルダで呼んで描く。command が無い・違う version のときは印を出さず、ボードは今までどおり読む（ADR-0042 決めたこと 14）', async () => {
  const tmp = await mkdtemp(path.join(os.tmpdir(), 'kanban-flow-'));
  try {
    const root = `${tmp}/projects/sample-site`;
    await mkdir(`${root}/kanban/issues/S-0008`, { recursive: true });
    await mkdir(`${root}/kanban/issues/S-0009`, { recursive: true });
    await writeFile(`${root}/kanban/config.yaml`, 'version: 3\n');
    await writeFile(`${root}/kanban/issues/S-0008/issue.yaml`, 'title: Review\nstatus: review\n');
    await writeFile(`${root}/kanban/issues/S-0009/issue.yaml`, 'title: Next\nstatus: todo\nafter: [S-0008]\n');
    let answer = null;
    const calls = [];
    const ns = await load(`${HOST}/kanban-board`, {
      window: () => ({ platform: 'linux' }), focused: () => null, panes: () => [], spaces: () => [{ active: true, folder: root }],
      agents: () => [], terminals: () => [], props: () => ({}), watchFolder: async () => { throw new Error('no watching here'); },
      readTextFile: async () => { throw new Error('read without the fixed script'); },
      exec: async (cmd, a, opts = {}) => {
        if (cmd === 'andon') { calls.push([cmd, [...a], opts.cwd]); return answer; }
        if (cmd === 'git') return { code: 128, stdout: '', stderr: 'not a git repository' };
        const r = spawnSync(cmd, a, { cwd: opts.cwd, encoding: 'utf8' });
        return { code: r.status, stdout: r.stdout, stderr: r.stderr };
      },
    });
    const read = async (out) => {
      answer = out;
      const b = new ns.default();
      b.reset();
      // Everything the board spawns (the read, then the marks), run to its end.
      const spawned = [];
      const cx = { notify: () => {}, spawn: (fn) => { spawned.push(fn(cx)); } };
      b.refresh(cx);
      for (let i = 0; i < spawned.length; i++) await spawned[i];
      return b;
    };
    const items = (version) => JSON.stringify({ version, at: '2026-11-21T10:00:00Z', project: 'sample-site', slots: {}, items: [{ kind: 'merge', id: 'S-0008', title: 'Review', since: '2026-11-21T04:43:35Z', holds: ['S-0009'] }] });
    let b = await read({ code: 0, stdout: items(1), stderr: '' });
    assert.equal(b.failure, null, `ボードが読めない: ${JSON.stringify(b.failure)}`);
    assert.deepEqual(calls.at(-1), ['andon', ['waiting', '--json'], root], 'andon waiting --json をプロジェクトのフォルダで呼んでいない');
    assert.deepEqual(Array.from(b.waits, ([k, v]) => `${k}=${v}`), ['sample-site/S-0008=merge']);
    const marks = (board) => Array.from(ns.flowRows(board.data.ledgers, board.data.tasks, board.waits).filter((r) => r.wait), (r) => r.task.id);
    assert.deepEqual(marks(b), ['S-0008']);
    for (const [why, out] of [['version 2', { code: 0, stdout: items(2), stderr: '' }], ['command が無い', { code: 127, stdout: '', stderr: 'andon: not found' }], ['読めない出力', { code: 0, stdout: 'not json', stderr: '' }]]) {
      b = await read(out);
      assert.equal(b.failure, null, `${why}: ボードが読めないことになった`);
      assert.equal(b.data.tasks.length, 2, `${why}: 課題を読んでいない`);
      assert.deepEqual(marks(b), [], `${why}: 印を出した`);
    }
    // A command that never answers stops only the marks: the ledger is read and read again.
    answer = new Promise(() => {});
    const hung = new ns.default();
    hung.reset();
    const spawned = [];
    const cx = { notify: () => {}, spawn: (fn) => { spawned.push(fn(cx)); } };
    hung.refresh(cx);
    await Promise.race([spawned[0], new Promise((r) => setTimeout(r, 3000))]);
    assert.equal(hung.reading, false, '返らない andon waiting でボードの読みが止まった');
    assert.equal(hung.data?.tasks.length, 2, '返らない andon waiting で課題を読んでいない');
    hung.refresh(cx);
    await spawned.at(-1);
    assert.ok(hung.checkedAt, '読み直していない');
  } finally {
    await rm(tmp, { recursive: true, force: true });
  }
});

await check('kanban-board: Flow はボタンで開き、棒に状態の名前、オーナー待ちに印、日付を待つ課題に日付を書く', async () => {
  const ui = recordingUi();
  const ns = await load(`${HOST}/kanban-board`, { window: () => ({ platform: 'linux' }), theme: () => ({ ...THEME, done: '#22aa55' }),
    focused: () => null, panes: () => [], spaces: () => [], agents: () => [], terminals: () => [], props: () => ({}) }, ui);
  const b = new ns.default();
  b.init({}, { focus_handle: () => ({ focus() {}, is_focused: () => false }), timer: { every() {}, after() {} }, spawn: () => {} });
  b.where = { key: JSON.stringify([null, null]), cwd: null, on: null };
  const ledger = ledgerOf('sample-site');
  b.root = ledger.root; b.searched = true;
  b.data = { ledgers: [ledger], tasks: TEN, columns: ledger.columns, unreadable: [] };
  b.waits = new Map([['sample-site/S-0008', 'merge']]);
  assert.ok(byId(b.render({}), 'wb-view-flow'), 'Flow のボタンが無い');
  b.view = 'flow';
  // render() re-reads when the place changes; the fake has no folder, so read the Flow body itself.
  const tree = b.flowView({ ...THEME, done: '#22aa55' });
  const text = (n) => walk(n).flatMap((x) => args(x, 'child').map((a) => a[0])).filter((c) => typeof c === 'string');
  assert.ok(text(byId(tree, 'wb-bar-sample-site/S-0003')).includes('Doing'), 'S-0003 の棒に状態の名前が無い');
  assert.deepEqual(text(byId(tree, 'wb-wait-sample-site/S-0008')), ['Merge'], 'オーナー待ちの印が無い');
  assert.ok(text(byId(tree, 'wb-flow-task:sample-site/S-0009')).includes('from 2026-12-01'), '日付を待つ課題に日付が無い');
  assert.ok(!byId(tree, 'wb-flow-task:sample-site/S-0007'), '済んで近くに開いた課題が無い S-0007 が出た');
  // Filtered to S-0005 alone, its bar still sits in stage 3 (two stages of 128px to its left).
  b.filter = 'S-0005';
  const filtered = b.flowView({ ...THEME, done: '#22aa55' });
  const widths = walk(byId(filtered, 'wb-flow-task:sample-site/S-0005')).flatMap((n) => args(n, 'w').map((a) => a[0]));
  assert.ok(widths.includes(256), `絞り込んだ S-0005 の棒が段 3 にない: ${widths}`);
  assert.ok(!byId(filtered, 'wb-flow-task:sample-site/S-0004'), '絞り込みで隠した S-0004 が出た');
});

// What kanban-board runs (ADR-0042 acceptance 12): the programs named in its exec() calls, against a
// fixed list. Nothing that writes the ledger, no network.
await check('kanban-board: exec を空白や別名で呼ばず、呼び先は決まった名前だけ（andon は waiting --json だけ）。README の What it runs に andon waiting の行がある（ADR-0042 受け入れ条件 12）', async () => {
  const src = await readFile(`${HOST}/kanban-board/main.js`, 'utf8');
  const programs = new Set();
  const calls = [...src.matchAll(/\bexec\s*\(([^,]+),/g)].map((m) => m[1]);
  // exec only as a call (and the import): handed on under another name, its calls would not be seen.
  const other = src.split('\n').filter((l) => !/^import .* from "herdr";$/.test(l)).join('\n').match(/\bexec\b(?!\s*\(|'s)/g);
  assert.equal(other, null, `exec を呼び出しのほかで使っている: ${other}`);
  assert.ok(calls.length >= 6, `exec の呼び出しを拾えていない: ${calls}`);
  for (const first of calls) {
    // The programs, not what a ternary compares (`platform === "windows" ? "explorer.exe" : …`).
    const names = [...first.replace(/===\s*"[^"]*"/g, '').matchAll(/"([^"]+)"/g)].map((m) => m[1]);
    assert.ok(names.length, `呼び先が文字列でない exec がある: exec(${first}`);
    names.forEach((n) => programs.add(n));
  }
  assert.deepEqual([...programs].sort(), ['andon', 'explorer.exe', 'git', 'ls', 'open', 'powershell', 'sh', 'xdg-open']);
  const andon = [...src.matchAll(/\bexec\s*\(\s*"andon",\s*(\[[^\]]*\])/g)].map((m) => m[1]);
  assert.deepEqual(andon, ['["waiting", "--json"]'], `andon の呼び方: ${andon}`);
  assert.doesNotMatch(src, /\bfetch\(|XMLHttpRequest|WebSocket|from "(node:)?(http|https|net)"/, 'ネットワークを使う');
  const readme = await readFile(`${HOST}/kanban-board/README.md`, 'utf8');
  assert.match(readme, /^\| `exec\("andon", \["waiting", "--json"\], \{cwd: <project>, on\}\)`/m, 'README の What it runs に andon waiting の行が無い');
});

// The inbox reading owner waits (ADR-0043): a fake exec answers `andon waiting --json --project <slug>`,
// a fake clock moves the 10-second window, and copies can share one store as the App's shared() does.
const T0 = Date.parse('2026-11-21T12:00:00Z');
const waitOut = (project, items) => ({ code: 0, stdout: JSON.stringify({ version: 1, at: '2026-11-21T12:00:00Z', project,
  slots: { limit: 4, working: 1, waiting: items.length, free: 3, waiting_full: false }, items }), stderr: '' });
const wait = (kind, id, since) => ({ kind, id, title: `title of ${id}`, since, holds: [] });
const PANE_LAYOUT = { id: 'root', children: [{ id: 'panel', children: [{ id: 'panel-body', type: 'stack', children: [{ id: 'waiting', type: 'block', content: { mod: 'waiting' } }] }] }] };
async function inboxCell(state, { store = {}, clock = { t: T0 }, reply = (a) => waitOut(a[3], state.items?.[a[3]] ?? []), layoutTree = null, ui = recordingUi(), calls = { exec: [], run: [] } } = {}) {
  const FakeDate = class extends Date { static now() { return clock.t; } };
  const ns = await load(`${HOST}/inbox`, {
    agents: () => state.agents ?? [], terminals: () => state.terms ?? [], spaces: () => [], panes: () => [], machines: () => state.machines ?? [],
    focused: () => state.focused ?? null, theme: () => THEME, setAgentOrder: () => {}, nextCommand: () => new Promise(() => {}),
    shared: (k) => store[k] ?? null, setShared: (k, val) => { store[k] = JSON.parse(JSON.stringify(val)); },
    run: (cmd, a) => calls.run.push([cmd, JSON.parse(JSON.stringify(a))]),
    exec: async (cmd, a, opts = {}) => { calls.exec.push([cmd, [...a], { on: opts.on ?? null }]); return reply(a, opts); },
    layout: () => layoutTree,
    hc: () => { throw new Error('the inbox called hc'); }, notify: () => { throw new Error('the inbox posted a notification'); },
  }, ui, { Date: FakeDate });
  const v = new ns.default();
  const pending = [], timers = [];
  let track = false;
  const cx = { focus_handle: () => ({ focus() {}, is_focused: () => false }), timer: { every: (ms, fn) => timers.push([ms, fn]), after() {} },
    spawn: (fn) => { const p = fn(cx); if (track) pending.push(p); return p; }, notify() {}, stop_propagation() {} };
  v.init({}, cx);
  track = true;
  const settle = async () => { while (pending.length) await pending.shift(); };
  // A render (what a terminal's change brings), then the reads it started.
  const look = async () => { v.render(cx); await settle(); return v.model(); };
  const tick = async () => { for (const [, fn] of timers) fn(cx); await settle(); return v.model(); };
  // Stop waiting for reads that never come back (a hung call).
  const forget = () => { pending.length = 0; };
  return { v, calls, store, clock, cx, look, tick, timers, forget, draw: () => v.render(cx) };
}
const cos = (address, project, status = 'idle', extra = {}) => sampleTerm(address, status, { project, role: 'chief-of-staff' }, extra);
const ce = (address, project, status = 'idle', extra = {}) => sampleTerm(address, status, { project, role: 'chief-engineer', issue: 'S-0009', title: 'Design' }, extra);
const plain = (x) => JSON.parse(JSON.stringify(x));
const lampOf = (store) => plain(store['inbox.lamp']);
const lit = (store) => store['inbox.lantern'] ?? null;
const LAMP0 = { light: null, failed: 0, blocked: 0, merge: 0, decision: 0, reply: 0, unread: 0, unreadable: 0 };

await check('inbox: オーナー待ちを読めているプロジェクトでは、役の返事は Needs you に出ず Projects の行の未読の印になり、行燈は消灯のまま。見ると消え、ペインの無い役は開いた turn の間は付かない。news.replied の有無で変わらない（ADR-0043 受け入れ条件 1・6）', async () => {
  const state = { terms: [cos('cell/t1', 'sample-site'), ce('cell/t2', 'sample-site')], machines: [{ name: 'cell' }], items: { 'sample-site': [] } };
  const { v, store, look, draw } = await inboxCell(state);
  let m = await look();
  assert.deepEqual(lampOf(store), LAMP0);
  // 1. The Chief of Staff's unseen reply: Projects, an unread mark, the lantern unlit.
  state.terms[0].status = 'done';
  m = await look();
  assert.ok(!m.needs.some((i) => i.address === 'cell/t1'), '役の返事が Needs you に出た');
  const row = m.projects.find((i) => i.address === 'cell/t1');
  assert.ok(row?.unread && !row.reply && row.kind === null, `未読の印が付かない: ${JSON.stringify(row)}`);
  assert.equal(lit(store), null, '返事で行燈が灯った');
  assert.deepEqual(lampOf(store), { ...LAMP0, unread: 1 });
  const mark = v.mark(THEME, row);
  assert.equal(last(mark, 'id'), 'unread-cell/t1');
  assert.deepEqual(colors(mark), [THEME.muted], '未読の印が muted の点でない');
  assert.ok(!colors(draw()).includes(AMBER), '琥珀を塗った');
  // 2. The same with the label the dispatcher writes for an acknowledged turn.
  state.terms[0].labels['news.replied'] = '0';
  m = await look();
  assert.ok(m.projects.find((i) => i.address === 'cell/t1')?.unread, 'news.replied で未読の印が消えた');
  // 3. A Chief Engineer's reply too.
  state.terms[1].status = 'done';
  m = await look();
  assert.ok(m.projects.find((i) => i.address === 'cell/t2')?.unread && lit(store) === null);
  assert.equal(lampOf(store).unread, 2);
  // 4. Looked at (Herdr turns done into idle): the mark goes.
  state.terms[0].status = 'idle';
  state.terms[1].status = 'idle';
  m = await look();
  assert.ok(!m.projects.some((i) => i.unread), '見たのに印が残る');
  // 5. A paneless role: opening it is looking, for that turn.
  state.terms[0].status = 'done';
  m = await look();
  v.open(m.projects.find((i) => i.address === 'cell/t1'));
  m = await look();
  assert.ok(!m.projects.find((i) => i.address === 'cell/t1').unread, '開いた turn で印が付いたまま');
  state.terms[0].turn = 2;
  assert.ok((await look()).projects.find((i) => i.address === 'cell/t1').unread, '次の turn で印が付かない');
});

await check('inbox: オーナー待ちを読めていても、止まった端末（permission・question・auth・種類なし）は琥珀、失敗は赤、両方なら赤（ADR-0043 受け入れ条件 2・6）', async () => {
  const state = { terms: [cos('cell/t1', 'sample-site'),
    sampleTerm('cell/t2', 'blocked', { project: 'sample-site', issue: 'S-0002' }, { report: { kind: 'permission' } }),
    sampleTerm('cell/t3', 'blocked', { project: 'sample-site', issue: 'S-0003' }, { report: { kind: 'question' } }),
    sampleTerm('cell/t4', 'blocked', { project: 'sample-site', issue: 'S-0004' }, { report: { kind: 'auth' } }),
    sampleTerm('cell/t5', 'blocked', { project: 'sample-site', issue: 'S-0005' })], machines: [{ name: 'cell' }], items: { 'sample-site': [] } };
  const { store, look } = await inboxCell(state);
  let m = await look();
  assert.deepEqual(plain(m.needs.map((i) => i.kind)).sort(), ['Auth', 'Permission', 'Question', null].sort());
  assert.deepEqual(lampOf(store), { ...LAMP0, light: 'amber', blocked: 4 });
  state.terms.push(sampleTerm('cell/t6', 'error', { project: 'sample-site', issue: 'S-0006' }));
  m = await look();
  assert.deepEqual(lampOf(store), { ...LAMP0, light: 'red', failed: 1, blocked: 4 });
  assert.equal(store['inbox.lantern'], 'red');
  state.terms.splice(1, 4);
  await look();
  assert.deepEqual(lampOf(store), { ...LAMP0, light: 'red', failed: 1 });
});

await check('inbox: merge 2 件・decision 1 件・task 1 件のプロジェクトは Needs you の上に件数の 1 行が出て琥珀。task だけなら行も灯りも無い。2 つのプロジェクトなら 2 行（ADR-0043 受け入れ条件 3・6）', async () => {
  const items = [wait('merge', 'S-0021', '2026-11-21T10:00:00Z'), wait('decision', 'S-0030', '2026-11-21T09:00:00Z'),
    wait('merge', 'S-0022', '2026-11-21T11:30:00Z'), wait('task', 'S-0031', '2026-11-20T08:00:00Z')];
  const state = { terms: [cos('cell/t1', 'sample-site')], machines: [{ name: 'cell' }], items: { 'sample-site': items } };
  const { store, look, draw } = await inboxCell(state);
  const m = await look();
  assert.equal(m.owed.length, 1);
  const tree = draw();
  const line = byId(tree, 'owed-cell/sample-site');
  assert.ok(line, '件数の行が無い');
  assert.deepEqual(plain(textOf(line)), ['sample-site', '2 to merge · 1 to decide', '3h'], '件数の行の中身（いちばん古い待ちは task を数えず 09:00）');
  assert.ok(colors(line).includes(AMBER), '件数の行に琥珀の印が無い');
  const all = textOf(tree);
  assert.ok(!all.includes('Nothing needs you.'), '「Nothing needs you.」が出た');
  // The line sits above Needs you, with no heading of its own.
  state.terms.push(sampleTerm('cell/t2', 'blocked', { project: 'sample-site', issue: 'S-0002' }));
  await look();
  const order = textOf(draw());
  assert.ok(order.indexOf('2 to merge · 1 to decide') < order.indexOf('Needs you'), '件数の行が Needs you の上にない');
  state.terms.pop();
  await look();
  assert.deepEqual(lampOf(store), { ...LAMP0, light: 'amber', merge: 2, decision: 1 });
  assert.equal(store['inbox.lantern'], 'amber');
  // Hand work alone: no line, unlit.
  const only = await inboxCell({ ...state, items: { 'sample-site': [items[3]] } });
  assert.equal((await only.look()).owed.length, 0, 'task だけで行が出た');
  assert.deepEqual(lampOf(only.store), LAMP0);
  assert.ok(textOf(only.draw()).includes('Nothing needs you.'));
  // Two projects: two lines, by name.
  const two = await inboxCell({ terms: [cos('cell/t1', 'sample-site'), cos('cell/t9', 'other-site')], machines: [{ name: 'cell' }],
    items: { 'sample-site': [items[0]], 'other-site': [items[1]] } });
  assert.deepEqual(plain((await two.look()).owed.map((w) => [w.project, w.merge, w.decision])), [['other-site', 0, 1], ['sample-site', 1, 0]]);
  assert.deepEqual(plain(textOf(byId(two.draw(), 'owed-cell/other-site')).slice(0, 2)), ['other-site', '1 to decide']);
  assert.deepEqual(lampOf(two.store), { ...LAMP0, light: 'amber', merge: 1, decision: 1 });
});

await check('inbox: 件数の行を押すとオーナー待ちのペインを開き、ペインが無ければそのプロジェクトの Chief of Staff へ行く。マシンを絞ると外の件数の行は消えて「ほかのマシンで待っている数」に足され、行燈は変わらない（ADR-0043 受け入れ条件 4・6）', async () => {
  const items = { 'sample-site': [wait('merge', 'S-0021', '2026-11-21T10:00:00Z'), wait('merge', 'S-0022', '2026-11-21T10:00:00Z')], 'other-site': [wait('decision', 'B-0001', '2026-11-21T11:00:00Z')] };
  const terms = [cos('cell/t1', 'sample-site'), cos('lab/t1', 'other-site')];
  const machines = [{ name: 'cell' }, { name: 'lab' }];
  const click = async (c) => { const tree = c.draw(); args(byId(tree, 'owed-cell/sample-site'), 'on_click').at(-1)[0]({}, c.cx); return c.calls.run; };
  // With the pane: show the panel, select the waiting block.
  const withPane = await inboxCell({ terms, machines, items }, { layoutTree: PANE_LAYOUT });
  await withPane.look();
  assert.deepEqual(await click(withPane), [['layout.show', { id: 'panel' }], ['layout.select', { stack: 'panel-body', child: 'waiting' }]]);
  // Without it: the project's Chief of Staff (paneless: peek; in a pane: focus).
  const without = await inboxCell({ terms, machines, items });
  await without.look();
  assert.deepEqual(await click(without), [['terminal.peek', { terminal: 'cell/t1' }]]);
  const paned = await inboxCell({ terms, machines, items, agents: [{ address: 'cell/t1', status: 'idle', program: sampleProgram, title: 'CoS', space_name: 'sample-site' }] });
  await paned.look();
  assert.deepEqual(await click(paned), [['agent.focus', { terminal: 'cell/t1' }]]);
  // Picking one machine.
  const before = lampOf(without.store);
  without.v.picked = ['cell'];
  const m = await without.look();
  assert.deepEqual(plain(m.owed.map((w) => w.project)), ['sample-site'], 'ほかのマシンの件数の行が残る');
  assert.equal(m.elsewhere, 1, 'ほかのマシンの待ちが数に足されない');
  assert.ok(textOf(without.draw()).includes('+1 waiting elsewhere'));
  assert.deepEqual(lampOf(without.store), before, 'マシンを絞ると行燈の値が変わった');
  // A filter doesn't hide a count line, nor change the lamp.
  without.v.picked = null;
  without.v.filter = 'nothing matches this';
  assert.equal((await without.look()).owed.length, 2, '検索で件数の行が消えた');
  assert.deepEqual(lampOf(without.store), before, '検索で行燈の値が変わった');
});

await check('inbox: andon waiting が 3 回続けて読めない（失敗・JSON でない・version 2・project 違い）と、役の返事が Reply で Needs you に出て琥珀、理由の 1 行が出る。2 回までは前の出力、読めれば戻る。Chief of Staff の無いプロジェクトの Chief Engineer も Reply（ADR-0043 受け入れ条件 5・6）', async () => {
  const good = (a) => waitOut(a[3], [wait('merge', 'S-0021', '2026-11-21T10:00:00Z')]);
  const bad = {
    'fails': () => ({ code: 1, stdout: '', stderr: 'andon waiting: kanban が見つからない\nmore' }),
    'not JSON': () => ({ code: 0, stdout: 'usage: andon', stderr: '' }),
    'version 2': (a) => ({ code: 0, stdout: JSON.stringify({ version: 2, project: a[3], items: [] }), stderr: '' }),
    'another project': () => waitOut('other-site', []),
    'throws': () => { throw new Error('spawn andon ENOENT'); },
  };
  for (const [name, fail] of Object.entries(bad)) {
    let answer = good;
    const state = { terms: [cos('cell/t1', 'sample-site', 'done')], machines: [{ name: 'cell' }] };
    const c = await inboxCell(state, { reply: (a) => answer(a) });
    let m = await c.look();
    assert.ok(m.projects.find((i) => i.address === 'cell/t1')?.unread, `${name}: 読めたのに未読の印でない`);
    answer = fail;
    for (let n = 1; n <= 2; n++) {
      c.clock.t += 31000;
      m = await c.tick();
      assert.equal(m.owed.length, 1, `${name}: ${n} 回目で前の出力を使わない`);
      assert.ok(!m.needs.length && m.lost.length === 0, `${name}: ${n} 回目で読めない扱いにした`);
    }
    c.clock.t += 31000;
    m = await c.tick();
    assert.deepEqual(plain(m.needs.map((i) => [i.address, i.kind])), [['cell/t1', 'Reply']], `${name}: 3 回目で Reply にならない`);
    assert.equal(m.owed.length, 0, `${name}: 読めないのに件数の行が残る`);
    assert.equal(lit(c.store), 'amber');
    assert.deepEqual(lampOf(c.store), { ...LAMP0, light: 'amber', reply: 1, unreadable: 1 }, `${name}: 行燈の内訳`);
    const why = textOf(byId(c.draw(), 'lost-cell/sample-site'));
    assert.equal(why.length, 1, `${name}: 理由が 1 行でない`);
    assert.match(why[0], /^sample-site: replies light the lantern \(.+\)$/);
    assert.ok(!why[0].includes('\n'));
    answer = good;
    c.clock.t += 31000;
    m = await c.tick();
    assert.ok(!m.needs.length && m.projects.find((i) => i.address === 'cell/t1').unread && m.lost.length === 0, `${name}: 読めたのに戻らない`);
    assert.deepEqual(lampOf(c.store), { ...LAMP0, light: 'amber', merge: 1, unread: 1 });
  }
  // Never read yet: lights by replies as before, with no reason line.
  const fresh = await inboxCell({ terms: [cos('cell/t1', 'sample-site', 'done')], machines: [{ name: 'cell' }] });
  assert.deepEqual(plain(fresh.v.model().needs.map((i) => i.kind)), ['Reply'], 'まだ読めていない間に Reply で灯らない');
  assert.equal(fresh.v.model().lost.length, 0);
  // A Chief Engineer whose project has no Chief of Staff here.
  const alone = await inboxCell({ terms: [cos('cell/t1', 'sample-site'), ce('cell/t2', 'other-site', 'done')], machines: [{ name: 'cell' }], items: { 'sample-site': [] } });
  const m = await alone.look();
  assert.deepEqual(plain(m.needs.map((i) => [i.address, i.kind])), [['cell/t2', 'Reply']]);
  assert.deepEqual(plain(m.lost.map((s) => s.project)), ['other-site']);
  assert.match(textOf(byId(alone.draw(), 'lost-cell/other-site'))[0], /no Chief of Staff/);
  assert.deepEqual(lampOf(alone.store), { ...LAMP0, light: 'amber', reply: 1, unreadable: 1 });
});

await check('inbox: exec で呼ぶのは andon waiting --json --project <名前> だけ。名前が決めた形でなければ呼ばず、同じ組の Chief of Staff が 2 つでも 1 回、写しが 2 つでも 10 秒に 1 回で行燈は同じ。タイマーは 1 本（ADR-0043 受け入れ条件 7）', async () => {
  const state = { terms: [cos('cell/t1', 'sample-site'), cos('cell/t2', 'sample-site'), cos('cell/t3', 'Bad/../x'), cos('t4', 'sample-site')],
    machines: [{ name: 'cell' }], items: { 'sample-site': [wait('decision', 'S-0030', '2026-11-21T09:00:00Z')] } };
  const store = {}, calls = { exec: [], run: [] }, clock = { t: T0 };
  const one = await inboxCell(state, { store, calls, clock });
  const two = await inboxCell(state, { store, calls, clock });
  assert.deepEqual(one.timers.map(([ms]) => ms), [30000], 'タイマーが 1 本の 30 秒でない');
  await one.look();
  await two.look();
  await two.tick();
  const want = [['andon', ['waiting', '--json', '--project', 'sample-site'], { on: 'cell' }], ['andon', ['waiting', '--json', '--project', 'sample-site'], { on: null }]];
  assert.deepEqual(calls.exec, want, '呼び出しが決まった一覧でない（組ごとに 1 回、名前の形の違うものは呼ばない）');
  assert.equal(lit(one.store), 'amber');
  assert.deepEqual(one.store, two.store);
  // The comparison sees a call it should: past the window, a state change reads again.
  clock.t += 11000;
  state.terms[0].status = 'working';
  await one.look();
  await two.look();
  assert.equal(calls.exec.length, 3, '状態が変わって 10 秒過ぎても読み直さない（比べる式が何も見ていない）');
  assert.deepEqual(calls.exec.at(-1), want[0]);
  // Within the window, a change doesn't call again.
  clock.t += 2000;
  state.terms[0].status = 'done';
  await one.look();
  await two.look();
  assert.equal(calls.exec.length, 3, '10 秒の中で 2 回呼んだ');
  const src = await readFile(`${HOST}/inbox/main.js`, 'utf8');
  assert.deepEqual(plain(src.match(/(?<![.\w])exec\([^)]*/g)), ['exec("andon", ["waiting", "--json", "--project", p.project], { on: p.machine === LOCAL ? null : p.machine }'], 'main.js の exec の呼び出しが 1 つでない');
  assert.doesNotMatch(src, /\bfetch\(|WebSocket|writeTextFile|\bhc\(/);
  // The andon mod's lantern still reads only inbox.lantern.
  assert.match(await readFile(`${HOST}/andon/brand.js`, 'utf8'), /const LANTERN = "inbox\.lantern";/);
  const readme = await readFile(`${HOST}/inbox/README.md`, 'utf8');
  assert.match(readme, /\| `exec\("andon", \["waiting", "--json", "--project", slug\], \{on\}\)` \|/, 'README の What it runs に andon waiting の行が無い');
});

await check('inbox: 名前の形が違うラベル（空白・大文字・引用符・パス）では呼ばず、出力の version・project の変種は読めない扱い。返らない呼び出しは 60 秒で諦めて読み直し、形の違う控えでは落ちない', async () => {
  for (const label of [' sample-site', 'sample-site ', 'Sample-site', "'sample-site'", 'sample site', 'a/b', '../x', '-']) {
    const c = await inboxCell({ terms: [cos('cell/t1', label)], machines: [{ name: 'cell' }] });
    await c.look();
    await c.tick();
    // '-' is shaped like a slug and is called: the check sees a call it should.
    assert.equal(c.calls.exec.length, label === '-' ? 1 : 0, `「${label}」で呼んだか、呼ぶべきものを呼ばなかった`);
  }
  const { parseWaiting } = await load(`${HOST}/inbox`, { machines: () => [], setAgentOrder: () => {}, nextCommand: () => null, shared: () => null, setShared: () => {} });
  const out = (o) => ({ code: 0, stdout: JSON.stringify(o), stderr: '' });
  for (const [r, want] of [
    [out({ version: '1', project: 'sample-site', items: [] }), /version 1$/], [out({ project: 'sample-site', items: [] }), /version undefined/],
    [out({ version: 1, project: 'sample-site ', items: [] }), /answered for sample-site$/], [out({ version: 1, project: 'Sample-site', items: [] }), /answered for/],
    [out({ version: 1, project: 'sample-site', items: {} }), /no items/], [{ code: 0, stdout: '', stderr: '' }, /no JSON/],
    [{ code: 127, stdout: '', stderr: '\nsh: andon: not found\n' }, /exited 127: sh: andon: not found$/]]) {
    const [items, why] = plain(parseWaiting(r, 'sample-site'));
    assert.equal(items, null, `読めない出力を読んだ: ${JSON.stringify(r)}`);
    assert.match(why, want);
  }
  assert.deepEqual(plain(parseWaiting(out({ version: 1, project: 'sample-site', items: [{ kind: 'merge', since: 'x', title: 't' }] }), 'sample-site')), [[{ kind: 'merge', since: 'x' }], null]);
  // A call that never comes back: after a minute, the timer calls again.
  let hang = true;
  const c = await inboxCell({ terms: [cos('cell/t1', 'sample-site')], machines: [{ name: 'cell' }] }, { reply: (a) => (hang ? new Promise(() => {}) : waitOut(a[3], [])) });
  c.v.render(c.cx);
  c.forget();
  assert.equal(c.calls.exec.length, 1);
  c.clock.t += 31000; for (const [, fn] of c.timers) fn(c.cx);
  assert.equal(c.calls.exec.length, 1, '返っていない呼び出しの間に、同じ写しがもう一度呼んだ');
  hang = false;
  c.clock.t += 31000;
  assert.equal((await c.tick()).lost.length, 0);
  assert.equal(c.calls.exec.length, 2, '返らない呼び出しを諦めない');
  // A store of another shape (an older or broken value) doesn't break the model.
  const odd = await inboxCell({ terms: [cos('cell/t1', 'sample-site', 'done')], machines: [{ name: 'cell' }] }, { store: { 'inbox.waiting': { 'cell/sample-site': { at: 0, ok: { items: 'nope' } } } }, reply: () => new Promise(() => {}) });
  assert.deepEqual(plain(odd.v.model().needs.map((i) => i.kind)), ['Reply']);
});

await check('inbox: 「次へ」は Needs you の行を古い順に回り、無くなれば未読の印のある役の行を古い順に回る（ADR-0043 受け入れ条件 8）', async () => {
  const state = { terms: [cos('cell/t1', 'sample-site'), ce('cell/t2', 'sample-site'), sampleTerm('cell/t3', 'idle', { project: 'sample-site', issue: 'S-0003' })],
    machines: [{ name: 'cell' }], items: { 'sample-site': [] } };
  const c = await inboxCell(state);
  await c.look();
  // The Chief Engineer replies first, then the Chief of Staff, then an issue's agent blocks.
  state.terms[0].status = 'working'; state.terms[1].status = 'working'; await c.look();
  c.clock.t += 1000; state.terms[1].status = 'done'; await c.look();
  c.clock.t += 1000; state.terms[0].status = 'done'; await c.look();
  c.clock.t += 1000; state.terms[2].status = 'blocked'; await c.look();
  const m = c.v.model();
  assert.deepEqual(plain(m.queue.map((i) => i.address)), ['cell/t3', 'cell/t2', 'cell/t1'], '次へが回る順');
  c.v.next(m);
  assert.deepEqual(c.calls.run.at(-1), ['terminal.peek', { terminal: 'cell/t3' }]);
  state.focused = { terminal: 'cell/t3' };
  c.v.next(c.v.model());
  assert.deepEqual(c.calls.run.at(-1), ['terminal.peek', { terminal: 'cell/t2' }], 'Needs you が無くなったあと未読の役へ行かない');
});

if (failed) process.exit(1);
