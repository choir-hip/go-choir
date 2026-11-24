// Settings > Voice and models: the ONE page that holds every model of every job (owner, 2026-11-24: "the downloads page is confusing. it should all be on voice and models setup", "all the models should be accessible and
// testable in the voices and models page. no downloads page"). Each job is a section; each section lists ALL the options the model list holds for it (offers.ts: the native side reads the list; this file writes no model's
// name, size or credit), and each row says what it is, its size, who it comes from and under what licence, whether it is recommended for THIS phone, and its true state, with the buttons that fit that state:
//   Download  the person's press on this row is the agreement to THIS model's files (consent.ts, keyed by the model id; search keeps the key "search" for the background search pass), Wi-Fi or mobile data.
//   Use       make it the model the phone uses for the job (native keeps the choice).
//   Remove    delete it from the phone (not the one in use).
//   Test      run it once on the phone and show what it did (native does the run: it needs the model itself).
// Nothing downloads without a press on a row. A press agrees to that one download: it never agrees to another model.
//
// Sections, in the order a person meets them: transcriber (hearing you), live (words as you speak: the streaming recognisers), bounds (knowing when you speak), voice (the voices), responder (the answering model),
// decision (quick decisions), speaker (knowing your voice), search (search on this phone). A job the list has no option for is not drawn.
import type { Storage } from './app.contract.d.ts';
import { consentAgree, consentPick, consentPicks, consentView } from './consent.ts';
import { accountGeneration, nativeLater } from './api.ts';
import { kickSearch, testSearch } from './search-kick.ts';
import { clock } from './clock.ts';   // the time a data source may know: the device's own, handed in as a fact
import { noVoice, type ModelTest, type VoiceFacts } from './onboarding.ts';
import { MIC_OFF_LINE, MIC_PRE_LINE, micPermission } from './micask.ts';
import { forFirstRun, needs, setupBytes, sizeLabel, STAGE_JOBS, type OfferJob, type Option } from './offers.ts';

export { sizeLabel };
export type ModelRow = {
  id: string; job: string; section: string; name: string; size: string; tag: string; source: string; credit: string; note: string; state: string; line: string; progress: number;
  canDownload: boolean; canUse: boolean; canRemove: boolean; canTest: boolean; downloadLabel: string; mobileLine: string; testing: boolean; testLine: string;
};
export type ModelSection = { section: string; tab: string; job: string; title: string; why: string; options: ModelRow[] };
export type ModelTab = { id: string; title: string };
export type FirstLine = { id: string; text: string; credit: string };
export type ModelsView = { tabs: ModelTab[]; jobs: ModelSection[]; rev: number; voiceReady: boolean; hearingReady: boolean; setupTotal: string; voiceNote: string; lines: FirstLine[]; voiceOffer: ModelRow[] };

/** One tab per kind of model, in plain words; each job's sections sit on its tab. */
const TABS: [string, string, string[]][] = [
  ['hearing', 'Hearing', ['transcriber', 'live', 'bounds']], ['voice', 'Voice', ['voice']], ['answering', 'Answering', ['responder']], ['deciding', 'Deciding', ['decision']],
  ['search', 'Search', ['search']], ['voiceprint', 'Your voice', ['speaker']],
];
const tabOf = (section: string): string => TABS.find((t) => t[2].includes(section))?.[0] ?? section;
const ORDER = ['transcriber', 'live', 'bounds', 'voice', 'responder', 'decision', 'speaker', 'search'];
const TITLES: Record<string, [string, string]> = {
  transcriber: ['Hearing you', 'Turns what you say into words when you stop speaking. Until one is on this phone the radio uses Apple\'s recogniser.'],
  live: ['Live words as you speak', 'Shows words while you are still talking, so the radio can tell when you are done.'],
  bounds: ['Knowing when you speak', 'Tells your voice from silence, so the radio knows when to listen.'],
  voice: ['The voice', 'What the radio speaks with. Until one is on this phone the radio uses the iPhone\'s built-in voice.'],
  responder: ['The answering model', 'Answers while the computer works, on this phone.'],
  decision: ['Quick decisions', 'A small model that decides what to do with what you said.'],
  speaker: ['Knowing your voice', 'Tells your voice from a TV or another person. Your voice print stays on this phone.'],
  search: ['Search on this phone', 'Finds your stories, documents and mail on the phone without sending the question anywhere.'],
};
/** The native job a section runs as (live words are the recogniser job). */
const nativeJob = (section: string): string => (section === 'live' ? 'transcriber' : section);
/** The key the person's agreement is kept under: the model's own id, except the search model, whose agreement the background search pass reads under "search". */
const agreementKey = (job: string, id: string): string => (job === 'search' ? 'search' : id);
/** The first run's one line when the voice on offer is withheld (a neural voice is fetched from Voice and models first). */
export const VOICE_NOTE = "The radio uses the iPhone's voice for now. A better voice is in Settings, Voice and models.";

/** The tests Exact runs itself (search: the index is shared code); native's are in the facts. */
const local = new Map<string, ModelTest>();
/** Sign-out (account.ts WIPES): the test lines belong to the account that ran them. */
export const forgetModelTests = (): void => { local.clear(); stamps.clear(); asked.clear(); };
/** Rows whose plain microphone line was shown: the next press goes on (the system prompt follows it). */
const asked = new Set<string>();
const LISTENS = ['transcriber', 'bounds', 'speaker'];
const stamps = new Map<string, number>(); let stampN = 0;
const seconds = (ms: number): string => `${(ms / 1000).toFixed(1)} s`;

function row(f: VoiceFacts, j: OfferJob, o: Option, section: string, blocked: boolean): ModelRow {
  const run = f.modelRuns?.find((r) => r.id === o.id);
  const test = local.get(`${j.job}:${o.id}`) ?? f.modelTests?.find((t) => t.key === `${j.job}:${o.id}`);
  const stage = STAGE_JOBS.includes(j.job);
  const running = !!run && (run.state === 'downloading' || run.state === 'waiting' || run.state === 'preparing');
  const failed = !o.ready && !running && run?.state === 'failed';
  const state = running ? 'downloading' : o.ready ? (o.inUse ? 'inuse' : 'ready') : failed ? 'failed' : 'none';
  const waits = !o.ready && !running && stage && blocked;
  const line = state === 'downloading' ? (run!.state === 'downloading' && run!.progress > 0 ? `Downloading, ${Math.round(run!.progress * 100)}%` : 'Getting ready')
    : state === 'inuse' ? 'In use' : state === 'ready' ? 'On this phone' : state === 'failed' ? (run!.line || 'The download did not finish.') : waits ? 'Waiting for the download in progress' : 'Not downloaded';
  const testing = !!test && (test.state === 'recording' || test.state === 'working' || test.state === 'testing');
  const testLine = !test ? '' : testing ? test.line : test.ms > 0 ? `${test.line} (${seconds(test.ms)})` : test.line;
  const tag = [o.recommended ? 'Recommended for this phone' : '', o.withheld ? 'Optional' : ''].filter(Boolean).join('. ');
  return {
    id: o.id, job: j.job, section, name: o.model, size: o.sizeLabel, tag, source: j.host, credit: o.credit ?? '', note: o.note ?? '', state, line, progress: state === 'downloading' ? (run?.progress ?? 0) : 0,
    canDownload: !o.ready && !running && !waits, canUse: o.ready && !o.inUse && section !== 'speaker' && section !== 'search', canRemove: o.removable === true && o.ready,
    canTest: o.ready && !testing, downloadLabel: f.onWiFi ? 'Download' : 'Download on mobile data',
    mobileLine: f.onWiFi ? '' : `You are on mobile data. This will use about ${sizeLabel(o.bytes)} of your plan.`, testing, testLine,
  };
}

/** The view, from the facts alone (M1: it calls nothing). `searchPresent`: the native state of the search file, when known. */
export function modelsView(f: VoiceFacts, searchPresent: boolean | null, rev: number, picks: Record<string, string> = {}): ModelsView {
  const offers: OfferJob[] = f.offers.map((j) => (j.job === 'search' && searchPresent !== null ? { ...j, options: j.options.map((o) => ({ ...o, ready: searchPresent })) } : j));
  const blocked = f.setupStarted && !f.setupFinished;
  const jobs: ModelSection[] = [];
  for (const section of ORDER) {
    const j = offers.find((x) => x.job === nativeJob(section));
    if (!j) continue;
    const mine = j.job === 'transcriber' ? j.options.filter((o) => (o.streams === true) === (section === 'live')) : j.options;
    if (mine.length === 0) continue;
    const [title, why] = TITLES[section] ?? [section, ''];
    jobs.push({ section, tab: tabOf(section), job: j.job, title, why, options: mine.map((o) => row(f, j, o, section, blocked)) });
  }
  const first = firstRunLines(f, picks);
  const voiceJob = offers.find((j) => j.job === 'voice');
  const rec = voiceJob ? forFirstRun([voiceJob])[0]?.options.find((o) => o.recommended) : undefined;
  return {
    tabs: TABS.filter((t) => jobs.some((s) => s.tab === t[0])).map((t) => ({ id: t[0], title: t[1] })), jobs, rev, voiceReady: f.voiceReady, hearingReady: f.hearingReady, setupTotal: first.total, lines: first.lines,
    voiceNote: offers.some((j) => j.job === 'voice') && !forFirstRun(offers).some((j) => j.job === 'voice') ? VOICE_NOTE : '',
    voiceOffer: voiceJob && rec ? [row(f, voiceJob, rec, 'voice', blocked)] : [],
  };
}

/** The first run's lines: the chosen (the person's pick, else the recommended) option of each job that is not on the phone, each file once, and their total. A withheld option is not the first run's. Search is not part of it. */
export function firstRunLines(f: VoiceFacts, picks: Record<string, string>): { lines: FirstLine[]; total: string } {
  const shown = forFirstRun(f.offers).filter((j) => j.group !== 'search');
  const seen = new Set<string>(); const lines: FirstLine[] = [];
  for (const j of shown) {
    const o = needs(j, picks);
    if (o && !seen.has(o.id)) { seen.add(o.id); lines.push({ id: o.id, text: `${o.model}, ${o.sizeLabel}`, credit: o.credit ?? '' }); }
  }
  const bytes = setupBytes(f.offers, picks);
  return { lines, total: bytes === 0 ? '' : sizeLabel(bytes) };
}

const present = async (): Promise<boolean | null> => {
  const later = nativeLater();
  if (!later) return null;
  try { const r = (await later({ op: 'embedState' })) as { present?: unknown } | null; return r && typeof r === 'object' ? r.present === true : null; } catch { return null; }
};
/** Source: the page. */
export async function modelsPage(facts: VoiceFacts | null | undefined, storage: Storage | undefined): Promise<ModelsView> {
  const c = await consentView(storage);
  return modelsView({ ...noVoice(), ...(facts ?? {}) }, await present(), c.rev, await consentPicks(storage));
}

/** The voice side's facts now (the network path, the offers), or the page's own when native answers nothing (a web build). */
async function liveFacts(later: ReturnType<typeof nativeLater>, page: VoiceFacts): Promise<VoiceFacts> {
  try { const r = (await later?.({ op: 'obFacts' })) as Partial<VoiceFacts> | null | undefined; if (r && typeof r === 'object' && Array.isArray(r.offers)) return { ...noVoice(), ...(r as VoiceFacts) }; } catch { /* the page's */ }
  return page;
}

/** The microphone is asked where it is first needed, after one plain line (micask.ts, docs/ship/consent-and-prompts.md): unasked shows the line and the NEXT press goes on; refused shows the declined line and starts nothing. */
async function micReady(key: string): Promise<boolean> {
  const gen = accountGeneration();
  const state = (await micPermission('')).state;
  if (gen !== accountGeneration()) return false;                                            // the account left while the permission was read: this press writes nothing and leaves nothing behind
  if (state === 'refused') { local.set(key, { key, state: 'ask', line: MIC_OFF_LINE, ms: 0 }); return false; }
  if (state === 'unasked' && !asked.has(key)) { asked.add(key); local.set(key, { key, state: 'ask', line: `${MIC_PRE_LINE} Press Test again to go on.`, ms: 0 }); return false; }
  asked.delete(key); return true;
}

/** The press. `op`: download | downloadMobile | use | remove | test. `section` and `id` name the row. Returns the page after it. */
export async function modelsAct(op: string, section: string, id: string, facts: VoiceFacts | null | undefined, storage: Storage | undefined): Promise<ModelsView> {
  const later = nativeLater();
  const f = await liveFacts(later, { ...noVoice(), ...(facts ?? {}) });                       // M11: the facts as they are at the press, not as the page was drawn
  const v = modelsView(f, await present(), 0, {});
  const r = v.jobs.find((s) => s.section === section)?.options.find((o) => o.id === id);
  if (!r) return modelsPage(f, storage);                                                        // M6: an id that is not offered
  const job = r.job;
  if (op === 'download' || op === 'downloadMobile') {
    const mobile = op === 'downloadMobile';
    if (r.canDownload && (mobile || f.onWiFi)) {                                                // M5: on mobile data only the mobile press starts anything
      if (!(await consentAgree(agreementKey(job, id), mobile && !f.onWiFi, storage))) return modelsPage(f, storage);   // M12: not recorded, nothing starts
      if (job === 'search') kickSearch(storage);
      else if (later) {
        const speaker = job === 'speaker';
        const stages = STAGE_JOBS.includes(job) ? [job] : [];
        const picks: Record<string, string> = STAGE_JOBS.includes(job) ? { [job]: id } : {};
        try { await later({ op: 'obSetupBegin', only: true, stages, picks, speaker, row: true }); }       // row: this press fails as itself (no wait, no fallback in its place)
         catch { /* the facts say it did not start */ }
      }
    }
  } else if (op === 'use') {
    if (r.canUse) {
      await consentPick(section === 'live' ? 'live' : job, id, storage);
      try { await later?.({ op: 'obUseModel', job, id, live: section === 'live' }); } catch { /* the facts say what is in use */ }
    }
  } else if (op === 'remove') {
    if (r.canRemove) { try { await later?.({ op: 'obRemoveModel', id }); } catch { /* the facts say it is still there */ } }
  } else if (op === 'test') {
    // The run needs the model itself (it loads it, listens, speaks, answers): native does it and reports through the facts (modelTests).
    if (r.canTest && job === 'search') {                                                       // search is shared code: Exact runs it (retrieval-live.ts)
      const key = `${job}:${id}`; const t0 = clock.ms; const gen = accountGeneration(); const mine = ++stampN; stamps.set(key, mine);
      local.set(key, { key, state: 'working', line: 'Searching your stories on this phone.', ms: 0 });
      let line: string | undefined; try { line = await testSearch('what is in my stories today', storage); } catch { line = undefined; }
      if (gen !== accountGeneration()) { if (stamps.get(key) === mine) { local.delete(key); stamps.delete(key); } return modelsPage(f, storage); }       // only this call's own line: the next account's is not ours to erase                         // the account left while it ran: its line is not the next account's
      local.set(key, line === undefined ? { key, state: 'failed', line: 'It did not work on this phone: the search index is not available.', ms: 0 } : { key, state: 'done', line, ms: Math.max(0, clock.ms - t0) });
    } else if (r.canTest && LISTENS.includes(job) && !(await micReady(`${job}:${id}`))) {
      // the microphone line was shown instead (micask.ts): nothing starts on this press
    } else if (r.canTest) { try { await later?.({ op: 'obModelTest', job, id }); } catch { /* the facts say it did not run */ } }
  }
  return modelsPage(f, storage);
}
