// The Stream Deck plugin andon.lamp (ADR-0045, acceptance 1 to 9): the real code with a fake WebSocket, clock
// and spawn and a real shared.json in a temporary folder; and once, its entry file as a process of its own.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { cp, mkdir, mkdtemp, readFile, readdir, rename, rm, writeFile } from 'node:fs/promises';
import http from 'node:http';
import { isBuiltin } from 'node:module';
import os from 'node:os';
import path from 'node:path';
import vm from 'node:vm';

import { ACTION, ARGS, PLUGIN, WORKING, deck, settle, svgOf, textsOf } from './streamdeck-fake.mjs';

let failed = 0;
async function check(name, fn) {
  try { await fn(); console.log(`ok   ${name}`); }
  catch (e) { failed++; console.log(`FAIL ${name}: ${e.message}`); }
}
async function withDeck(options, fn) {
  const d = await deck(options);
  try { await fn(d); } finally { await d.done(); }
}

const AMBER = '#F0A202', RED = '#D8453B';
const value = (d, fields = {}) => ({ version: 2, at: d.clock, light: null, failed: 0, blocked: 0, merge: 0, decision: 0, task: 0, later: 0, reply: 0, unread: 0, unreadable: 0, ...fields });
const colorOf = (image) => [['red', RED], ['amber', AMBER]].filter(([, c]) => svgOf(image).includes(c)).map(([n]) => n).join('+') || 'dark';
const isUnknown = (image) => colorOf(image) === 'dark' && textsOf(image).join() === '?';

await check('1 登録: 4 つの引数の port へつなぎ、登録の語と名前の 1 通を初めに送る', () => withDeck({}, async (d) => {
  assert.deepEqual(d.sockets.map((s) => s.url), ['ws://127.0.0.1:12345']);
  assert.deepEqual(d.sent, [], 'つながる前に送った');
  d.open();
  assert.deepEqual(d.sent, [{ event: 'registerPlugin', uuid: 'plugin-instance' }]);
}));

await check('1 切断: WebSocket が切れるか失敗すると終わり、つなぎ直さない', async () => {
  await withDeck({}, async (d) => { d.open(); d.close(); assert.deepEqual([d.exits, d.sockets.length], [[0], 1]); });
  await withDeck({}, async (d) => { d.fail(); assert.deepEqual([d.exits, d.sockets.length], [[1], 1]); });
});

await check('1 知らない通知: 何も送らず、何も読まず、何も起動しない', () => withDeck({}, async (d) => {
  d.open();
  for (const m of [{ event: 'systemDidWakeUp' }, { event: 'keyUp', action: ACTION, context: 'k' }, { event: 'willAppear', action: 'andon.lamp.other', context: 'k' },
    { event: 'keyDown', action: 'andon.lamp.other', context: 'k' }, { event: 'willAppear', action: ACTION }, 'not json', 'null', '"text"']) d.receive(m);
  await settle();
  assert.deepEqual([d.sent.length, d.reads, d.runs, d.exits], [1, [], [], []]);
}));

await check('2 色: 消灯・琥珀・赤で 1 通ずつ。同じ値の読み直しでは送らない（4 つの場合）', async () => {
  for (const [light, color] of [[null, 'dark'], ['amber', 'amber'], ['red', 'red']]) {
    await withDeck({}, async (d) => {
      d.lamp(value(d, { light, merge: light ? 1 : 0 }));
      d.event('willAppear');
      assert.equal(d.images().length, 1, `${light}: 1 通でない`);
      assert.equal(colorOf(d.images()[0]), color, `${light} の色`);
      assert.ok(!isUnknown(d.images()[0]), `${light} が分からないになった`);
      d.tick(); d.tick();
      assert.equal(d.images().length, 1, `${light}: 同じ値で送り直した`);
      d.lamp(value(d, { light, merge: light ? 1 : 0 }));
      d.tick();
      assert.equal(d.images().length, 1, `${light}: at だけの違いで送り直した`);
    });
  }
  await withDeck({}, async (d) => {
    d.event('willAppear');
    for (const light of [null, 'amber', 'red']) { d.lamp(value(d, { light })); d.tick(); }
    assert.deepEqual(d.images().map(colorOf), ['dark', 'dark', 'amber', 'red'], '変わるたびの 1 通でない');
  });
});

await check('3 内訳: 合計と、急ぐ順に 0 でない 2 行まで。消灯は数なし（3 つの場合）', async () => {
  for (const [fields, want] of [
    [{ light: 'red', failed: 1, blocked: 1, decision: 1, merge: 2 }, ['5', '1 failed', '1 blocked']],
    [{ light: 'amber', merge: 2 }, ['2', '2 merge']],
    [{ light: null, unread: 3, unreadable: 2 }, []],
    [{ light: 'amber', reply: 1, merge: 2, decision: 3, unread: 9, unreadable: 9 }, ['6', '3 decide', '2 merge']],
    [{ light: 'amber', reply: 4 }, ['4', '4 reply']],
    [{ light: 'amber', task: 1, later: 3 }, ['1', '1 task']],
    [{ light: 'amber', merge: 1, task: 2, reply: 1, later: 5 }, ['4', '1 merge', '2 task']],
    [{ light: null, later: 3 }, []],
    [{ light: 'amber', merge: 2, blocked: '</text><script>', decision: -1, reply: 1.5 }, ['2', '2 merge']],
  ]) {
    await withDeck({}, async (d) => {
      d.lamp(value(d, fields));
      d.event('willAppear');
      assert.deepEqual(textsOf(d.images()[0]), want, JSON.stringify(fields));
    });
  }
});

await check('4 分からない: 読めない 7 つの場合。消灯と違う絵で、読める値で灯りに戻る', async () => {
  const bad = {
    'まだ読めていない': null,
    'inbox.lamp が無い': () => ({ 'inbox.lantern': 'amber' }),
    'version が 1（前の inbox）': (d) => ({ 'inbox.lamp': value(d, { version: 1, light: 'amber' }) }),
    'version が 3': (d) => ({ 'inbox.lamp': value(d, { version: 3, light: 'amber' }) }),
    'light が 3 つでない': (d) => ({ 'inbox.lamp': value(d, { light: 'green' }) }),
    'at が無い': (d) => ({ 'inbox.lamp': value(d, { at: undefined }) }),
    'at が文字': (d) => ({ 'inbox.lamp': value(d, { at: String(d.clock) }) }),
    'at が 91 秒先': (d) => ({ 'inbox.lamp': value(d, { at: d.clock + 91000 }) }),
    'version が文字': (d) => ({ 'inbox.lamp': value(d, { version: '2' }) }),
    'light が無い': (d) => ({ 'inbox.lamp': value(d, { light: undefined }) }),
    'light が配列': (d) => ({ 'inbox.lamp': value(d, { light: ['red'] }) }),
    'light が継いだ名前': (d) => ({ 'inbox.lamp': value(d, { light: 'toString' }) }),
    '前のバージョンの形': () => ({ 'inbox.lamp': { light: 'amber', failed: 0, blocked: 1, merge: 0, decision: 0, reply: 0, unread: 0, unreadable: 0 } }),
    'inbox.lamp が null': () => ({ 'inbox.lamp': null }),
    '中身が配列': () => [],
    '中身が null': () => 'null',
  };
  let dark;
  await withDeck({}, async (d) => { d.lamp(value(d)); d.event('willAppear'); dark = d.images()[0]; });
  for (const [why, make] of Object.entries(bad)) {
    await withDeck({}, async (d) => {
      if (make) { d.lamp(value(d, { light: 'amber', merge: 1 })); d.event('willAppear'); d.put(make(d)); d.tick(0); } else d.event('willAppear');
      const got = d.images().at(-1);
      assert.ok(isUnknown(got), `${why}: 分からないの絵でない`);
      assert.notEqual(got, dark, `${why}: 消灯と同じ絵`);
      d.lamp(value(d, { light: 'amber', merge: 1 }));
      d.tick();
      assert.equal(colorOf(d.images().at(-1)), 'amber', `${why}: 読める値で戻らない`);
    });
  }
});

await check('5 古さ: 91 秒で分からない、新しい at で戻る。読めない回は 90 秒まで前の値（3 つの場合）', async () => {
  await withDeck({}, async (d) => {
    d.lamp(value(d, { light: 'amber', merge: 1 }));
    d.event('willAppear');
    d.tick(90000);
    assert.equal(d.images().length, 1, '90 秒ちょうどで変わった');
    d.tick(1000);
    assert.ok(isUnknown(d.images().at(-1)), '91 秒で分からないにならない');
    d.lamp(value(d, { light: 'amber', merge: 1 }));
    d.tick();
    assert.equal(colorOf(d.images().at(-1)), 'amber', '新しい at で戻らない');
  });
  for (const [why, spoil] of [['消えた', (d) => d.remove()], ['JSON でない', (d) => d.put('{"inbox.lamp": {"ver')], ['空', (d) => d.put('')]]) {
    await withDeck({}, async (d) => {
      d.lamp(value(d, { light: 'amber', merge: 1 }));
      d.event('willAppear');
      spoil(d);
      for (let t = 0; t < 90000; t += 2000) d.tick();
      assert.equal(d.images().length, 1, `${why}: 90 秒たつ前に送った`);
      d.tick();
      assert.ok(isUnknown(d.images().at(-1)), `${why}: 90 秒のあとも古い灯りのまま`);
    });
  }
});

await check('6 押す: do の終わり方ごとの起動と showAlert。連打は捨てる（6 つの場合）', async () => {
  const CALL = ['do', 'inbox.call'];
  const how = (call, bare = { code: 0 }) => (_bin, args) => (args.length ? call : bare);
  const cases = [
    ['0', how({ code: 0 }), [CALL, []], 0],
    ['1', how({ code: 1 }), [CALL, []], 1],
    ['2', how({ code: 2 }), [CALL], 1],
    ['起動できない', how({ error: 'ENOENT' }), [CALL], 1],
    ['起動の呼び出しが投げる', how({ throws: 'spawn EINVAL' }), [CALL], 1],
    ['信号で終わる', how({ code: null }), [CALL], 1],
    ['5 秒で終わらない', how({}), [CALL], 1],
    ['0 のあと前に出せない', how({ code: 0 }, { error: 'EACCES' }), [CALL, []], 1],
    ['1 のあと前に出せない', how({ code: 1 }, { error: 'EACCES' }), [CALL, []], 1],
    ['0 のあと呼び出しが投げる', how({ code: 0 }, { throws: 'spawn EINVAL' }), [CALL, []], 1],
  ];
  for (const [why, ends, want, alerts] of cases) {
    await withDeck({ ends }, async (d) => {
      d.event('keyDown');
      await settle();
      if (why === '5 秒で終わらない') {
        assert.deepEqual([...d.timeouts.values()].map((t) => t.ms), [5000]);
        for (const t of d.timeouts.values()) t.fn();
        await settle();
        assert.ok(d.runs[0].killed, '終わらない do を止めない');
      } else assert.equal(d.timeouts.size, 0, `${why}: 待ちが残った`);
      assert.deepEqual(d.runs.map((r) => r.args), want, `${why}: 起動の一覧`);
      assert.equal(d.alerts().length, alerts, `${why}: showAlert の数`);
      for (const [i, r] of d.runs.entries()) {
        assert.equal(r.bin, 'herdr-app', why);
        assert.equal(r.options.shell, false, `${why}: シェルを通す`);
        assert.equal(r.options.windowsHide, true, `${why}: 窓が出る`);
        assert.equal(r.options.cwd, d.tmp, `${why}: ホームで起動しない`);
        assert.equal(r.options.stdio, 'ignore', `${why}: 入出力をつないだ`);
        assert.equal(r.options.env.HERDR_APP_HOME, path.dirname(d.file), `${why}: 環境変数を渡さない`);
        assert.equal(r.options.detached === true, i === 1, `${why}: 切り離しの指定`);
      }
      if (want.length === 2 && alerts === 0) assert.ok(d.runs[1].unref, '前に出す起動を待っている');
    });
  }
  let calls = 0;
  await withDeck({ ends: () => (++calls === 1 ? {} : { code: 0 }) }, async (d) => {
    d.event('keyDown'); await settle();
    d.event('keyDown', 'key-2'); d.event('keyDown'); await settle();
    assert.equal(d.runs.length, 1, '前の押下の間に起動した');
    for (const t of d.timeouts.values()) t.fn();
    await settle();
    d.event('keyDown'); await settle();
    assert.equal(d.runs.length, 3, '終わったあとの押下が効かない');
  });
});

await check('7 置き場と実行ファイル（6 つの場合）', async () => {
  for (const [env, want] of [
    [{ HERDR_APP_HOME: '/srv/app', XDG_CONFIG_HOME: '/srv/xdg' }, '/srv/app/shared.json'],
    [{ HERDR_APP_HOME: '', XDG_CONFIG_HOME: '/srv/xdg' }, '/srv/xdg/herdr-app/shared.json'],
    [{ XDG_CONFIG_HOME: '' }, '/srv/person/.config/herdr-app/shared.json'],
  ]) await withDeck({ env, home: '/srv/person' }, async (d) => { d.event('willAppear'); assert.deepEqual(d.reads, [want]); });
  const missing = (bin) => (bin === 'herdr-app' || bin === '/srv/herdr' ? { error: 'ENOENT' } : { code: 0 });
  const installed = 'C:\\Local\\Programs\\herdr-app\\herdr-app.exe';
  for (const [why, platform, env, ends, want] of [
    ['HERDR_APP_BIN', 'linux', { HERDR_APP_BIN: '/srv/herdr' }, undefined, ['/srv/herdr', '/srv/herdr']],
    ['PATH', 'win32', { LOCALAPPDATA: 'C:\\Local' }, undefined, ['herdr-app', 'herdr-app']],
    ['空の HERDR_APP_BIN', 'linux', { HERDR_APP_BIN: '' }, undefined, ['herdr-app', 'herdr-app']],
    ['Windows で PATH に無い', 'win32', { LOCALAPPDATA: 'C:\\Local' }, missing, ['herdr-app', installed, installed]],
    ['指定が無いファイル', 'win32', { HERDR_APP_BIN: '/srv/herdr', LOCALAPPDATA: 'C:\\Local' }, missing, ['/srv/herdr']],
    ['Windows で LOCALAPPDATA も無い', 'win32', {}, missing, ['herdr-app']],
    ['Windows でない', 'linux', { LOCALAPPDATA: 'C:\\Local' }, missing, ['herdr-app']],
  ]) {
    await withDeck({ platform, env, ends }, async (d) => { d.event('keyDown'); await settle(); assert.deepEqual(d.runs.map((r) => r.bin), want, why); });
  }
});

await check('8 見えている間だけ読む。タイマーは 1 本（5 つの場合）', () => withDeck({}, async (d) => {
  d.open();
  assert.deepEqual([d.reads.length, d.intervals.size], [0, 0], '見える前に読んだ');
  d.lamp(value(d, { light: 'amber', merge: 1 }));
  d.event('willAppear', 'a');
  d.event('willAppear', 'b');
  assert.deepEqual([...d.intervals.values()].map((t) => t.ms), [2000], 'タイマーが 1 本の 2 秒でない');
  assert.deepEqual(d.images('b'), d.images('a'), 'あとから見えたキーに今の絵が無い');
  d.lamp(value(d, { light: 'red', failed: 1 }));
  d.tick();
  assert.deepEqual(d.images('b'), d.images('a'), '2 つのキーの絵が違う');
  assert.equal(colorOf(d.images('b').at(-1)), 'red');
  d.event('willDisappear', 'b');
  assert.equal(d.intervals.size, 1, '1 つ残っているのに読むのをやめた');
  d.event('willAppear', 'a');
  assert.equal(d.images('a').length, 3, '同じキーの willAppear に送らない');
  assert.equal(d.intervals.size, 1, 'タイマーが増えた');
  d.event('willDisappear', 'a');
  const reads = d.reads.length;
  d.tick();
  assert.deepEqual([d.intervals.size, d.reads.length], [0, reads], '無くなったあとも読んだ');
  // Back in view after a while (the PC slept): judged by the clock then, and sent.
  d.clock += 600000;
  d.event('willAppear', 'a');
  assert.ok(isUnknown(d.images('a').at(-1)), '戻ったキーが古い灯りのまま');
  assert.equal(d.intervals.size, 1);
}));

// The Working key (acceptance 15 to 17). `hc events` never ends unless a case says so.
const live = (rest = () => ({ code: 0 })) => (bin, args) => (args[0] === 'events' ? {} : rest(bin, args));
const term = (address, state, labels = { role: 'chief-of-staff' }, extra = {}) => ({ address, state: 'live', labels, status: state ? { app: 'claude-code', state } : null, ...extra });
const list = (terminals, seq = 10) => ({ channel: null, seq, terminals });
const ev = (terminal, type, data, seq = 100) => ({ jsonrpc: '2.0', method: 'event', params: { data, seq, terminal, type } });
const W = (d) => d.images('w1');
const wtexts = (d) => textsOf(W(d).at(-1));
const FIRST = [term('t1', 'working'), term('t2', 'working', { issue: 'S-0001', project: 'p' }), term('t3', 'working', {}),
  term('t4', 'blocked'), term('t5', 'done', { role: 'chief-engineer', issue: 'S-0002' })];

await check('15 Working: 見えると hc events を 1 つ起動し、role か issue の working を数える（5 つの場合）', async () => {
  await withDeck({ ends: live() }, async (d) => {
    d.event('willAppear', 'w1', WORKING);
    d.event('willAppear', 'w2', WORKING);
    const runs = d.hcRuns();
    assert.equal(runs.length, 1, 'hc events が 1 つでない');
    assert.deepEqual([runs[0].bin, runs[0].args], ['hc', ['events', '--json']]);
    const o = runs[0].options;
    assert.deepEqual([o.shell, o.windowsHide, o.detached, o.cwd, o.stdio[1]], [false, true, undefined, d.tmp, 'pipe'], '起動の指定');
    assert.deepEqual(wtexts(d), ['?'], '初めの 1 通の前は ?');
    d.line(list(FIRST));
    assert.deepEqual(wtexts(d), ['2', 'working'], '初めの 1 通の数');
    assert.deepEqual(d.images('w2'), W(d), '2 つのキーの絵が違う');
    d.line(ev('t4', 'status', { status: { app: 'claude-code', state: 'working' } }));
    assert.deepEqual(wtexts(d), ['3', 'working'], '状態が変わった通知');
    d.line({ jsonrpc: '2.0', method: 'missed', params: list([term('t1', 'working')], 200) });
    assert.deepEqual(wtexts(d), ['1', 'working'], '遅れたあとの一覧');
    d.line(ev('t1', 'status', { status: { app: 'claude-code', state: 'done' } }, 150));
    assert.deepEqual(wtexts(d), ['1', 'working'], '一覧より前の seq の通知を数えた');
    d.line(ev('t1', 'status', { status: { app: 'claude-code', state: 'done' } }, 201));
    assert.deepEqual(wtexts(d), ['idle'], '0 のときに idle でない');
    for (const image of [...W(d), ...d.images('w2')]) assert.equal(colorOf(image), 'dark', '琥珀か赤で塗った');
  });
  await withDeck({ ends: live() }, async (d) => {
    d.event('willAppear', 'w1', WORKING);
    d.line({ jsonrpc: '2.0', method: 'missed', params: { seq: 5 } });
    d.line(ev('t1', 'created', term('t1', 'working'), 6));
    assert.deepEqual(wtexts(d), ['?'], '一覧の前の通知で数えた');
    d.line(list(FIRST));
    d.line('{"jsonrpc":"2.0","method":"heartbeat","params":{"seq":11}}');
    d.line('not json');
    d.line(ev('t9', 'status', { status: { state: 'working' } }));
    assert.deepEqual(wtexts(d), ['2', 'working'], '読めない行か知らない端末で変わった');
    const sends = W(d).length;
    d.line(ev('t3', 'title', { title: 'x' }));
    assert.equal(W(d).length, sends, '数が変わらないのに送った');
    // A created worker, a label change, a finished and a removed one; a line in two pieces.
    const created = JSON.stringify(ev('t6', 'created', term('t6', 'working', { issue: 'S-0003' })));
    d.hcRuns()[0].child.stdout.emit('data', created.slice(0, 20));
    assert.deepEqual(wtexts(d), ['2', 'working'], '半分の行で数えた');
    d.hcRuns()[0].child.stdout.emit('data', `${created.slice(20)}\n`);
    assert.deepEqual(wtexts(d), ['3', 'working'], '作られた担当');
    d.line(ev('t3', 'labels', { labels: { role: 'chief-engineer' } }));
    assert.deepEqual(wtexts(d), ['4', 'working'], 'ラベルを足した端末');
    d.line(ev('t1', 'finished', { exit_code: 0, reason: 'exited' }));
    d.line(ev('t2', 'removed', {}));
    assert.deepEqual(wtexts(d), ['2', 'working'], '終わった端末と消えた端末');
    d.line(ev('t6', 'labels', { labels: { issue: '' } }));
    assert.deepEqual(wtexts(d), ['1', 'working'], '空のラベル');
  });
});

await check('16 Working: 無音と子の終わりで ?、30 秒後に起動し直す。キーが無くなると終わらせる（6 つの場合）', async () => {
  await withDeck({ ends: live() }, async (d) => {
    d.event('willAppear', 'w1', WORKING);
    d.line(list(FIRST));
    d.later(30000);
    d.line('{"jsonrpc":"2.0","method":"heartbeat","params":{"seq":11}}');
    d.later(44000);
    assert.deepEqual(wtexts(d), ['2', 'working'], '行が届いていたのに ? にした');
    d.later(2000);
    assert.deepEqual(wtexts(d), ['?'], '46 秒の無音で ? にならない');
    assert.ok(d.hcRuns()[0].killed, '無音の子を終わらせない');
    d.line(list(FIRST), d.hcRuns()[0]);
    assert.deepEqual(wtexts(d), ['?'], '終わらせた子の行を読んだ');
    // ? came at 45 seconds after the last line, a second ago.
    d.later(28000);
    assert.equal(d.hcRuns().length, 1, '30 秒たつ前に起動し直した');
    d.later(1000);
    assert.equal(d.hcRuns().length, 2, '30 秒で起動し直さない');
    d.line(list([term('t1', 'working')]));
    assert.deepEqual(wtexts(d), ['1', 'working']);
  });
  await withDeck({ ends: live() }, async (d) => {
    d.event('willAppear', 'w1', WORKING);
    d.line(list(FIRST));
    d.hcRuns()[0].child.emit('exit', 1, null);
    assert.deepEqual(wtexts(d), ['?'], '子が終わっても ? にならない');
    d.later(30000);
    assert.equal(d.hcRuns().length, 2, '子が終わって 30 秒で起動し直さない');
    d.event('willDisappear', 'w1', WORKING);
    assert.ok(d.hcRuns()[1].killed, 'キーが無くなっても子を終わらせない');
    assert.equal(d.timeouts.size, 0, '待ちが残った');
    const sends = d.sent.length;
    d.line(list(FIRST), d.hcRuns()[1]);
    d.later(120000);
    assert.deepEqual([d.hcRuns().length, d.sent.length], [2, sends], 'キーが無くなったあとに起動か送信をした');
    d.event('willAppear', 'w1', WORKING);
    assert.deepEqual([d.hcRuns().length, wtexts(d)], [3, ['?']], '見え直したキーで起動し直さないか、前の数を出した');
  });
  await withDeck({ ends: live(), env: { HERDR_APP_HOME: '/nowhere', HC_BIN: '/srv/hc' } }, async (d) => {
    d.event('willAppear', 'w1', WORKING);
    assert.deepEqual(d.hcRuns().map((r) => r.bin), ['/srv/hc'], 'HC_BIN を起動しない');
  });
  await withDeck({ ends: live() }, async (d) => {
    d.lamp(value(d, { light: 'amber', merge: 1 }));
    d.event('willAppear');
    d.tick();
    d.event('keyDown');
    await settle();
    assert.deepEqual(d.hcRuns(), [], 'Lamp のキーだけで hc を起動した');
  });
  for (const [why, how] of [['起動できない', { error: 'ENOENT' }], ['呼び出しが投げる', { throws: 'spawn EINVAL' }]]) {
    await withDeck({ ends: (bin, args) => (args[0] === 'events' ? how : { code: 0 }) }, async (d) => {
      d.lamp(value(d, { light: 'amber', merge: 1 }));
      d.event('willAppear', 'w1', WORKING);
      await settle();
      d.event('willAppear');
      assert.equal(colorOf(d.images('key-1').at(-1)), 'amber', `${why}: Lamp の絵が送られない`);
      assert.deepEqual(wtexts(d), ['?'], `${why}: Working が ? でない`);
      d.lamp(value(d, { light: 'red', failed: 1 }));
      d.tick();
      assert.equal(colorOf(d.images('key-1').at(-1)), 'red', `${why}: Lamp が変わらない`);
      d.later(30000);
      await settle();
      assert.equal(d.hcRuns().length, 2, `${why}: 30 秒で起動し直さない`);
    });
  }
});

await check('17 Working を押す: herdr-app status が 0 なら前に出す。0 でなければ showAlert（3 つの場合と manifest）', async () => {
  for (const [why, status, want, alerts] of [
    ['0', { code: 0 }, [['status'], []], 0],
    ['1', { code: 1 }, [['status']], 1],
    ['起動できない', { error: 'ENOENT' }, [['status']], 1],
    ['5 秒で終わらない', {}, [['status']], 1],
  ]) {
    await withDeck({ ends: (bin, args) => (args[0] === 'status' ? status : { code: 0 }) }, async (d) => {
      d.event('keyDown', 'w1', WORKING);
      await settle();
      if (why === '5 秒で終わらない') { d.later(5000); await settle(); }
      assert.deepEqual(d.runs.map((r) => r.args), want, `${why}: 起動の一覧`);
      assert.deepEqual(d.alerts(), Array(alerts).fill('w1'), `${why}: showAlert`);
      for (const [i, r] of d.runs.entries()) {
        assert.deepEqual([r.bin, r.options.shell, r.options.windowsHide, r.options.cwd], ['herdr-app', false, true, d.tmp], why);
        assert.equal(r.options.detached === true, i === 1, `${why}: 切り離しの指定`);
      }
    });
  }
  const manifest = JSON.parse(await readFile(`${PLUGIN}/manifest.json`, 'utf8'));
  assert.deepEqual(manifest.Actions.map((a) => [a.UUID, a.Name, a.UserTitleEnabled, a.SupportedInMultiActions]),
    [[ACTION, 'Lamp', false, false], [WORKING, 'Working', false, false]]);
});

async function listing(dir, under = '') {
  const out = [];
  for (const e of await readdir(path.join(dir, under), { withFileTypes: true })) {
    const name = path.posix.join(under, e.name);
    if (e.isDirectory()) out.push(`${name}/`, ...await listing(dir, name)); else out.push(name);
  }
  return out;
}

// What makes the folder need more than Stream Deck's own Node.js: a file or an import it shouldn't have.
async function folderFaults(dir) {
  const names = await listing(dir);
  const out = [];
  for (const name of names) {
    if (name.endsWith('/')) { if (name !== 'imgs/') out.push(`folder ${name}`); continue; }
    if (!(name === 'manifest.json' || name === 'package.json' || /^[\w-]+\.(js|md)$/.test(name) || /^imgs\/[\w@-]+\.(svg|png)$/.test(name))) out.push(`file ${name}`);
    if (name === 'package.json') {
      const keys = Object.keys(JSON.parse(await readFile(path.join(dir, name), 'utf8'))).sort();
      if (keys.join() !== 'private,type') out.push(`package.json has ${keys}`);
    }
    if (!/\.[cm]?js$/.test(name)) continue;
    const src = await readFile(path.join(dir, name), 'utf8');
    const mod = new vm.SourceTextModule(src, { identifier: name });
    for (const spec of mod.moduleRequests?.map((r) => r.specifier) ?? mod.dependencySpecifiers) {
      if (spec === 'node:module') out.push(`${name} imports ${spec}`);
      else if (spec.startsWith('node:')) { if (!isBuiltin(spec)) out.push(`${name} imports ${spec}, which isn't there`); }
      else if (!/^\.\/[\w-]+\.js$/.test(spec) || !names.includes(spec.slice(2))) out.push(`${name} imports ${spec}`);
    }
    // What the module record doesn't list.
    if (/\b(?:import|require|createRequire|getBuiltinModule)\s*(?:\/\*[^]*?\*\/\s*)*\(/.test(src)) out.push(`${name} loads at run time`);
  }
  return out;
}

await check('9 フォルダ: 依存も lock も無く、読むのは node: と自分の部品だけ（当たる場合つき）', async () => {
  assert.deepEqual(await folderFaults(PLUGIN), []);
  const spoiled = {
    'lock': ['package-lock.json', '{}'], 'ほかの lock': ['yarn.lock', ''], 'node_modules': ['node_modules/ws/index.js', ''],
    '依存の宣言': ['package.json', '{"private":true,"type":"module","dependencies":{"ws":"1"}}'],
    '二重引用符': ['x.js', 'import ws from "ws";'], '単引用符': ['x.js', "import ws from 'ws';"], '空白なし': ['x.js', 'import{x}from"ws";'],
    '空白 2 つと改行': ['x.js', 'import  x  from\n  "ws";'], '注釈': ['x.js', 'import x from /* c */ "ws";'], '束ねない import': ['x.js', 'import "ws";'],
    'export from': ['x.js', 'export * from "ws";'], 'node: の無い組み込み': ['x.js', 'import fs from "fs";'],
    '在りえない組み込み': ['x.js', 'import x from "node:no-such";'], '在りえない部品': ['x.js', 'import x from "./no-such.js";'],
    '外のファイル': ['x.js', 'import x from "../x.js";'], '動的な import': ['x.js', 'await import ("ws");'],
    'require': ['x.js', 'const x = require(\'ws\');'], 'createRequire': ['x.js', 'import { createRequire } from "node:module";'],
    'ほかの拡張子': ['x.mjs', 'import "ws";'], '下のフォルダ': ['lib/x.js', 'import "ws";'], '画像の置き場のコード': ['imgs/x.js', 'import "ws";'],
  };
  for (const [why, [name, text]] of Object.entries(spoiled)) {
    const tmp = await mkdtemp(path.join(os.tmpdir(), 'andon-lamp-folder-'));
    try {
      await cp(PLUGIN, tmp, { recursive: true });
      await mkdir(path.dirname(path.join(tmp, name)), { recursive: true });
      await writeFile(path.join(tmp, name), text);
      assert.notDeepEqual(await folderFaults(tmp), [], `${why}: 当たらない`);
    } finally { await rm(tmp, { recursive: true, force: true }); }
  }
});

async function manifestFaults(dir, manifest, base = path.basename(dir)) {
  const out = [];
  const m = manifest;
  if (`${m.UUID}.sdPlugin` !== base) out.push('UUID');
  if (m.Nodejs?.Version !== '26') out.push('Node.js');
  const [major, minor] = String(m.Software?.MinimumVersion).split('.').map(Number);
  if (!(major > 7 || (major === 7 && minor >= 1))) out.push('Stream Deck');
  if (m.OS?.map((o) => o.Platform).sort().join() !== 'mac,windows') out.push('OS');
  if (m.Category !== 'Andon') out.push('category');
  if (m.Actions?.map((a) => a.UUID).join() !== `${ACTION},${WORKING}`) out.push('actions');
  const names = { [ACTION]: 'Lamp', [WORKING]: 'Working' };
  for (const a of m.Actions ?? []) {
    if (!a.UUID.startsWith(`${m.UUID}.`) || a.Name !== names[a.UUID]) out.push('action');
    if (a.UserTitleEnabled !== false) out.push('UserTitleEnabled');
    if (a.SupportedInMultiActions !== false) out.push('SupportedInMultiActions');
  }
  const files = await listing(dir);
  if (!files.includes(m.CodePath)) out.push('CodePath');
  for (const image of [m.Icon, m.CategoryIcon, ...(m.Actions ?? []).flatMap((a) => [a.Icon, ...a.States.map((s) => s.Image)])]) {
    if (!files.includes(`${image}.svg`) && !(files.includes(`${image}.png`) && files.includes(`${image}@2x.png`))) out.push(`image ${image}`);
  }
  for (const [file, px] of [[`${m.Icon}.png`, 256], [`${m.Icon}@2x.png`, 512]]) {
    const png = files.includes(file) ? await readFile(path.join(dir, file)) : null;
    if (png?.readUInt32BE(16) !== px || png?.readUInt32BE(20) !== px) out.push(`size ${file}`);
  }
  return out;
}

await check('9 manifest: UUID・Node.js 26・アプリ 7.1・2 つの偽の指定・画像（当たる場合つき）', async () => {
  const manifest = JSON.parse(await readFile(`${PLUGIN}/manifest.json`, 'utf8'));
  assert.deepEqual(await manifestFaults(PLUGIN, manifest), []);
  for (const [why, spoil] of Object.entries({
    UUID: (m) => { m.UUID = 'andon.lamps'; }, Node: (m) => { m.Nodejs.Version = '24'; }, '7.0': (m) => { m.Software.MinimumVersion = '7.0'; },
    '6.9': (m) => { m.Software.MinimumVersion = '6.9'; }, OS: (m) => { m.OS.pop(); }, '2 つ目': (m) => { m.Actions.push(m.Actions[0]); },
    タイトル: (m) => { m.Actions[0].UserTitleEnabled = true; }, タイトルの指定なし: (m) => { delete m.Actions[0].UserTitleEnabled; },
    マルチアクション: (m) => { delete m.Actions[0].SupportedInMultiActions; }, 名前: (m) => { m.Actions[0].Name = 'Lamps'; },
    入口: (m) => { m.CodePath = 'no-such.js'; }, 画像: (m) => { m.Actions[0].States[0].Image = 'imgs/no-such'; },
    'Working が無い': (m) => { m.Actions.pop(); }, 'Working の名前': (m) => { m.Actions[1].Name = 'Lamp'; },
    'Working のタイトル': (m) => { m.Actions[1].UserTitleEnabled = true; }, 'Working のマルチアクション': (m) => { m.Actions[1].SupportedInMultiActions = true; },
    'Working の画像': (m) => { m.Actions[1].Icon = 'imgs/no-such'; }, '順が逆': (m) => { m.Actions.reverse(); },
  })) {
    const copy = structuredClone(manifest);
    spoil(copy);
    assert.notDeepEqual(await manifestFaults(PLUGIN, copy), [], `${why}: 当たらない`);
  }
});

// A WebSocket server just big enough for one plugin.
function frames(socket, take) {
  let buf = Buffer.alloc(0);
  socket.on('data', (chunk) => {
    buf = Buffer.concat([buf, chunk]);
    while (buf.length >= 2) {
      let len = buf[1] & 0x7f, at = 2;
      if (len === 126) { if (buf.length < 4) return; len = buf.readUInt16BE(2); at = 4; }
      const mask = buf[1] & 0x80 ? 4 : 0;
      if (buf.length < at + mask + len) return;
      const body = Buffer.from(buf.subarray(at + mask, at + mask + len));
      if (mask) for (let i = 0; i < len; i++) body[i] ^= buf[at + (i % 4)];
      if ((buf[0] & 0x0f) === 1) take(JSON.parse(body.toString()));
      buf = buf.subarray(at + mask + len);
    }
  });
}

await check('本物の入口: repo の外へ写したフォルダを node で動かすと、登録し、絵を送り、切れると終わる', async () => {
  const tmp = await mkdtemp(path.join(os.tmpdir(), 'andon-lamp-entry-'));
  const server = http.createServer();
  let child, socket, guard;
  try {
    const dir = path.join(tmp, 'plugins', 'andon.lamp.sdPlugin');
    await cp(PLUGIN, dir, { recursive: true });
    await mkdir(path.join(tmp, 'app'));
    const lamp = { version: 2, at: Date.now(), light: 'amber', failed: 0, blocked: 0, merge: 2, decision: 0, task: 0, later: 0, reply: 0, unread: 0, unreadable: 0 };
    await writeFile(path.join(tmp, 'shared.json'), JSON.stringify({ 'inbox.lamp': lamp }));
    await rename(path.join(tmp, 'shared.json'), path.join(tmp, 'app', 'shared.json'));
    const got = [];
    server.on('upgrade', (req, s) => {
      socket = s;
      const accept = createHash('sha1').update(`${req.headers['sec-websocket-key']}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`).digest('base64');
      s.on('error', () => {});
      s.write(`HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\n\r\n`);
      frames(s, (m) => {
        got.push(m);
        if (got.length === 1) {
          const body = Buffer.from(JSON.stringify({ event: 'willAppear', action: ACTION, context: 'k' }));
          s.write(Buffer.concat([Buffer.from([0x81, body.length]), body]));
        } else s.end(Buffer.from([0x88, 0x00]));
      });
    });
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    const args = [...ARGS];
    args[1] = String(server.address().port);
    child = spawn(process.execPath, [path.join(dir, 'plugin.js'), ...args], { cwd: tmp, env: { HOME: tmp, HERDR_APP_HOME: path.join(tmp, 'app') }, stdio: 'ignore' });
    const code = await new Promise((resolve, reject) => {
      guard = setTimeout(() => reject(new Error('10 秒で終わらない')), 10000);
      child.on('error', reject);
      child.on('exit', resolve);
    });
    assert.deepEqual(got[0], { event: 'registerPlugin', uuid: 'plugin-instance' });
    assert.deepEqual(textsOf(got[1].payload.image), ['2', '2 merge']);
    assert.equal(code, 0);
  } finally {
    clearTimeout(guard);
    child?.kill();
    socket?.destroy();
    await new Promise((resolve) => server.close(resolve));
    await rm(tmp, { recursive: true, force: true });
  }
});

if (failed) process.exit(1);
