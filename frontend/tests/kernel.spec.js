// Front-end kernel, slice MA2.2
// (docs/design/modular-architecture-plan-2026-11-04.md section 5.1/5.2).
// No browser or server: contract tests on the kernel, the K0 leak
// differential, and the structural gate against fixture trees.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createKernel } from '../src/lib/kernel/index';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');

const tick = () => new Promise((r) => setTimeout(r, 0));

// ---- scope disposal order ----

test('children die before the parent effects; effects reverse LIFO and are awaited', async () => {
  const k = createKernel();
  const log = [];
  const parent = k.scope('p');
  const child = k.scope('c', parent);
  parent.effect(() => { log.push('p1'); });
  parent.effect(async () => { await tick(); log.push('p2'); });
  child.effect(() => { log.push('c1'); });
  child.effect(async () => { await tick(); log.push('c2'); });
  await parent.dispose();
  expect(log).toEqual(['c2', 'c1', 'p2', 'p1']);
  expect(k.snapshot().scopes.children).toEqual([]);
});

test('a throwing disposer does not stop the rest; errors are reported; double dispose is shared', async () => {
  const k = createKernel();
  const log = [];
  const s = k.scope('s');
  s.effect(() => { log.push('a'); });
  s.effect(() => { throw new Error('boom'); });
  s.effect(() => { log.push('c'); });
  const [r1, r2] = await Promise.all([s.dispose(), s.dispose()]);
  expect(log).toEqual(['c', 'a']);
  expect(r1.errors).toHaveLength(1);
  expect(r2).toBe(r1);
  expect(() => s.effect(() => {})).toThrow(/disposed/);
});

test('an effect disposer run early removes it from the scope and runs once', async () => {
  const k = createKernel();
  const s = k.scope('s');
  let n = 0;
  const off = s.effect(() => { n++; });
  await off();
  await off();
  await s.dispose();
  expect(n).toBe(1);
});

// ---- service ranking, failover, swap order ----

function setup(opts) {
  const k = createKernel(opts);
  const sys = k.scope('system');
  k.services.define(sys, 'menu.renderer', 'choir.service/menu.renderer/v1');
  return { k, sys };
}

test('ranking: policy, selection, priority, recency, base; rank.high is needed above 50', async () => {
  const { k, sys } = setup();
  const S = 'menu.renderer';
  const p = (id, extra = {}) => k.services.provide(k.scope(id), S, { id, impl: id, ...extra });
  p('base', { base: true, priority: 50 });
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('base');
  p('claim', { priority: 90 }); // no grant: counts as 50, ties base, newer wins
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('claim');
  p('granted', { priority: 80, grants: ['rank.high'] });
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('granted');
  const unsel = k.services.select(sys, S, 'base');
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('base');
  const unpol = k.services.setPolicy(sys, S, 'claim');
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('claim');
  await unpol();
  await unsel();
  const undecl = k.services.decline(sys, S, 'granted');
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('claim');
  await undecl(); // disposers enqueue; settled() is the wait
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('granted');
});

test('failover: watchdog un-ranks a provider after N failures in the window; base serves', async () => {
  let t = 0;
  const { k } = setup({ now: () => t, watchdog: { failures: 2, windowMs: 100 } });
  const S = 'menu.renderer';
  k.services.provide(k.scope('b'), S, { id: 'base', impl: 'base', base: true, priority: 10 });
  k.services.provide(k.scope('n'), S, { id: 'new', impl: 'new', priority: 60, grants: ['rank.high'] });
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('new');
  expect(k.services.reportFailure(S, 'new', 'throw 1')).toBe(false);
  t = 500; // outside the window: the first failure aged out
  expect(k.services.reportFailure(S, 'new', 'throw 2')).toBe(false);
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('new');
  t = 520;
  expect(k.services.reportFailure(S, 'new', 'budget overrun')).toBe(true); // the report that trips it says so, once
  expect(k.services.reportFailure(S, 'new', 'after')).toBe(false);
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('base');
  const entry = k.snapshot().services[S].providers.find((x) => x.id === 'new');
  expect(entry.tripped).toBe('budget overrun');
  expect(entry.healthy).toBe(false);
});

test('no healthy candidate: no active provider and consumers stay inactive', async () => {
  const { k } = setup();
  const S = 'menu.renderer';
  k.services.provide(k.scope('b'), S, { id: 'base', impl: 'base', base: true });
  let on = 0;
  k.services.consume(k.scope('c'), [S], () => { on++; });
  await k.services.settled();
  expect(on).toBe(1);
  expect(k.services.trip(S, 'base', 'set aside by the test')).toBe(true);
  expect(k.services.trip(S, 'base', 'again')).toBe(false);
  expect(k.snapshot().services[S].providers[0].tripped).toBe('set aside by the test');
  await k.services.settled();
  expect(k.services.resolve(S)).toBeUndefined();
  expect(k.snapshot().services[S].consumers[0].active).toBe(false);
});

test('swap order: consumers off, old provider off, new provider on, consumers on', async () => {
  const { k } = setup();
  const S = 'menu.renderer';
  const log = [];
  const prov = (id, extra = {}) => k.services.provide(k.scope(id), S, {
    id, impl: id,
    activate: (act) => { log.push(`${id}:on`); act.effect(() => { log.push(`${id}:off`); }); },
    ...extra,
  });
  prov('old', { base: true });
  k.services.consume(k.scope('c1'), [S], (svc, act) => {
    log.push(`c1:on(${svc[S]})`);
    act.effect(() => { log.push('c1:off'); });
  });
  k.services.consume(k.scope('c2'), [S], (svc, act) => {
    log.push(`c2:on(${svc[S]})`);
    act.effect(() => { log.push('c2:off'); });
  });
  await k.services.settled();
  log.length = 0;
  prov('new', { priority: 60, grants: ['rank.high'] });
  await k.services.settled();
  expect(log).toEqual(['c2:off', 'c1:off', 'old:off', 'new:on', 'c1:on(new)', 'c2:on(new)']);
});

test('a consumer needing two services waits for both', async () => {
  const k = createKernel();
  const sys = k.scope('sys');
  let on = 0;
  k.services.consume(sys, ['a', 'b'], () => { on++; });
  const pa = k.services.provide(k.scope('pa'), 'a', { id: 'pa', impl: 1 });
  await k.services.settled();
  expect(on).toBe(0);
  k.services.provide(k.scope('pb'), 'b', { id: 'pb', impl: 2 });
  await k.services.settled();
  expect(on).toBe(1);
  await pa();
  expect(k.snapshot().services.a.consumers[0].active).toBe(false);
});

test('a provider whose activate throws is un-ranked and the next one serves', async () => {
  const { k } = setup();
  const S = 'menu.renderer';
  k.services.provide(k.scope('b'), S, { id: 'base', impl: 'base', base: true, priority: 10 });
  k.services.provide(k.scope('x'), S, {
    id: 'bad', impl: 'bad', priority: 60, grants: ['rank.high'],
    activate: () => { throw new Error('nope'); },
  });
  await k.services.settled();
  expect(k.services.resolve(S)).toBe('base');
});

// ---- pipelines ----

test('pipelines: tier order, frozen input, origin stamped, failures skipped, re-entry refused', async () => {
  const k = createKernel();
  const sys = k.scope('sys');
  k.pipelines.define(sys, 'menu.build', { budgetMs: 50 });
  k.pipelines.define(sys, 'other', { budgetMs: 50 });
  const order = [];
  const use = (id, tier, fn) => k.pipelines.use(k.scope(id), 'menu.build', { id, tier }, fn);
  use('core1', 'core', (items) => { order.push('core1'); return [...items, { label: 'c', origin: 'forged' }]; });
  use('user1', 'user', (items) => { order.push('user1'); return [...items, { label: 'u' }]; });
  use('policy1', 'policy', (items) => { order.push('policy1'); });
  use('mutator', 'app', (items) => { order.push('mutator'); items[0].label = 'x'; });
  use('slow', 'app', () => new Promise(() => {}));
  use('reenter', 'app', (items, ctx) => ctx.run('menu.build', items));
  const r = await k.pipelines.run('menu.build', [{ label: 'seed' }]);
  expect(order).toEqual(['policy1', 'user1', 'mutator', 'core1']);
  expect(r.items.map((i) => i.label)).toEqual(['seed', 'u', 'c']);
  expect(r.items.map((i) => i.origin)).toEqual([undefined, 'user1', 'core1']);
  expect(r.skipped.map((s) => s.handler).sort()).toEqual(['mutator', 'reenter', 'slow']);
  expect(r.skipped.find((s) => s.handler === 'slow').reason).toBe('over budget');
});

// ---- re-entrant disposal (review B1) ----

const withTimeout = (p, ms, what) =>
  Promise.race([p, new Promise((_, rej) => setTimeout(() => rej(new Error(`${what} did not resolve`)), ms))]);

test('a provider whose activate consumes on its act scope swaps and disposes without deadlock', async () => {
  const { k } = setup();
  const S = 'menu.renderer';
  k.services.provide(k.scope('base'), S, { id: 'base', impl: 'base', base: true });
  const pkg = k.scope('derived');
  k.services.provide(pkg, S, {
    id: 'd', impl: 'd', priority: 70, grants: ['rank.high'],
    activate: (act) => { k.services.consume(act, [S], () => {}); },
  });
  await withTimeout(k.services.settled(), 1000, 'settled after provide');
  expect(k.services.resolve(S)).toBe('d');
  const base2 = k.scope('base2');
  k.services.provide(base2, S, { id: 'e', impl: 'e', priority: 90, grants: ['rank.high'] });
  await withTimeout(k.services.settled(), 1000, 'settled after swap');
  expect(k.services.resolve(S)).toBe('e');
  const report = await withTimeout(pkg.dispose(), 1000, 'dispose');
  expect(report.errors).toEqual([]);
  await withTimeout(k.services.settled(), 1000, 'settled after dispose');
  expect(k.services.errors).toEqual([]);
});

test('a consumer fn that provides on its act scope tears down without deadlock', async () => {
  const { k } = setup();
  const S = 'menu.renderer';
  k.services.define(k.root, 'inner', 'choir.service/inner/v1');
  const pkg = k.scope('c');
  k.services.provide(k.scope('base'), S, { id: 'base', impl: 'base', base: true });
  k.services.consume(pkg, [S], (svc, act) => {
    k.services.provide(act, 'inner', { id: 'i', impl: 'i' });
  });
  await withTimeout(k.services.settled(), 1000, 'settled');
  expect(k.services.resolve('inner')).toBe('i');
  const report = await withTimeout(pkg.dispose(), 1000, 'dispose');
  expect(report.errors).toEqual([]);
  await withTimeout(k.services.settled(), 1000, 'settled after dispose');
  expect(k.services.resolve('inner')).toBeUndefined();
});

test('a failing queue job is recorded in errors and trace, not swallowed', async () => {
  const { k } = setup();
  k.services.provide(k.scope('base'), 'menu.renderer', { id: 'base', impl: 'base', base: true });
  await k.services.settled();
  // a consumer whose owner died before its start job runs makes fork throw inside the job
  const dead = k.scope('dead');
  k.services.consume(dead, ['menu.renderer'], () => {});
  await dead.dispose();
  await k.services.settled();
  expect(k.services.errors.length).toBe(k.services.trace.filter((t) => t.startsWith('job-error:')).length);
});

test('a pipeline item that cannot be structured-cloned is reported per handler', async () => {
  const k = createKernel();
  const sys = k.scope('system');
  k.pipelines.define(sys, 'p', { budgetMs: 50 });
  k.pipelines.use(sys, 'p', { id: 'h' }, (items) => items);
  const r = await k.pipelines.run('p', [{ f: () => 1 }]);
  expect(r.skipped.map((x) => x.handler)).toEqual(['h']);
});

// ---- the K0 leak differential ----

test('leak differential: install, activate, exercise, uninstall; snapshot equals baseline', async () => {
  const k = createKernel();
  const sys = k.scope('system');
  k.services.define(sys, 'menu.renderer', 'choir.service/menu.renderer/v1');
  k.pipelines.define(sys, 'menu.build', { budgetMs: 50 });
  k.services.provide(k.scope('base'), 'menu.renderer', { id: 'base', impl: 'base', base: true });
  await k.services.settled();
  // Scope ids and states are in the snapshot, so identity leaks show. The
  // provider/consumer act scopes are recreated by design on every swap (new
  // id); every scope a caller created keeps its id and must match exactly.
  const stripActIds = (node) => {
    if (node && typeof node === 'object') {
      if (typeof node.name === 'string' && /^(provider|consumer):/.test(node.name)) node.id = 'act';
      for (const c of node.children ?? []) stripActIds(c);
    }
    return node;
  };
  const view = () => {
    const snap = JSON.parse(JSON.stringify(k.snapshot()));
    stripActIds(snap.scopes);
    return snap;
  };
  const baseline = view();

  for (let round = 0; round < 3; round++) {
    const pkg = k.scope(`pkg-${round}`);
    const timers = [];
    k.services.define(pkg, `extra.${round}`, `choir.service/extra.${round}/v1`);
    k.services.provide(pkg, 'menu.renderer', {
      id: `fixture-${round}`, impl: 'fixture', priority: 70, grants: ['rank.high'],
      activate: (act) => {
        const t = setInterval(() => {}, 1000);
        timers.push(t);
        act.effect(() => clearInterval(t));
      },
    });
    k.services.consume(pkg, ['menu.renderer'], (svc, act) => {
      const t = setTimeout(() => {}, 100000);
      act.effect(() => clearTimeout(t));
    });
    k.services.select(pkg, 'menu.renderer', `fixture-${round}`);
    k.services.decline(pkg, 'menu.renderer', 'nobody');
    k.pipelines.use(pkg, 'menu.build', { id: `h-${round}` }, (items) => [...items, { x: 1 }]);
    await k.services.settled();
    expect(k.services.resolve('menu.renderer')).toBe('fixture');
    expect((await k.pipelines.run('menu.build', [])).items).toHaveLength(1);
    expect(view()).not.toEqual(baseline);
    const report = await pkg.dispose();
    expect(report.errors).toEqual([]);
    await k.services.settled();
    expect(k.services.resolve('menu.renderer')).toBe('base');
    expect(view()).toEqual(baseline);
  }
});

test('snapshot is deeply read-only', async () => {
  const k = createKernel();
  const snap = k.snapshot();
  expect(Object.isFrozen(snap)).toBe(true);
  expect(Object.isFrozen(snap.scopes)).toBe(true);
});

// ---- the structural gate, against fixture trees ----

function makeFixture(mutate) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-kernel-'));
  fs.cpSync(path.join(frontendRoot, 'src/lib/kernel'), path.join(root, 'src/lib/kernel'), { recursive: true });
  fs.mkdirSync(path.join(root, 'src/lib/apps'), { recursive: true });
  fs.copyFileSync(path.join(frontendRoot, 'src/lib/apps/registry.ts'), path.join(root, 'src/lib/apps/registry.ts'));
  // The gate reads package names from the manifests (every app but Texture is a package).
  fs.cpSync(path.join(frontendRoot, 'src/packages'), path.join(root, 'src/packages'), {
    recursive: true,
    filter: (src) => fs.statSync(src).isDirectory() || src.endsWith('choir-package.json') || src.endsWith('base-packages.json'),
  });
  if (mutate) mutate(root);
  return root;
}

function gate(root) {
  return spawnSync(process.execPath, [path.join(frontendRoot, 'scripts/check-kernel.mjs'), '--root', root], { encoding: 'utf8' });
}

function append(root, file, text) {
  fs.appendFileSync(path.join(root, 'src/lib/kernel', file), text);
}

test('gate: the real tree passes', () => {
  const r = gate(frontendRoot);
  expect(r.stderr).toBe('');
  expect(r.status).toBe(0);
});

test('gate: a bare or out-of-kernel import is refused', () => {
  for (const line of ["import x from 'svelte';\n", "import y from '../app-sdk';\n", "const m = import('./scope');\n"]) {
    const root = makeFixture((r) => append(r, 'scope.ts', line));
    const r = gate(root);
    expect(r.status, line).toBe(1);
    expect(r.stderr).toMatch(/imports|import\(\)|dynamic import/);
  }
});

test('gate: an app id, package name or desk in kernel source is refused', () => {
  for (const word of ['browser', 'management', 'texture']) {
    const root = makeFixture((r) => append(r, 'service.ts', `\nconst x = '${word}';\n`));
    const r = gate(root);
    expect(r.status, word).toBe(1);
    expect(r.stderr).toContain(`names "${word}"`);
  }
});

test('gate: an unlisted export, or a stale listing, is refused', () => {
  let root = makeFixture((r) => append(r, 'effect.ts', '\nexport function sneaky() {}\n'));
  let r = gate(root);
  expect(r.status).toBe(1);
  expect(r.stderr).toContain('exports "sneaky"');
  root = makeFixture((x) => {
    const f = path.join(x, 'src/lib/kernel/responsibilities.json');
    fs.writeFileSync(f, fs.readFileSync(f, 'utf8').replace('"runLifo"', '"runLifo", "ghost"'));
  });
  r = gate(root);
  expect(r.status).toBe(1);
  expect(r.stderr).toContain('lists "ghost"');
  root = makeFixture((x) => fs.writeFileSync(path.join(x, 'src/lib/kernel/extra.ts'), 'export const z = 1;\n'));
  r = gate(root);
  expect(r.status).toBe(1);
  expect(r.stderr).toContain('in no responsibility');
});
