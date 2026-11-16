// Where an app body is, for the crash guard (lib/crash-guard.js). A window's
// body calls the function windowStep() gives it at component setup; the step
// is written synchronously to the pending record, so when the page dies a held
// window can say what it was doing. Apps import this through app-sdk, never
// the desktop. Outside a window body (or with no guard) the call does nothing.
//
// A step is foreground by default: it says the body is doing mount-time work,
// and the window is not proven until a quiet period after the latest one (and,
// until a step with done:true, up to a ceiling). {background:true} is for work
// that may run long and is not what opening is (a text-search pass).
import { getContext, setContext } from 'svelte';

const KEY = Symbol('choir.window-step');
export type WindowStepOptions = { background?: boolean; done?: boolean };
type Sink = (windowId: string, text: string, options?: WindowStepOptions) => void;
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
export function windowStep(): (text: string, options?: WindowStepOptions) => void {
  let get: (() => string) | undefined;
  try { get = getContext(KEY) as (() => string) | undefined; } catch { get = undefined; }
  return (text: string, options: WindowStepOptions = {}) => {
    try {
      const id = typeof get === 'function' ? get() : '';
      if (id && sink) sink(id, String(text), { background: Boolean(options?.background), done: Boolean(options?.done) });
    } catch {
      // A breadcrumb never breaks the app.
    }
  };
}
