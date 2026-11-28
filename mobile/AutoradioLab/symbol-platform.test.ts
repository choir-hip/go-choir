// A `symbol:sf/<name>` draws on Apple only: on Android and web it is blank, and `bun exact.mjs contract build app.contract` says so with a `lower-symbol-platform` warning.
// The fix is a role every host draws (`symbol:microphone`, `symbol:document`) or an `icon.<name>` token in app.forma with an SVG (art that has to be drawn first).
// This runs the real build (about 15 s) and fails on any such warning not in NO_ART, the symbols that still wait for art. Remove a name when its icon lands; the list may only shrink.
import { expect, test } from 'bun:test';
import { fileURLToPath } from 'node:url';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';

// The build needs the Exact checkout (exact.mjs: EXACT2, default ../.exact-src). CI has none (ci.yml stubs only @exact/passkeys), so there this is skipped, not passed:
// it runs wherever the shared-code loop runs with Exact, which is every lane and the build tree. Skipping is said in the test output.
const HERE = fileURLToPath(new URL('.', import.meta.url));
const EXACT = existsSync(resolve(HERE, process.env.EXACT2 ?? '../.exact-src', 'scripts/app-cli.mjs'));

const NO_ART = new Set(['sf/chevron.down', 'sf/text.badge.plus', 'sf/hourglass', 'sf/mic.slash']);

const build = (): string => {
  const p = Bun.spawnSync([process.execPath, 'exact.mjs', 'contract', 'build', 'app.contract'], { cwd: HERE, stdout: 'pipe', stderr: 'pipe', timeout: 240000, killSignal: 'SIGKILL' });
  expect(p.exitCode).toBe(0);
  return p.stdout.toString() + p.stderr.toString();
};
const warned = (out: string): { where: string; sym: string }[] =>
  out.split('\n').filter(l => l.includes('[lower-symbol-platform]')).map(l => ({ where: l.replace(/^.*AutoradioLab\//, '').replace(/ warning.*$/, ''), sym: /symbol `([^`]+)`/.exec(l)?.[1] ?? l }));

test.skipIf(!EXACT)('P1 the build warns of no blank-off-Apple symbol except the ones waiting for art; the list only shrinks', () => {
  const w = warned(build());
  expect(w.filter(x => !NO_ART.has(x.sym)).map(x => `${x.where} ${x.sym}`)).toEqual([]);
  expect([...new Set(w.map(x => x.sym))].sort()).toEqual([...NO_ART].sort());
}, 260000);
