// The shell's one kernel (lib/kernel, slice MA2.2) and the scope each package
// runs in. Services that take registrations (commands, themes, links, ...) put
// every registration in the owner's scope, so uninstalling a package disposes
// them all and the leak differential (snapshot() equal to the baseline) holds.
import { createKernel, type Scope } from './kernel';

export const shellKernel = createKernel();

const scopes = new Map<string, Scope>();

/** The scope of package `name`, created on first use. */
export function packageScope(name: string): Scope {
  const existing = scopes.get(name);
  if (existing && !existing.disposed) return existing;
  const scope = shellKernel.scope(`package:${name}`);
  scopes.set(name, scope);
  return scope;
}

/** Dispose a package's scope and every registration in it (uninstall, or an update replacing its set). */
export async function disposePackageScope(name: string): Promise<void> {
  const scope = scopes.get(name);
  scopes.delete(name);
  if (scope) await scope.dispose();
}

export function hasPackageScope(name: string): boolean {
  const scope = scopes.get(name);
  return Boolean(scope && !scope.disposed);
}
