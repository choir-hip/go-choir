// Is there a native package on this host? One fact, decided from the native handle (not from what the native open-pages set holds, which is empty at every iPhone cold launch until the
// first native call answers). The deck's page set, the shared reader and the shared notebook use it: they are the Android path and must do no work on iOS.
// "native" is decided at once by a usable handle that answers a probe, and is sticky. "none" is decided only after a wait for a handle to appear (a module may load late) and a probe that
// fails, throws or never answers. Before the decision the source has not answered, and the contracts read that as native (only "none" switches to the shared twins).
import { nativeLater, nativeHasAnswered } from './api.ts';

let decided: 'native' | 'none' | null = null;
const wasNative = (): boolean => decided === 'native';       // read afresh (a signal may have set it while a probe was out)
export function resetHost(): void { decided = null; }

const probe = async (probeMs: number): Promise<boolean> => {
  const later = nativeLater();
  if (!later) return false;
  try {
    const r = await Promise.race([later({ op: 'keystate' }), new Promise<'timeout'>((res) => setTimeout(() => res('timeout'), probeMs))]);
    return r !== 'timeout' && !!r && typeof (r as { state?: unknown }).state === 'string';
  } catch { return false; }
};

/** `waitMs`: how long to wait (real time) for a handle to appear and answer before deciding "none". `nativeSeen`: the Swift package itself has answered (the native open-pages set has a top page), or
 * an app-module op has answered (api.ts): native exists, for good. It is the recovery from a bad window ("none" is re-asked by the contract with this flag) and what keeps native from being downgraded.
 * Each answer probes for itself: its timers and native calls are its own, and only the DECISION (`decided`) is shared. A probe is never shared as a promise: Exact drops the timers of an answer that is let
 * go, so a shared probe whose answer was let go while it slept would stay pending, and every answer awaiting it would fail "pending on nothing" (docs/problems/shared-settle-promise-fails-every-request-2026-11-25.md). */
export async function hostMode(waitMs = 1500, probeMs = 800, nativeSeen = false): Promise<'native' | 'none'> {
  if (nativeSeen || nativeHasAnswered()) decided = 'native';
  if (decided === 'native') return 'native';
  if (decided === 'none') waitMs = Math.max(probeMs, 400);   // already waited once: a later ask re-probes with a short budget of its own (0 lost the race to any real bridge round trip)
  let stop = false;
  const loop = (async (): Promise<'native' | 'none'> => {
    while (!stop) {
      if (await probe(probeMs)) return 'native';
      await new Promise((r) => setTimeout(r, 25));
    }
    return 'none';
  })();
  const timer = new Promise<'none'>((res) => setTimeout(() => res('none'), waitMs));     // real time, not rounds: a hanging probe cannot stretch it
  const r = await Promise.race([loop, timer]);
  stop = true;
  decided = wasNative() || r === 'native' || nativeHasAnswered() ? 'native' : 'none';     // never write "none" over a "native" a signal set while this probe was out
  return decided;
}
