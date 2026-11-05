// Which package is calling (review r1 B5). The SDK's module-level functions
// have no caller, so identity comes from the component tree: AppHost provides
// the window's app id for everything rendered inside it, and an SDK function
// that needs the caller reads it during component setup. Outside a package
// component there is none, and the function refuses.
import { getContext, setContext } from 'svelte';

const KEY = Symbol.for('choir.package-identity');

/** AppHost calls this once per window; `get` reads the window's app id. */
export function providePackageIdentity(get: () => string): void {
  setContext(KEY, get);
}

/** The package whose component is initializing, or '' when called from anywhere else. */
export function currentPackageName(): string {
  try {
    const get = getContext(KEY) as (() => string) | undefined;
    return typeof get === 'function' ? String(get() || '') : '';
  } catch {
    return '';
  }
}
