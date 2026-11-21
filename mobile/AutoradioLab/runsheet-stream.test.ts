// The stream rules of GET /api/autoradio/sheet/stream, ported from RunSheetStoreTests.test29 (Swift). The failure list:
//  S1 the path carries the version the phone has (`after`, only when above 0) and the epoch it holds (`epoch`, only when above 0).
//  S2 503 and every other 5xx reconnect after 1, 2, 4 ... 60 s; 429 waits 30 s a failure up to 300 and opens no other stream; 404, 501 and any other status give up.
//  S3 a `sheet` event is a whole sheet: it replaces the local one and its epoch becomes the phone's (the sheet's own, else the phone's).
//  S4 a `sheet_ops` that does not begin at the phone's version asks for a reconnect from the phone's version; one that does changes nothing; its ops are not folded here.
//  S5 an unknown or broken event is ignored.
import { describe, expect, test } from 'bun:test';
import { streamPath, streamRetry, streamEvent } from './runsheet-stream.ts';
import { RunSheetStore } from './runsheet-store.ts';

describe('the stream rules', () => {
  test('S1 the path', () => {
    expect(streamPath(undefined, 0)).toBe('/api/autoradio/sheet/stream');
    expect(streamPath(12, 7)).toBe('/api/autoradio/sheet/stream?after=12&epoch=7');
    expect(streamPath(0, 7)).toBe('/api/autoradio/sheet/stream?epoch=7');
    expect(streamPath(12, 0)).toBe('/api/autoradio/sheet/stream?after=12');
  });
  test('S2 what to do when the request is answered', () => {
    expect(streamRetry(503, 1)).toEqual({ kind: 'after', seconds: 2 });
    expect(streamRetry(503, 9)).toEqual({ kind: 'after', seconds: 60 });
    expect(streamRetry(502, 0)).toEqual({ kind: 'after', seconds: 1 });
    expect(streamRetry(429, 2)).toEqual({ kind: 'tooManyStreams', seconds: 60 });
    expect(streamRetry(429, 0)).toEqual({ kind: 'tooManyStreams', seconds: 30 });
    expect(streamRetry(429, 99)).toEqual({ kind: 'tooManyStreams', seconds: 300 });
    for (const s of [404, 501, 400, 401, 200]) expect(streamRetry(s, 0)).toEqual({ kind: 'giveUp' });
  });
  test('S3 a sheet event replaces the local sheet and takes its epoch', () => {
    const s = new RunSheetStore();
    const full = '{"version":9,"sheet_epoch":5,"entries":[{"id":"e1","kind":"story","placed_by":"desk","state":"ahead","title":"T","text":"x","est_seconds":3}]}';
    expect(streamEvent(s, 'sheet', full)).toEqual({ kind: 'sheet' });
    expect(s.sheetEpoch).toBe(5); expect(s.entry('e1')).toBeDefined();
    s.sheetEpoch = 3;
    streamEvent(s, 'sheet', '{"version":10,"entries":[]}');
    expect(s.sheetEpoch).toBe(3);                                       // a sheet with no epoch keeps the phone's
  });
  test('S4 a sheet_ops that does not begin at the phone\'s version asks to reconnect from it', () => {
    const s = new RunSheetStore();
    streamEvent(s, 'sheet', '{"version":9,"sheet_epoch":5,"entries":[]}');
    expect(streamEvent(s, 'sheet_ops', '{"from_version":99,"to_version":100,"ops":[]}')).toEqual({ kind: 'resync', from: 9 });
    expect(streamEvent(s, 'sheet_ops', '{"from_version":9,"to_version":10,"ops":[]}')).toEqual({ kind: 'ignored' });
  });
  test('S5 unknown and broken events are ignored', () => {
    const s = new RunSheetStore();
    expect(streamEvent(s, 'ping', '')).toEqual({ kind: 'ignored' });
    expect(streamEvent(s, 'sheet', 'not json')).toEqual({ kind: 'ignored' });
    expect(streamEvent(s, 'sheet', '[1]')).toEqual({ kind: 'ignored' });
    expect(streamEvent(s, 'sheet_ops', '{}')).toEqual({ kind: 'ignored' });
  });
});
