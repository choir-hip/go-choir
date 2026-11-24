// What a clip row's Play and Queue put on the run sheet (clips.ts decides WHICH clip; this makes the entry and places it). It replaces the Swift `clipPlay` handler (AutoradioModule+Joins.swift) and
// `HearIt` (DJQueueAdapter.swift). The native ClipStore that handler filled is gone: its only reader was `HearIt`; the fingerprint locator (ClipLocator.swift) is not called by any production code since
// the run sheet player was deleted, so nothing native needs the clip's record (docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md, 2026-11-23).
// A clip with a usable fingerprint is offered as the clip (the episode's audio from its start to its end); any other is its episode from the start (clips.ts `usable`).
import { parseClip, usable, GONE, type Clip } from './clips.ts';
import { audio as audioEntry } from './runsheet-entries.ts';
import { episodeKey } from './podcast-model.ts';
import type { Entry } from './runsheet-model.ts';
import type { NativeReader } from './api.ts';
import type { Storage as Store } from './app.contract.d.ts';
import { radioPlayEntry } from './radio-bridge.ts';

/** The entry a clip plays as. The episode id is the podcast key of the feed and the guid (else the audio address); a clip with no feed has none. */
export function clipEntryOf(c: Clip): Entry {
  const episode_id = c.feedURL === '' ? undefined : episodeKey(c.feedURL, c.episodeGUID !== '' ? c.episodeGUID : c.audioURL);
  const a = { url: c.audioURL, title: c.episodeTitle === '' ? c.show : c.episodeTitle, show: c.show, ...(episode_id !== undefined ? { episode_id } : {}) };
  return usable(c.fingerprint) ? audioEntry({ ...a, start: c.start, end: c.end }, c.clipID) : audioEntry(a);
}

export type Placer = (op: string, e: Entry) => Promise<void>;

/** `op` is play or queue; `record` is the clip's row as the server sent it (clips.ts hands it with the ack). */
export async function radioPlayClip(storage: Store | undefined, native: NativeReader | undefined, op: string, id: string, record: string, place?: Placer): Promise<{ status: string; message: string }> {
  if (op !== 'play' && op !== 'queue') return { status: 'ok', message: '' };
  let raw: unknown = null;
  try { raw = JSON.parse(record); } catch { /* not a record */ }
  const c = parseClip(raw);
  if (c === null || c.clipID !== id) return { status: 'error', message: GONE };
  await (place ?? ((o, e) => radioPlayEntry(storage as never, native, o, e)))(op, clipEntryOf(c));
  return { status: 'ok', message: '' };
}
