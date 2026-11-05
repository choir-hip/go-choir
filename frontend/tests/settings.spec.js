// Package settings, slice S6 (lib/services/settings.ts). No browser: the
// service on the real kernel, with the preference route stubbed at fetch.
// Failure modes S1 to S7 are listed in the service's header.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite; let S; let X; let L;
test.beforeAll(async () => {
  globalThis.window = new EventTarget();
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  S = await vite.ssrLoadModule('/src/lib/services/settings.ts');
  X = await vite.ssrLoadModule('/src/lib/contributions.ts');
  L = await vite.ssrLoadModule('/src/lib/live-events.js');
});
test.afterAll(async () => { await vite?.close(); delete globalThis.window; });

const section = () => ({ section: 'Notes', fields: [
  { key: 'autosave', type: 'boolean', default: true, label: 'Autosave' },
  { key: 'font', type: 'string', default: 'serif', label: 'Font', enum: ['serif', 'mono'] },
  { key: 'size', type: 'number', default: 14, label: 'Size' },
] });

function server() {
  const store = new Map(); const calls = []; let fail = 0;
  globalThis.fetch = async (url, init = {}) => {
    const method = init.method || 'GET';
    const key = String(url).replace(/^.*\/api\/preferences\//, '').split('?')[0];
    calls.push([method, key, init.body]);
    if (fail) return new Response(JSON.stringify({ error: 'nope' }), { status: fail });
    if (method === 'PUT') { store.set(key, JSON.parse(init.body).value); return new Response(JSON.stringify({ key, value: store.get(key) })); }
    return new Response(JSON.stringify({ key, value: store.get(key), default: !store.has(key) }));
  };
  return { store, calls, failWith: (n) => { fail = n; } };
}
const sections = () => { let v; S.settingSections.subscribe((x) => { v = x; })(); return v; };

test('S1, S2: bad sections are skipped, the good ones apply, a key twice is refused', async () => {
  const warn = console.warn; const notes = []; console.warn = (m) => notes.push(String(m));
  try {
    await X.setPackageContributions('s1pkg', { settings: [
      { section: 'Bad type', fields: [{ key: 'a', type: 'object', default: {}, label: 'A' }] },
      { section: 'Bad default', fields: [{ key: 'a', type: 'boolean', default: 'yes', label: 'A' }] },
      { section: 'Bad enum', fields: [{ key: 'a', type: 'boolean', default: true, label: 'A', enum: ['x'] }] },
      { section: 'Dot key', fields: [{ key: 'a.b', type: 'boolean', default: true, label: 'A' }] },
      section(),
      { section: 'Again', fields: [{ key: 'autosave', type: 'boolean', default: false, label: 'Again' }] },
    ] }, 'v1');
    expect(sections().map((s) => `${s.pkg}/${s.section}`)).toEqual(['s1pkg/Notes']);
    expect(notes.length).toBe(5);
  } finally { console.warn = warn; await X.setPackageContributions('s1pkg', null, ''); }
  expect(sections()).toEqual([]);
});

test('get falls back to the default, set validates first, then sends, caches and notifies (S3, S4)', async () => {
  const srv = server();
  await X.setPackageContributions('notes', { settings: [section()] }, 'v1');
  expect(await S.getSetting('autosave', 'notes')).toBe(true); // server has none: the response omits value
  expect(await S.getSetting('nope', 'notes')).toBeUndefined();
  expect(await S.getSetting('autosave')).toBeUndefined(); // no package named
  const heard = []; const off = S.onSetting('font', (v) => heard.push(v), 'notes');
  expect(await S.setSetting('font', 'mono', 'notes')).toEqual({ ok: true });
  expect(heard).toEqual(['mono']);
  expect(await S.getSetting('font', 'notes')).toBe('mono');
  const before = srv.calls.length;
  for (const [key, value] of [['font', 'comic'], ['font', 3], ['autosave', 'yes'], ['size', NaN], ['size', Infinity], ['nope', 1]]) {
    const r = await S.setSetting(key, value, 'notes');
    expect(r.ok, `${key}=${String(value)}`).toBe(false);
  }
  expect(srv.calls.length).toBe(before); // S4: nothing was sent
  expect(heard).toEqual(['mono']);
  off();
  await S.setSetting('font', 'serif', 'notes');
  expect(heard).toEqual(['mono']); // the disposer stops it
  await X.setPackageContributions('notes', null, '');
});

test('S5: a server refusal returns the reason and keeps the last good value', async () => {
  const srv = server();
  await X.setPackageContributions('notes', { settings: [section()] }, 'v1');
  await S.setSetting('size', 20, 'notes');
  srv.failWith(400);
  const r = await S.setSetting('size', 30, 'notes');
  expect(r).toEqual({ ok: false, error: 'nope' });
  expect(await S.getSetting('size', 'notes')).toBe(20);
  await X.setPackageContributions('notes', null, '');
});

test('S6: another window\'s write updates the cache and listeners; this window\'s echo is ignored', async () => {
  server();
  await X.setPackageContributions('notes', { settings: [section()] }, 'v1');
  const heard = []; S.onSetting('size', (v) => heard.push(v), 'notes');
  const send = (payload) => L.dispatchLiveEvent({ kind: 'setting.updated', payload });
  send({ key: 'pkg.notes.size', value: 33, source_device_id: 'another-device' });
  expect(heard).toEqual([33]);
  expect(await S.getSetting('size', 'notes')).toBe(33);
  send({ key: 'pkg.notes.size', value: 44, source_device_id: L.currentDeviceId() });
  send({ key: 'pkg.notes.size', value: 'not a number', source_device_id: 'another-device' });
  send({ key: 'pkg.ghost.size', value: 1, source_device_id: 'another-device' });
  send({ key: 'theme', value: 1, source_device_id: 'another-device' });
  expect(heard).toEqual([33]);
  await X.setPackageContributions('notes', null, '');
});

test('S7: uninstall removes the sections, the cache and the listeners; the install again starts clean', async () => {
  server();
  await X.setPackageContributions('notes', { settings: [section()] }, 'v1');
  const heard = []; S.onSetting('size', (v) => heard.push(v), 'notes');
  await S.setSetting('size', 5, 'notes');
  await X.setPackageContributions('notes', null, '');
  expect(sections()).toEqual([]);
  expect(await S.getSetting('size', 'notes')).toBeUndefined();
  expect((await S.setSetting('size', 6, 'notes')).ok).toBe(false);
  await X.setPackageContributions('notes', { settings: [section()] }, 'v2');
  L.dispatchLiveEvent({ kind: 'setting.updated', payload: { key: 'pkg.notes.size', value: 9, source_device_id: 'another-device' } });
  expect(heard).toEqual([5]); // the old listener left with the old scope
  await X.setPackageContributions('notes', null, '');
});

test('two packages with the same key do not collide', async () => {
  const srv = server();
  await X.setPackageContributions('aaa', { settings: [section()] }, 'v1');
  await X.setPackageContributions('bbb', { settings: [section()] }, 'v1');
  await S.setSetting('size', 1, 'aaa');
  await S.setSetting('size', 2, 'bbb');
  expect(srv.calls.map((c) => c[1])).toEqual(['pkg.aaa.size', 'pkg.bbb.size']);
  expect(await S.getSetting('size', 'aaa')).toBe(1);
  expect(await S.getSetting('size', 'bbb')).toBe(2);
  await X.setPackageContributions('aaa', null, ''); await X.setPackageContributions('bbb', null, '');
});

test('the manifest gate (check-packages) refuses a bad settings declaration and accepts a good one', () => {
  const run = (mutate) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-settings-'));
    fs.cpSync(root, dir, { recursive: true, filter: (src) => !/node_modules|dist|test-results|\.timestamp-/.test(src) });
    fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'));
    const p = path.join(dir, 'src/packages/image/choir-package.json');
    const m = JSON.parse(fs.readFileSync(p, 'utf8'));
    m.contributes = { settings: mutate() };
    fs.writeFileSync(p, JSON.stringify(m, null, 2));
    const r = spawnSync(process.execPath, ['scripts/check-packages.mjs'], { cwd: dir, encoding: 'utf8' });
    fs.rmSync(dir, { recursive: true, force: true });
    return { status: r.status, out: r.stdout + r.stderr };
  };
  const field = (over = {}) => ({ key: 'a', type: 'boolean', default: true, label: 'A', ...over });
  expect(run(() => [section()]).status).toBe(0);
  expect(run(() => [{ section: 'S', fields: [field({ type: 'object' })] }]).out).toMatch(/type must be/);
  expect(run(() => [{ section: 'S', fields: [field({ default: 'yes' })] }]).out).toMatch(/default must be a boolean/);
  expect(run(() => [{ section: 'S', fields: [field({ key: 'a.b' })] }]).out).toMatch(/key must be a lowercase word/);
  expect(run(() => [{ section: 'S', fields: [field(), field()] }]).out).toMatch(/listed twice/);
  expect(run(() => [{ section: 'S', fields: [field({ enum: ['x'] })] }]).out).toMatch(/enum is a list/);
});
