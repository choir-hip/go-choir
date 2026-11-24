// The model files voice needs, offered with their sizes before any byte is fetched (owner, 2026-11-23: "yes, we do need to download the 'optional' voice model downloads. from hugging face ... apple builtin recognizer and voice isnt
// good enough"). Three groups, each one consented download, never hidden behind another action:
//   voice       the radio's speaking voice. What a listener who only listens needs first (the first screen offers this one alone); until it is on the phone the radio speaks with the iPhone's built-in voice.
//   voice-rest  hearing you, knowing when you speak, the small language model: what voice needs. Voice is offered as ready only once they are present; turning voice on without them opens this page.
//   search      the model that searches the phone's own stories, documents and mail.
// Names and sizes come from the plan the native side reads from the model lists (onboarding.ts PlanRow); this file writes no model's name or size. A newer model in a list is a new row with its own size and the same press.
// Each group's button is the person's agreement to that download; it is recorded (consent.ts) because the background search pass is the one automatic path and may only fetch what was agreed to.
//
// Failure list (each line has a test in downloads.test.ts):
//  D1 Nothing downloads until a button is pressed: the view is built from facts alone and calls nothing.
//  D2 Each group lists its files with the plan's own sizes, their sum, the host, and says "mobile data" before starting when the phone is not on Wi-Fi.
//  D3 A group is ready only when every file is present; a group with a row that is not ready offers the download (a newer model appears as an offer, not a silent fetch).
//  D4 Start on Wi-Fi records the agreement and starts that group alone; on mobile data it starts nothing until the mobile press, which records the mobile agreement.
//  D5 A second press while a group is downloading starts nothing again.
//  D6 The sizes are written as the native side writes them: a plan with no files is "nothing to download".
import type { Storage } from './app.contract.d.ts';
import { consentAct, consentView } from './consent.ts';
import { nativeLater } from './api.ts';
import { kickSearch } from './search-kick.ts';
import { noVoice, type PlanRow, type VoiceFacts } from './onboarding.ts';

export type Group = {
  id: string; title: string; why: string; lines: string[]; total: string; host: string; ready: boolean; busy: boolean; status: string; onWiFi: boolean; mobileLine: string;
  startLabel: string; mobileLabel: string; hostLine: string;
};
export type Downloads = { groups: Group[]; rev: number; voiceReady: boolean; hearingReady: boolean };

/** The size words, as the native side writes them (SetupCatalog.sizeLabel). */
export function sizeLabel(bytes: number): string {
  if (!(bytes > 0)) return 'Built in';
  if (bytes >= 1_000_000_000) return `${(bytes / 1e9).toFixed(1)} GB`;
  return `${Math.max(1, Math.round(bytes / 1e6))} MB`;
}
const TITLES: Record<string, [string, string]> = {
  voice: ["The radio's voice", "What the radio speaks with. Until it is on this phone the radio uses the iPhone's built-in voice."],
  'voice-rest': ['Hearing you', 'What voice needs to hear and understand you. Voice turns on only once these are on this phone.'],
  search: ['Search on this phone', "Finds your stories, documents and mail on the phone without sending the question anywhere."],
};
const ORDER = ['voice', 'voice-rest', 'search'];
/** Native stage names each group runs through voice setup (search is not a voice stage: its file comes through the search pass). */
export const GROUP_STAGES: Record<string, string[]> = { voice: ['voice'], 'voice-rest': ['transcriber', 'bounds', 'responder'] };

/** D1 to D3, D6: the view, from the facts and whether the search model is on the phone. Calls nothing. */
export function downloadsView(f: VoiceFacts, searchPresent: boolean | null, rev: number): Downloads {
  const plan: PlanRow[] = f.plan.map((r) => (r.group === 'search' && searchPresent !== null ? { ...r, ready: searchPresent } : r));
  const groups: Group[] = [];
  for (const id of ORDER) {
    const rows = plan.filter((r) => r.group === id);
    if (rows.length === 0) continue;
    const need = rows.filter((r) => !r.ready);
    const total = need.reduce((a, r) => a + r.bytes, 0);
    const busy = id !== 'search' && f.setupStarted && !f.setupFinished && f.rows.some((r) => GROUP_STAGES[id].includes(r.stage) && (f.runStages.length === 0 || f.runStages.includes(r.stage)) && (r.state === 'downloading' || r.state === 'waiting' || r.state === 'preparing'));
    const part = f.rows.filter((r) => GROUP_STAGES[id]?.includes(r.stage) && r.state === 'downloading').map((r) => r.progress);
    const status = need.length === 0 ? 'On this phone.' : busy ? (part.length ? `Downloading, ${Math.round((part.reduce((a, b) => a + b, 0) / part.length) * 100)}%.` : 'Getting ready.') : '';
    const host = rows[0].host;
    const [title, why] = TITLES[id] ?? [id, ''];
    groups.push({
      id, title, why, ready: need.length === 0, busy, status, onWiFi: f.onWiFi, host,
      lines: rows.map((r) => `${r.name}: ${r.model}, ${r.sizeLabel}${r.ready ? ', on this phone' : ''}`),
      total: need.length === 0 ? 'Nothing to download.' : sizeLabel(total),
      hostLine: `${need.length === 0 ? '' : `${sizeLabel(total)} from ${host}. ${host} sees this phone's address and which file is asked for. `}These are data files, not apps.`.trim(),
      mobileLine: f.onWiFi ? '' : `You are on mobile data. This will use about ${sizeLabel(total)} of your plan.`,
      startLabel: f.onWiFi ? 'Download on Wi-Fi' : 'Wait for Wi-Fi', mobileLabel: 'Download now',
    });
  }
  return { groups, rev, voiceReady: f.voiceReady, hearingReady: f.hearingReady };
}

const present = async (): Promise<boolean | null> => {
  const later = nativeLater();
  if (!later) return null;
  try { const r = (await later({ op: 'embedState' })) as { present?: unknown } | null; return r && typeof r === 'object' ? r.present === true : null; } catch { return null; }
};
/** Source: what voice needs, group by group. */
export async function downloadsPage(facts: VoiceFacts | null | undefined, storage: Storage | undefined): Promise<Downloads> {
  const c = await consentView(storage);
  return downloadsView({ ...noVoice(), ...(facts ?? {}) }, await present(), c.rev);
}

/** The press. `op` start | startMobile; `group` one of the group ids. Returns the view after it. Search is started by the search pass once agreed: `kickSearch` asks it to run now. */
export async function downloadsAct(op: string, group: string, facts: VoiceFacts | null | undefined, storage: Storage | undefined): Promise<Downloads> {
  const f = { ...noVoice(), ...(facts ?? {}) };
  const later = nativeLater();
  const known = ORDER.includes(group);
  const g = known ? downloadsView(f, null, 0).groups.find((x) => x.id === group) : undefined;
  const mobile = op === 'startMobile';
  if (g && !g.ready && !g.busy && (op === 'start' ? f.onWiFi : mobile)) {                          // D4, D5: on mobile data only the mobile press starts anything
    await consentAct(mobile ? 'agreeModelsMobile' : 'agreeModels', storage);
    if (group === 'search') kickSearch();
    else if (later) { try { await later({ op: 'obSetupBegin', stages: GROUP_STAGES[group] }); } catch { /* the facts say it did not start */ } }
  }
  return downloadsPage(f, storage);
}
