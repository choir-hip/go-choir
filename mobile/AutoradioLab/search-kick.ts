// The one hook between the downloads page and the search pass (kept apart: downloads.ts and retrieval-live.ts sit in the api.ts / app.ts import cycle, and a hook set at module level must not meet a module not yet initialised).
let kick: (() => void) | undefined;
export const onKickSearch = (fn: () => void): void => { kick = fn; };
export const kickSearch = (): void => { kick?.(); };
