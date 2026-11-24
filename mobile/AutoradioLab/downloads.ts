// The model files voice needs, offered with their sizes before any byte is fetched (owner, 2026-11-23: "yes, we do need to download the 'optional' voice model downloads. from hugging face ... apple builtin recognizer and voice isnt
// good enough"). Three groups, each its own consented download (the agreement is per group: one group's press never agrees to another's files):
//   voice       the radio's speaking voice. What a listener who only listens needs first (the first screen offers this one alone); until it is on the phone the radio speaks with the iPhone's built-in voice.
//   voice-rest  hearing you, knowing when you speak, the small language model: what voice needs. Voice is offered as ready only once they are present; turning voice on without them opens this page.
//   search      the model that searches the phone's own stories, documents and mail.
// Each group lists its JOBS (hearing you, knowing when you speak, the voice, the answering model, knowing your voice, search); each job lists its options, one marked recommended for THIS phone, and the person may choose another
// (kept: consent.ts `picks`). Names, sizes and the recommendation come from the model list the native side reads (offers.ts, ModelList.swift); this file writes no model's name or size. A newer model in the list is a new option
// with its own size and the same press, and a different phone may be recommended a different option for every job.
// Each group's button is the person's agreement to that download; it is recorded (consent.ts) because the background search pass is the one automatic path and may only fetch what was agreed to.
//
// Failure list (each line has a test in downloads.test.ts):
//  D1 Nothing downloads until a button is pressed: the view is built from facts alone and calls nothing.
//  D2 Each group lists its files with the plan's own sizes, their sum, the host, and says "mobile data" before starting when the phone is not on Wi-Fi.
//  D3 A group is ready only when every file is present; a group with a row that is not ready offers the download (a newer model appears as an offer, not a silent fetch).
//  D4 Start on Wi-Fi records the agreement and starts that group alone; on mobile data it starts nothing until the mobile press, which records the mobile agreement.
//  D5 A second press while a group is downloading starts nothing again.
//  D6 The sizes are written as the native side writes them: a plan with no files is "nothing to download".
//  D7 Each job's options are listed with name and size, the recommended one tagged; with no pick the recommended one is chosen; the group's total is the chosen options not yet on the phone, not the sum of the options.
//  D8 A pick is kept (consent.ts), only for a job and option that are on offer, and the press that starts a group hands native exactly the chosen option of each job (never another); a job with nothing that fits starts nothing.
//  D14 A press reads the facts as they are NOW (the network path moved since the page was drawn): on mobile data only the mobile press starts anything.
//  D9 Fluid Speech Pro-like names are not special-cased: an option appears only when the list offers it.
import type { Storage } from './app.contract.d.ts';
import { consentAgree, consentPick, consentPicks, consentView } from './consent.ts';
import { nativeLater } from './api.ts';
import { kickSearch } from './search-kick.ts';
import { noVoice, type VoiceFacts } from './onboarding.ts';
import { chosen, needs, picksFor, setupBytes, sizeLabel, type OfferJob } from './offers.ts';

export { sizeLabel };
export type OptionView = { id: string; text: string; tag: string; note: string; credit: string; selected: boolean; ready: boolean };
export type JobView = { job: string; title: string; options: OptionView[]; ready: boolean; none: string };
export type Group = {
  id: string; title: string; why: string; jobs: JobView[]; total: string; host: string; ready: boolean; busy: boolean; status: string; onWiFi: boolean; mobileLine: string;
  startLabel: string; mobileLabel: string; hostLine: string; canStart: boolean;
};
export type Downloads = { groups: Group[]; rev: number; voiceReady: boolean; hearingReady: boolean; setupTotal: string };

const TITLES: Record<string, [string, string]> = {
  voice: ["The radio's voice", "What the radio speaks with. Until it is on this phone the radio uses the iPhone's built-in voice."],
  'voice-rest': ['Hearing you', 'What voice needs to hear and understand you. Voice turns on only once these are on this phone.'],
  search: ['Search on this phone', 'Finds your stories, documents and mail on the phone without sending the question anywhere.'],
};
const JOB_TITLES: Record<string, string> = {
  transcriber: 'Hearing you', decision: 'Quick decisions', bounds: 'Knowing when you speak', voice: 'The voice', responder: 'Answering while the computer works', speaker: 'Knowing your voice', search: 'Search on this phone',
};
const ORDER = ['voice', 'voice-rest', 'search'];
/** Native stage names each group runs through voice setup (search is not a voice stage: its file comes through the search pass; the voice-print model is fetched beside the setup). */
export const GROUP_STAGES: Record<string, string[]> = { voice: ['voice'], 'voice-rest': ['transcriber', 'bounds', 'decision', 'responder'] };

/** D1 to D3, D6, D7: the view, from the facts, the person's picks and whether the search model is on the phone. Calls nothing. */
export function downloadsView(f: VoiceFacts, searchPresent: boolean | null, rev: number, picks: Record<string, string> = {}): Downloads {
  const offers: OfferJob[] = f.offers.map((j) => (j.job === 'search' && searchPresent !== null ? { ...j, options: j.options.map((o) => ({ ...o, ready: searchPresent })) } : j));
  const groups: Group[] = [];
  for (const id of ORDER) {
    const js = offers.filter((j) => j.group === id);
    if (js.length === 0) continue;
    const todo = js.map((j) => needs(j, picks)).filter((o): o is NonNullable<typeof o> => !!o);
    const total = todo.reduce((a, o) => a + o.bytes, 0);
    const ready = todo.length === 0;
    const busy = id !== 'search' && f.setupStarted && !f.setupFinished && f.rows.some((r) => GROUP_STAGES[id].includes(r.stage) && (f.runStages.length === 0 || f.runStages.includes(r.stage)) && (r.state === 'downloading' || r.state === 'waiting' || r.state === 'preparing'));
    const part = f.rows.filter((r) => GROUP_STAGES[id]?.includes(r.stage) && r.state === 'downloading').map((r) => r.progress);
    const status = ready ? 'On this phone.' : busy ? (part.length ? `Downloading, ${Math.round((part.reduce((a, b) => a + b, 0) / part.length) * 100)}%.` : 'Getting ready.') : '';
    const host = js[0].host;
    const [title, why] = TITLES[id] ?? [id, ''];
    const jobs: JobView[] = js.map((j) => {
      const c = chosen(j, picks);
      return {
        job: j.job, title: JOB_TITLES[j.job] ?? j.job, ready: !needs(j, picks),
        options: j.options.map((o) => ({ id: o.id, text: `${o.model}, ${o.sizeLabel}`, tag: [o.recommended ? 'Recommended for this phone' : '', o.ready ? 'On this phone' : ''].filter(Boolean).join('. '), note: o.note ?? '', credit: o.credit ?? '', selected: c?.id === o.id, ready: o.ready })),
        none: c ? '' : 'Nothing here fits this phone; it keeps what is built in.',
      };
    });
    groups.push({
      id, title, why, jobs, ready, busy, status, onWiFi: f.onWiFi, host, canStart: !ready && !busy,
      total: ready ? 'Nothing to download.' : sizeLabel(total),
      hostLine: `${ready ? '' : `${sizeLabel(total)} from ${host}. ${host} sees this phone's address and which file is asked for. `}These are data files, not apps.`.trim(),
      mobileLine: f.onWiFi ? '' : `You are on mobile data. This will use about ${sizeLabel(total)} of your plan.`,
      startLabel: f.onWiFi ? 'Download on Wi-Fi' : 'Wait for Wi-Fi', mobileLabel: 'Download now',
    });
  }
  const setup = setupBytes(offers, picks);
  return { groups, rev, voiceReady: f.voiceReady, hearingReady: f.hearingReady, setupTotal: setup === 0 ? '' : sizeLabel(setup) };
}

const present = async (): Promise<boolean | null> => {
  const later = nativeLater();
  if (!later) return null;
  try { const r = (await later({ op: 'embedState' })) as { present?: unknown } | null; return r && typeof r === 'object' ? r.present === true : null; } catch { return null; }
};
/** Source: what voice needs, group by group. */
export async function downloadsPage(facts: VoiceFacts | null | undefined, storage: Storage | undefined): Promise<Downloads> {
  const c = await consentView(storage);
  return downloadsView({ ...noVoice(), ...(facts ?? {}) }, await present(), c.rev, await consentPicks(storage));
}

/** The voice side's facts now (the network path, the offers), or the page's own when native answers nothing (a web build). */
async function liveFacts(later: ReturnType<typeof nativeLater>, page: VoiceFacts): Promise<VoiceFacts> {
  try { const r = (await later?.({ op: 'obFacts' })) as Partial<VoiceFacts> | null | undefined; if (r && typeof r === 'object' && Array.isArray(r.offers)) return { ...noVoice(), ...(r as VoiceFacts) }; } catch { /* the page's */ }
  return page;
}

/** The press. `op` start | startMobile | pick; `group` one of the group ids (for `pick`: the job, and `arg` the option). Returns the view after it. Search is started by the search pass once agreed: `kickSearch` asks it to run now. */
export async function downloadsAct(op: string, group: string, facts: VoiceFacts | null | undefined, storage: Storage | undefined, arg = ''): Promise<Downloads> {
  const later = nativeLater();
  const f = await liveFacts(later, { ...noVoice(), ...(facts ?? {}) });                        // D14: the facts as they are at the press, not as the page was drawn
  if (op === 'pick') {                                                                              // D8
    const j = f.offers.find((x) => x.job === group);
    if (j && j.options.some((o) => o.id === arg)) await consentPick(group, arg, storage);
    return downloadsPage(f, storage);
  }
  const picks = await consentPicks(storage);
  const known = ORDER.includes(group);
  const g = known ? downloadsView(f, null, 0, picks).groups.find((x) => x.id === group) : undefined;
  const mobile = op === 'startMobile';
  if (g && g.canStart && (op === 'start' ? f.onWiFi : mobile)) {                                    // D4, D5: on mobile data only the mobile press starts anything
    if (!(await consentAgree(group, mobile, storage))) return downloadsPage(f, storage);          // D11: not recorded, nothing starts
    if (group === 'search') kickSearch(storage);
    else if (later) {
      const jobs = f.offers.filter((j) => j.group === group);
      const chosenPicks = picksFor(jobs, [group], picks);
      const wants = (job: string): boolean => { const j = jobs.find((x) => x.job === job); return !!j && !!needs(j, picks); };
      const stages = GROUP_STAGES[group].filter((s) => s in chosenPicks && wants(s));
      const speaker = group === 'voice-rest' && wants('speaker');
      if (stages.length > 0 || speaker) { try { await later({ op: 'obSetupBegin', only: true, stages, picks: chosenPicks, speaker }); } catch { /* the facts say it did not start */ } }
    }
  }
  return downloadsPage(f, storage);
}
