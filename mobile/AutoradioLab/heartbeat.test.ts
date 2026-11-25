import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { clock } from './clock.ts';
import { heartbeat } from './heartbeat.ts';

test('the heartbeat logs one line with the app clock and counts', () => {
  const lines: string[] = [];
  const orig = console.log; console.log = (s: string) => { lines.push(s); };
  try { clock.ms = 1234; const a = heartbeat(); clock.ms = 5678; const b = heartbeat(); expect(b.n).toBe(a.n + 1); } finally { console.log = orig; }
  expect(lines[0]).toMatch(/^AUTORADIO js heartbeat n=\d+ t=1234$/);
  expect(lines[1]).toMatch(/ t=5678$/);
});

test('the spine asks it again every five seconds', () => {
  const c = readFileSync(new URL('./spine.contract', import.meta.url), 'utf8');
  expect(/resource hb = heartbeat\(hbTick\) as shape Heartbeat/.test(c)).toBe(true);
  expect(/task hbTimer when hb\.n >= 0 key=hbTick\n\s+after\(5000, hbBump\)/.test(c)).toBe(true);
});
