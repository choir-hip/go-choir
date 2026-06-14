export async function fetchPulseSummary() {
  const res = await fetch('/api/pulse/summary', {
    method: 'GET',
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Pulse summary failed (${res.status})`);
  }
  return res.json();
}
