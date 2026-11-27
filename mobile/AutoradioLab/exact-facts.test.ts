// The clock comes from Exact, not from native code (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 5). Failure list, written first:
//  H1 Activity's "ago" is measured from the date the contract passes (Exact's `exactTime` plus `performanceNow()`), not from a native fact: the same change at two passed dates reads two ages.
//  H2 the native settings facts carry no clock: the interface's SettingsFacts has no `now`, and the Swift reads no `Date()` for it.
//  H3 the contract asks `exactTime()` and passes the date after `with`.
// (The app's version stays the native settingsFacts `appVersion`: the bundle's version is not provable from here, see the change entry.)
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { activityInfo, activityPage, noFacts, when } from './activity.ts';

describe('the clock', () => {
  test('H1 Activity\'s ages are measured from the date the contract passes (activityPage hands it on as `now`)', async () => {
    const at = Date.UTC(2026, 10, 25, 12, 0, 0);
    const body = { events: [{ inbox_seq: 1, level: 'major', changelog: { event_id: 'e1', kind: 'wake_decision', occurred_at: new Date(at).toISOString(), summary: 's', subject: {} } }], seen_seq: 0 };
    const get = async () => body;
    const soon = await activityInfo({ now: at + 2 * 3600_000, signedIn: true, radio: [], live: [] }, get);
    const later = await activityInfo({ now: at + 3 * 24 * 3600_000, signedIn: true, radio: [], live: [] }, get);
    expect(soon.desk[0].ago).toBe('2 hours ago');
    expect(later.desk[0].ago).not.toBe(soon.desk[0].ago);
    expect(Object.keys(noFacts())).toEqual(['appVersion', 'radio', 'turns']);
  });
  test('H1b before the date is known no age is shown, signed in, even for an item far in the future of a near-zero clock', async () => {
    const at = Date.UTC(2026, 10, 25, 12, 0, 0);
    const body = { events: [{ inbox_seq: 1, level: 'major', changelog: { event_id: 'e1', kind: 'wake_decision', occurred_at: new Date(at).toISOString(), summary: 's', subject: {} } }], seen_seq: 0 };
    const known = await activityInfo({ now: 5_000, signedIn: true, radio: [], live: [] }, async () => body);      // a number that is a placeholder (0 + performanceNow()) would give a date, not an age
    expect(known.desk[0].ago).toBe('25 Nov, 12:00');                  // the old behaviour for a known clock earlier than the item: an absolute date, never negative
    const unknown = await activityInfo({ now: null, signedIn: true, radio: [], live: [] }, async () => body);
    expect(unknown.status).toBe('ok'); expect(unknown.desk[0].ago).toBe(''); expect(unknown.desk[0].label).not.toBe('');
    expect(JSON.stringify(unknown)).not.toMatch(/ago"?:"[^"]*(-|NaN|Infinity)/);
    expect(when(at, null)).toBe(''); expect(when(at, at + 90_000)).toBe('a minute ago');
  });
  test('H1c activityPage forwards `known` and `now`: unknown shows no age, known measures from the date passed', async () => {
    const at = Date.UTC(2026, 10, 25, 12, 0, 0);
    const body = { events: [{ inbox_seq: 1, level: 'major', changelog: { event_id: 'e1', kind: 'wake_decision', occurred_at: new Date(at).toISOString(), summary: 's', subject: {} } }], seen_seq: 0 };
    const a = await activityPage(noFacts(), true, 0, '', false, false, false, at + 7_200_000, async () => body);
    const b = await activityPage(noFacts(), true, 0, '', false, false, true, at + 7_200_000, async () => body);
    expect(a.status).toBe('ok'); expect(a.desk[0].ago).toBe('');
    expect(b.desk[0].ago).toBe('2 hours ago');
  });
  test('H2 the native facts have no clock', () => {
    const iface = JSON.parse(readFileSync('../exact-package/interface/settings.json', 'utf8')) as { shapes: Record<string, Record<string, string>> };
    expect(Object.keys(iface.shapes.SettingsFacts).sort()).toEqual(['appVersion', 'radio', 'turns']);
    expect(readFileSync('../exact-package/apple/AutoradioModule+Settings.swift', 'utf8')).not.toContain('Date()');
  });
  test('H3 the contract asks exactTime and passes the date after with', () => {
    const c = readFileSync('members-settings.contract', 'utf8');
    expect(c).toContain('resource stime = exactTime()');
    expect(c).toContain('with current(stime), stime.epochAtZero + performanceNow()');
  });
});
