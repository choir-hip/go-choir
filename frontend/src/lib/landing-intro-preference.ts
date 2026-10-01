/*
  Landing intro preference.

  Presentation-only, deliberately quarantined from durable computer state.

  The `computer-live-sync` contract forbids localStorage/sessionStorage in
  App.svelte, Desktop.svelte, TextureEditor.svelte and the app surfaces,
  because durable client state must have exactly one authority: the
  computer. A "have I shown the orientation film" flag is not computer state
  — it is a rendering hint about this browser profile — so it lives here,
  behind its own module, where it is auditable and can never be mistaken for
  an authoritative value.

  Storage is session-scoped by design: a returning visitor within the same
  tab session goes straight to their desktop. A fresh session gets the deck.
  `?intro=0` forces it off and `?intro=1` forces it on, which is how the
  E2E proof captures both states deterministically.

  Note that `reduced` no longer suppresses the deck. Nothing in it autoplays
  — it is driven entirely by the visitor's own scrolling — so a reduced-motion
  visitor still gets the argument, standing still, by scrolling. Suppressing
  it would have been a different decision than the one the previous, timed
  film forced on us, and it would have taken the narrative away from the one
  audience most likely to be reading rather than watching.
*/

const KEY = 'choir.intro.seen.v1';

export function shouldPlayLandingIntro(search: string, reduced: boolean): boolean {
  void reduced;
  return introOverride(search) ?? !hasSeenIntro();
}

export function introOverride(search: string): boolean | null {
  try {
    const params = new URLSearchParams(search || '');
    if (!params.has('intro')) return null;
    const value = String(params.get('intro') || '').trim().toLowerCase();
    if (value === '0' || value === 'false' || value === 'off') return false;
    if (value === '1' || value === 'true' || value === 'on') return true;
    return null;
  } catch (_err) {
    return null;
  }
}

export function hasSeenIntro(): boolean {
  try {
    return window.sessionStorage?.getItem(KEY) === '1';
  } catch (_err) {
    return false;
  }
}

export function markIntroSeen(): void {
  try {
    window.sessionStorage?.setItem(KEY, '1');
  } catch (_err) {
    // A blocked storage quota must never break the desktop. The only cost of
    // failure here is that the film plays one more time.
  }
}
