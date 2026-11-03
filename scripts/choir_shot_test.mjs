// Failure modes of scripts/choir-shot.mjs, then one real run: node scripts/choir_shot_test.mjs
// The real run needs CHOIR_FRONTEND_DEPS and CHOIR_BROWSER (a capsule job has both)
// and is skipped without them.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { parseArgs, plan } from './choir-shot.mjs';

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

if (process.env.CHOIR_FRONTEND_DEPS && process.env.CHOIR_BROWSER) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'shot-test-'));
  const file = path.join(dir, 'shots.mjs');
  fs.writeFileSync(file, `export default [
    { name: 'shell', selector: 'body' },
    { name: 'broken', selector: '#does-not-exist-anywhere' },
  ];`);
  const out = path.join(dir, 'screens');
  const run = (...a) => spawnSync('node', [new URL('./choir-shot.mjs', import.meta.url).pathname, '--shots', file, '--out', out, ...a], { encoding: 'utf8' });
  const one = run('--only', 'shell', '--preset', 'mobile');
  assert.equal(one.status, 0, one.stdout + one.stderr);
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
