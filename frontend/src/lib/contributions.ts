// The manifest `contributes` object as the declarative front door of the
// kernel (docs/design/modular-architecture-2026-11-04.md section 1). Each key
// has one owning service, which registers a point here; at install the loader
// hands the key's entries to that point inside the package's scope, and at
// uninstall (or an update that replaces the set) the scope's disposal reverses
// every effect. A point cannot tell a declarative entry from one the package
// registers in code, because both are effects in the same scope.
//
// Failure modes:
//  C1 a point registers after the package was applied: it is replayed, so
//     import order never decides whether a contribution is live;
//  C2 an entry the point refuses (bad id, locked key): that entry is skipped
//     and reported to the console, the rest of the package still applies;
//  C3 a package applied twice with the same fingerprint: nothing happens, so a
//     refresh that changes nothing never churns registrations;
//  C4 a package replaced by a different release: the old scope is disposed
//     before the new entries apply, never after.
import type { Scope } from './kernel';
import { disposePackageScope, hasPackageScope, packageScope } from './shell-kernel';

export type ContributionPoint = (pkg: string, entries: unknown[], scope: Scope) => void;

const points = new Map<string, ContributionPoint>();
const applied = new Map<string, { fingerprint: string; contributes: Record<string, unknown> }>();

function applyKeys(pkg: string, contributes: Record<string, unknown>, only?: string): void {
  const scope = packageScope(pkg);
  for (const [key, entries] of Object.entries(contributes)) {
    if (only && key !== only) continue;
    const point = points.get(key);
    if (!point || !Array.isArray(entries) || entries.length === 0) continue;
    try {
      point(pkg, entries, scope);
    } catch (err) {
      console.warn(`[contributes] ${pkg}: ${key} not applied:`, err);
    }
  }
}

/** A service registers the point that owns a contributes key. Replays packages already applied (C1). */
export function registerContributionPoint(key: string, point: ContributionPoint): void {
  points.set(key, point);
  for (const [pkg, record] of applied) {
    if (record.contributes[key] !== undefined) applyKeys(pkg, record.contributes, key);
  }
}

/**
 * Set what a package contributes (null removes it). `fingerprint` names the
 * release (an install id and digest, or the in-tree name), so an unchanged set
 * is not re-applied (C3).
 */
export function setPackageContributions(pkg: string, contributes: unknown, fingerprint: string): Promise<void> {
  // `core` is the shell's own identity (the commands service gives it privileges a package never has); no package carries it.
  if (pkg === 'core') return Promise.reject(new Error('"core" is the shell\'s name, not a package\'s'));
  // One change at a time, in call order: a dispose that is still awaiting never interleaves with the next apply (C4).
  const next = chain.then(() => change(pkg, contributes, fingerprint));
  chain = next.catch(() => undefined);
  return next;
}

let chain: Promise<void> = Promise.resolve();

async function change(pkg: string, contributes: unknown, fingerprint: string): Promise<void> {
  const body = contributes && typeof contributes === 'object' && !Array.isArray(contributes) ? (contributes as Record<string, unknown>) : null;
  const previous = applied.get(pkg);
  if (previous && body && previous.fingerprint === fingerprint) return;
  if (previous || hasPackageScope(pkg)) {
    applied.delete(pkg);
    await disposePackageScope(pkg);
  }
  if (!body) return;
  applied.set(pkg, { fingerprint, contributes: body });
  applyKeys(pkg, body);
}

/** Packages whose contributions are live. */
export function contributingPackages(): string[] {
  return [...applied.keys()].sort();
}
