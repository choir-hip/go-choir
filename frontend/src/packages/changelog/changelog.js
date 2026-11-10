import { fetchWithRenewal } from '../../lib/app-sdk';

// The changelog read (docs/design/autopaper-step0-contracts-2026-11-10.md, 5.2).
// The owner is the signed-in user; the seen cursor lives on the owner's computer.

export async function fetchChangelog({ minor = false, before = 0, limit = 50 } = {}) {
  const q = new URLSearchParams({ limit: String(limit) });
  if (minor) q.set('minor', '1');
  if (before > 0) q.set('before', String(before));
  const res = await fetchWithRenewal(`/api/changelog?${q}`, { method: 'GET', headers: { Accept: 'application/json' } });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Changelog failed (${res.status})`);
  }
  return res.json();
}

export async function markChangelogSeen(seq) {
  const res = await fetchWithRenewal('/api/changelog/seen', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify({ seq }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Could not mark the changelog seen (${res.status})`);
  }
  return res.json();
}

// What an event links to: the document and revision it names on this
// computer, else its published route. Anything else is plain text.
export function eventTarget(changelog) {
  const subject = changelog?.subject || {};
  const ref = changelog?.tape_ref || {};
  const docId = subject.doc_id || ref.doc_id || '';
  if (docId) return { kind: 'document', docId, revisionId: ref.revision_id || '' };
  const route = subject.route_path || '';
  if (route.startsWith('/')) return { kind: 'route', route };
  return null;
}

export function formatWhen(iso) {
  const t = Date.parse(iso || '');
  if (Number.isNaN(t)) return '';
  return new Date(t).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

const KIND_LABELS = {
  parent_used: 'Your work was used',
  oped_verdict: 'Op-ed verdict',
  update_started: 'Update started',
  update_progress: 'Update progress',
  update_finished: 'Update finished',
  update_interrupted: 'Update interrupted',
  revision_committed: 'Revision saved',
  wake_decision: 'Desk wake decision',
  oped_arrival: 'Op-ed arrived',
  gate_unavailable: 'Gate unavailable',
};
export const kindLabel = (kind) => KIND_LABELS[kind] || String(kind || 'Event').replace(/_/g, ' ');
