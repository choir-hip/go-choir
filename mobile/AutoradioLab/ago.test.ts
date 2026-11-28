// agoText (ago/ago.contract) against the TypeScript `when()` it replaced (activity.ts on origin/main before 2026-11-28, copied below as the reference): every case of the fixture
// (ago/app.contract, with the texts ago/app.test.contract expects) is worked out by the old rule, and the two agree on every one. The fixture itself runs under Exact's web host
// (`cd ago && bun exact.mjs test web`); this test runs it when Exact is present (a CI job without mobile/.exact-src runs only the comparison).
import { describe, expect, test } from 'bun:test';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { stamp } from './activity.ts';

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
function when(atMs: number, now: number | null): string {
  if (now === null || !(atMs > 86400000)) return '';
  const age = (now - atMs) / 1000;
  if (age >= 0 && age < 3600) { const m = Math.max(1, Math.floor(age / 60)); return m === 1 ? 'a minute ago' : `${m} minutes ago`; }
  if (age >= 0 && age < 86400) { const h = Math.floor(age / 3600); return h === 1 ? 'an hour ago' : `${h} hours ago`; }
  const d = new Date(atMs); const p = (n: number): string => String(n).padStart(2, '0');
  return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}, ${p(d.getUTCHours())}:${p(d.getUTCMinutes())}`;
}
const num = (e: string): number => { if (!/^[0-9+\-\s]+$/.test(e)) throw new Error(e); return Function(`return (${e})`)() as number; };
const cases = [...readFileSync('ago/app.contract', 'utf8').matchAll(/text agoText\(([^,]+), ([^,]+), (true|false), "S"\) testId="g-(\w+)"/g)].map((m) => ({ at: num(m[1]), now: num(m[2]), known: m[3] === 'true', id: m[4] }));
const expected = new Map([...readFileSync('ago/app.test.contract', 'utf8').matchAll(/expect text "g-(\w+)" == "([^"]*)"/g)].map((m) => [m[1], m[2]]));

describe('agoText against the old when()', () => {
  test('the fixture has the edges: unknown date, 0, the one-day floor, a future time, 0/59/119/120 s, 59 min 59 s, 1 h, 1 h 59 min, 2 h, 23 h 59 min, 1 day, older', () => {
    expect(cases.length).toBe(15); expect(expected.size).toBe(15);
  });
  test('every case reads the same text as the old rule (a stamp is the row\'s own UTC stamp)', () => {
    for (const c of cases) {
      const old = when(c.at, c.known ? c.now : null), want = expected.get(c.id)!;
      expect([c.id, want === 'S' ? stamp(c.at) : want]).toEqual([c.id, old]);
    }
  });
  const exact = process.env.EXACT2 ?? resolve('../.exact-src');
  test.skipIf(!existsSync(resolve(exact, 'scripts/app-cli.mjs')))('the fixture passes under the web host', () => {
    const r = Bun.spawnSync(['./exact.mjs', 'test', 'web'], { cwd: resolve('ago'), env: { ...process.env, EXACT2: exact }, stdout: 'pipe', stderr: 'pipe' });
    const out = r.stdout.toString() + r.stderr.toString();
    expect(out).toContain('1 passed, 0 failed'); expect(r.exitCode).toBe(0);
  }, 280000);
});
