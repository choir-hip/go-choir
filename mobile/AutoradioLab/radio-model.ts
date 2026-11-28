// The Autoradio page: the run sheet made readable. Pure view model from the run sheet and the player's state to the words and rows the page
// draws, and the gestures the page sends as ops. Ported from RadioPage.swift. The page only reads the run sheet and sends the ops the bar and
// the old sheet sent; it owns no state of the radio. Nothing here reads a clock or a file: the caller passes `now` and the zone's offset.
//
// Ways this can fail (each is a test in radio.test.ts; numbers are the Swift list's):
//  1  the status line is wrong for a state (On air, Paused, Ready, Listening, Writing the next stretch, Can't speak: <reason>, offline)
//  2  the listener speaks: the on-air card must dim and the line say Listening, whatever else is true
//  3  the player could not speak while the sheet still says something is on air: the reason is on the line and the card, with one action
//  4  an answer is on air: the page must say which entry it will go back to (the last one an answer cut), never another
//  5  the cold start (nothing played yet) must greet with the real counts and minutes, and only then (not after something was heard)
//  6  a transition is the station talking between two entries: it is not a row in Up next or Heard, and counts as no entry
//  7  Up next must hold every entry ahead in order, with who placed each, the length and the first five split from the rest
//  8  "Why this" exists only when the entry carries a forecast; an entry with none has no control and no empty text
//  9  the sentence being spoken is the one at the player's index; an index past the end shows no sentence, not a crash
// 10  podcasts and clips show no sentence (no transcript on the phone): the progress line alone
// 12  Heard lists the last entries that ended, newest first, with one word each; a failed entry is not called finished
// 13  a reorder is one `move` op (after the entry that now precedes it, `next` at the head, `end` at the foot); a swipe is one `remove`;
//     a tap is the same op "play now" sends (`move` at now); replay from Heard is one `place` at now of a fresh copy
// 14  an op for an entry that is no longer ahead changes nothing (the reducer drops it); the page does not mint a second way
// 15  a time of zero or unknown is no length, not "0 sec"
import { audioSpec, bookOf, byWords, displayTitle, filesAddress, mintId, paperRoute } from './runsheet-entries.ts';
import { ahead as aheadOf, onAirEntry, type Entry, type Op, type Sheet } from './runsheet-model.ts';

export type BuiltOn = { n: number; title: string; address: string; role: string; note: string };
export type PageRow = { n: number; id: string; kind: string; title: string; length: string; by: string; byWord: string; note: string; builtOn: BuiltOn[] };
export type HeardRow = { n: number; id: string; kind: string; title: string; outcome: string; builtOn: BuiltOn[]; publisher: string };
export type PageInfo = {
  state: string; statusLine: string; dim: boolean; playing: boolean; dateLine: string;
  onAir: boolean; kindLabel: string; title: string; source: string; published: string; sentence: string;
  elapsed: string; remaining: string; fraction: number; thenBack: string; openAddress: string; why: string; forecast: string; canWhy: boolean;
  cannotSpeak: string; action: string; heardWords: string; greeting: string; ahead: PageRow[]; aheadMore: number; writing: boolean; emptyNote: string;
  notice: string; heard: HeardRow[]; builtOn: BuiltOn[]; canPlay: boolean; takePublisher: string; takeTranscript: string; takeRecord: string;
};

export type PageInput = {
  sheet: Sheet;
  /** `BarInfo.state`: idle | playing | paused | listening | talking | heard | holding | revised | error */
  barState?: string;
  heardWords?: string; playing?: boolean; sentence?: string;
  /** 0...1 through the entry on air. */
  progress?: number;
  /** Remote audio's own labels ("1:12", "-3:40"); the entry is not spoken. */
  remoteAudio?: boolean; spokenElapsed?: number; elapsedLabel?: string; remainingLabel?: string;
  /** The player's message ("Can't speak: ...", "Could not play ..."); "" when all is well. */
  message?: string; notice?: string; writing?: boolean; dry?: boolean;
  /** The radio is wanted (it was started and not stopped): what is written next starts by itself. */
  wants?: boolean;
  /** When the entry on air was published ("2 h ago"); "" when not known. */
  published?: string;
  /** Ahead entries whose neighbours changed since they were written. */
  stale?: string[];
  /** The current version of stories the phone holds, by route. */
  versions?: Record<string, number>;
  /** The takes the phone holds, by id (for the transcript and record of a take on air). */
  takeLookup?: (id: string) => { transcript: string; record_words: string } | undefined;
  /** Epoch milliseconds, and the listener's zone as minutes east of UTC. */
  now: number; utcOffsetMinutes?: number;
};

export const NEXT_SHOWN = 5;
export const HEARD_SHOWN = 5;
/** Nothing played yet: nothing heard, or what was heard last ended more than half an hour ago (an earlier listening). */
export const COLD_AFTER_SECONDS = 30 * 60;
const isTail = (e: Entry): boolean => e.state === 'heard' || e.state === 'skipped';

export function kindLabel(e: Entry): string {
  switch (e.kind) {
    case 'opening': return 'Opening';
    case 'transition': return 'Transition';
    case 'story': return 'Story';
    case 'clip': case 'episode': return 'Podcast';
    case 'take': return 'Take';
    case 'page': return paperRoute(e) !== undefined ? 'Story' : 'Page';
    case 'document': return 'Book';
    case 'mail': return 'Mail';
    case 'result': return 'Result';
    case 'answer': return 'Answer';
  }
}
/** "45 sec", "3 min"; "" when the entry has no estimate. */
export function length(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 1) return '';
  return seconds < 90 ? `${Math.round(seconds)} sec` : `${Math.round(seconds / 60)} min`;
}
/** "3:07" or "1:02:09". */
export function clockLabel(seconds: number): string {
  const s = Math.floor(Math.max(0, Number.isFinite(seconds) ? seconds : 0));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
  const p = (n: number) => String(n).padStart(2, '0');
  return h > 0 ? `${h}:${p(m)}:${p(sec)}` : `${m}:${p(sec)}`;
}
const SMALL = ['zero', 'one', 'two', 'three', 'four', 'five', 'six', 'seven', 'eight', 'nine', 'ten', 'eleven', 'twelve', 'thirteen', 'fourteen', 'fifteen', 'sixteen', 'seventeen', 'eighteen', 'nineteen', 'twenty'];
const TENS: Record<number, string> = { 25: 'twenty-five', 30: 'thirty', 35: 'thirty-five', 40: 'forty', 45: 'forty-five', 50: 'fifty', 55: 'fifty-five', 60: 'sixty' };
export const word = (n: number): string => (n >= 0 && n < SMALL.length ? SMALL[n] : (TENS[n] ?? String(n)));
export const outcomeWord = (e: Entry): string => e.ended?.outcome ?? 'finished';

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const local = (now: number, off: number): Date => new Date(now + off * 60000);
/** "Tue 14 Nov" */
export function dateLine(now: number, off = 0): string { const d = local(now, off); return `${DAYS[d.getUTCDay()]} ${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}`; }
/** The app's one day-part word: morning from 5, afternoon from 12, evening from 18, plain "Hello" after midnight. */
export function hello(now: number, off = 0): string {
  const h = local(now, off).getUTCHours();
  return h < 5 ? 'Hello' : h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening';
}

/** "Good evening. Six things are ready, about eighteen minutes. Press play and I'll start with the opening." */
export function greeting(ahead: Entry[], now: number, off = 0): string {
  const said = ahead.filter((e) => e.kind !== 'transition');
  if (said.length === 0) return '';
  const n = said.length;
  const minutes = Math.max(1, Math.round(said.reduce((a, e) => a + e.est_seconds, 0) / 60));
  const w = word(n);
  const things = n === 1 ? 'One thing is ready' : `${w.charAt(0).toUpperCase() + w.slice(1)} things are ready`;
  const about = minutes === 1 ? 'about a minute' : `about ${word(minutes)} minutes`;
  const start = said[0].kind === 'opening' ? 'the opening' : displayTitle(said[0]);
  return `${hello(now, off)}. ${things}, ${about}. Press play and I’ll start with ${start}.`;
}

/** The page an entry's title opens: a story, a book, a file. */
export function openAddress(e: Entry): string {
  const r = paperRoute(e);
  if (r !== undefined) return 'pub:' + r;
  // A desk story (kind story) names the story it tells in `source` (the first pub: cite): its title opens that story.
  const s = e.source?.ref;
  if (s && s.startsWith('pub:') && e.kind === 'story') return 'pub:' + s.slice(4).split('@')[0];
  const b = bookOf(e);
  if (b) return 'doc:' + b.id;
  return filesAddress(e) ?? '';
}

/** The stories an entry was built on: its cites and its source that name a story's page (`pub:<route>[@version]` or `texture:<id>`), once each,
 *  in order. The title is the label the server gave. An opening, an answer or a clip with no story has none. */
export function builtOn(e: Entry, versions: Record<string, number> = {}): BuiltOn[] {
  const seen = new Set<string>(), out: BuiltOn[] = [];
  const add = (ref: string, label: string, role?: string) => {
    if (!ref.startsWith('pub:') && !ref.startsWith('texture:')) return;
    const address = ref.startsWith('pub:') ? 'pub:' + ref.slice(4).split('@')[0] : ref;
    if (address.length <= 4 || seen.has(address)) return;
    seen.add(address);
    const b: BuiltOn = { n: out.length, title: '', address, role: role && role !== 'subject' ? role : '', note: '' };
    const m = /^pub:[^@]*@v(\d+)$/.exec(ref);
    if (m) { const now = versions[address.slice(4)]; if (now !== undefined && now !== Number(m[1])) b.note = `cited v${m[1]}, now v${now}`; }
    const t = label.trim();
    b.title = t !== '' ? t : address.slice(address.indexOf(':') + 1);
    out.push(b);
  };
  for (const c of e.cites ?? []) add(c.ref, c.label, c.role);
  if (e.source) add(e.source.ref, e.source.label);
  return out;
}

/** A sentence from what an entry carries when it carries no stated cause. */
export function builtWhy(e: Entry): string {
  const subjects = (e.cites ?? []).filter((c) => c.role === 'subject').map((c) => c.label).filter((l) => l !== '');
  const drawn = subjects.length === 0 ? '' : ` It draws on ${subjects.slice(0, 3).join(', ')}.`;
  switch (e.placed_by) {
    case 'listener': return 'You added this' + (e.kind === 'answer' ? ' by asking.' : '.');
    case 'phone':
      if (e.kind === 'opening') return 'The station opens each listening, from the front page it holds.';
      if (e.kind === 'answer') return 'You asked, and this is the answer.';
      return 'The radio put this here.' + drawn;
    case 'desk':
      if (e.kind === 'transition') return 'The station speaks between two entries.';
      return 'The desk wrote this for you.' + drawn;
  }
}

/** The entry an answer will hand back to: the latest one an answer cut (the player puts its rest back after the answers). */
export function held(sheet: Sheet): Entry | undefined {
  return [...sheet.entries].reverse().find((e) => isTail(e) && e.ended?.outcome === 'cut' && e.kind !== 'answer' && e.kind !== 'transition');
}

function isCold(tail: Entry[], now: number): boolean {
  const last = tail.at(-1);
  if (!last) return true;
  const at = last.ended?.at;
  const t = at ? Date.parse(at) : NaN;
  if (!at || !Number.isFinite(t)) return false;     // a heard entry with no time on it counts as lately heard
  return (now - t) / 1000 > COLD_AFTER_SECONDS;
}

const takeId = (e: Entry): string | undefined => (e.source?.ref.startsWith('take:') ? e.source.ref.slice(5) : undefined);

/** What the radio says that is not a failure to play or speak. */
const PLAIN_WORDS = ['That address is too long to add.'];
export function make(i: PageInput): PageInfo {
  const off = i.utcOffsetMinutes ?? 0;
  const air = onAirEntry(i.sheet);
  const ahead = aheadOf(i.sheet).filter((e) => e.kind !== 'transition');
  const tail = i.sheet.entries.filter((e) => isTail(e) && e.kind !== 'transition');
  const playing = i.playing ?? false, notice = i.notice ?? '', writing = i.writing ?? false;
  const o: PageInfo = {
    state: 'ready', statusLine: '', dim: false, playing: air !== undefined && playing, dateLine: i.now > 0 ? dateLine(i.now, off) : '', onAir: false, kindLabel: '', title: '',
    source: '', published: '', sentence: '', elapsed: '', remaining: '', fraction: 0, thenBack: '', openAddress: '',
    why: '', forecast: '', canWhy: false, cannotSpeak: '', action: '', heardWords: '', greeting: '', ahead: [], aheadMore: 0, writing, emptyNote: '', notice,
    heard: [], builtOn: [], canPlay: true, takePublisher: '', takeTranscript: '', takeRecord: '',
  };

  // the on-air card
  if (air) {
    o.onAir = true;
    o.kindLabel = kindLabel(air);
    o.title = displayTitle(air);
    const show = audioSpec(air)?.show;
    o.source = show ? show : (air.source?.label ?? '');
    o.published = i.published ?? '';
    o.openAddress = openAddress(air);
    o.builtOn = builtOn(air, i.versions);
    if (air.kind === 'take') {
      o.takePublisher = air.source?.label ?? '';
      const id = takeId(air), t = id ? i.takeLookup?.(id) : undefined;
      if (t) { o.takeTranscript = t.transcript; o.takeRecord = t.record_words; }
    }
    if (!i.remoteAudio) o.sentence = i.sentence ?? '';   // no "N of M": the sentence is the one in the unit on air, so a total would change under the listener
    const p = i.progress ?? 0;
    o.fraction = Math.min(1, Math.max(0, Number.isFinite(p) ? p : 0));
    if (i.remoteAudio) { o.elapsed = i.elapsedLabel ?? ''; o.remaining = i.remainingLabel ?? ''; }
    else if (air.est_seconds >= 1) {
      const done = air.est_seconds * o.fraction;
      o.elapsed = clockLabel(done); o.remaining = '-' + clockLabel(Math.max(0, air.est_seconds - done));
    } else if ((i.spokenElapsed ?? 0) >= 1) o.elapsed = clockLabel(i.spokenElapsed ?? 0);        // no estimate (a story played from its page before its words were loaded): the time the voice has run, and no time left to say
    if (air.kind === 'answer') { const h = held(i.sheet); if (h) o.thenBack = `Then back to: ${displayTitle(h)}`; }
    // Why this: the desk's own cause and forecast when the entry carries them, else a sentence built from what the entry does carry.
    const f = air.forecast;
    if (f) {
      const cause = f.caused_by.trim();
      if (cause !== '') o.why = cause.charAt(0).toUpperCase() + cause.slice(1) + (cause.endsWith('.') ? '' : '.');
      if (f.p_heard !== undefined && f.p_heard >= 0 && f.p_heard <= 1) o.forecast = `The desk forecast a ${Math.round(f.p_heard * 100)} in 100 chance that you hear it through.`;
    }
    if (o.why === '') o.why = builtWhy(air);
    o.canWhy = o.why !== '';
  }

  // up next
  const stale = new Set(i.stale ?? []);
  const rows: PageRow[] = ahead.map((e, k) => ({
    n: k, id: e.id, kind: kindLabel(e), title: displayTitle(e), length: length(e.est_seconds), by: e.placed_by,
    byWord: e.placed_by === 'listener' ? 'You' : e.placed_by === 'desk' ? 'Desk' : 'Phone', note: stale.has(e.id) ? 'Written for other neighbours' : '', builtOn: builtOn(e, i.versions),
  }));
  o.ahead = rows;
  o.aheadMore = Math.max(0, rows.length - NEXT_SHOWN);
  if (rows.length === 0) {
    o.emptyNote = writing ? 'The desk is writing what comes next.'
      : notice !== '' ? 'Nothing is coming up yet.'
        : i.dry ? 'Nothing is coming up. Open the mark below to find a story, a page or a podcast, and add it.' : 'The desk will write more shortly.';
  }

  // An entry that never made a sound (failed) was not heard: it is not listed.
  o.heard = tail.filter((e) => e.ended?.outcome !== 'failed').reverse().slice(0, HEARD_SHOWN).map((e, k) => ({
    n: k, id: e.id, kind: kindLabel(e), title: displayTitle(e), outcome: outcomeWord(e), publisher: e.kind === 'take' ? (e.source?.label ?? '') : '', builtOn: builtOn(e, i.versions),
  }));

  // the status line, one state at a time; the first that is true wins
  // (the greeting is not offered while the radio cannot speak: it would say "Press play" over the reason)
  const listening = ['listening', 'talking', 'heard'].includes(i.barState ?? 'idle');
  const failure = playing ? '' : (i.message ?? '').trim();
  if (i.now > 0 && !air && failure === '' && ahead.length > 0) o.greeting = greeting(ahead, i.now, off);
  if (listening) {
    o.state = 'listening'; o.statusLine = 'Listening'; o.dim = true;
    const w = (i.heardWords ?? '').trim();
    o.heardWords = w === 'Listening…' ? '' : w;
  } else if (failure !== '') {
    let why = failure;
    for (const p of ['Can’t speak: ', "Can't speak: "]) if (why.startsWith(p)) why = why.slice(p.length);
    // A word from the radio that is not a failure to speak (an address too long to add) is said as it is, not labelled "Can’t speak".
    const spoke = !PLAIN_WORDS.includes(failure);
    o.state = 'error'; o.cannotSpeak = spoke ? why : ''; o.statusLine = spoke ? `Can’t speak: ${why}` : failure; o.action = spoke || /Tap play/.test(failure) ? 'Try again' : '';
  } else if (notice !== '') {
    o.state = 'offline'; o.statusLine = 'Offline: the desk cannot write ahead';
  } else if (air) {
    o.state = playing ? 'onair' : 'paused'; o.statusLine = playing ? 'On air' : 'Paused';
  } else if (writing) {
    o.state = 'writing';
    o.statusLine = i.wants ? 'Writing the next stretch. It will start by itself.' : 'Writing the next stretch';
    o.canPlay = !i.wants;
  } else if (o.greeting !== '' && isCold(tail, i.now)) {
    o.state = 'cold'; o.statusLine = 'Ready';
  } else {
    o.state = 'ready'; o.statusLine = 'Ready';
  }
  return o;
}

// ---- the gestures the page sends: what a hand on the page means, as the one op each sends ----

export type Gesture =
  | { kind: 'reorder'; id: string; before?: string }   // drag a row to a new place: `before` is the entry it now sits above (absent: the end)
  | { kind: 'remove'; id: string }                     // swipe a row away
  | { kind: 'playNow'; id: string }                    // tap a row: play it now (a `move` at now)
  | { kind: 'replay'; id: string };                    // tap a row of Heard: play it again (a fresh copy placed at now)

/** The op a reorder sends: a `move` after the entry that will precede it, `next` when it goes first, `end` when it goes last.
 *  Undefined when nothing would change (it is not ahead, or it already sits there). */
export function reorderOp(sheet: Sheet, id: string, before: string | undefined, opId: string): Op | undefined {
  const ahead = aheadOf(sheet);
  const from = ahead.findIndex((e) => e.id === id);
  if (from < 0) return undefined;
  const rest = ahead.filter((e) => e.id !== id);
  let at: number;
  if (before !== undefined) { const k = rest.findIndex((e) => e.id === before); if (k < 0) return undefined; at = k; } else at = rest.length;
  if (at === from) return undefined;
  const base = { op_id: opId, by: 'listener' as const, op: 'move' as const, id, base_version: 0 };
  if (at === 0) return { ...base, at: 'next' };
  if (at === rest.length) return { ...base, at: 'end' };
  return { ...base, at: 'after', after: rest[at - 1].id };
}

/** What a gesture does, as data: the op to send, or the entry to place now, or nothing. The player (or the store) carries it out. */
export type Action =
  | { kind: 'op'; op: Op }
  | { kind: 'remove'; id: string }
  | { kind: 'playUpcoming'; index: number }
  | { kind: 'placeNow'; entry: Entry }
  | { kind: 'nothing' };
export function perform(sheet: Sheet, g: Gesture, opId: string): Action {
  switch (g.kind) {
    case 'reorder': { const op = reorderOp(sheet, g.id, g.before, opId); return op ? { kind: 'op', op } : { kind: 'nothing' }; }
    case 'remove': return aheadOf(sheet).some((e) => e.id === g.id) ? { kind: 'remove', id: g.id } : { kind: 'nothing' };
    case 'playNow': { const i = aheadOf(sheet).findIndex((e) => e.id === g.id); return i < 0 ? { kind: 'nothing' } : { kind: 'playUpcoming', index: i }; }
    case 'replay': {
      const old = sheet.entries.find((e) => e.id === g.id);
      if (!old || !isTail(old)) return { kind: 'nothing' };
      const again: Entry = { ...old, id: mintId(old.placed_by === 'listener' ? 'l-' : 'p-'), state: 'ahead', placed_by: 'listener' };
      delete again.ended;
      return { kind: 'placeNow', entry: again };
    }
  }
}
export { byWords };
