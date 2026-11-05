// Commands and shortcuts, slice S2 (lib/services/commands.ts). No browser:
// the service on the real kernel, loaded through vite for its module graph.
// Failure modes M1 to M7 are listed in the service's header.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite;
let C; let K; let X;
test.beforeAll(async () => {
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  C = await vite.ssrLoadModule('/src/lib/services/commands.ts');
  K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
  X = await vite.ssrLoadModule('/src/lib/contributions.ts');
});
test.afterAll(async () => { await vite?.close(); });

function fresh() {
  const kernel = (K.shellKernel);
  const calls = [];
  const svc = C.createCommands({
    scopeFor: (owner) => K.packageScope(`t-${owner}`),
    actions: { openApp: (a) => calls.push(['openApp', a]), openFile: (p) => calls.push(['openFile', p]), copyText: (t) => calls.push(['copyText', t]), toast: (m) => calls.push(['toast', m]) },
  });
  return { svc, calls, kernel };
}
const key = (k, extra = {}) => ({ key: k, metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, defaultPrevented: false, isComposing: false, target: { tagName: 'DIV', isContentEditable: false }, preventDefault() { this.defaultPrevented = true; }, ...extra });
const decl = (id, over = {}) => ({ id, title: id, scope: 'global', run: { action: 'toast', message: id }, ...over });

test('M1: a package cannot bind the locked key; the shell can', () => {
  const { svc } = fresh();
  svc.registerCommand(decl('calc.open'), 'calc');
  expect(() => svc.registerKeybinding({ key: 'cmd+k', command: 'calc.open' }, 'calc')).toThrow(/locked/);
  expect(() => svc.registerKeybinding({ key: 'Ctrl+K', command: 'calc.open' }, 'calc')).toThrow(/locked/);
  svc.registerCommand({ ...decl('core.desk'), run: { handler: true } }, 'core', () => {});
  expect(() => svc.registerKeybinding({ key: 'Mod+K', command: 'core.desk' }, 'core')).not.toThrow();
});

test('M2, M3: duplicate ids and ids outside the owner namespace are refused', () => {
  const { svc } = fresh();
  svc.registerCommand(decl('calc.open'), 'calc');
  expect(() => svc.registerCommand(decl('calc.open'), 'calc')).toThrow(/already registered/);
  expect(() => svc.registerCommand(decl('mail.send'), 'calc')).toThrow(/namespace/);
  expect(() => svc.registerCommand(decl('calc'), 'calc')).toThrow(/<package>\.<name>/);
  expect(() => svc.registerCommand({ ...decl('calc.x'), run: { handler: true } }, 'calc')).toThrow(/registered none/);
  expect(() => svc.registerCommand(decl('calc.y', { run: { action: 'rm -rf' } }), 'calc')).toThrow(/run.action/);
  expect(() => svc.registerKeybinding({ key: 'Mod+J', command: 'mail.send' }, 'calc')).toThrow(/outside/);
});

test('declarative actions run; when and scope gate them', async () => {
  const { svc, calls } = fresh();
  svc.registerCommand(decl('calc.a', { run: { action: 'openApp', app: 'files' } }), 'calc');
  svc.registerCommand(decl('calc.b', { scope: 'app', run: { action: 'openFile', path: 'a/b.txt' } }), 'calc');
  svc.registerCommand(decl('calc.c', { when: 'online && !busy', run: { action: 'copyText', text: 'x' } }), 'calc');
  expect(await svc.run('calc.a')).toBe(true);
  expect(await svc.run('calc.b')).toBe(false);
  svc.setContext('window.app', 'calc');
  expect(await svc.run('calc.b')).toBe(true);
  expect(await svc.run('calc.c')).toBe(false);
  svc.setContext('online', true);
  expect(await svc.run('calc.c')).toBe(true);
  svc.setContext('busy', true);
  expect(await svc.run('calc.c')).toBe(false);
  expect(calls).toEqual([['openApp', 'files'], ['openFile', 'a/b.txt'], ['copyText', 'x']]);
  expect(C.evaluateWhen('a == b && c != d', { a: 'b', c: 'e' })).toBe(true);
});

test('keys resolve by tier, then priority, then age; a binding to a missing command is inert (M4)', () => {
  const { svc } = fresh();
  svc.registerCommand(decl('calc.a'), 'calc');
  svc.registerCommand(decl('pad.a'), 'pad');
  svc.registerCommand({ ...decl('core.a'), run: { handler: true } }, 'core', () => {});
  svc.registerKeybinding({ key: 'Mod+Shift+P', command: 'pad.a' }, 'pad', 'app', 1);
  svc.registerKeybinding({ key: 'mod+shift+p', command: 'calc.a' }, 'calc', 'app', 5);
  expect(svc.resolveKey('Mod+Shift+P').spec.command).toBe('calc.a');
  svc.registerKeybinding({ key: 'Mod+Shift+P', command: 'core.a' }, 'core');
  expect(svc.resolveKey('Mod+Shift+P').spec.command).toBe('calc.a'); // an app binding outranks core's on a key that is not locked
  svc.registerKeybinding({ key: 'Alt+Z', command: 'calc.ghost' }, 'calc');
  expect(svc.resolveKey('Alt+Z')).toBeNull();
});

test('M5, M6: handleKeydown does not fire a bare key while typing; a throwing handler does not escape', async () => {
  const { svc } = fresh();
  const seen = [];
  svc.registerCommand({ ...decl('calc.h'), run: { handler: true } }, 'calc', () => { seen.push('h'); });
  svc.registerCommand({ ...decl('calc.t'), run: { handler: true } }, 'calc', () => { throw new Error('boom'); });
  svc.registerKeybinding({ key: 'g', command: 'calc.h' }, 'calc');
  svc.registerKeybinding({ key: 'Mod+Y', command: 'calc.h' }, 'calc');
  svc.registerKeybinding({ key: 'Mod+T', command: 'calc.t' }, 'calc');
  const typing = { target: { tagName: 'INPUT', isContentEditable: false } };
  expect(svc.handleKeydown(key('g', typing))).toBe(false);
  expect(svc.handleKeydown(key('g'))).toBe(true);
  expect(svc.handleKeydown(key('y', { ctrlKey: true, ...typing }))).toBe(true);
  const warn = console.warn; console.warn = () => {};
  expect(svc.handleKeydown(key('t', { metaKey: true }))).toBe(true);
  await new Promise((r) => setTimeout(r, 10));
  console.warn = warn;
  expect(seen).toEqual(['h', 'h']);
});

test('M7: contributes.commands and keybindings apply in the package scope and uninstall leaves the baseline', async () => {
  const before = JSON.stringify(K.shellKernel.snapshot());
  const live = C.commands;
  await X.setPackageContributions('fixture', {
    commands: [decl('fixture.hello', { run: { action: 'toast', message: 'hi' } }), { ...decl('fixture.code'), run: { handler: true } }, decl('other.nope')],
    keybindings: [{ key: 'Mod+Shift+H', command: 'fixture.hello' }, { key: 'Mod+K', command: 'fixture.hello' }],
  }, 'v1');
  expect(live.has('fixture.hello')).toBe(true);
  expect(live.has('other.nope')).toBe(false); // refused (M3), the rest applied
  expect(live.bindings().filter((b) => b.owner === 'fixture').map((b) => b.key)).toEqual(['Mod+Shift+H']); // Mod+K refused (M1)
  const dispose = C.registerCommand({ ...decl('fixture.code'), run: { handler: true } }, () => {});
  expect(live.has('fixture.code')).toBe(true);
  await dispose();
  await X.setPackageContributions('fixture', null, '');
  expect(live.has('fixture.hello')).toBe(false);
  expect(live.bindings().some((b) => b.owner === 'fixture')).toBe(false);
  expect(JSON.stringify(K.shellKernel.snapshot())).toBe(before);
});

test('an unchanged fingerprint is not re-applied; a changed one replaces the set', async () => {
  const live = C.commands;
  await X.setPackageContributions('fx2', { commands: [decl('fx2.a')] }, 'one');
  await X.setPackageContributions('fx2', { commands: [decl('fx2.a'), decl('fx2.b')] }, 'one');
  expect(live.has('fx2.b')).toBe(false);
  await X.setPackageContributions('fx2', { commands: [decl('fx2.b')] }, 'two');
  expect(live.has('fx2.a')).toBe(false);
  expect(live.has('fx2.b')).toBe(true);
  await X.setPackageContributions('fx2', null, '');
});

test('the manifest gate (check-packages) refuses a bad command, a foreign id and a locked key', () => {
  const run = (mutate) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-cmds-'));
    fs.cpSync(root, dir, { recursive: true, filter: (src) => !/node_modules|dist|test-results/.test(src) });
    fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'));
    const p = path.join(dir, 'src/packages/image/choir-package.json');
    const m = JSON.parse(fs.readFileSync(p, 'utf8'));
    m.contributes = mutate(m.name);
    fs.writeFileSync(p, JSON.stringify(m, null, 2));
    const r = spawnSync(process.execPath, ['scripts/check-packages.mjs'], { cwd: dir, encoding: 'utf8' });
    fs.rmSync(dir, { recursive: true, force: true });
    return { status: r.status, out: r.stdout + r.stderr };
  };
  const good = run((n) => ({ desks: [], commands: [decl(`${n}.zoom`)], keybindings: [{ key: 'Mod+Shift+Z', command: `${n}.zoom` }] }));
  expect(good.status, good.out).toBe(0);
  expect(run((n) => ({ commands: [decl('files.zoom')] })).out).toMatch(/outside the namespace/);
  expect(run((n) => ({ commands: [decl(`${n}.z`)], keybindings: [{ key: 'cmd+k', command: `${n}.z` }] })).out).toMatch(/locked to the shell/);
  expect(run((n) => ({ commands: [decl(`${n}.z`, { run: { action: 'format' } })] })).out).toMatch(/run.action/);
  expect(run((n) => ({ commands: [decl(`${n}.z`)], keybindings: [{ key: 'Mod+J', command: `${n}.missing` }] })).out).toMatch(/not in this package's commands/);
});
