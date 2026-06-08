export function browserOpenableSourceURL(value: unknown): string {
  const raw = String(value || '').trim();
  if (!raw) return '';
  try {
    const url = new URL(raw);
    if (url.protocol === 'http:' || url.protocol === 'https:') return raw;
  } catch (err) {
    return '';
  }
  return '';
}
