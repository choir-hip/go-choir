// Deep links, slice S7 (lib/services/links.ts). A package's contributes.links
// turn URL parameters into an app launch; the rules L1 to L6 are in the
// service's header. No browser: the service on the real kernel.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite; let L; let X; let K;
test.beforeAll(async () => {
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  L = await vite.ssrLoadModule('/src/lib/services/links.ts');
  X = await vite.ssrLoadModule('/src/lib/contributions.ts');
  K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
});
test.afterAll(async () => { await vite?.close(); });

const notes = () => ({
  app: 'notes',
  params: [{ name: 'note', context: 'noteId' }, { name: 'tag' }, { name: 'view', context: 'view', required: false }],
  launch: { windowTitle: 'Notes', requireAny: ['note', 'tag'], context: { pinned: true, source: 'param:tag' } },
});
const svc = () => L.createLinks((id) => (id === 'notes' ? { name: 'Notes', icon: 'N' } : null));

test('a link launches the app with the parameters as context; consumed parameters are cleared with it', () => {
  const s = svc();
  s.add(notes(), 'notes');
  expect(s.resolve('?app=notes&note=7&view=wide')).toEqual({
    kind: 'app_launch', source: 'url', appId: 'notes', appName: 'Notes', icon: 'N',
    appContext: { noteId: '7', view: 'wide', pinned: true, source: '', windowTitle: 'Notes' },
  });
  expect(s.resolve('?app=notes&tag=x').appContext.source).toBe('x');
  expect(s.consumedParams()).toEqual(['note', 'tag', 'view']);
});

test('requireAny, required and unknown apps give no launch', () => {
  const s = svc();
  s.add({ ...notes(), params: [...notes().params.slice(0, 1), { name: 'must', required: true }, { name: 'tag' }] }, 'notes');
  expect(s.resolve('?app=notes&note=1')).toBeNull(); // must is required
  expect(s.resolve('?app=notes&must=1')).toBeNull(); // requireAny: note or tag
  expect(s.resolve('?app=notes&must=1&note=1')).not.toBeNull();
  expect(s.resolve('?app=other&note=1')).toBeNull();
});

test('L3: a long value or a control character refuses the link', () => {
  const s = svc();
  s.add(notes(), 'notes');
  expect(s.resolve(`?app=notes&note=${'x'.repeat(513)}`)).toBeNull();
  expect(s.resolve('?app=notes&note=a%00b')).toBeNull();
});

test('L1, L2, L4, L6: a foreign app, a shell parameter, a repeated app and an expression are refused', () => {
  const s = svc();
  expect(() => s.add({ ...notes(), app: 'email' }, 'notes')).toThrow(/not the package's own/);
  expect(() => s.add({ ...notes(), params: [{ name: 'doc' }] }, 'notes')).toThrow(/shell's/);
  expect(() => s.add({ ...notes(), params: [{ name: 'app' }] }, 'notes')).toThrow(/shell's/);
  s.add(notes(), 'notes');
  expect(() => s.add(notes(), 'notes')).toThrow(/already declared/);
  expect(() => svc().add({ ...notes(), launch: { context: { x: 'param:ghost' } } }, 'notes')).toThrow(/not declared/);
  expect(() => svc().add({ ...notes(), launch: { context: { x: { eval: 1 } } } }, 'notes')).toThrow(/literal/);
  expect(() => svc().add({ ...notes(), launch: { requireAny: ['ghost'] } }, 'notes')).toThrow(/not a parameter/);
});

test('L5: the shell service applies a package link in its scope and uninstall leaves the baseline', async () => {
  const before = JSON.stringify(K.shellKernel.snapshot());
  await X.setPackageContributions('notes', { links: [notes()] }, 'v1');
  expect(L.links.has('notes')).toBe(true);
  expect(L.links.consumedParams()).toContain('note');
  await X.setPackageContributions('notes', null, '');
  expect(L.links.has('notes')).toBe(false);
  expect(L.links.consumedParams()).not.toContain('note');
  expect(JSON.stringify(K.shellKernel.snapshot())).toBe(before);
});

test('the manifest gate refuses a bad link and accepts a good one', () => {
  const run = (mutate) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-links-'));
    fs.cpSync(root, dir, { recursive: true, filter: (src) => !/node_modules|dist|test-results|\.timestamp-/.test(src) });
    fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'));
    const p = path.join(dir, 'src/packages/image/choir-package.json');
    const m = JSON.parse(fs.readFileSync(p, 'utf8'));
    m.contributes = mutate(m.name);
    fs.writeFileSync(p, JSON.stringify(m, null, 2));
    const r = spawnSync(process.execPath, ['scripts/check-packages.mjs'], { cwd: dir, encoding: 'utf8' });
    fs.rmSync(dir, { recursive: true, force: true });
    return { status: r.status, out: r.stdout + r.stderr };
  };
  const link = (over) => (n) => ({ links: [{ ...notes(), app: n, ...over }] });
  const good = run((n) => ({ links: [{ app: n, params: [{ name: 'photo', context: 'photoId' }], launch: { requireAny: ['photo'] } }] }));
  expect(good.status, good.out).toBe(0);
  expect(run(link({ app: 'files' })).out).toMatch(/app must be the package's own/);
  expect(run(link({ params: [{ name: 'doc' }] })).out).toMatch(/parameter the shell owns/);
  expect(run(link({ launch: { context: { a: 'param:zzz' } } })).out).toMatch(/not declared/);
});

test('L7: the signed-out home is a data rule: autopaper by default, one claimant, params optional only for it', () => {
  const home = L.links.homeIntent();
  expect(home).toMatchObject({ appId: 'autopaper', appContext: { preview: true, windowTitle: 'Autopaper' } });
  const s = svc();
  s.add({ app: 'notes', params: [], launch: { home: true, context: { preview: true } } }, 'notes');
  expect(s.homeIntent().appId).toBe('notes');
  expect(() => s.add({ app: 'other', params: [], launch: { home: true } }, 'other')).toThrow(/already the signed-out home/);
  expect(() => svc().add({ app: 'notes', params: [] }, 'notes')).toThrow(/at least one parameter/);
  expect(() => svc().add({ app: 'notes', params: [], launch: { home: 'yes' } }, 'notes')).toThrow(/home must be a boolean/);
  expect(svc().homeIntent()).toBeNull();
});
