import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

let terminals = [], panes = [], spaces = [], calls = [];
const context = vm.createContext({});
const api = new vm.SyntheticModule(['terminals', 'panes', 'spaces', 'run'], function () {
  this.setExport('terminals', () => terminals);
  this.setExport('panes', () => panes);
  this.setExport('spaces', () => spaces);
  this.setExport('run', (name, args) => {
    assert.ok(['space.rename', 'tab.rename'].includes(name), name);
    calls.push([name, JSON.parse(JSON.stringify(args))]);
  });
}, { context });
const ui = new vm.SyntheticModule(['View', 'div'], function () {
  this.setExport('View', class {});
  this.setExport('div', () => ({}));
}, { context });
const mod = new vm.SourceTextModule(await readFile('package/host/andon/main.js', 'utf8'), { context });
await mod.link(name => {
  if (name === 'herdr') return api;
  if (name === 'gpui-kit') return ui;
  throw Error(`Unexpected import: ${name}`);
});
await mod.evaluate();
const view = new mod.namespace.default();
const slug = 'sample-site';
function add({ issue, title, role, work = false, machine = null, address = 't1' }) {
  const prefix = machine ? `${machine}/` : '';
  const sid = `andon-${slug}${work ? `-${issue}` : ''}`;
  const tid = `andon-${slug}-${role === 'chief-of-staff' ? 'cos' : issue}`;
  let space = spaces.find(s => s.id === prefix + sid);
  if (!space) spaces.push(space = { id: prefix + sid, machine, name: 'unnamed', tabs: [] });
  space.tabs.push({ id: prefix + tid, name: '1' });
  terminals.push({ address: prefix + address, state: 'live', labels: {
    project: slug, role, issue, title, 'herdr.space': sid, 'herdr.tab': tid,
  } });
  panes.push({ terminal: prefix + address, tab: prefix + tid });
}
function render() { calls = []; view.render(); return calls; }
add({ role: 'chief-of-staff' });
add({ role: 'chief-engineer', issue: 'A-1', title: '設計', address: 't2' });
add({ issue: 'A-2', title: '修正', work: true, address: 't3' });
add({ issue: 'A-2', title: '別マシン', work: true, machine: 'sample-machine', address: 't3' });
assert.deepEqual(render(), [
  ['space.rename', { id: 'andon-sample-site', name: slug }],
  ['tab.rename', { id: 'andon-sample-site-cos', name: 'CoS' }],
  ['tab.rename', { id: 'andon-sample-site-A-1', name: 'CE A-1 設計' }],
  ['space.rename', { id: 'andon-sample-site-A-2', name: 'A-2 修正' }],
  ['tab.rename', { id: 'andon-sample-site-A-2', name: 'A-2' }],
  ['space.rename', { id: 'sample-machine/andon-sample-site-A-2', name: 'A-2 別マシン' }],
  ['tab.rename', { id: 'sample-machine/andon-sample-site-A-2', name: 'A-2' }],
]);
// Echo the App's changes: a second render must issue no command.
for (const [kind, { id, name }] of calls) {
  const targets = kind === 'space.rename' ? spaces : spaces.flatMap(s => s.tabs);
  targets.find(t => t.id === id).name = name.trim();
}
assert.deepEqual(render(), []);
// The dispatcher has no project label. One ledger: it shares the project's
// command-side Space; else it sits in the plant's own Space.
terminals.push({ address: 't9', state: 'live', labels: { role: 'dispatcher', 'herdr.space': 'andon-sample-site', 'herdr.tab': 'andon-sample-site-dispatcher' } });
spaces[0].tabs.push({ id: 'andon-sample-site-dispatcher', name: '1' });
panes.push({ terminal: 't9', tab: 'andon-sample-site-dispatcher' });
terminals.push({ address: 't8', state: 'live', labels: { role: 'dispatcher', 'herdr.space': 'andon-sample-plant-plant', 'herdr.tab': 'andon-sample-plant-dispatcher' } });
spaces.push({ id: 'andon-sample-plant-plant', machine: null, name: 'unnamed', tabs: [{ id: 'andon-sample-plant-dispatcher', name: '1' }] });
panes.push({ terminal: 't8', tab: 'andon-sample-plant-dispatcher' });
assert.deepEqual(render(), [
  ['tab.rename', { id: 'andon-sample-site-dispatcher', name: 'dispatcher' }],
  ['space.rename', { id: 'andon-sample-plant-plant', name: 'sample-plant plant' }],
  ['tab.rename', { id: 'andon-sample-plant-dispatcher', name: 'dispatcher' }],
]);
for (const [kind, { id, name }] of calls) {
  const targets = kind === 'space.rename' ? spaces : spaces.flatMap(s => s.tabs);
  targets.find(t => t.id === id).name = name.trim();
}
assert.deepEqual(render(), []);
// Reviewer has its own issue/title but the worker's Space and tab. No rename, even alone.
const worker = terminals[2];
terminals.push({ ...worker, address: 't4', labels: { ...worker.labels, issue: 'A-3', title: 'レビュー' } });
panes.push({ terminal: 't4', tab: panes[2].tab });
assert.deepEqual(render(), []);
terminals = terminals.filter(t => t !== worker);
assert.deepEqual(render(), []);
// An updated title is picked up from labels, never from the terminal title.
terminals[1].labels.title = '設計を改訂';
assert.deepEqual(render(), [['tab.rename', { id: 'andon-sample-site-A-1', name: 'CE A-1 設計を改訂' }]]);
// Missing labels, unrelated role/space, dead terminals, and terminals not yet
// adopted must not issue commands. Reuse an unnamed real target to catch writes.
spaces = [{ id: 'andon-sample-site', name: 'unnamed', tabs: [{ id: 'andon-sample-site-A-1', name: '1' }] }];
panes = [{ terminal: 't1', tab: 'andon-sample-site-A-1' }];
const valid = { project: slug, role: 'chief-engineer', issue: 'A-1', title: '設計', 'herdr.space': 'andon-sample-site' };
for (const labels of [{}, { ...valid, project: '' }, { ...valid, role: 'dispatcher' },
  { ...valid, 'herdr.space': 'another-space' }, { ...valid, title: '' },
  { ...valid, role: undefined }, // a worker belongs in its own issue's Space
  // The old label task= is not read (ADR-0008 decision 7): a terminal carrying only it is not named.
  { project: slug, role: 'chief-engineer', task: 'A-1', title: '設計', 'herdr.space': 'andon-sample-site' }]) {
  terminals = [{ address: 't1', state: 'live', labels }];
  assert.deepEqual(render(), []);
}
terminals = [{ address: 't1', state: 'finished', labels: valid }];
assert.deepEqual(render(), []);
terminals[0].state = 'live';
panes = [];
assert.deepEqual(render(), []);
spaces = [];
assert.deepEqual(render(), []);
// App rename trims names before saving. Repeated snapshots must settle even
// when a ledger title contains trailing whitespace or control characters.
panes = [{ terminal: 't1', tab: 'andon-sample-site-A-1' }];
for (const [title, expected] of [
  ['修正 ', 'CE A-1 修正'], ['  修正  ', 'CE A-1 修正'],
  ['一行目\n二行目\t次\u0007', 'CE A-1 一行目 二行目 次'], ['  \t\n', null],
]) {
  spaces = [{ id: 'andon-sample-site', name: slug, tabs: [{ id: panes[0].tab, name: '1' }] }];
  terminals = [{ address: 't1', state: 'live', labels: { ...valid, title } }];
  assert.deepEqual(render(), expected ? [['tab.rename', { id: panes[0].tab, name: expected }]] : []);
  for (const [, { name }] of calls) spaces[0].tabs[0].name = name.trim();
  assert.deepEqual(render(), []);
}
// A worker that carries only the old label task= is not named either.
spaces = [{ id: 'andon-sample-site-A-2', name: 'unnamed', tabs: [{ id: 'andon-sample-site-A-2', name: '1' }] }];
panes = [{ terminal: 't1', tab: 'andon-sample-site-A-2' }];
terminals = [{ address: 't1', state: 'live', labels: { project: slug, task: 'A-2', title: '修正', 'herdr.space': 'andon-sample-site-A-2' } }];
assert.deepEqual(render(), [], 'ラベル task= を読んだ');
terminals[0].labels = { project: slug, issue: 'A-2', title: '修正', 'herdr.space': 'andon-sample-site-A-2' };
assert.deepEqual(render(), [['space.rename', { id: 'andon-sample-site-A-2', name: 'A-2 修正' }], ['tab.rename', { id: 'andon-sample-site-A-2', name: 'A-2' }]]);
console.log('ok   App mod: names from issue= (not task=), idempotence, reviewer, remote placement, and rename-only API');
