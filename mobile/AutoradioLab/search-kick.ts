// The one hook between the Voice and models page and the search pass (kept apart: models.ts and retrieval-live.ts sit in the api.ts / app.ts import cycle, and a hook set at module level must not meet a module not yet initialised).
// The press hands its storage over: the search pass reads the agreement from storage, and before any retrieval call it has none (a press that recorded but fetched nothing).
import type { Storage } from './app.contract.d.ts';
let kick: ((storage: Storage | undefined) => void) | undefined;
export const onKickSearch = (fn: (storage: Storage | undefined) => void): void => { kick = fn; };
export const kickSearch = (storage?: Storage): void => { kick?.(storage); };

/** The Test button of the search model (models.ts): runs a fixed query over the phone's own stories and documents (retrieval-live.ts registers it) and says what it found. */
export type SearchTrial = (query: string) => Promise<string>;
let trial: SearchTrial | undefined;
/** Registers the trial and returns the one it replaced (a test puts it back). */
export const onTestSearch = (fn: SearchTrial): SearchTrial | undefined => { const was = trial; trial = fn; return was; };
export const testSearch = async (query: string): Promise<string | undefined> => (trial ? trial(query) : undefined);
