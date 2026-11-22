// A fake Stream Deck around the real plugin (package/host/streamdeck/): a WebSocket, a clock, timers and a
// spawn that only record. shared.json is a real file in a folder of its own.
import { EventEmitter } from 'node:events';
import { mkdtemp, mkdir, rm } from 'node:fs/promises';
import { readFileSync, writeFileSync, rmSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

export const PLUGIN = 'package/host/streamdeck/andon.lamp.sdPlugin';
export const ARGS = ['-port', '12345', '-pluginUUID', 'plugin-instance', '-registerEvent', 'registerPlugin', '-info', '{}'];
export const ACTION = 'andon.lamp.light';

export const settle = () => new Promise((resolve) => setImmediate(resolve));

// The SVG in a data: URL, and what it says.
export function svgOf(image) {
  const head = 'data:image/svg+xml,';
  if (!image.startsWith(head)) throw new Error(`not an SVG data URL: ${image.slice(0, 40)}`);
  return decodeURIComponent(image.slice(head.length));
}
export const textsOf = (image) => [...svgOf(image).matchAll(/<text[^>]*>([^<]*)<\/text>/g)].map((m) => m[1]);

// `ends(bin, args)` says how a spawned program ends: { code }, { error }, { throws }, or {} (never ends).
export async function deck({ argv = ARGS, env, platform = 'linux', home, at = Date.parse('2026-11-22T12:00:00Z'), ends = () => ({ code: 0 }) } = {}) {
  const { start } = await import(path.resolve(PLUGIN, 'lamp.js'));
  const tmp = await mkdtemp(path.join(os.tmpdir(), 'andon-lamp-'));
  await mkdir(path.join(tmp, 'app'));
  const file = path.join(tmp, 'app', 'shared.json');
  const d = { tmp, file, clock: at, sockets: [], sent: [], runs: [], reads: [], exits: [], intervals: new Map(), timeouts: new Map() };
  let id = 0;
  class Socket extends EventEmitter {
    constructor(url) { super(); this.url = url; d.sockets.push(this); }
    addEventListener(name, fn) { this.on(name, fn); }
    send(data) { d.sent.push(JSON.parse(data)); }
  }
  start({
    argv, platform, home: home ?? tmp, WebSocket: Socket,
    env: env ?? { HERDR_APP_HOME: path.dirname(file) },
    now: () => d.clock,
    exit: (code) => { d.exits.push(code); },
    readFile: (name, enc) => { d.reads.push(name); return readFileSync(name, enc); },
    timers: {
      setInterval: (fn, ms) => { d.intervals.set(++id, { fn, ms }); return id; },
      clearInterval: (n) => d.intervals.delete(n),
      setTimeout: (fn, ms) => { d.timeouts.set(++id, { fn, ms }); return id; },
      clearTimeout: (n) => d.timeouts.delete(n),
    },
    spawn: (bin, args, options) => {
      const how = ends(bin, args);
      const child = new EventEmitter();
      const run = { bin, args: [...args], options, killed: false, unref: false };
      d.runs.push(run);
      if (how.throws) throw new Error(how.throws);
      child.kill = () => { run.killed = true; };
      child.unref = () => { run.unref = true; };
      queueMicrotask(() => {
        if (how.error) return child.emit('error', Object.assign(new Error(how.error), { code: how.error }));
        child.emit('spawn');
        if ('code' in how) child.emit('exit', how.code, null);
      });
      return child;
    },
  });
  const socket = d.sockets[0];
  d.open = () => socket.emit('open', {});
  d.receive = (message) => socket.emit('message', { data: typeof message === 'string' ? message : JSON.stringify(message) });
  d.close = () => socket.emit('close', {});
  d.fail = () => socket.emit('error', {});
  d.event = (event, context = 'key-1', action = ACTION) => d.receive({ event, action, context, payload: {} });
  d.put = (text) => writeFileSync(file, typeof text === 'string' ? text : JSON.stringify(text));
  d.lamp = (value) => d.put({ 'inbox.lamp': value });
  d.remove = () => rmSync(file, { recursive: true, force: true });
  d.tick = (ms = 2000) => { d.clock += ms; for (const { fn } of [...d.intervals.values()]) fn(); };
  d.images = (context) => d.sent.filter((m) => m.event === 'setImage' && (!context || m.context === context)).map((m) => m.payload.image);
  d.alerts = () => d.sent.filter((m) => m.event === 'showAlert').map((m) => m.context);
  d.done = () => rm(tmp, { recursive: true, force: true });
  return d;
}
