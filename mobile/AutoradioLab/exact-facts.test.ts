// The clock comes from Exact, not from native code (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 5). Failure list, written first:
//  H1 Activity's "ago" is measured from the date the contract passes (Exact's `exactTime` plus `performanceNow()`), not from a native fact: the same change at two passed dates reads two ages.
//  H2 the native settings facts carry no clock: the interface's SettingsFacts has no `now`, and the Swift reads no `Date()` for it.
//  H3 the contract asks `exactTime()` and passes the date after `with`.
// (The app's version stays the native settingsFacts `appVersion`: the bundle's version is not provable from here, see the change entry.)
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { activityInfo, activityPage, noFacts } from './activity.ts';

describe('the clock', () => {
  test('H1 Activity\'s ages are measured from the date the contract passes (activityPage hands it on as `now`)', async () => {
    const at = Date.UTC(2026, 10, 25, 12, 0, 0);
    const body = { events: [{ inbox_seq: 1, level: 'major', changelog: { event_id: 'e1', kind: 'wake_decision', occurred_at: new Date(at).toISOString(), summary: 's', subject: {} } }], seen_seq: 0 };
    const get = async () => body;
    const soon = await activityInfo({ now: at + 2 * 3600_000, signedIn: true, radio: [], live: [] }, get);
    const later = await activityInfo({ now: at + 3 * 24 * 3600_000, signedIn: true, radio: [], live: [] }, get);
    expect(soon.desk[0].ago).toBe('2 hours ago');
    expect(later.desk[0].ago).not.toBe(soon.desk[0].ago);
    expect(activityPage.length).toBe(7);                                    // the date is its seventh argument: nothing in `Facts` carries one
    expect(Object.keys(noFacts())).toEqual(['appVersion', 'radio', 'turns']);
  });
  test('H2 the native facts have no clock', () => {
    const iface = JSON.parse(readFileSync('../exact-package/interface/settings.json', 'utf8')) as { shapes: Record<string, Record<string, string>> };
    expect(Object.keys(iface.shapes.SettingsFacts).sort()).toEqual(['appVersion', 'radio', 'turns']);
    expect(readFileSync('../exact-package/apple/AutoradioModule+Settings.swift', 'utf8')).not.toContain('Date()');
  });
  test('H3 the contract asks exactTime and passes the date after with', () => {
    const c = readFileSync('members-settings.contract', 'utf8');
    expect(c).toContain('resource stime = exactTime()');
    expect(c).toContain('with stime.epochAtZero + performanceNow()');
  });
});
