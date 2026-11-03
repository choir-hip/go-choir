// Failure modes of scripts/choir-shot.mjs, then one real run: node scripts/choir_shot_test.mjs
// The real run needs CHOIR_FRONTEND_DEPS and CHOIR_BROWSER (a capsule job has both)
// and is skipped without them.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import vm from 'node:vm';
import {
  autoputerEnv, isProxied, missingDependencies, pickAutoputerBinary, parseArgs, plan, themeIdsFrom, themeSeedScript, themeAnswer,
  parseColor, over, resolveBackground, contrastRatio, requiredRatio, samplePixels, measureNode, summarizeContrast, contrastDir,
} from './choir-shot.mjs';

const throws = (f, re) => assert.throws(f, re);
throws(() => parseArgs([]), /--shots FILE is required/);
throws(() => parseArgs(['--shots', 'x', '--preset', 'watch']), /--preset watch/);
throws(() => parseArgs(['--shots', 'x', '--size', '10']), /WIDTHxHEIGHT/);
throws(() => parseArgs(['--shots', 'x', '--size', '800x600', '--preset', 'mobile']), /exclusive/);
throws(() => parseArgs(['--shots', 'x', '--bogus']), /unknown flag/);
throws(() => parseArgs(['--shots']), /needs a value/);

const shots = [{ name: 'home' }, { name: 'detail', path: '/d' }];
throws(() => plan([], parseArgs(['--shots', 'x'])), /non-empty/);
throws(() => plan([{ name: '../x' }], parseArgs(['--shots', 'x'])), /letters, digits/);
throws(() => plan([{ name: 'a' }, { name: 'a' }], parseArgs(['--shots', 'x'])), /duplicate/);
throws(() => plan(shots, parseArgs(['--shots', 'x', '--only', 'nope'])), /no such shot/);
assert.deepEqual(plan(shots, parseArgs(['--shots', 'x'])).map((i) => i.file), ['home.png', 'detail.png']);
assert.deepEqual(plan(shots, parseArgs(['--shots', 'x', '--only', 'home', '--preset', 'mobile,desktop'])).map((i) => i.file), ['home-mobile.png', 'home.png']);
assert.deepEqual(plan(shots, parseArgs(['--shots', 'x', '--only', 'detail', '--size', '1000x700'])).map((i) => [i.file, i.viewport.width]), [['detail-1000x700.png', 1000]]);

// The autoputer binary order, the proxy rule, the recipe env and the opt-out.
assert.equal(pickAutoputerBinary((f) => true), '/var/lib/artifact/release/bin/autoputer');
assert.equal(pickAutoputerBinary((f) => f === '/tmp/autoputer'), '/tmp/autoputer');
assert.equal(pickAutoputerBinary(() => false), '');
assert.ok(isProxied('/api/shell/bootstrap') && isProxied('/api'));
assert.ok(!isProxied('/apixyz') && !isProxied('/auth/session') && !isProxied('/assets/a.js'));
const env = autoputerEnv({});
assert.deepEqual([env.RUNTIME_STORE_PATH, env.AUTOPUTER_FILES_ROOT, env.AUTOPUTER_PORT, env.CHOIR_GUEST_TRUST_LOOPBACK], ['/computer/state', '/computer/files', '8082', '1']);
assert.equal(parseArgs(['--shots', 'x']).computer, true);
assert.equal(parseArgs(['--shots', 'x', '--no-computer']).computer, false);

// Themes: the shell's own preset list, argument parsing, naming, the seed and the route answer.
const known = themeIdsFrom(fs.readFileSync(new URL('../frontend/src/lib/theme.ts', import.meta.url), 'utf8'));
assert.ok(known.includes('futuristic-noir') && known.includes('aero-chrome-heart') && known.length >= 7, known.join());
assert.deepEqual(parseArgs(['--shots', 'x', '--theme', 'london-salmon,liquid-metal', '--contrast', '.win'], known).themes, ['london-salmon', 'liquid-metal']);
assert.equal(parseArgs(['--shots', 'x', '--contrast', '.win']).contrast, '.win');
throws(() => parseArgs(['--shots', 'x', '--theme', 'neon'], known), /unknown theme preset/);
throws(() => parseArgs(['--shots', 'x', '--theme', 'liquid-metal,liquid-metal'], known), /twice/);
throws(() => parseArgs(['--shots', 'x', '--theme', ','], known), /at least one/);
throws(() => plan([{ name: 'a', theme: 'neon' }], parseArgs(['--shots', 'x']), known), /unknown theme preset/);
{
  const o = parseArgs(['--shots', 'x', '--theme', 'london-salmon,liquid-metal', '--contrast', '.w'], known);
  const items = plan([{ name: 'a' }, { name: 'b', theme: 'vector-heart', contrast: '.b' }], o, known);
  assert.deepEqual(items.map((i) => i.file), ['a.london-salmon.png', 'a.liquid-metal.png', 'b.vector-heart.png']);
  assert.deepEqual(items.map((i) => i.contrastFile), ['a.london-salmon.contrast.json', 'a.liquid-metal.contrast.json', 'b.vector-heart.contrast.json']);
  assert.equal(items[2].contrast, '.b');
  const sized = plan([{ name: 'a' }], parseArgs(['--shots', 'x', '--theme', 'liquid-metal', '--preset', 'mobile,desktop'], known), known);
  assert.deepEqual(sized.map((i) => i.file), ['a.liquid-metal-mobile.png', 'a.liquid-metal.png']);
  assert.deepEqual(plan([{ name: 'a' }], parseArgs(['--shots', 'x', '--theme', 'liquid-metal', '--size', '1000x700'], known), known).map((i) => i.file), ['a.liquid-metal-1000x700.png']);
  assert.deepEqual(plan([{ name: 'a' }], parseArgs(['--shots', 'x'])).map((i) => [i.file, i.theme, i.contrastFile]), [['a.png', null, '']]);
}
{
  // The seed script, run as the page would, stores exactly what App.svelte's loadThemeBootCache parses.
  const store = {};
  vm.runInNewContext(themeSeedScript('london-salmon'), { window: { localStorage: { setItem: (k, v) => { store[k] = v; } } } });
  assert.deepEqual(Object.keys(store), ['choir.theme.boot.v2']);
  assert.deepEqual(JSON.parse(store['choir.theme.boot.v2']), { schema_version: 2, id: 'london-salmon' });
  vm.runInNewContext(themeSeedScript('x'), { window: {} }); // a throwing storage must not break the page
}
assert.deepEqual(themeAnswer('liquid-metal', 'GET', '/api/preferences/theme'), { theme: { schema_version: 2, id: 'liquid-metal' } });
assert.equal(themeAnswer('liquid-metal', 'PUT', '/api/preferences/theme'), null);
assert.equal(themeAnswer('liquid-metal', 'GET', '/api/preferences/other'), null);
assert.equal(themeAnswer(null, 'GET', '/api/preferences/theme'), null);

// Contrast: colors, the background walk, ratios and pixel fallback.
assert.deepEqual(parseColor('rgba(10, 20, 30, 0.5)'), { r: 10, g: 20, b: 30, a: 0.5 });
assert.deepEqual(parseColor('rgb(10 20 30 / 50%)'), { r: 10, g: 20, b: 30, a: 0.5 });
assert.equal(parseColor('oklch(0.5 0.1 20)'), null);
const white = { r: 255, g: 255, b: 255, a: 1 }, black = { r: 0, g: 0, b: 0, a: 1 };
assert.ok(Math.abs(contrastRatio(black, white) - 21) < 1e-9);
assert.equal(requiredRatio(14, '400'), 4.5);
assert.equal(requiredRatio(24, '400'), 3);
assert.equal(requiredRatio(19, '700'), 3);
{
  const half = over({ r: 0, g: 0, b: 0, a: 0.5 }, white);
  assert.ok(Math.abs(half.r - 127.5) < 1e-9 && half.a === 1);
  // translucent black over translucent white over an opaque red
  const r = resolveBackground([
    { color: { r: 0, g: 0, b: 0, a: 0.5 }, image: false },
    { color: { r: 255, g: 255, b: 255, a: 0.5 }, image: false },
    { color: { r: 255, g: 0, b: 0, a: 1 }, image: false },
    { color: { r: 0, g: 0, b: 255, a: 1 }, image: false }, // below the first opaque layer: ignored
  ]);
  assert.equal(r.kind, 'opaque');
  assert.deepEqual([r.color.r, r.color.g, r.color.b].map(Math.round), [128, 64, 64]); // white at .5 over red, then black at .5
  assert.equal(resolveBackground([{ color: { r: 0, g: 0, b: 0, a: 0 }, image: false }, { color: white, image: false }]).color.r, 255); // transparent skipped
  assert.equal(resolveBackground([{ color: { r: 9, g: 9, b: 9, a: 0.3 }, image: false }, { color: white, image: true }]).kind, 'pixel'); // image above the opaque layer
  assert.equal(resolveBackground([{ color: white, image: true }]).reason, 'image background');
  assert.equal(resolveBackground([{ color: { r: 9, g: 9, b: 9, a: 0.3 }, image: false }, { color: { r: 0, g: 0, b: 0, a: 0 }, image: false }]).kind, 'pixel'); // reaches the root: never black
  assert.equal(resolveBackground([{ color: null, image: false }]).kind, 'pixel');
}
{
  const png = { width: 4, height: 2, data: new Uint8Array(4 * 2 * 4) };
  for (let i = 0; i < 8; i++) { png.data.set(i < 4 ? [255, 255, 255, 255] : [30, 30, 30, 255], i * 4); }
  assert.equal(samplePixels(png, { x: 0, y: 0, width: 4, height: 2 }, 100).length, 8);
  assert.equal(samplePixels(png, { x: 10, y: 0, width: 4, height: 2 }).length, 0);
  const node = (layers, rect = { x: 0, y: 0, width: 4, height: 2 }, color = 'rgb(255, 255, 255)') => ({ path: 'p', text: 't', color, fontSize: 14, fontWeight: '400', rect, layers });
  const grad = [{ bg: 'rgba(0, 0, 0, 0)', image: true }];
  const m = measureNode(node(grad), png);
  assert.ok(m.ratio < 1.5 && /^pixels/.test(m.method), JSON.stringify(m)); // white text over the white half: the lowest sample counts
  assert.equal(measureNode(node(grad), null).method, 'unmeasured: image background');
  assert.equal(measureNode(node(grad), null).ratio, null);
  const op = measureNode(node([{ bg: 'rgb(0, 0, 0)', image: false }]), null);
  assert.ok(op.ratio > 20 && op.method === 'opaque-ancestor');
  const { report, line } = summarizeContrast({ file: 'a.png' }, [node([{ bg: 'rgb(0, 0, 0)', image: false }]), node([{ bg: 'rgb(250, 250, 250)', image: false }])], null);
  assert.equal(report.failing.length, 1);
  assert.equal(report.min_ratio, 1.04);
  assert.match(line, /min 1\.04 over 2\/2 text nodes, 1 failing, 0 unmeasured/);
}
assert.equal(contrastDir('/var/lib/artifact/screens'), '/var/lib/artifact/contrast');

// A locked tree that lacks a package.json dependency is named, not built against.
{
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'shot-deps-'));
  fs.mkdirSync(path.join(d, 'node_modules', 'have'), { recursive: true });
  fs.writeFileSync(path.join(d, 'node_modules', 'have', 'package.json'), '{}');
  fs.mkdirSync(path.join(d, 'fe'));
  fs.writeFileSync(path.join(d, 'fe', 'package.json'), JSON.stringify({ dependencies: { have: '1', gone: '1' }, devDependencies: { alsogone: '1' } }));
  assert.deepEqual(missingDependencies(path.join(d, 'fe'), path.join(d, 'node_modules')), ['gone', 'alsogone']);
}

if (process.env.CHOIR_FRONTEND_DEPS && process.env.CHOIR_BROWSER) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'shot-test-'));
  const file = path.join(dir, 'shots.mjs');
  fs.writeFileSync(file, `export default [
    { name: 'shell', selector: 'body' },
    { name: 'broken', selector: '#does-not-exist-anywhere' },
  ];`);
  const out = path.join(dir, 'screens');
  const run = (...a) => spawnSync('node', [new URL('./choir-shot.mjs', import.meta.url).pathname, '--shots', file, '--out', out, ...a], { encoding: 'utf8' });
  // Problem 18: a stray frontend/node_modules (a .vite cache dir, a half install)
  // must not hide the locked dependencies from the build, and must be left alone.
  const fe = new URL('../frontend/', import.meta.url).pathname;
  const stray = path.join(fe, 'node_modules');
  const hadStray = fs.existsSync(stray);
  if (!hadStray) fs.mkdirSync(path.join(stray, '.vite'), { recursive: true });
  const one = run('--only', 'shell', '--preset', 'mobile');
  try {
    assert.equal(one.status, 0, one.stdout + one.stderr);
    assert.ok(fs.existsSync(path.join(stray, '.vite')) || hadStray, 'the helper changed the workspace node_modules');
  } finally {
    if (!hadStray) fs.rmSync(stray, { recursive: true, force: true });
  }
  assert.deepEqual(fs.readdirSync(out), ['shell-mobile.png']);
  assert.equal(fs.readFileSync(path.join(out, 'shell-mobile.png')).subarray(1, 4).toString(), 'PNG');
  // A shot whose selector never appears fails alone, names itself, and the others are still taken.
  const both = run();
  assert.equal(both.status, 1);
  assert.match(both.stderr, /broken\.png FAILED/);
  assert.ok(fs.existsSync(path.join(out, 'shell.png')));
  console.log('choir_shot_test: real run ok');
} else {
  console.log('choir_shot_test: no CHOIR_FRONTEND_DEPS/CHOIR_BROWSER; skipped the real run');
}
console.log('choir_shot_test: ok');
