// What the phone knows of GET /api/autoradio/sheet/stream (docs/reference/AUTORADIO-SHEET.md), as pure rules (ported from RunSheetStream.swift). Not a live connection: nothing opens
// the stream yet; this is what that connection will use, and its rules are tested now.
import { decodeSheet } from './runsheet-model.ts';
import type { RunSheetStore } from './runsheet-store.ts';

export const streamPath = (after: number | undefined, epoch: number): string => {
  const q: string[] = [];
  if (after !== undefined && after > 0) q.push(`after=${after}`);
  if (epoch > 0) q.push(`epoch=${epoch}`);
  return '/api/autoradio/sheet/stream' + (q.length ? '?' + q.join('&') : '');
};

export type StreamRetry = { kind: 'after'; seconds: number } | { kind: 'tooManyStreams'; seconds: number } | { kind: 'giveUp' };
/** What to do when the stream's request is answered with `status`; `attempt` counts the failures in a row. */
export function streamRetry(status: number, attempt: number): StreamRetry {
  const backoff = (): StreamRetry => ({ kind: 'after', seconds: Math.min(60, 2 ** Math.min(attempt, 6)) });
  if (status === 429) return { kind: 'tooManyStreams', seconds: Math.min(300, 30 * Math.max(attempt, 1)) };
  if (status === 404 || status === 501) return { kind: 'giveUp' };
  if (status >= 500 && status <= 599) return backoff();
  return { kind: 'giveUp' };
}

export type StreamEventResult = { kind: 'sheet' } | { kind: 'resync'; from: number } | { kind: 'ignored' };
/** One server-sent event. `sheet` replaces the local sheet; a `sheet_ops` that does not begin at the phone's version asks for a reconnect from it (its ops are not folded here). */
export function streamEvent(store: RunSheetStore, name: string, data: string): StreamEventResult {
  let body: unknown;
  try { body = data === '' ? undefined : JSON.parse(data); } catch { return { kind: 'ignored' }; }
  if (name === 'sheet') {
    try { const theirs = decodeSheet(body); store.adopt(theirs, theirs.sheet_epoch > 0 ? theirs.sheet_epoch : store.sheetEpoch); return { kind: 'sheet' }; } catch { return { kind: 'ignored' }; }
  }
  if (name === 'sheet_ops') {
    const from = (body as { from_version?: unknown } | undefined)?.from_version;
    if (typeof from !== 'number') return { kind: 'ignored' };
    return from === store.sheet.version ? { kind: 'ignored' } : { kind: 'resync', from: store.sheet.version };
  }
  return { kind: 'ignored' };
}
