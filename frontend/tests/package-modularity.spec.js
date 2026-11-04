// The modularity gate (docs/design/modularity-check-build-2026-11-04.md).
//
// No server and no computer: the spec builds this tree's shell into a temp
// directory, serves it with a stub backend, and proves three things for every
// first-party package, writing one receipt:
//
//   1. the SDK surface equals its lock for SDK_VERSION, and the step from the
//      previous lock is bumped enough for what changed (L1-L8);
//   2. each package's SDK names are in the lock, and its requires_sdk admits
//      no shell that lacks one (L9, L10);
//   3. each package's smoke contract passes in the new shell, bundled
//      (in-tree) and as an installed module/v1 module through the shell's real
//      loader (C1-C9, C13).
//
// The negative tests prove the guards can fail (L11, L12, C2, C6, C10, C15).
//
// Lock history (L12): CHOIR_MODULARITY_BASE (CI sets it) names the ref whose
// merge base with HEAD the locks are held to; locally it defaults to
// origin/main when that resolves.
//
// Receipt: $CHOIR_MODULARITY_RECEIPT, default
// frontend/test-results/modularity-receipt.json (CI uploads it), and a copy in
// the test's output directory with one screenshot per package and mode.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  checkLock, compareSurfaces, extractSurface, lockHistory, minSdkFor, requiresProblem, surfaceDigest,
} from '../scripts/sdk-surface.mjs';
import {
  buildShell, checkPackages, distDigest, loadPackages, makeReceipt, serveDist, validateContract, writeReceipt,
} from '../scripts/package-contracts.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const RECEIPT = process.env.CHOIR_MODULARITY_RECEIPT || path.join(frontendRoot, 'test-results/modularity-receipt.json');
const resolves = (ref) => spawnSync('git', ['-C', frontendRoot, 'rev-parse', '--verify', '--quiet', `${ref}^{commit}`]).status === 0;
const LOCK_BASE = process.env.CHOIR_MODULARITY_BASE || (resolves('origin/main') ? 'origin/main' : '');

test.describe.configure({ mode: 'serial' });

test.describe('modularity: the SDK lock and every package contract', () => {
  // N4: a missing build tool fails the gate; a skipped gate would read green.
  test('the frontend dependencies the gate needs are installed', () => {
    for (const dep of ['vite', 'typescript', '@sveltejs/vite-plugin-svelte', 'svelte']) {
      expect(fs.existsSync(path.join(frontendRoot, 'node_modules', dep, 'package.json')), `frontend/node_modules/${dep} (run pnpm install)`).toBe(true);
    }
  });

  let tmp;
  let server;
  let surface;
  const parts = { sdk: null, shell: null, packages: [], checks: {} };

  test.beforeAll(async () => {
    test.setTimeout(180_000);
    tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-modularity-'));
    // C7: the shell under test is built from this tree now, never a stale dist.
    const dist = await buildShell({ outDir: path.join(tmp, 'dist') });
    parts.shell = { digest: distDigest(dist) };
    server = await serveDist(dist);
    surface = extractSurface({ frontendRoot });
  });

  test.afterAll(async ({}, testInfo) => {
    // C11: the receipt is written whatever passed.
    const receipt = makeReceipt(parts);
    writeReceipt(RECEIPT, receipt);
    writeReceipt(testInfo.outputPath('modularity-receipt.json'), receipt);
    await server?.close();
    if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
  });

  test('the SDK surface equals its lock and every bump is enough (L1-L8)', () => {
    if (process.env.CI) expect(LOCK_BASE, 'CHOIR_MODULARITY_BASE must name the base in CI').not.toBe('');
    const r = checkLock({ frontendRoot, surface, base: LOCK_BASE });
    parts.sdk = {
      ok: r.ok, version: r.version, previous: r.previous, bump: r.bump, digest: r.digest, lock: r.lock,
      names: r.names, types: Object.keys(surface.types).length, typescript: surface.typescript, module_abi: surface.module_abi, problems: r.problems,
      history: r.history ? { base: r.history.base, merge_base: r.history.merge_base, changed: r.history.changed } : 'not checked (no base)',
      diff: r.diff ? { removed: r.diff.removed, added: r.diff.added, changed: r.diff.changed.map((c) => ({ name: c.name, need: c.need })), required: r.diff.required } : null,
    };
    expect(r.problems, r.problems.join('\n')).toEqual([]);
    // L7: no machine path in a lock.
    for (const file of fs.readdirSync(path.join(frontendRoot, 'src/lib/sdk-surface'))) {
      expect(fs.readFileSync(path.join(frontendRoot, 'src/lib/sdk-surface', file), 'utf8')).not.toMatch(/\/Users\/|\/home\/|\/private\/|[A-Z]:\\/);
    }
  });

  test('the lock rules refuse what they must (L11)', () => {
    const clone = () => JSON.parse(JSON.stringify(surface));
    const need = (mutate) => {
      const after = clone();
      mutate(after);
      return compareSurfaces(surface, after, { frontendRoot }).required;
    };
    const cases = {
      unchanged: [need(() => {}), 'none'],
      removed: [need((s) => delete s.names.clampNumber), 'major'],
      added: [need((s) => { s.names.newSdkThing = { kind: 'value', type: '() => void' }; }), 'minor'],
      value_widened: [need((s) => { s.names.fileExtension.type = '(name: string | number) => string'; }), 'minor'],
      // TypeScript holds a new optional parameter assignable both ways, so it
      // reads as a rewording: no bump (a residual the build doc names).
      value_optional_param_added: [need((s) => { s.names.appTitle.type = '(kind: any, extra?: number) => string'; }), 'none'],
      value_new_required_param: [need((s) => { s.names.appTitle.type = '(kind: any, extra: number) => string'; }), 'major'],
      value_return_widened: [need((s) => { s.names.fileExtension.type = '(name: string) => string | number'; }), 'major'],
      value_kind_changed: [need((s) => { s.names.clampNumber.kind = 'type'; }), 'major'],
      type_field_added: [need((s) => { s.types['import("./launch-file").LaunchFile'].type = s.types['import("./launch-file").LaunchFile'].type.replace('{ ', '{ extra: number; '); }), 'major'],
      type_union_reordered: [need((s) => { s.types['import("./apps/registry").AppSurfaceKind'].type = '"document" | "standard" | "media"'; }), 'none'],
      type_referenced_missing: [need((s) => { s.names.appTitle.type = '(kind: import("./no-such-file").Gone) => string'; }), 'major'],
      module_abi: [need((s) => { s.module_abi = 'choir.app-frontend/module/v2'; }), 'major'],
    };
    const got = Object.fromEntries(Object.entries(cases).map(([k, [g]]) => [k, g]));
    const want = Object.fromEntries(Object.entries(cases).map(([k, [, w]]) => [k, w]));
    expect(got).toEqual(want);

    // The version rules, against locks in a temp directory.
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-sdk-locks-'));
    try {
      const withVersion = (s, v) => ({ ...s, sdk_version: v, digest: surfaceDigest(s) });
      const lock = (s) => fs.writeFileSync(path.join(dir, `${s.sdk_version}.json`), JSON.stringify(s));
      const added = clone();
      added.names.newSdkThing = { kind: 'value', type: '() => void' };
      const removed = clone();
      delete removed.names.clampNumber;
      lock(withVersion(surface, '1.0.0'));
      const verdict = (s) => {
        const r = checkLock({ frontendRoot, dir, surface: s });
        return r.ok ? 'ok' : r.problems.join(' / ');
      };
      const rules = {
        same_version_changed: verdict(withVersion(added, '1.0.0')),
        patch_for_addition: (lock(withVersion(added, '1.0.1')), verdict(withVersion(added, '1.0.1'))),
        minor_for_addition: (fs.rmSync(path.join(dir, '1.0.1.json')), lock(withVersion(added, '1.1.0')), verdict(withVersion(added, '1.1.0'))),
        backwards: verdict(withVersion(surface, '0.9.0')),
        no_lock_for_version: verdict(withVersion(added, '1.2.0')),
        floor: verdict(withVersion({ ...surface, names: { SDK_VERSION: surface.names.SDK_VERSION } }, '1.1.0')),
        types_floor: verdict(withVersion({ ...added, types: {} }, '1.1.0')),
      };
      fs.rmSync(path.join(dir, '1.1.0.json'));
      // M1: a fabricated intermediate lock that already holds the break
      // (1.0.5 = removed) cannot carry it into 1.0.6 as "no change".
      lock(withVersion(removed, '1.0.5'));
      lock(withVersion(removed, '1.0.6'));
      rules.fabricated_intermediate = verdict(withVersion(removed, '1.0.6'));
      fs.rmSync(path.join(dir, '1.0.5.json'));
      fs.rmSync(path.join(dir, '1.0.6.json'));
      rules.minor_for_removal = (lock(withVersion(removed, '1.1.0')), verdict(withVersion(removed, '1.1.0')));
      fs.rmSync(path.join(dir, '1.1.0.json'));
      rules.major_for_removal = (lock(withVersion(removed, '2.0.0')), verdict(withVersion(removed, '2.0.0')));
      expect(rules.same_version_changed).toMatch(/differs from the lock for 1\.0\.0 \(needs a minor bump\): added newSdkThing/);
      expect(rules.patch_for_addition).toMatch(/1\.0\.0 -> 1\.0\.1 is a patch bump, but the surface change needs minor/);
      expect(rules.minor_for_addition).toBe('ok');
      expect(rules.backwards).toMatch(/older than the newest lock/);
      expect(rules.no_lock_for_version).toMatch(/no lock for SDK_VERSION 1\.2\.0/);
      expect(rules.floor).toMatch(/floor 30/);
      expect(rules.types_floor).toMatch(/references 0 local types \(floor 1\)/);
      expect(rules.minor_for_removal).toMatch(/is a minor bump, but the surface change needs major: removed clampNumber/);
      expect(rules.major_for_removal).toBe('ok');
      expect(rules.fabricated_intermediate).toMatch(/locks 1\.0\.0 -> 1\.0\.5 are a patch bump, but their surfaces differ by a major change: removed clampNumber/);

      // L10, against these locks: a name added in 1.1.0 needs ^1.1.
      lock(withVersion(added, '1.1.0'));
      expect(minSdkFor(['appTitle', 'newSdkThing'], dir)).toBe('1.1.0');
      expect(requiresProblem('^1.0', ['appTitle', 'newSdkThing'], dir)).toMatch(/admits SDK 1\.0\.0, which lacks newSdkThing; the package needs at least 1\.1\.0/);
      expect(requiresProblem('^1.1', ['appTitle', 'newSdkThing'], dir)).toBe('');
      parts.checks.lock_rules = { pass: true, cases: got, rules };
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test('a lock that exists at the base is never changed, deleted or renamed (L12)', () => {
    const repo = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-lock-history-'));
    const git = (...args) => {
      const r = spawnSync('git', ['-C', repo, '-c', 'user.name=t', '-c', 'user.email=t@example.invalid', ...args], { encoding: 'utf8' });
      if (r.status !== 0) throw new Error(`git ${args.join(' ')}: ${r.stderr}`);
      return r.stdout.trim();
    };
    try {
      const dir = path.join(repo, 'frontend/src/lib/sdk-surface');
      fs.mkdirSync(dir, { recursive: true });
      git('init', '-q', '-b', 'main');
      fs.writeFileSync(path.join(dir, '1.0.0.json'), '{"v":"1.0.0"}\n');
      git('add', '.');
      git('commit', '-q', '-m', 'base');
      git('checkout', '-q', '-b', 'topic');
      const verdict = () => {
        const h = lockHistory({ base: 'main', dir });
        return h.problems.join(' / ') || 'ok';
      };
      const reset = () => {
        git('checkout', '-q', '--', '.');
        for (const f of fs.readdirSync(dir)) if (f !== '1.0.0.json') fs.rmSync(path.join(dir, f));
      };
      const cases = {};
      cases.unchanged = verdict();
      fs.writeFileSync(path.join(dir, '1.1.0.json'), '{"v":"1.1.0"}\n');
      cases.added = verdict();
      reset();
      fs.writeFileSync(path.join(dir, '1.0.0.json'), '{"v":"1.0.0","smaller":true}\n');
      cases.modified_uncommitted = verdict();
      git('commit', '-q', '-am', 'rewrite');
      cases.modified_committed = verdict();
      git('reset', '-q', '--hard', 'main');
      fs.rmSync(path.join(dir, '1.0.0.json'));
      cases.deleted = verdict();
      fs.writeFileSync(path.join(dir, '1.0.1.json'), '{"v":"1.0.0"}\n');
      cases.renamed = verdict();
      reset();
      fs.writeFileSync(path.join(dir, '0.9.0.json'), '{"v":"0.9.0"}\n');
      cases.added_below_base = verdict();
      reset();
      cases.unreadable_base = lockHistory({ base: 'no-such-ref', dir }).problems.join(' / ');
      expect(cases.unchanged).toBe('ok');
      expect(cases.added).toBe('ok');
      expect(cases.modified_uncommitted).toMatch(/1\.0\.0\.json existed at .* and is modified/);
      expect(cases.modified_committed).toMatch(/1\.0\.0\.json existed at .* and is modified/);
      expect(cases.deleted).toMatch(/1\.0\.0\.json existed at .* and is deleted/);
      expect(cases.renamed).toMatch(/1\.0\.0\.json existed at .* and is deleted \(or renamed\)/);
      expect(cases.added_below_base).toMatch(/the new lock 0\.9\.0\.json sorts below 1\.0\.0\.json/);
      expect(cases.unreadable_base).toMatch(/cannot find the merge base of no-such-ref/);
      parts.checks.lock_history = { pass: true, cases };
    } finally {
      fs.rmSync(repo, { recursive: true, force: true });
    }
  });

  test('every package carries a valid contract (C1, C2)', () => {
    const packages = loadPackages();
    expect(packages.map((p) => p.name).length).toBeGreaterThanOrEqual(3);
    expect(packages.filter((p) => p.problems.length).map((p) => `${p.name}: ${p.problems.join('; ')}`)).toEqual([]);
    // C2: the schema refuses what it must.
    const good = { schema: 'choir.package-contract/v1', ready: '[data-x]' };
    expect(validateContract(good)).toEqual([]);
    expect(validateContract({ ...good, surprise: 1 })).toContain('unknown field "surprise"');
    expect(validateContract({ ...good, ready: '' })).toContain('ready must be a non-empty selector');
    expect(validateContract({ ...good, schema: 'v0' })).toEqual([expect.stringMatching(/schema must be/)]);
    expect(validateContract({ ...good, api: [{ method: 'GET', path: '/elsewhere' }] })).toContain('api[0].path must start with /api/');
    expect(validateContract({ ...good, allow_console: ['('] })).toContain('allow_console[0] must be a regular expression');
    for (const broad of ['GET /api/*', 'GET /api/catalog/*', 'GET /api/catalog*', 'GET /api/a/*/b']) {
      expect(validateContract({ ...good, allow_unanswered: [{ request: broad, reason: 'a good long reason' }] }), broad).toEqual([expect.stringMatching(/is too broad/)]);
    }
    expect(validateContract({ ...good, allow_unanswered: [{ request: 'GET /api/catalog/releases/*', reason: 'a good long reason' }] })).toEqual([]);
    // L1: the computer's module build embeds a byte copy of the boundary plugin.
    expect(fs.readFileSync(path.join(frontendRoot, 'scripts/package-boundary-plugin.mjs')).equals(fs.readFileSync(path.join(frontendRoot, '../internal/pkgruntime/package-boundary-plugin.mjs'))), 'internal/pkgruntime/package-boundary-plugin.mjs must be a byte copy of frontend/scripts/package-boundary-plugin.mjs').toBe(true);
    expect(validateContract({ ...good, allow_unanswered: [{ request: 'GET /api/x' }] })).toContain('allow_unanswered[0].reason must say why (10 characters or more)');
    expect(validateContract({ ...good, allow_unanswered: [{ request: '/api/x', reason: 'a good long reason' }] })).toEqual([expect.stringMatching(/request must be "METHOD \/api\/path"/)]);
    expect(validateContract({ ...good, expect: [{ selector: 'a', count: -1 }] })).toContain('expect[0].count must be a whole number');
  });

  test('every first-party package passes its contract in the new shell, bundled and installed (C3-C9, L9, L10)', async ({ browser }, testInfo) => {
    test.setTimeout(240_000);
    const lockNames = new Set(Object.keys(surface.names));
    const sdkCheck = (row) => {
      const unknown = row.sdk_imports.filter((n) => !lockNames.has(n));
      if (unknown.length) row.problems.push(`imports SDK names the lock for ${surface.sdk_version} lacks: ${unknown.join(', ')} (L9)`);
      row.min_sdk = minSdkFor(row.sdk_imports);
      const problem = requiresProblem(row.requires_sdk, row.sdk_imports);
      if (problem) row.problems.push(`${problem} (L10)`);
    };
    const rows = await checkPackages({ browser, base: server.base, packages: loadPackages(), sdkCheck, screenshotDir: testInfo.outputPath('screens') });
    parts.packages = rows;
    const failures = rows.filter((r) => !r.pass).map((r) => `${r.name}: ${[...r.problems, ...Object.values(r.modes).flatMap((m) => m.problems.map((p) => `${m.mode}: ${p}`))].join('\n  ')}`);
    expect(failures, failures.join('\n')).toEqual([]);
    for (const r of rows) {
      expect(r.modes['in-tree'].pass && r.modes.installed.pass).toBe(true);
      expect(r.sdk_imports.length).toBeGreaterThan(0);
      expect(r.module.sha256).toMatch(/^[0-9a-f]{64}$/);
    }
  });

  test('the runner fails what it must, and the shell refuses a mismatched app cleanly (C6, C10)', async ({ browser }) => {
    test.setTimeout(180_000);
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-contract-fixtures-'));
    const manifest = (name, extra = {}) => ({
      schema: 'choir.app-package/v1', name, version: '1.0.0', origin: 'first_party',
      display: { title: name, icon: 'F', description: 'Contract fixture' },
      app: {
        entry: 'App.svelte', abi: 'choir.app-frontend/svelte-inline/v1', aliases: [],
        launcher: { menu: true, desktopIcon: false, order: 950 }, window: { singleton: false, heavy: false },
        auth: { preview: 'private' }, theme: { surface: 'standard', shellDataAttr: `data-${name}-window`, contentClass: `${name}-content` },
        ...extra,
      },
      opens: [], kinds: [], uses_kinds: [], desk: null, backend: null, grants: [],
    });
    const contract = (ready = '[data-fixture-app]', more = {}) => ({ schema: 'choir.package-contract/v1', ready, expect: [{ selector: '[data-fixture-title]', text: 'Fixture' }], ...more });
    const fixture = (name, app, { extra, ready, more } = {}) => {
      fs.mkdirSync(path.join(dir, name), { recursive: true });
      fs.writeFileSync(path.join(dir, name, 'choir-package.json'), JSON.stringify(manifest(name, extra)));
      fs.writeFileSync(path.join(dir, name, 'App.svelte'), app);
      fs.writeFileSync(path.join(dir, name, 'choir-contract.json'), JSON.stringify(contract(ready, more)));
    };
    const GOOD = `<script>import { appTitle } from '../../lib/app-sdk';</script>\n<section data-fixture-app><p data-fixture-title>Fixture {appTitle('fixture')}</p></section>\n`;
    fixture('fx-good', GOOD);
    fixture('fx-vacuous', GOOD, { ready: '[data-desk-menu-button]' });
    fixture('fx-missing', GOOD, { ready: '[data-never-there]' });
    fixture('fx-throws', `<script>throw new Error('fixture throws on mount');</script>\n<p>never</p>\n`);
    fixture('fx-console', `<script>import { onMount } from 'svelte'; onMount(() => console.error('fixture logged an error'));</script>\n<section data-fixture-app><p data-fixture-title>Fixture</p></section>\n`);
    fixture('fx-late-throw', `<script>import { onMount } from 'svelte'; onMount(() => { setTimeout(() => { throw new Error('fixture throws later'); }, 50); });</script>\n<section data-fixture-app><p data-fixture-title>Fixture</p></section>\n`);
    fixture('fx-no-name', `<script>import { noSuchSdkName } from '../../lib/app-sdk';</script>\n<section data-fixture-app><p data-fixture-title>Fixture {noSuchSdkName}</p></section>\n`);
    fixture('fx-future-sdk', GOOD, { extra: { requires_sdk: '^9.0' } });
    const FETCHES = `<script>import { onMount } from 'svelte'; import { fetchWithRenewal } from '../../lib/app-sdk'; onMount(() => { fetchWithRenewal('/api/fixture/data').catch(() => {}); });</script>\n<section data-fixture-app><p data-fixture-title>Fixture</p></section>\n`;
    fixture('fx-unanswered', FETCHES);
    fixture('fx-allowed', FETCHES, { more: { allow_unanswered: [{ request: 'GET /api/fixture/data', reason: 'the fixture renders without its data' }] } });
    fixture('fx-stubbed', FETCHES, { more: { api: [{ method: 'GET', path: '/api/fixture/data', status: 200, json: { ok: true } }] } });
    fixture('fx-sdk-global', `<script>const t = globalThis.__choirAppSdk.appTitle('fixture');</script>\n<section data-fixture-app><p data-fixture-title>Fixture {t}</p></section>\n`);
    fixture('fx-sdk-template', '<script>const t = globalThis[`__choirAppSdk`].appTitle(\'fixture\');</script>\n<section data-fixture-app><p data-fixture-title>Fixture {t}</p></section>\n');
    try {
      // The probation path: trees from elsewhere, installed mode only.
      const rows = await checkPackages({ browser, base: server.base, packages: loadPackages(dir), modes: ['installed'] });
      const by = Object.fromEntries(rows.map((r) => [r.name, r]));
      const problems = (n) => by[n].modes.installed.problems.join(' / ');
      for (const n of ['fx-good', 'fx-allowed', 'fx-stubbed']) expect(by[n].pass, `${n}: ${problems(n)}`).toBe(true);
      expect(by['fx-allowed'].modes.installed.allowed_unanswered).toContain('GET /api/fixture/data');
      expect(problems('fx-unanswered')).toMatch(/unanswered: GET \/api\/fixture\/data \(stub it/);
      // The SDK global would let a package use names its sdk_imports never lists: the build refuses it.
      for (const n of ['fx-sdk-global', 'fx-sdk-template']) expect(by[n].problems.join(' / '), n).toMatch(/module\/v1 build failed.*__choirAppSdk is the shell's, not a package's/);
      expect(problems('fx-vacuous')).toMatch(/ready selector "\[data-desk-menu-button\]" did not show inside the fx-vacuous window/);
      expect(problems('fx-missing')).toMatch(/ready selector "\[data-never-there\]" did not show/);
      expect(problems('fx-throws')).toMatch(/did not mount the installed module \(state failed\).*fixture throws on mount/);
      expect(problems('fx-console')).toMatch(/console\.error: fixture logged an error/);
      expect(problems('fx-late-throw')).toMatch(/uncaught: .*fixture throws later/);
      expect(problems('fx-no-name')).toMatch(/state refused\).*uses SDK names this shell lacks: noSuchSdkName/);
      expect(problems('fx-future-sdk')).toMatch(/state refused\).*needs SDK \^9\.0; this shell has 1\.0\.0/);
      // A refused or failed app never takes the shell down with it.
      for (const n of ['fx-throws', 'fx-no-name', 'fx-future-sdk']) expect(by[n].modes.installed.boot_errors).toEqual([]);
      for (const n of Object.keys(by).filter((k) => !['fx-good', 'fx-allowed', 'fx-stubbed'].includes(k))) expect(by[n].pass, n).toBe(false);
      parts.checks.runner_negative = {
        pass: true,
        rows: rows.map((r) => ({ name: r.name, pass: r.pass, sdk_imports: r.sdk_imports, problems: [...r.problems, ...r.modes.installed.problems] })),
      };
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});
