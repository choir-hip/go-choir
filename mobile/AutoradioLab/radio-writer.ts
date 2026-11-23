// The desk's writer, in shared code (it was Swift: RunSheetWriter, ProgrammeRouteFeed, RouteStretchSource.candidates and StretchCandidates). It asks the programme route for entries and places them at the end
// of the sheet, `open` first and then `extend` whenever the spoken time ahead is short, with one ask in flight. Nothing is swapped in: a desk that fails leaves the sheet as it is and says so once.
// The decisions (when to ask, what a segment becomes, what an ended ask says, which stories are already told) are programme-model.ts; this module is the request and the placing.
// The route answers an event stream to a POST, which shared code cannot read as it arrives (docs/reference/exact/gaps.md 36): the whole answer is read when it ends, so the entries of one write arrive together.
import { lapYield, lap } from './lap.ts';
import { cutBytes, oneLine } from './runsheet-entries.ts';
import { AHEAD_FLOOR_SECONDS, StretchDecoder, askOutcome, entryFrom, placeOp, plainWords, shouldAskMore, toldRoutes, type Incoming } from './programme-model.ts';
import { parseStory, weight } from './paper-model.ts';
import { candidateJSON, candidates as takeCandidates, merged, title as takeTitle, type Kept, type Take } from './takes.ts';
import type { RadioRuntime } from './radio-runtime.ts';
import type { Entry } from './runsheet-model.ts';

export const MAX_ITEMS = 40, MAX_TEXT_ITEMS = 10, MAX_TITLE_BYTES = 300, MAX_DEK_BYTES = 1000, MAX_TEXT_BYTES = 8 * 1024, MAX_ROUTE_BYTES = 256;
export const PROGRAMME_PATH = '/api/autoradio/programme';

export type Candidate = { route: string; version: number; title: string; dek: string; text?: string };
const obj = (v: unknown): Record<string, unknown> => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {});

export const validRoute = (r: string): boolean => r !== '' && new TextEncoder().encode(r).length <= MAX_ROUTE_BYTES && !/[@#\s\u0000-\u001f\u007f-\u009f]/.test(r);

/** The candidates from the front list (the paper's weight first). `skipping`: routes already told in this listening. `onlyRoutes`: a list handed over, in its order. */
export function fromFront(front: unknown, now: number, skipping: Set<string> = new Set(), onlyRoutes?: string[]): Candidate[] {
  const rows = Array.isArray(obj(front).stories) ? (obj(front).stories as unknown[]) : [];
  const scored: { w: number; c: Candidate }[] = [];
  const seen = new Set<string>();
  for (const row of rows) {
    const s = parseStory(row);
    if (!s || !validRoute(s.route) || skipping.has(s.route) || seen.has(s.route)) continue;
    seen.add(s.route);
    const title = cutBytes(oneLine(s.title), MAX_TITLE_BYTES), dek = cutBytes(oneLine(s.dek), MAX_DEK_BYTES);
    if (title === '' || dek === '') continue;
    const r = obj(row), n = (k: string): number | undefined => (typeof r[k] === 'number' && Number.isFinite(r[k]) ? Math.trunc(r[k] as number) : undefined);
    scored.push({ w: weight(s, now), c: { route: s.route, version: Math.max(1, n('latest_revision') ?? n('version_count') ?? 1), title, dek } });
  }
  if (onlyRoutes) {
    const by = new Map(scored.map((x) => [x.c.route, x.c]));
    const told = new Set<string>(), out: Candidate[] = [];
    for (const r of onlyRoutes) { const c = by.get(r); if (c && !told.has(r)) { told.add(r); out.push(c); } }
    return out.slice(0, MAX_ITEMS);
  }
  return scored.map((x, i) => ({ ...x, i })).sort((a, b) => (a.w !== b.w ? b.w - a.w : a.i - b.i)).map((x) => x.c).slice(0, MAX_ITEMS);
}

/** Paragraph text for the model: cut at a paragraph that fits in 8 KB (or inside the first one). */
export function shapeText(paragraphs: string[]): string | undefined {
  let out = '';
  for (const p of paragraphs.map((x) => x.trim()).filter((x) => x !== '')) {
    const next = out === '' ? p : out + '\n\n' + p;
    if (new TextEncoder().encode(next).length > MAX_TEXT_BYTES) { if (out === '') out = cutBytes(p, MAX_TEXT_BYTES); break; }
    out = next;
  }
  return out === '' ? undefined : out;
}

/** The request's `candidates`: the stories with the first ten's text, then the takes (together at most 40). */
export function candidateBody(items: Candidate[], texts: Record<string, string[]>, takes: Take[]): Record<string, unknown>[] {
  const stories = items.slice(0, MAX_ITEMS).map((c, i) => {
    const t = i < MAX_TEXT_ITEMS ? shapeText(texts[c.route] ?? []) : undefined;
    const o: Record<string, unknown> = { route: c.route, version: c.version, title: c.title, dek: c.dek };
    if (t) o.text = t;
    return o;
  });
  return merged(stories, takes);
}

export function requestBody(cause: string, candidates: Record<string, unknown>[], aheadTitles: string[], editionText: string, now: Date, tz: string, requestId: string): Record<string, unknown> {
  const body: Record<string, unknown> = { request_id: requestId, cause, local_time: now.toISOString().replace(/\.\d{3}Z$/, 'Z'), tz, candidates };
  const titles = aheadTitles.slice(0, 20).map((t) => cutBytes(t, MAX_TITLE_BYTES)).filter((t) => t !== '');
  if (titles.length) body.ahead_titles = titles;
  if (editionText !== '') body.edition_text = editionText;
  return body;
}

/** What the phone says first, every time, from the front titles it holds: no model, no network. */
export function phoneOpening(lines: { title: string; dek?: string }[], now: Date, offsetMinutes: number, id: string): Entry {
  const h = new Date(now.getTime() + offsetMinutes * 60000).getUTCHours();
  const hello = h < 5 ? 'Hello' : h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening';
  const sentence = (s: string): string => { const t = s.trim(); return t === '' ? '' : '.!?'.includes(t[t.length - 1]) ? t : t + '.'; };
  const strip = (s: string): string => s.replace(/^[.!? \n]+|[.!? \n]+$/g, '');
  const seen = new Set<string>();
  const named = lines.filter((l) => { const t = l.title.trim(); if (t === '' || seen.has(t.toLowerCase())) return false; seen.add(t.toLowerCase()); return true; });
  let text = `${hello}. This is Autoradio.`;
  if (named.length) {
    const first = named[0];
    text += " From the paper's front page: " + sentence(first.title);
    const firstSentence = (first.dek ?? '').trim().split(/(?<=[.!?])\s+/)[0] ?? '';
    const d = strip(firstSentence), t = strip(first.title).toLowerCase();
    if (d !== '' && !d.toLowerCase().startsWith(t) && !t.startsWith(d.toLowerCase())) text += ' ' + sentence(d);
    const rest = named.slice(1, 3).map((l) => strip(l.title));
    if (rest.length) text += ' Also: ' + rest.join('; and ') + '.';
  } else text += " I don't have the front page yet.";
  text = cutBytes(text, 8 * 1024);
  return { id, kind: 'opening', placed_by: 'phone', state: 'ahead', title: 'Opening', text, est_seconds: Math.max(1, Math.round(text.split(/\s+/).filter(Boolean).length / 2.6)), revision: 0 };
}

export type WriterDeps = {
  rt: RadioRuntime;
  front(): Promise<unknown>;
  /** The paragraphs of the first stories' text, by route, within a deadline; a story whose text is late goes without it. */
  texts(routes: string[]): Promise<Record<string, string[]>>;
  takes(): Promise<{ takes: Take[]; kept: Kept }>;
  edition(): Promise<string>;
  /** POST the programme route; the whole answer as text. Throws with `kind` 'signedOut' | 'transport' | 'other'. */
  post(body: Record<string, unknown>): Promise<string>;
  now(): Date; tz(): string; requestId(): string; generation(): number;
  /** Stories held on the phone, to play when the desk cannot be reached and fewer than two playable entries are ahead. */
  offlineFill?(): Entry[] | Promise<Entry[]>;
};

/** An ask that has not answered in this long is OVERDUE: it may still answer (a desk that streams for two minutes), and when it does its entries are used if nothing newer was applied. Being overdue lets the pulse start
 *  ONE more ask for the dry spell, not a loop: a slow desk costs two calls, not one every 120 s, and the first answer to arrive is the one that is applied (the other is dropped as older). An ask that has not answered
 *  in twice this long can no longer answer (an answer let go, a fetch dropped, a continuation that never ran) and is abandoned. A deadline on a hang, not a limit on calls. The transport's own timeout is the same
 *  120 s (api.ts AUTHED_TIMEOUT_MS), so a real request slower than this fails there; the request itself cannot be cancelled from here (no abort signal reaches Exact's fetch in the reference), so an overdue ask
 *  that is passed is left to end. */
export const ASK_DEADLINE_MS = 120_000;
const GIVE_UP_MS = 2 * ASK_DEADLINE_MS;

/** Between two entries of a long answer: lets the next turn start (Exact gives a turn about 100 ms of CPU; Bun has no scheduler). */
const yieldTurn = (label: string): Promise<void> => lapYield(`writer: ${label}`);

type Flight = { id: number; startedAt: number; epoch: number; task?: Promise<void> };

export class Writer {
  latency = 0; dry = false; notice = ''; asks: string[] = [];
  /** The id of the ask `checkAhead` last started: a caller that waits for its ask names it, so it can abandon only that one (never another ask that started meanwhile). */
  lastId = 0;
  private opened = false; private quietOn?: string; private stops = 0;
  private flights: Flight[] = []; private nextId = 0;
  /** Entries the desk placed so far: an answer is used only when nothing newer was applied since its ask began. */
  private applied = 0;
  /** The one extra ask of this dry spell has been made. Cleared when an answer is applied or an ask fails, or a new spell begins (an ask starts with none in flight). */
  private reasked = false;
  constructor(private d: WriterDeps) {}
  get inFlight(): boolean { return this.flights.length > 0; }
  settle(): Promise<void> { return Promise.all(this.flights.map((f) => f.task ?? Promise.resolve())).then(() => undefined); }
  open(): void { this.opened = true; this.ask('open'); }
  /** The radio opened: the desk is asked from now on. `start` false only marks it (the pulse starts the ask inside an answer that waits for it). */
  openAhead(start = true): void { this.opened = true; if (start) this.checkAhead(); }
  private quiet(): boolean { return this.quietOn !== undefined && this.quietOn === this.d.rt.store.onAir()?.id; }
  private age(f: Flight): number { return this.d.now().getTime() - f.startedAt; }
  /** The sheet changed or an entry started: is the time ahead short? */
  checkAhead(): boolean {
    if (!this.opened) return false;
    for (const f of this.flights.filter((x) => this.age(x) > GIVE_UP_MS)) this.abandon('no answer in twice its deadline', f.id);
    if (!this.wouldAsk()) return false;
    if (this.flights.length === 0) this.reasked = false;
    else { this.reasked = true; console.log(`writer: the desk's ask is overdue (${Math.round(this.age(this.flights[0]) / 1000)} s); one more is made for this spell and the first answer to arrive is used`); }
    this.ask('extend');
    return true;
  }
  /** Would an ask start now? (The pulse says so; a mutation then asks, and waits: a long fetch needs an answer that stays.) */
  wouldAsk(): boolean {
    if (!this.opened || this.quiet() || !shouldAskMore(this.d.rt.store.ahead(), this.latency)) return false;
    if (this.flights.length === 0) return true;
    return this.flights.length === 1 && !this.reasked && this.age(this.flights[0]) > ASK_DEADLINE_MS;      // overdue: one more, once per spell
  }
  /** One ask can no longer answer, or its answer is not wanted: it is let go (its late result is dropped whole, as after a stop). Only the ask named: another that started meanwhile is not touched. */
  abandon(why: string, id: number): void {
    const i = this.flights.findIndex((f) => f.id === id);
    if (i < 0) return;
    console.log(`writer: the desk's ask ${id} did not answer (${why}); letting it go`);
    this.flights.splice(i, 1);
    this.publish(this.flights.length > 0);
  }
  /** Wait for the ask `id`, but not for ever: at `ms` that ask (and no other) is abandoned, so a mutation that waits on it returns. */
  async settleWithin(ms: number, id: number): Promise<void> {
    const f = this.flights.find((x) => x.id === id);
    if (!f?.task) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const late = new Promise<boolean>((res) => { timer = setTimeout(() => res(true), ms); });
    const timedOut = await Promise.race([f.task.then(() => false, () => false), late]);
    if (timer !== undefined) clearTimeout(timer);
    if (timedOut) this.abandon(`no answer in ${Math.round(ms / 1000)} s`, id);
  }
  stop(): void { this.stops += 1; this.flights = []; this.reasked = false; this.quietOn = undefined; this.publish(false); }
  private publish(writing: boolean): void { this.d.rt.setDesk({ notice: this.notice, writing, dry: this.dry }); }

  private ask(cause: string): void {
    const gen = this.d.generation(), stops = this.stops, store = this.d.rt.store, airId = store.onAir()?.id, started = this.d.now().getTime();
    const fl: Flight = { id: ++this.nextId, startedAt: started, epoch: this.applied };
    this.lastId = fl.id; this.flights.push(fl); this.asks.push(cause); this.publish(true);
    const live = (): boolean => gen === this.d.generation() && stops === this.stops && this.flights.includes(fl);
    fl.task = (async () => {
      let added = 0, received = 0, dropped = 0, firstAt: number | undefined, failure: { kind: 'cancelled' | 'empty' | 'signedOut' | 'transport' | 'other' } | undefined;
      try {
        const now = this.d.now();
        const skipping = cause === 'open' ? new Set<string>() : toldRoutes(store.sheet.entries);
        const front = await this.d.front();
        await yieldTurn('after the front list');                          // the front list's decoding and the candidates below are not one turn (the proof's radioWrite took 146 ms and was refused: no desk entries followed)
        let items = fromFront(front, now.getTime(), skipping);
        if (items.length === 0 && skipping.size > 0) items = fromFront(front, now.getTime());   // everything told: the desk is still asked, with the whole list
        if (items.length === 0) { failure = { kind: 'empty' }; throw new Error('no story to tell'); }
        const texts = await this.d.texts(items.slice(0, MAX_TEXT_ITEMS).map((c) => c.route));
        const { takes: all, kept } = await this.d.takes();
        await yieldTurn('after takes');
        await yieldTurn('before candidates');
        const picked = takeCandidates(all, kept);
        const body = requestBody(cause, candidateBody(items, texts, picked), store.ahead(20).map((e) => e.title || ''), await this.d.edition(), now, this.d.tz(), this.d.requestId());
        await yieldTurn('before post');
        const raw = await this.post(body);
        lap(`writer: the desk's reply received (${raw.length} chars)`);
        if (!live()) return;                                                                  // signed out, stopped or let go meanwhile: nothing of the answer is applied
        if (this.applied !== fl.epoch) { console.log(`writer: the desk's ask ${fl.id} answered after a newer one was applied; its answer is dropped`); this.drop(fl); return; }
        const dec = new StretchDecoder();
        const takeById = new Map(picked.map((t) => [t.takeID, t]));
        const look = { clip: () => undefined, take: (id: string) => { const t = takeById.get(id); return t ? { take_id: t.takeID, publisher: t.publisher, duration: t.duration, title: takeTitle(t.transcript), responds_to: t.respondsTo ? { route: t.respondsTo.route, title: t.respondsTo.title } : undefined } : undefined; } };
        for (const ev of [...dec.feed(raw), ...dec.finish()]) {
          if (ev.type === 'segment') {
            if (!live() || this.applied !== fl.epoch + added) return;          // let go, stopped, or another ask placed entries meanwhile: nothing more of its answer is placed
            const e = entryFrom(ev.segment as Incoming, look as never);
            if (!e) continue;
            if (firstAt === undefined) firstAt = this.d.now().getTime();
            received += 1;
            const r = this.d.rt.director.applyFromDesk(placeOp(e, cause), cause);
            if (r?.applied) { added += 1; this.applied += 1; } else dropped += 1;
            await yieldTurn('after placing an entry');                        // each placement saves the whole sheet: one turn is 100 ms of CPU, and a stretch is dozens of entries
          } else if (ev.type === 'error' && added === 0) failure = { kind: 'other' };
        }
      } catch (err) {
        const k = (err as { kind?: string } | undefined)?.kind;
        console.log(`writer: the desk's stretch failed (${cause}): ${k ?? 'other'}, status ${(err as { status?: number } | undefined)?.status ?? 'none'}, ${err instanceof Error ? err.message.slice(0, 160) : String(err).slice(0, 160)}`);
        if (!failure) failure = { kind: k === 'signedOut' || k === 'transport' ? k : 'other' };
      }
      if (!live()) return;                                                                   // stop() already cleared the flags; a later ask owns them now
      if (firstAt !== undefined) this.latency = Math.max(0.5, (firstAt - started) / 1000);
      const out = askOutcome({ added, received, dropped, failed: failure !== undefined && failure.kind !== 'empty', cause, airId });
      this.drop(fl); this.dry = out.dry;
      if (added > 0) this.flights = [];                                                      // answered: any other ask of the spell is older, its answer would be dropped as such
      if (added > 0 || failure) this.reasked = false;
      if (out.clearQuiet) this.quietOn = undefined; else if (out.quietOn !== undefined) this.quietOn = out.quietOn;
      if (out.notice === 'clear') this.notice = ''; else if (out.notice === 'say') this.notice = plainWords(failure) ?? '';
      this.publish(false);
      // No second ask is started from here (it would run in no answer: the radioWrite that waited for this one has returned, and what a continuation fetches after that is never run). The tick moved
      // (`publish`), so the pulse says write again and a new radioWrite asks, inside its own answer.
      if (added === 0 && failure && this.d.offlineFill && store.ahead().filter((e) => e.kind !== 'transition').length < 2) {
        const fill = await this.d.offlineFill();
        if (gen !== this.d.generation() || stops !== this.stops) return;                       // another account or a stop while the store was read
        for (const e of fill) store.place(e, 'end', { by: 'phone', reason: 'the desk could not be reached: a story held on this phone' });
      }
    })();
  }
  private drop(f: Flight): void { const i = this.flights.indexOf(f); if (i >= 0) this.flights.splice(i, 1); }
  private async post(body: Record<string, unknown>): Promise<string> {
    try { return await this.d.post(body); }
    catch (e) {
      // `ahead_titles` is new on the route; a server that does not know it refuses the whole body (400) before any model call: one more request without it.
      if ((e as { status?: number } | undefined)?.status === 400 && body.ahead_titles !== undefined) { const { ahead_titles: _drop, ...rest } = body; return await this.d.post(rest); }
      throw e;
    }
  }
}
export { AHEAD_FLOOR_SECONDS, candidateJSON };
