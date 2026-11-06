// isSingleHttpUrl mirrors the conductor's content rule
// (content.ClassifyPromptBarContentIntent): text that is one absolute http or
// https URL opens its display app. With a focus in view the chip defaults to
// the conductor for it, so a pasted link still reaches its app unless the
// owner taps to keep the focus (review 284 item 1).
export function isSingleHttpUrl(text: string): boolean {
  const fields = String(text || '').trim().split(/\s+/).filter(Boolean);
  if (fields.length !== 1) return false;
  try {
    const parsed = new URL(fields[0]);
    return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && parsed.host !== '';
  } catch {
    return false;
  }
}
