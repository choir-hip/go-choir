import { fetchWithRenewal, AuthRequiredError } from './auth.js';
import { currentDeviceId } from './live-events.js';

export async function fetchThemePreference() {
  const res = await fetchWithRenewal('/api/preferences/theme', { method: 'GET' });
  if (!res.ok) {
    if (res.status === 401) throw new AuthRequiredError();
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Theme preference fetch failed (${res.status})`);
  }
  const body = await res.json();
  return body.theme || {};
}

export async function saveThemePreference(theme) {
  const res = await fetchWithRenewal('/api/preferences/theme', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'X-Choir-Device': currentDeviceId(),
    },
    body: JSON.stringify({ theme: theme || {} }),
  });
  if (!res.ok) {
    if (res.status === 401) throw new AuthRequiredError();
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Theme preference save failed (${res.status})`);
  }
  const body = await res.json();
  return body.theme || {};
}
