// Event and timer helpers for packages (SDK 1.1.0, slice U1). A package does
// not call window.addEventListener, document.addEventListener or setInterval
// itself (the package boundary refuses them): it asks the SDK, which returns a
// disposer, so what a package listens to and schedules is visible to the shell
// and reversed with the package. First-party and installed packages use the
// same helpers.
//
// Failure modes:
//  E1 a disposer called twice: the second call does nothing;
//  E2 no window or document (a server render or a test): the helper returns a
//     disposer that does nothing, and never throws;
//  E3 an interval below 50ms or a non-function handler: refused, so a package
//     cannot spin the main thread.

export type Disposer = () => void;

function listen(target: EventTarget | undefined, type: string, fn: EventListenerOrEventListenerObject, options?: boolean | AddEventListenerOptions): Disposer {
  if (!target || typeof fn !== 'function' && !(fn && typeof (fn as EventListenerObject).handleEvent === 'function')) return () => {};
  target.addEventListener(type, fn, options);
  let done = false;
  return () => {
    if (done) return;
    done = true;
    target.removeEventListener(type, fn, options);
  };
}

/** Listen on the window. Returns the disposer. */
export function onWindowEvent(type: string, fn: EventListenerOrEventListenerObject, options?: boolean | AddEventListenerOptions): Disposer {
  return listen(typeof window === 'undefined' ? undefined : window, type, fn, options);
}

/** Listen on the document. Returns the disposer. */
export function onDocumentEvent(type: string, fn: EventListenerOrEventListenerObject, options?: boolean | AddEventListenerOptions): Disposer {
  return listen(typeof document === 'undefined' ? undefined : document, type, fn, options);
}

/** Run `fn` every `ms` milliseconds (at least 50). Returns the disposer. */
export function repeatEvery(fn: () => void, ms: number): Disposer {
  if (typeof fn !== 'function' || !Number.isFinite(ms) || ms < 50) throw new Error('repeatEvery needs a function and an interval of at least 50ms');
  const timer = setInterval(fn, ms);
  let done = false;
  return () => {
    if (done) return;
    done = true;
    clearInterval(timer);
  };
}
