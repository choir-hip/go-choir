// Content items: owner-scoped records of something read or captured
// (POST /api/content/items). Moved out of texture.js so an app that is not
// Texture (Web Lens) can create one through the SDK (createContentItem).
import { fetchWithRenewal } from './auth.js';

export async function createContentItem(payload = {}) {
  const res = await fetchWithRenewal('/api/content/items', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Create content item failed (${res.status})`);
  }

  return res.json();
}
