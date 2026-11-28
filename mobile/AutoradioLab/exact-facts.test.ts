// The clock comes from Exact, not from native code (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 5). Failure list, written first:
//  H1 Activity's source takes no clock: it answers each row's own time and UTC stamp, the same whenever it is asked; "ago" is made at render in the contract from Exact's date (agoText).
//  H2 the native settings facts carry no clock: the interface's SettingsFacts has no `now`, and the Swift reads no `Date()` for it.
//  H3 the contract asks `exactTime()`; no resource's `with` reads the clock (a value that changes every frame asks the source every frame: about 42 changelog requests a second, 2026-11-28), and Activity is asked only while its page is on top.
// (The app's version stays the native settingsFacts `appVersion`: the bundle's version is not provable from here, see the change entry.)
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { activityInfo, activityPage, noFacts } from './activity.ts';
import { contracts, code } from './contract-rules.ts';

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
    const bad: string[] = [];
    for (const f of contracts()) f.lines.forEach((raw, n) => {
      const l = code(raw).trim();
      if (l.startsWith('resource ') && /performanceNow\(\)|epochAtZero/.test(l)) bad.push(`${f.name}:${n + 1} a resource reads the clock: ${l.slice(0, 80)}`);
    });
    expect(bad).toEqual([]);
  });
  test('H4 the age is made where it is drawn, from the row\'s time and Exact\'s date, and not before the date is known', () => {
    const a = readFileSync('activity.contract', 'utf8');
    expect(a).toContain('agoText(ev.at, stime.epochAtZero + performanceNow(), stimeKnown, ev.stamp)');
    expect(a).toContain('fn agoText(at: number, now: number, known: bool, stamp: string): string = (!known or at <= 86400000 ? ""');
  });
});
