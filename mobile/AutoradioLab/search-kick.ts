// The one hook between the downloads page and the search pass (kept apart: downloads.ts and retrieval-live.ts sit in the api.ts / app.ts import cycle, and a hook set at module level must not meet a module not yet initialised).
// The press hands its storage over: the search pass reads the agreement from storage, and before any retrieval call it has none (a press that recorded but fetched nothing).
import type { Storage } from './app.contract.d.ts';
let kick: ((storage: Storage | undefined) => void) | undefined;
export const onKickSearch = (fn: (storage: Storage | undefined) => void): void => { kick = fn; };
export const kickSearch = (storage?: Storage): void => { kick?.(storage); };
