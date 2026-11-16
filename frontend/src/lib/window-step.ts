// Where an app body is, for the crash guard (lib/crash-guard.js). A window's
// body calls the function windowStep() gives it at component setup; the step
// is written synchronously to the pending record, so when the page dies a held
// window can say what it was doing. Apps import this through app-sdk, never
// the desktop. Outside a window body (or with no guard) the call does nothing.
import { getContext, setContext } from 'svelte';

const KEY = Symbol('choir.window-step');
type Sink = (windowId: string, text: string) => void;
let sink: Sink | null = null;

/** Desktop installs the guard's writer; returns the remover. */
export function setWindowStepSink(next: Sink | null): () => void {
  sink = next;
  return () => { if (sink === next) sink = null; };
}

/** AppHost calls this once per window. */
export function provideWindowStep(get: () => string): void {
  setContext(KEY, get);
}

/** Call during component setup. The returned function records the step, never throws. */
export function windowStep(): (text: string) => void {
  let get: (() => string) | undefined;
  try { get = getContext(KEY) as (() => string) | undefined; } catch { get = undefined; }
  return (text: string) => {
    try {
      const id = typeof get === 'function' ? get() : '';
      if (id && sink) sink(id, String(text));
    } catch {
      // A breadcrumb never breaks the app.
    }
  };
}
