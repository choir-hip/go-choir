// The About page's build source under bun test. Failure list (each line has a test below):
//  B1 the app's commit shows as short sha and the commit's UTC date; a dirty tree says `-dirty` on the sha.
//  B2 a build that knew no git says `unknown`, never a blank, and the date is not invented.
//  B3 the server's commit is read from /health (`build.commit`, else `platform.deployed_commit`) with its deploy date; a failure or a
//     body with no commit says `not reported`, never a blank or a dash; no key is sent (the route is public).
//  B4 the build info the generator writes has the shape about.ts reads (a bad value is `unknown`, not a crash).
import { beforeEach, describe, expect, test } from 'bun:test';
import { bindNative, resetSession, ORIGIN } from './api.ts';
import { aboutBuild, appCommitLine, serverCommitLine } from './about.ts';
import { BUILD_INFO } from './build-info.gen.ts';

let calls: { url: string; headers: unknown }[] = [];
let handler: () => Response;
beforeEach(() => {
  calls = []; resetSession(); bindNative({ available: false } as never);
  handler = () => new Response('{}', { status: 200 });
  globalThis.fetch = (async (u: string, init?: RequestInit) => { calls.push({ url: String(u), headers: init?.headers }); return handler(); }) as never;
});

const health = { status: 'ok', build: { service: 'proxy', commit: '0eef16ec794b4e98596c0adac58c0e8eed157051', deployed_at: '2026-11-20T13:38:45Z' }, platform: { deployed_commit: 'aaaaaaaa1111aaaa', deployed_at: '2026-11-20T13:38:45Z' } };

describe('appCommitLine', () => {
  test('B1 short sha and UTC date', () => {
    expect(appCommitLine({ commit: '19519df2b1c2d3e4f5', dirty: false, committed: '2026-11-20 14:03' })).toBe('19519df2b · 2026-11-20 14:03 UTC');
  });
  // `dirty` is scoped to mobile/ (scripts/gen-build-info.sh): the app is dirty when anything under mobile/ differs from HEAD.
  test('B1 a dirty tree says so', () => {
    expect(appCommitLine({ commit: '19519df2b1c2d3e4f5', dirty: true, committed: '2026-11-20 14:03' })).toBe('19519df2b-dirty · 2026-11-20 14:03 UTC');
  });
  test('B2 no git is unknown, with no date', () => {
    expect(appCommitLine({ commit: 'unknown', dirty: false, committed: '' })).toBe('unknown');
    expect(appCommitLine({ commit: '', dirty: false, committed: '' })).toBe('unknown');
    expect(appCommitLine({ commit: 'unknown', dirty: true, committed: '' })).toBe('unknown');
  });
  test('B4 junk is unknown', () => {
    expect(appCommitLine(null as never)).toBe('unknown');
    expect(appCommitLine({ commit: 5, dirty: 'x', committed: {} } as never)).toBe('unknown');
    expect(appCommitLine({ commit: 'not a sha!', dirty: false, committed: '' } as never)).toBe('unknown');
  });
  test('B4 the generated file has a usable shape', () => {
    expect(appCommitLine(BUILD_INFO)).not.toBe('');
    expect(typeof BUILD_INFO.dirty).toBe('boolean');
  });
});

describe('serverCommitLine', () => {
  test('B3 build.commit and its deploy date', () => expect(serverCommitLine(health)).toBe('0eef16ec7 · 2026-11-20 13:38 UTC'));
  test('B3 the platform receipt when build has no commit', () => expect(serverCommitLine({ platform: health.platform })).toBe('aaaaaaaa1 · 2026-11-20 13:38 UTC'));
  test('B3 a commit with no date is the sha alone', () => expect(serverCommitLine({ build: { commit: 'abcdef0123456789' } })).toBe('abcdef012'));
  test('B3 nothing reported', () => { expect(serverCommitLine({})).toBe(''); expect(serverCommitLine(null)).toBe(''); expect(serverCommitLine({ build: { commit: 7 } })).toBe(''); });
});

describe('aboutBuild', () => {
  test('B3 reads /health once, with no key', async () => {
    handler = () => new Response(JSON.stringify(health), { status: 200 });
    const v = await aboutBuild({ tick: 0 });
    expect(calls.map((c) => c.url)).toEqual([`${ORIGIN}/health`]);
    expect(JSON.stringify(calls[0].headers)).not.toContain('uthorization');
    expect(v.server).toBe('0eef16ec7 · 2026-11-20 13:38 UTC');
    expect(v.commit).toBe(appCommitLine(BUILD_INFO));
  });
  test('B3 a refused, garbled or dead /health says not reported and keeps the commit', async () => {
    handler = () => new Response('{}', { status: 503 });
    expect((await aboutBuild({ tick: 0 })).server).toBe('not reported');
    handler = () => new Response('<html>', { status: 200 });
    expect((await aboutBuild({ tick: 0 })).server).toBe('not reported');
    handler = () => new Response('{"status":"ok"}', { status: 200 });
    expect((await aboutBuild({ tick: 0 })).server).toBe('not reported');
    globalThis.fetch = (async () => { throw new Error('offline'); }) as never;
    const v = await aboutBuild({ tick: 0 });
    expect(v.server).toBe('not reported'); expect(v.commit).not.toBe('');
  });
});
