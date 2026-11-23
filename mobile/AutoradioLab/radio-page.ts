// The Autoradio page's source, in shared code (it was Swift: AutoradioModule+RadioPage.swift gathered the input and RadioPage.make decided; the model is radio-model.ts).
// What stays native, and why: the FACTS. The run sheet itself is the native run sheet player's until the queue and its posted ops move (slices 3 and 4), so
// the source `radioPageFacts` hands it over as the store holds it (wire JSON, applied ops left out), with what only native code knows: whether the radio plays, the sentence being spoken
// and the audio's place in it, remote audio's clock, the desk's flags, when the story on air was published, the story versions the phone holds, the take on air. It decides nothing.
// The bar's facts and the conversation's entries come in too: whether a turn is open is the BAR's state (bar.ts), so an error held on the bar beats "Listening" as it did in Swift.
// Nothing is kept here and nothing is stored: every call is from its facts.
import { barInfo } from './bar.ts';
import { clockLabel, make, type PageInfo } from './radio-model.ts';
import { paperRoute } from './runsheet-entries.ts';
import { decodeSheet, emptySheet, type Sheet } from './runsheet-model.ts';
import { relative } from './paper-model.ts';

export type RadioFacts = {
  at: number; sheet: string; playing: boolean; sentences: string[]; sentenceIndex: number; progress: number;
  remoteAudio: boolean; remoteElapsed: number; spokenElapsed: number; remoteDuration: number; wants: boolean; message: string; notice: string; writing: boolean; dry: boolean;
  /** The title and route of the programme on air (the headline being read and the story it is); "" when none. */
  airTitle: string; airRoute: string; publishedAt: number; versionRoutes: string[]; versionNumbers: number[]; stale: string[]; takeTranscript: string; takeRecord: string;
};
export const EMPTY_FACTS: RadioFacts = {
  at: 0, sheet: '', playing: false, sentences: [], sentenceIndex: 0, progress: 0, remoteAudio: false, remoteElapsed: 0, spokenElapsed: 0, remoteDuration: 0, wants: false, message: '', notice: '', writing: false, dry: false,
  airTitle: '', airRoute: '', publishedAt: 0, versionRoutes: [], versionNumbers: [], stale: [], takeTranscript: '', takeRecord: '',
};

const obj = (v: unknown): Record<string, unknown> => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);
const strs = (v: unknown): string[] => (Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []);

let readLast: { text: string; sheet: Sheet } | undefined;       // the same text is the same sheet: the page is asked every second while the radio sounds, and the text can be a megabyte
function readSheet(text: string): Sheet {
  if (text === '') return emptySheet();
  if (readLast && readLast.text === text) return readLast.sheet;
  let sheet: Sheet;
  try { sheet = decodeSheet(JSON.parse(text)); } catch { return emptySheet(); }
  readLast = { text, sheet };
  return sheet;
}

/** An entry placed by a play that carried no title (the native play took only the route) has an empty title and label; the programme on air has the headline. The on-air entry of a story or
 * a paper page takes it, for its title and its source label (the small line and the built-on row). Nothing else is renamed, and nothing is invented when there is no programme title. */
function namedOnAir(sheet: Sheet, airTitle: string, airRoute: string, remote: boolean): Sheet {
  const t = airTitle.trim();
  if (t === '' || airRoute === '' || remote) return sheet;
  const i = sheet.entries.findIndex((e) => e.state === 'on_air');
  const e = sheet.entries[i];
  // only a paper page whose route IS the programme's: while another document is still loaded the headline on air is that one's, not this entry's
  if (!e || e.kind !== 'page' || paperRoute(e) !== airRoute || e.title.trim() !== '' || (e.source?.label ?? '') !== '') return sheet;
  const named = { ...e, title: t, source: e.source ? { ...e.source, label: t } : e.source };
  return { ...sheet, entries: sheet.entries.map((x, k) => (k === i ? named : x)) };
}

/** The listener's zone as minutes east of UTC, read from the platform (a date made from a given time, never the clock). */
const zoneMinutes = (at: number): number => { try { return -new Date(at * 1000).getTimezoneOffset(); } catch { return 0; } };

/** The page for the native facts, the bar's facts and the conversation's entries. `offsetMinutes` overrides the zone (tests). */
export function radioPage(rawFacts: unknown, rawBar: unknown, rawEntries: unknown, offsetMinutes?: number): PageInfo {
  const f = obj(rawFacts);
  const at = num(f.at) > 0 ? num(f.at) : 0;   // a non-positive time is no clock (a host with no native facts): no date, no greeting, no relative time
  const b = barInfo(rawBar, rawEntries);
  const versions: Record<string, number> = {};
  const routes = strs(f.versionRoutes), numbers = Array.isArray(f.versionNumbers) ? f.versionNumbers.map(num) : [];
  if (routes.length === numbers.length) routes.forEach((r, i) => { versions[r] = numbers[i]; });
  const remote = f.remoteAudio === true, duration = num(f.remoteDuration), elapsed = num(f.remoteElapsed);
  const published = num(f.publishedAt);
  const takeTranscript = str(f.takeTranscript), takeRecord = str(f.takeRecord);
  return make({
    sheet: namedOnAir(readSheet(str(f.sheet)), str(f.airTitle), str(f.airRoute), remote),
    barState: b.state, heardWords: b.text,
    playing: f.playing === true || b.state === 'playing',
    sentences: strs(f.sentences), sentenceIndex: Math.trunc(num(f.sentenceIndex)), progress: num(f.progress),
    remoteAudio: remote, elapsedLabel: remote ? clockLabel(elapsed) : '', remainingLabel: remote && duration > 0 ? '-' + clockLabel(Math.max(0, duration - elapsed)) : '',
    message: str(f.message), notice: str(f.notice), writing: f.writing === true, dry: f.dry === true, wants: f.wants === true,
    published: at > 0 && published > 0 ? relative(published * 1000, at * 1000) : '',
    spokenElapsed: Math.max(0, num(f.spokenElapsed)), stale: strs(f.stale), versions,
    takeLookup: takeTranscript !== '' || takeRecord !== '' ? () => ({ transcript: takeTranscript, record_words: takeRecord }) : undefined,
    now: at * 1000, utcOffsetMinutes: offsetMinutes ?? (at > 0 ? zoneMinutes(at) : 0),
  });
}
