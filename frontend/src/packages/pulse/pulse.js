import { fetchWithRenewal } from '../../lib/app-sdk';

export async function fetchPulseSummary() {
  const res = await fetchWithRenewal('/api/pulse/summary', {
    method: 'GET',
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Pulse summary failed (${res.status})`);
  }
  return res.json();
}

/**
 * The owner's reading of the host's capacity (live memory, disk, computers and
 * the sign-up room). Only the platform operator, signed in with a passkey, may
 * read it; anyone else gets null and the page shows only whether sign-up is
 * open.
 */
export async function fetchOperatorCapacity() {
  try {
    const res = await fetchWithRenewal('/api/platform/operator/capacity', {
      method: 'GET',
      headers: { Accept: 'application/json' },
    });
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}
