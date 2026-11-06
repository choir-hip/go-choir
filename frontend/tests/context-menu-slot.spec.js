// The context menu on the slot, slice S4 (lib/services/context-menu.ts). No
// browser: the service on the real kernel, loaded through vite. Failure modes
// X1 to X9 are listed in the service's header; the K0 renderer-swap drill runs
// here (a renderer that fails its contract is refused and the base menu still
// opens; one that passes is used and can be set aside from the menu itself).
import { test, expect } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite; let M; let C; let K; let X; let O;
const shown = [];
let waiter = null;
test.beforeAll(async () => {
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  M = await vite.ssrLoadModule('/src/lib/services/context-menu.ts');
  C = await vite.ssrLoadModule('/src/lib/services/commands.ts');
  K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
  X = await vite.ssrLoadModule('/src/lib/contributions.ts');
  O = await vite.ssrLoadModule('/src/lib/services/open-with-menu.ts');
  M.installBaseRenderer((req) => { shown.push(['base', req]); waiter?.(); });
  await K.shellKernel.services.settled();
});
test.afterAll(async () => { await vite?.close(); });

const next = () => new Promise((resolve) => { waiter = resolve; });
const labels = (req) => req.items.map((i) => (i.separator ? '---' : i.label));
async function present(request) {
  const done = next();
  const before = shown.length;
  expect(M.presentMenu(request)).toBe(true);
  await done;
  expect(shown.length).toBe(before + 1);
  return shown.at(-1);
}
const base = () => [{ id: 'open', label: 'Open', onSelect: () => {} }, { id: 'rename', label: 'Rename', onSelect: () => {} }];

test('X8: a target nothing intercepts opens synchronously through the base renderer', () => {
  const before = shown.length;
  expect(M.presentMenu({ x: 1, y: 2, items: base() })).toBe(true);
  expect(shown.length).toBe(before + 1);
  expect(shown.at(-1)[1].target.kind).toBe('other');
  expect(M.presentMenu({ x: 1, y: 2, items: [] })).toBe(false);
});

test('Open with is a host contribution on file targets, after Open, with the app\'s own callback', async () => {
  const picked = [];
  const [, req] = await Promise.resolve(present({ x: 0, y: 0, items: base(), target: { kind: 'file', name: 'Deck.PPTX', data: { openWith: (o) => picked.push(o.appId) } } }));
  expect(labels(req).slice(0, 3)).toEqual(['Open', 'Open with', 'Rename']);
  expect(req.target).toEqual({ kind: 'file', ext: 'pptx', name: 'Deck.PPTX' }); // data never leaves the host
  const sub = req.items[1].items;
  expect(sub.length).toBeGreaterThan(0);
  sub[0].onSelect();
  expect(picked.length).toBe(1);
  // a folder, or a file row that passes no callback, gets none
  const [, other] = await Promise.resolve(present({ x: 0, y: 0, items: base(), target: { kind: 'folder', name: 'docs' } }));
  expect(labels(other)).toEqual(['Open', 'Rename']);
});

test('X1, X2: a renderer that fails its contract is refused; one that throws falls back and is un-ranked; one that passes is used and can be set aside', async () => {
  const scope = K.packageScope('menu-compact');
  expect(() => M.registerContextMenuRenderer(scope, { id: 'broken', impl: {} })).toThrow(/render must be a function/);
  // the base is the host's last-resort menu (H2: menu-default is an ordinary package provider)
  expect(() => M.registerContextMenuRenderer(scope, { id: 'last-resort', impl: { render() {} } })).toThrow(/shell's/);
  await K.shellKernel.services.settled();
  let [who] = await Promise.resolve(present({ x: 0, y: 0, items: base() }));
  expect(who).toBe('base');

  const custom = [];
  const off = M.registerContextMenuRenderer(scope, { id: 'compact', priority: 40, impl: { render(req) { custom.push(req); } } });
  await K.shellKernel.services.settled();
  expect(K.shellKernel.services.resolve(M.RENDERER_SLOT)).toBeTruthy();
  const before = shown.length;
  M.presentMenu({ x: 0, y: 0, items: base() });
  expect(custom.length).toBe(1);
  expect(shown.length).toBe(before); // the base did not also render
  // the essential escape hatch is there and cannot be missing
  const last = custom[0].items.at(-1);
  expect(last).toMatchObject({ id: 'use-default-menu', essential: true });
  last.onSelect();
  await K.shellKernel.services.settled();
  [who] = await Promise.resolve(present({ x: 0, y: 0, items: base() }));
  expect(who).toBe('base'); // declined: the base serves again
  await off();
  await K.disposePackageScope('menu-compact');
  await K.shellKernel.services.settled();

  // a renderer that throws: the base renders this menu, and three failures un-rank it
  const scope2 = K.packageScope('menu-bad');
  M.registerContextMenuRenderer(scope2, { id: 'bad', priority: 40, impl: { render() { throw new Error('boom'); } } });
  await K.shellKernel.services.settled();
  for (let i = 0; i < 3; i++) {
    const n = shown.length;
    M.presentMenu({ x: 0, y: 0, items: base() });
    expect(shown.length, `menu ${i} answered by the base`).toBe(n + 1);
  }
  await K.shellKernel.services.settled();
  const info = K.shellKernel.services.describe().services[M.RENDERER_SLOT];
  expect(info.active).toBe(M.BASE_RENDERER_ID);
  expect(info.providers.find((p) => p.id === 'bad').tripped).toMatch(/boom/);
  await K.disposePackageScope('menu-bad');
  await K.shellKernel.services.settled();
});

test('contributes.menus adds a package command to a target; selecting it runs it with the target; uninstall leaves the baseline', async () => {
  await K.shellKernel.services.settled();
  const baseline = JSON.stringify(K.shellKernel.snapshot());
  const toasts = [];
  const unbind = C.bindCommandActions({ openApp() {}, openFile() {}, copyText() {}, setTheme() {}, toast: (m) => toasts.push(m) });
  const contributes = {
    commands: [
      { id: 'tools.hello', title: 'Say hello', scope: 'target:file', run: { action: 'toast', message: 'hello' } },
      { id: 'tools.anywhere', title: 'Anywhere', scope: 'global', run: { action: 'toast', message: 'anywhere' }, when: 'target.ext == json' },
    ],
    menus: [
      { target: 'file', command: 'tools.hello', group: 'tools', order: 2 },
      { target: 'file', command: 'tools.anywhere', group: 'tools', order: 1 },
      { target: 'window', command: 'tools.hello' },
    ],
  };
  await X.setPackageContributions('tools', contributes, 'v1');
  let [, req] = await Promise.resolve(present({ x: 0, y: 0, items: base(), target: { kind: 'file', name: 'a.txt' } }));
  expect(labels(req)).toEqual(['Open', 'Rename', '---', 'Say hello']); // "Anywhere" needs ext json
  [, req] = await Promise.resolve(present({ x: 0, y: 0, items: base(), target: { kind: 'file', name: 'a.json' } }));
  expect(labels(req)).toEqual(['Open', 'Rename', '---', 'Anywhere', 'Say hello']);
  req.items.at(-1).onSelect();
  await new Promise((r) => setTimeout(r, 20));
  expect(toasts).toEqual(['hello']);
  // a window target gets the global/target:window ones only: "Say hello" is target:file, so it is not shown
  const n = shown.length;
  M.presentMenu({ x: 0, y: 0, items: base(), target: { kind: 'window' } });
  await new Promise((r) => setTimeout(r, 20));
  expect(labels(shown.at(-1)[1])).toEqual(['Open', 'Rename']);
  expect(shown.length).toBe(n + 1);
  await X.setPackageContributions('tools', null, '');
  unbind();
  await K.shellKernel.services.settled();
  expect(JSON.stringify(K.shellKernel.snapshot())).toBe(baseline);
  [, req] = await Promise.resolve(present({ x: 0, y: 0, items: base(), target: { kind: 'file', name: 'a.json' } }));
  expect(labels(req)).toEqual(['Open', 'Rename']);
});

test('X3, X4, X5: a handler cannot remove or edit an essential item, run a callback it invented, or break the menu', async () => {
  const scope = K.packageScope('menu-tidy');
  const items = [
    { id: 'close', label: 'Close', essential: true, onSelect: () => {} },
    { id: 'rename', label: 'Rename', onSelect: () => {} },
    { id: 'delete', label: 'Delete', danger: true, onSelect: () => {} },
  ];
  M.useContextMenuBuild(scope, { id: 'menu-tidy:tidy', targets: ['file'] }, (list) => list
    .filter((i) => i.id !== 'close') // remove the essential one
    .map((i) => (i.id === 'rename' ? { ...i, label: 'Rename it' } : i)) // relabel a normal one
    .concat([
      { id: 'forged', label: 'No command' }, // X5 dropped
      { id: 'ghost', label: 'Unregistered', command: 'tidy.nope' }, // X5 dropped
    ]));
  M.useContextMenuBuild(scope, { id: 'menu-tidy:throws', targets: ['file'] }, () => { throw new Error('bad handler'); });
  const warn = console.warn; const notes = []; console.warn = (m) => notes.push(String(m));
  try {
    const [, req] = await Promise.resolve(present({ x: 0, y: 0, items, target: { kind: 'file', name: 'x.txt' } }));
    expect(req.items.map((i) => `${i.label}${i.essential ? '!' : ''}`).sort()).toEqual(['Close!', 'Delete', 'Rename it']);
    expect(req.items.find((i) => i.id === 'delete').danger).toBe(true);
    expect(notes.join(' ')).toMatch(/menu-tidy:throws skipped/);
    // another target is not intercepted at all: synchronous, untouched
    const n = shown.length;
    M.presentMenu({ x: 0, y: 0, items, target: { kind: 'window' } });
    expect(shown.length).toBe(n + 1);
    expect(shown.at(-1)[1].items.length).toBe(3);
  } finally { console.warn = warn; }
  await K.disposePackageScope('menu-tidy');
});

test('contributes.menus entries naming another package\'s command are refused', async () => {
  const warn = console.warn; const notes = []; console.warn = (m) => notes.push(String(m));
  try {
    await X.setPackageContributions('mine', { commands: [{ id: 'mine.a', title: 'A', scope: 'global', run: { action: 'toast', message: 'a' } }], menus: [{ target: 'file', command: 'other.a' }] }, 'v1');
    expect(notes.join(' ')).toMatch(/outside "mine"/);
    const [, req] = await Promise.resolve(present({ x: 0, y: 0, items: base(), target: { kind: 'file', name: 'a.txt' } }));
    expect(labels(req)).toEqual(['Open', 'Rename']);
  } finally { console.warn = warn; await X.setPackageContributions('mine', null, ''); }
});
