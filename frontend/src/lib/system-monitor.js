import { fetchWithRenewal } from './auth.js';

export async function fetchSystemStatus() {
  const res = await fetchWithRenewal('/api/system/status', {
    method: 'GET',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `System status failed (${res.status})`);
  }
  return res.json();
}

export async function wakeCurrentComputer() {
  const res = await fetchWithRenewal('/api/system/recovery', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action: 'wake_current_computer' }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Computer wake failed (${res.status})`);
  }
  return res.json();
}
