// The App mods Andon ships for the screen (inbox, kanban-board, context-header, files-plus) and the
// andon layout: their JavaScript parses, the layout carries the naming block, and the two places
// changed for Andon behave (context-header's name, kanban-board's ledger lookup) against a fake API.
import assert from 'node:assert/strict';
import { readFile, readdir, mkdtemp, mkdir, writeFile, symlink, rm } from 'node:fs/promises';
import os from 'node:os';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import vm from 'node:vm';

const HOST = 'package/host';
const MODS = ['andon', 'inbox', 'kanban-board', 'context-header', 'files-plus'];
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
  for (const mod of ['inbox', 'context-header', 'files-plus']) assert.ok(used.includes(`"mod":"${mod}"`), `${mod} を指していない`);
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
async function load(dir, herdr, ui = NO_UI) {
  const context = vm.createContext({ console });
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

await check('kanban-board: version のある kanban/ だけを台帳とみなし、symlink・大きすぎるファイル・alias の入れ子を読む前に拒む（固定 script を本物の sh で）', async () => {
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
    assert.equal(b.root, `${plant}/projects/sample-site`, 'version の無い kanban/ を越えて台帳を見つけていない');
    assert.equal(b.plant, plant);
    const viaLink = new Board();
    viaLink.reset();
    await viaLink.find(`${tmp}/linked`, null);
    assert.equal(viaLink.root, null, 'symlink の kanban/ を台帳とみなした');
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

await check('kanban-board: 形式 2 の台帳は読まず、手で移す 1 行を出す（ADR-0008 決めたこと 7）', async () => {
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
    assert.doesNotMatch(src, /kanban[\/\\]+tasks|task\.yaml/, `${mod} が古い台帳の path を読む`);
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

if (failed) process.exit(1);
