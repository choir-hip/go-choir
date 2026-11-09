// Failover is silent with one visible record (slice L4;
// docs/design/modular-architecture-2026-11-04.md decision 4 and section 9,
// "a provider throws on a hot path"). The kernel keeps the reason a provider
// was set aside in its snapshot (Settings "Who provides this?" and the
// recovery surface read it); this is the one toast, said once per trip: the
// kernel's trip and reportFailure return true only on the call that set the
// provider aside.
import { toast } from '../shell-services';

type FailoverListener = (slot: string, id: string, reason: string) => void;
const listeners = new Set<FailoverListener>();
/**
 * Hear each trip (the kernel's trip and reportFailure return true once per
 * provider set aside). installed-surfaces.ts uses it to bring the in-tree
 * package back when an installed one that displaced it fails.
 */
export function onFailover(fn: FailoverListener): () => void {
  listeners.add(fn);
  return () => { listeners.delete(fn); };
}

export function announceFailover(slot: string, id: string, reason: string): void {
  for (const fn of listeners) {
    try { fn(slot, id, reason); } catch (err) { console.warn('[shell] failover listener failed:', err); }
  }
  console.warn(`[shell] ${id} was set aside from ${slot}: ${reason}`);
  toast(`${id} stopped drawing ${slot} and was set aside (${reason}). Settings, "Who provides this?", shows it.`, { kind: 'error' });
}
