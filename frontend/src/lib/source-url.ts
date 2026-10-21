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

// webLensAddress turns what a person typed into an address Web Lens may link
// to, or '' when it must refuse. Only http and https addresses survive: a
// javascript:, data:, file:, blob: or any other scheme never becomes a link,
// and an address with user info (https://example.com@elsewhere.test) is
// refused because the part before the @ is not the host. A bare host such as
// example.com gets https://. The string returned is the string checked, so
// what the page links to is what was validated.
export function webLensAddress(value: unknown): string {
  const raw = String(value || '').trim();
  if (!raw || /\s/.test(raw)) return '';
  const hasScheme = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(raw);
  // A scheme without // (javascript:, data:, mailto:, blob:) is never a host.
  if (!hasScheme && /^[a-zA-Z][a-zA-Z0-9+.-]*:(?!\d+(?:[/?#]|$))/.test(raw)) return '';
  const candidate = hasScheme ? raw : raw.includes('.') ? `https://${raw}` : '';
  if (!candidate) return '';
  try {
    const url = new URL(candidate);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return '';
    if (url.username || url.password) return '';
    if (!url.hostname) return '';
  } catch (err) {
    return '';
  }
  return candidate;
}

// storedSnapshotApplies says whether a stored reader snapshot may still be
// shown and imported for the current address. The snapshot is the immutable
// text of the address it was taken from (snapshotURL); once the address
// changes it belongs to nothing the person is looking at, so it is hidden
// rather than relabelled with the new address.
export function storedSnapshotApplies(snapshotURL: string, target: string): boolean {
  return snapshotURL === target;
}
