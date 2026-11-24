// The model offers, as pure data: what the native side reads from the model list (ModelList.swift, ModelOffer.jobs) for this phone, and what the person has picked. This file writes no model's name or size: an option's name and
// size are the list's own, and which option is "recommended for this phone" is the list's answer for this phone (the first entry that fits it), so a newer model or a different phone is a different offer with no change here.
// Each job (hearing you, knowing when you speak, the voice, the answering model, knowing your voice, search) has its options; the person's pick for a job is kept (consent.ts `picks`); no pick is the recommended one.
export type Option = { id: string; model: string; bytes: number; sizeLabel: string; recommended: boolean; ready: boolean; note?: string; credit?: string; withheld?: boolean; removable?: boolean };
export type OfferJob = { job: string; group: string; host: string; options: Option[] };

/** The size words, as the native side writes them (SetupCatalog.sizeLabel). */
export function sizeLabel(bytes: number): string {
  if (!(bytes > 0)) return 'Built in';
  if (bytes >= 1_000_000_000) return `${(bytes / 1e9).toFixed(1)} GB`;
  return `${Math.max(1, Math.round(bytes / 1e6))} MB`;
}

/** The option the person has chosen for a job: their pick when it is still on offer, else the recommended one, else none (the job stays on what is built in). */
export function chosen(j: OfferJob, picks: Record<string, string>): Option | undefined {
  return j.options.find((o) => o.id === picks[j.job]) ?? j.options.find((o) => o.recommended);
}
/** Does this job still need a download for what is chosen? None chosen: nothing to fetch. */
export const needs = (j: OfferJob, picks: Record<string, string>): Option | undefined => { const c = chosen(j, picks); return c && !c.ready ? c : undefined; };

/** The jobs voice setup runs as stages (the voice-print and search files are fetched beside it, not as stages). */
export const STAGE_JOBS = ['voice', 'transcriber', 'bounds', 'decision', 'responder'];
/** job -> chosen model id, for the stage jobs of these groups (what is handed to the native setup). */
export function picksFor(offers: OfferJob[], groups: string[], picks: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const j of offers) if (groups.includes(j.group) && STAGE_JOBS.includes(j.job)) { const c = chosen(j, picks); if (c) out[j.job] = c.id; }
  return out;
}

/** The offers as the FIRST RUN sees them: a withheld option (a library model with no local-only load and no manifest yet) is not offered there at all: Settings > Downloads is where it is fetched first. A job with nothing left is gone. */
export const forFirstRun = (offers: OfferJob[]): OfferJob[] => offers.map((j) => ({ ...j, options: j.options.filter((o) => !o.withheld) })).filter((j) => j.options.length > 0);

/** What the first-run press fetches, in bytes: the chosen option of every job of the two voice groups, each file once (the decision and the responder may be one file), the voice-print model among them. The welcome and the setup page show this one number. */
export function setupBytes(offers: OfferJob[], picks: Record<string, string>): number {
  const seen = new Set<string>(); let total = 0;
  for (const j of forFirstRun(offers)) if (j.group !== 'search') { const o = needs(j, picks); if (o && !seen.has(o.id)) { seen.add(o.id); total += o.bytes; } }
  return total;
}
/** The setup page's total line, in the words the native side writes (SetupCatalog.totalLabel), for this number. */
export function totalWords(bytes: number, onWiFi: boolean): string {
  if (!(bytes > 0)) return 'Nothing to download. The radio will use what is already on this phone.';
  const size = bytes >= 1_000_000_000 ? `About ${(bytes / 1e9).toFixed(1)} GB.` : `About ${Math.max(1, Math.round(bytes / 1e7)) * 10} MB.`;
  return onWiFi ? `${size} On Wi-Fi this takes a few minutes.` : `${size} You are on mobile data, so I will ask before I start.`;
}
