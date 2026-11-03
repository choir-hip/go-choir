// Failure modes of scripts/choir-shot.mjs, then one real run: node scripts/choir_shot_test.mjs
// The real run needs CHOIR_FRONTEND_DEPS and CHOIR_BROWSER (a capsule job has both)
// and is skipped without them.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { autoputerEnv, isProxied, missingDependencies, pickAutoputerBinary, parseArgs, plan } from './choir-shot.mjs';

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
