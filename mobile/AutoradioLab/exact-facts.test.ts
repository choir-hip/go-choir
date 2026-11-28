// The clock comes from Exact, not from native code (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 5). Failure list, written first:
//  H1 Activity's source takes no clock: it answers each row's own time and UTC stamp, the same whenever it is asked; "ago" is made at render in the contract from Exact's date (agoText).
//  H2 the native settings facts carry no clock: the interface's SettingsFacts has no `now`, and the Swift reads no `Date()` for it.
//  H3 the contract asks `exactTime()`; no resource's `with` reads the clock (a value that changes every frame asks the source every frame: about 42 changelog requests a second, 2026-11-28), and Activity is asked only while its page is on top.
// (The app's version stays the native settingsFacts `appVersion`: the bundle's version is not provable from here, see the change entry.)
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { activityInfo, activityPage, noFacts } from './activity.ts';
import { contracts, clockInResources } from './contract-rules.ts';

describe('the clock', () => {
  const at = Date.UTC(2026, 10, 25, 12, 0, 0);
  const body = { events: [{ inbox_seq: 1, level: 'major', changelog: { event_id: 'e1', kind: 'wake_decision', occurred_at: new Date(at).toISOString(), summary: 's', subject: {} } }], seen_seq: 0 };
  test('H1 the source gives each row its own time and stamp, with no clock: asked at any moment it answers the same', async () => {
    const a = await activityInfo({ signedIn: true, radio: [], live: [] }, async () => body);
    const b = await activityPage(noFacts(), true, 0, '', false, false, async () => body);
    expect(a.desk[0]).toMatchObject({ at, stamp: '25 Nov, 12:00' });
    expect(b).toEqual(a);
    expect(JSON.stringify(a)).not.toMatch(/ago/);
    expect(Object.keys(noFacts())).toEqual(['appVersion', 'radio', 'turns']);
  });
  test('H2 the native facts have no clock', () => {
    const iface = JSON.parse(readFileSync('../exact-package/interface/settings.json', 'utf8')) as { shapes: Record<string, Record<string, string>> };
    expect(Object.keys(iface.shapes.SettingsFacts).sort()).toEqual(['appVersion', 'radio', 'turns']);
    expect(readFileSync('../exact-package/apple/AutoradioModule+Settings.swift', 'utf8')).not.toContain('Date()');
  });
  test('H3 the contract asks exactTime; no resource reads the clock after `with` or in its arguments; Activity waits for its page', () => {
    const c = readFileSync('members-settings.contract', 'utf8');
    expect(c).toContain('resource stime = exactTime()');
    expect(c).toContain('cmpt.checked) when pg.top.startsWith("activity:") as shape ActivityInfo');
    expect(clockInResources(contracts())).toEqual([]);
  });
  test('H3b the guard catches every spelling of the defect (small contracts as strings), and the real contracts pass', () => {
    const run = (src: string): string[] => clockInResources([{ name: 't.contract', lines: src.split('\n') }]);
    const caught: Record<string, string> = {
      'with and performanceNow': 'members A\n  resource r = src(a) with stime.epochAtZero + performanceNow() as shape S',
      'spaces in the call': 'members A\n  resource r = src(a) with performanceNow ( ) as shape S',
      'epochAtZero as an argument': 'members A\n  resource r = src(stime.epochAtZero) as shape S',
      'a derive in the arguments': 'members A\n  derive nowMs = stime.epochAtZero + performanceNow()\n  resource r = src(nowMs) as shape S',
      'a derive of a derive, in with': 'members A\n  derive a1 = performanceNow() / 1000\n  derive a2 = a1 + 1\n  resource r = src(x) with a2 as shape S',
      'a fn that reads it, called in when': 'members A\n  fn stamp(): number = performanceNow()\n  resource r = src(x) when stamp() > 5 as shape S',
      'a state assigned it in an action': 'members A\n  state at0 = 0\n  action step\n    at0 = performanceNow()\n  resource r = src(at0) as shape S',
      'a state assigned it through a let': 'members A\n  state at0 = 0\n  action step\n    let v = performanceNow()\n    at0 = v\n  resource r = src(x) with at0 as shape S',
      'a state assigned a derive of it': 'members A\n  derive nowMs = stime.epochAtZero\n  state at0 = 0\n  action step\n    at0 = nowMs + 1\n  resource r = src(at0) as shape S',
      'a resource over several lines': 'members A\n  resource r = src(a,\n      b)\n    with performanceNow()\n    as shape S',
      'the page clock actNow in a resource': 'members A\n  state actNow = 0\n  action stepActClock\n    actNow = performanceNow()\n  resource r = src(a) when actNow > 0 as shape S',
    };
    for (const [name, src] of Object.entries(caught)) { const r = run(src); expect([name, r.length]).toEqual([name, 1]); }
    const fine = 'members A\n  state n = 0\n  action bump\n    let t = performanceNow()\n    n = n + 1\n  derive d = n + 1\n  resource r = src(d, n) as shape S\n  resource q = src(a, current(r)) with d as shape S\n  task go when n > 0\n    every(500, bump)';
    expect(run(fine)).toEqual([]);
    expect(clockInResources(contracts())).toEqual([]);
  });
  test('H4 the age is made where it is drawn, from the row\'s time and Exact\'s date, and not before the date is known', () => {
    const a = readFileSync('activity.contract', 'utf8');
    expect(a).toContain('agoText(ev.at, stime.epochAtZero + actNow, stimeKnown and actNow > 0, ev.stamp)'); expect(a).not.toContain('performanceNow()');
    expect(readFileSync('ago/ago.contract', 'utf8')).toContain('fn agoText(at: number, now: number, known: bool, stamp: string): string = (!known or at <= 86400000 ? ""');
  });
});
