// The AI-written disclosure at the top of an Autopaper story (owner notes
// §27 item 14; EU AI Act Art. 50(4) applies to AI-written public-interest
// text published without human editorial control). Autopaper stories are
// written by the paper's agent desks and published without a human editor,
// so every one carries the line. A person's own published Texture does not:
// the label follows the Autopaper story context, never the text.

export const AI_DISCLOSURE_TEXT =
  'Written by AI. Autopaper’s agent desks wrote this story from the sources it links; no human editor reviewed it before publication.';

export function aiDisclosureHTML(appContext: Record<string, unknown> | null | undefined): string {
  if (appContext?.autopaperStory !== true) return '';
  return `<p class="texture-ai-disclosure" data-texture-ai-disclosure>${AI_DISCLOSURE_TEXT}</p>`;
}
