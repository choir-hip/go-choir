// The unseen-changelog pop-up (lane E; autopaper contracts 5.2). While the
// owner is signed in, the shell asks the computer for the changelog head and
// the seen cursor; when major events lie past the cursor it posts one
// notification per new head. Opening the changelog app moves the cursor (on
// the owner's computer, not in this browser), which stops the pop-up.
//
// Failures: the read fails or the owner is signed out: nothing is posted and
// the next tick tries again; the same head is never announced twice in one
// page session, so a slow reader is not nagged on every tick.
import { fetchWithRenewal } from './app-sdk';

export const CHANGELOG_POLL_MS = 60_000;

type Announce = (message: string) => void;

export function startChangelogWatch(isSignedIn: () => boolean, announce: Announce): () => void {
  let announcedHead = 0;
  let stopped = false;

  async function tick() {
    if (stopped || !isSignedIn()) return;
    try {
      const res = await fetchWithRenewal('/api/changelog?limit=1', { method: 'GET', headers: { Accept: 'application/json' } });
      if (!res.ok) return;
      const page = await res.json();
      const unseen = Number(page?.unseen_major || 0);
      const head = Number(page?.head || 0);
      if (unseen > 0 && head > announcedHead) {
        announcedHead = head;
        announce(`${unseen}${page?.unseen_more ? '+' : ''} new in the Changelog`);
      }
    } catch {
      // The next tick retries.
    }
  }

  const first = setTimeout(tick, 3000);
  const timer = setInterval(tick, CHANGELOG_POLL_MS);
  return () => {
    stopped = true;
    clearTimeout(first);
    clearInterval(timer);
  };
}
