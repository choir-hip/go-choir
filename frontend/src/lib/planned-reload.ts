// A planned reload (the operating contract's planned-restart rule, at the page
// layer): the app's own reloads (restart to update, account switch, the app
// reload button) call this just before reloading so the guard
// (lib/crash-guard.js) treats the page's markers as clean at the next load.
// A reload the user makes on a hung page does not call it and still holds.
let sink: (() => void) | null = null;

/** Desktop installs the guard's writer; returns the remover. */
export function setPlannedReloadSink(next: (() => void) | null): () => void {
  sink = next;
  return () => { if (sink === next) sink = null; };
}

/** Synchronously, just before window.location.reload(). Never throws. */
export function notePlannedReload(): void {
  try { sink?.(); } catch { /* the reload goes on */ }
}
