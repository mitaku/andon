// The App mods Andon ships for the screen (inbox, kanban-board, context-header, files-plus) and the
// andon layout: their JavaScript parses, the layout carries the naming block, and the two places
// changed for Andon behave (context-header's name, kanban-board's ledger lookup) against a fake API.
import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import vm from 'node:vm';

const HOST = 'package/host';
const MODS = ['inbox', 'kanban-board', 'context-header', 'files-plus'];
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

// Loads a mod's main.js in a VM with fake herdr, gpui and fs modules. Relative imports are loaded too.
async function load(dir, herdr) {
  const context = vm.createContext({ console });
  const synthetic = (exports) => new vm.SyntheticModule(Object.keys(exports), function () {
    for (const [k, v] of Object.entries(exports)) this.setExport(k, v);
  }, { context });
  const el = new Proxy(function () {}, { get: (_t, k) => (k === 'then' ? undefined : () => el), apply: () => el });
  const ui = { View: class {}, div: () => el, svg: () => el, image: () => el, v_flex: () => el, h_flex: () => el, TextView: {} };
  const known = {
    herdr: synthetic(herdr),
    'gpui-kit': synthetic({ View: ui.View, div: ui.div, svg: ui.svg, image: ui.image }),
    'gpui-base': synthetic({ v_flex: ui.v_flex, h_flex: ui.h_flex, TextView: ui.TextView }),
    'fs/promises': synthetic({ readdir: async () => [] }),
  };
  const cache = new Map();
  const source = async (file) => {
    if (!cache.has(file)) cache.set(file, new vm.SourceTextModule(await readFile(file, 'utf8'), { context, identifier: file }));
    return cache.get(file);
  };
  const main = await source(`${dir}/main.js`);
  await main.link(async (spec, ref) => known[spec] ?? source(path.join(path.dirname(ref.identifier), spec)));
  await main.evaluate();
  return main.namespace.default;
}

await check('context-header: Andon の担当・レビュアー・Chief Engineer はラベルから名付け、2行目で名前を繰り返さない', async () => {
  let paneList = [], termList = [], spaceList = [], current = null;
  const Header = await load(`${HOST}/context-header`, {
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
  at('p1', 't1', 'andon-sample-site-A-0002', { project: 'sample-site', task: 'A-0002', title: 'Fix login' });
  at('p2', 't2', 'andon-sample-site-A-0002', { project: 'sample-site', task: 'A-0003', title: 'Review  of\tA-0002' });
  at('p3', 't3', 'andon-sample-site-A-0001', { project: 'sample-site', role: 'chief-engineer', task: 'A-0001', title: 'Billing design' });
  at('p4', 't4', 'andon-sample-site-cos', { project: 'sample-site', role: 'chief-of-staff' });
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
});

await check('kanban-board: kanban/config.yaml に version がある所だけを台帳とみなし、chief_of_staff を読み、alias の入れ子の task.yaml を拒む', async () => {
  const fsys = new Map([
    ['/w/repo/kanban/config.yaml', 'title: not a ledger\n'],
    ['/w/plant/projects/sample-site/kanban/config.yaml', 'version: 2\nchief_of_staff: t2\nrepos: [github.com/sample/repo]\n'],
  ]);
  // A short task.yaml whose aliases nest 22 deep: 4M copies if expanded. It must be refused.
  let bomb = 'a0: &a0 [x]\n';
  for (let i = 1; i <= 22; i++) bomb += `a${i}: &a${i} [*a${i - 1}, *a${i - 1}]\n`;
  const tasks = new Map([['A-0001', 'title: Fine\nstatus: todo\n'], ['A-0002', bomb]]);
  const Board = await load(`${HOST}/kanban-board`, {
    agents: () => [], terminals: () => [], panes: () => [], spaces: () => [], focused: () => null,
    readTextFile: async (p) => { if (!fsys.has(p)) throw new Error('missing'); return fsys.get(p); },
    watchFolder: async () => {}, run: () => {}, theme: () => ({}), props: () => ({}), closeOverlay: () => {},
    window: () => ({ platform: 'linux' }),
    exec: async (cmd, args) => {
      if (cmd === 'git') return { code: 0, stdout: '/w/plant/.ghq/github.com/sample/repo/.git\n', stderr: '' };
      if (cmd === 'sh' && args[1].includes('projects/*/kanban/config.yaml')) {
        const dir = args[3];
        const hits = [...fsys].filter(([p]) => p.startsWith(`${dir}/projects/`) && p.endsWith('/kanban/config.yaml'));
        return { code: 0, stdout: hits.map(([p, t]) => `\x1e${p.split('/').at(-3)}\n${t}\n`).join(''), stderr: '' };
      }
      if (cmd === 'sh' && args[1].includes('kanban/tasks/')) {
        return { code: 0, stdout: [...tasks].map(([id, t]) => `\x1e${id}\n${t}\n`).join(''), stderr: '' };
      }
      return { code: 1, stdout: '', stderr: '' };
    },
  });
  // From inside a product's repo that has a kanban/ folder of its own: that folder is not the ledger.
  const b = new Board();
  b.reset();
  await b.find('/w/plant/.ghq/github.com/sample/repo/src', null);
  assert.equal(b.root, '/w/plant/projects/sample-site');
  assert.equal(b.plant, '/w/plant');
  const plain = new Board();
  plain.reset();
  await plain.find('/w/repo', null);
  assert.notEqual(plain.root, '/w/repo', 'version の無い kanban/ を台帳とみなした');
  const { ledger } = await b.readLedger(b.root, null);
  assert.equal(ledger.chiefOfStaff, 't2');
  const started = Date.now();
  const { tasks: read } = await b.readLedger(b.root, null);
  JSON.stringify(read);
  assert.ok(Date.now() - started < 2000, 'alias の入れ子の展開に時間がかかった');
  assert.equal(read.find(t => t.id === 'A-0001')?.title, 'Fine');
  assert.match(String(read.find(t => t.id === 'A-0002')?.broken), /maxAliases/, 'alias の入れ子を読めないタスクにしていない');
});

if (failed) process.exit(1);
