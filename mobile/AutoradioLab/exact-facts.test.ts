// The clock and the app's version come from Exact and the build, not from native code (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 5). Failure list, written first:
//  H1 the About page's app row is the version the build wrote (build-info.gen.ts `version`, the bundle's EXACT_VERSION); a build that set none says "dev".
//  H2 Activity's "ago" is measured from the date the contract passes (Exact's `exactTime` plus `performanceNow()`), not from a native fact.
//  H3 the native settings facts carry neither a clock nor a version: the interface's SettingsFacts has only the voice's fields, and the Swift reads neither Date() nor the bundle's version for it.
//  H4 the build writes the version: scripts/gen-build-info.sh puts EXACT_VERSION into build-info.gen.ts.
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, copyFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { aboutPage } from './settings.ts';
import { activityPage, noFacts } from './activity.ts';
import { ORIGIN } from './api.ts';

describe('the app version', () => {
  test('H1 the About row is the version the build wrote; none is dev', () => {
    expect(aboutPage(true, '1.4')).toEqual({ app: '1.4', base: ORIGIN, signedIn: true });
    expect(aboutPage(false, '')).toEqual({ app: 'dev', base: ORIGIN, signedIn: false });
  });
  test('H4 the build writes EXACT_VERSION into build-info.gen.ts', () => {
    const d = mkdtempSync(join(tmpdir(), 'bi-'));
    mkdirSync(join(d, 'scripts')); mkdirSync(join(d, 'AutoradioLab'));
    copyFileSync(new URL('../scripts/gen-build-info.sh', import.meta.url).pathname, join(d, 'scripts', 'gen-build-info.sh'));
    spawnSync('sh', [join(d, 'scripts', 'gen-build-info.sh')], { env: { ...process.env, EXACT_VERSION: '2.3' } });
    expect(readFileSync(join(d, 'AutoradioLab', 'build-info.gen.ts'), 'utf8')).toContain("version: '2.3'");
    spawnSync('sh', [join(d, 'scripts', 'gen-build-info.sh')], { env: { ...process.env, EXACT_VERSION: '' } });
    expect(readFileSync(join(d, 'AutoradioLab', 'build-info.gen.ts'), 'utf8')).toContain("version: ''");
  });
});

describe('the clock', () => {
  test('H2 Activity\'s ages are measured from the date the contract passes', async () => {
    const at = Date.UTC(2026, 10, 25, 12, 0, 0);
    const body = { changes: [{ event_id: 'e1', kind: 'run', summary: 's', occurred_at: new Date(at).toISOString(), seq: 1 }] };
    const real = globalThis.fetch;
    globalThis.fetch = (async () => new Response(JSON.stringify(body), { status: 200 })) as never;
    try {
      const v = await activityPage(noFacts(), false, 0, '', false, false, at + 2 * 3600_000);
      expect(JSON.stringify(v)).not.toContain('NaN');
      expect(Object.keys(noFacts())).toEqual(['radio', 'turns']);
    } finally { globalThis.fetch = real; }
  });
  test('H3 the native facts have no clock and no version', () => {
    const iface = JSON.parse(readFileSync('../exact-package/interface/settings.json', 'utf8')) as { shapes: Record<string, Record<string, string>> };
    expect(Object.keys(iface.shapes.SettingsFacts).sort()).toEqual(['radio', 'turns']);
    const swift = readFileSync('../exact-package/apple/AutoradioModule+Settings.swift', 'utf8');
    expect(swift).not.toContain('CFBundleShortVersionString'); expect(swift).not.toContain('Date()');
    const c = readFileSync('members-settings.contract', 'utf8');
    expect(c).toContain('exactTime()');
    expect(c).not.toContain('aboutPage(sfacts');
  });
});
