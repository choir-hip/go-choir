// The clock comes from Exact, not from native code (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 5). Failure list, written first:
//  H1 Activity's source takes no clock: it answers each row's own time and UTC stamp, the same whenever it is asked; "ago" is made at render in the contract from Exact's date (agoText).
//  H2 the native settings facts carry no clock: the interface's SettingsFacts has no `now`, and the Swift reads no `Date()` for it.
//  H3 the contract asks `exactTime()`; no resource's `with` reads the clock (a value that changes every frame asks the source every frame: about 42 changelog requests a second, 2026-11-28), and Activity is asked only while its page is on top.
// (The app's version stays the native settingsFacts `appVersion`: the bundle's version is not provable from here, see the change entry.)
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { activityInfo, activityPage, noFacts } from './activity.ts';
import { join } from 'node:path';
import { clockLines, resourcesNaming, withFixtures } from './contract-rules.ts';

describe('the clock', () => {
  const at = Date.UTC(2026, 10, 25, 12, 0, 0);
  const body = { events: [{ inbox_seq: 1, level: 'major', changelog: { event_id: 'e1', kind: 'wake_decision', occurred_at: new Date(at).toISOString(), summary: 's', subject: {} } }], seen_seq: 0 };
  test('H1 the source gives each row its own time and stamp, with no clock: asked at any moment it answers the same', async () => {
    const a = await activityInfo({ signedIn: true, radio: [], live: [] }, async () => body);
    const b = await activityPage(noFacts(), true, 0, '', false, false, async () => body);
    expect(a.desk[0]).toMatchObject({ at, stamp: '25 Nov, 12:00' });
    expect(b).toEqual(a);
    for (const r of [...a.desk, ...a.computer, ...a.paper, ...a.radio, ...a.live]) expect('ago' in r).toBe(false);
    expect(Object.keys(noFacts())).toEqual(['appVersion', 'radio', 'turns']);
  });
  test('H2 the native facts have no clock', () => {
    const iface = JSON.parse(readFileSync('../exact-package/interface/settings.json', 'utf8')) as { shapes: Record<string, Record<string, string>> };
    expect(Object.keys(iface.shapes.SettingsFacts).sort()).toEqual(['appVersion', 'radio', 'turns']);
    expect(readFileSync('../exact-package/apple/AutoradioModule+Settings.swift', 'utf8')).not.toContain('Date()');
  });
  // THE RULE: a clock value must never reach a resource's arguments, `with` or `when` (Exact asks a source again at every change of any of them; a clock changes every frame).
  // Before adding a line below, or a name to CLOCK_NAMES, check where its value goes. This is a pin that forces a look, not an analyser: it cannot follow a value, so a person does.
  const CLOCK_LINES = [
    'activity.contract: text agoText(ev.at, stime.epochAtZero + actNow, stimeKnown and actNow > 0, ev.stamp) font-family=th.fontLabel font-size=dyn(tx.sizeMeta) color=th.muted font-variant-numeric="tabular-nums"', // the row's age, drawn from the page clock; reaches no resource
    'members-mark.contract: let now = performanceNow()', // mkEntered: a local, into mkFrom and mkSince
    'members-mark.contract: mkT = performanceNow()', // the mark's frame clock (mkStep); read by mkHot and the mark's derives, which no resource names
    'members-settings.contract: actNow = performanceNow()', // the Activity page's clock, set at open, on Refresh and every 30 s; read only by activity.contract
    'settings-types.contract: epochAtZero: number', // the shape of exactTime's answer
    'spine.contract: mkCloseAt = performanceNow()', // when the launcher was closed; read by the mark
    'spine.contract: mkPressAt = performanceNow()', // when the mark was pressed; read by the mark
  ];
  // The names that carry a clock (states, derives, locals): read from the lines above. No resource declaration names one.
  const CLOCK_NAMES = ['actNow', 'mkT', 'mkSince', 'mkFrom', 'mkPressAt', 'mkCloseAt', 'mkHot', 'mkIdle', 'now'];
  test('H3 the contract asks exactTime; Activity waits for its page; every line that reads the clock is pinned, and no resource names a clock or a name that carries one', () => {
    const c = readFileSync(join(import.meta.dir, 'members-settings.contract'), 'utf8');
    expect(c).toContain('resource stime = exactTime()');
    expect(c).toContain('cmpt.checked) when pg.top.startsWith("activity:") as shape ActivityInfo');
    expect(clockLines(withFixtures())).toEqual(CLOCK_LINES.map((l) => l.replace(/ \/\/.*$/, '').replace(/^(\S+?): /, '$1: ')).sort());
    expect(resourcesNaming(withFixtures(), [...CLOCK_NAMES, 'performanceNow', 'epochAtZero'])).toEqual([]);
  });
  test('H3b the name check on small contracts: caught in arguments, with, when and over several lines; not for a string, a field or another resource', () => {
    const run = (src: string): string[] => resourcesNaming([{ name: 't.contract', lines: src.split('\n') }], ['actNow', 'performanceNow']);
    for (const src of ['  resource r = src(actNow) as shape S', '  resource r = src(a) with actNow as shape S', '  resource r = src(a) when actNow > 0 as shape S', '  resource r = src(a) with performanceNow() as shape S',
      '  resource r = src(a,\n      b)\n    with actNow\n    as shape S', '  resource r = src(a) when\n      actNow > 5 as shape S']) expect([src, run(src).length]).toEqual([src, 1]);
    for (const src of ['  resource r = src("actNow") as shape S', '  resource r = src(`actNow ${a}`) as shape S', '  resource r = src(a.actNow) as shape S', '  resource r = src(a) as shape S // actNow\n  state actNow = 0', '  resource r = src(a) as shape S\n  derive d = actNow + 1', '  resource r = src(actNowx) as shape S']) expect([src, run(src)]).toEqual([src, []]);
  });
  test('H4 the age is made where it is drawn, from the row\'s time and Exact\'s date, and not before the date is known', () => {
    const a = readFileSync('activity.contract', 'utf8');
    expect(a).toContain('agoText(ev.at, stime.epochAtZero + actNow, stimeKnown and actNow > 0, ev.stamp)'); expect(a).not.toMatch(/performanceNow/);
    expect(readFileSync('ago/ago.contract', 'utf8')).toContain('fn agoText(at: number, now: number, known: bool, stamp: string): string = (!known or at <= 86400000 ? ""');
  });
});
