// Effects: every registration in the kernel returns a Disposer. The scope
// collects them and reverses them LIFO, awaiting each.
//
// Failure modes this file answers (written before the code):
//  F1 a disposer throws or rejects: the rest still run; the errors are returned.
//  F2 a disposer is called twice: the second call is a no-op.
//  F3 a registration after disposal: refused, never silently leaked.

export type Disposer = () => void | Promise<void>;

export interface DisposeReport {
  errors: unknown[];
}

/** Wrap a disposer so a second call does nothing (F2). */
export function once(fn: Disposer): Disposer {
  let done = false;
  return () => {
    if (done) return;
    done = true;
    return fn();
  };
}

/** Run disposers last-in first-out, awaiting each; collect failures (F1). */
export async function runLifo(effects: Disposer[]): Promise<DisposeReport> {
  const errors: unknown[] = [];
  while (effects.length > 0) {
    const fn = effects.pop() as Disposer;
    try {
      await fn();
    } catch (err) {
      errors.push(err);
    }
  }
  return { errors };
}
