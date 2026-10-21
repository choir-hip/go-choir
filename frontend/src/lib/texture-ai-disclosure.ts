// The AI-written disclosure at the bottom of an Autopaper story, before the
// related block (owner notes §27 item 14, placement §28; EU AI Act Art. 50(4) applies to AI-written public-interest
// text published without human editorial control). Autopaper stories are
// written by the paper's agent desks and published without a human editor,
// so every one carries the line. A person's own published Texture does not:
// the label follows the Autopaper story context, never the text.
//
// A story is known by the server's record too: a resolved Autopaper story
// carries publication.kind "story", so a story opened at its public Texture
// route (an old link, an email, a new tab) shows the line without the
// Autopaper window that would otherwise supply the context.
//
// An opinion piece is the other kind of public page: a signed-in reader's own
// text, listed in the Opinion section. It carries an "Opinion" label with the
// author's byline at the top and a human-author notice at the bottom in place
// of the AI line. Both follow the server's record (publication.kind and
// publication.author_label from the resolved bundle), never the link or the
// window context, so a crafted link cannot hide the AI disclosure of a story.

export const AI_DISCLOSURE_TEXT =
  'Written by AI. Autopaper’s agent desks wrote this story from the sources it links; no human editor reviewed it before publication.';

export const OPINION_NOTICE_TEXT =
  'Opinion. Written and published by a reader; Autopaper’s agent desks did not write it and no editor reviewed it.';

type PublicationLike = { kind?: unknown; author_label?: unknown } | null | undefined;

function escapeHTML(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function isOpinion(publication: PublicationLike): boolean {
  return publication?.kind === 'opinion';
}

function authorOf(publication: PublicationLike): string {
  return typeof publication?.author_label === 'string' ? publication.author_label.trim() : '';
}

export function aiDisclosureHTML(appContext: Record<string, unknown> | null | undefined): string {
  if (appContext?.autopaperStory !== true) return '';
  return `<p class="texture-ai-disclosure" data-texture-ai-disclosure>${AI_DISCLOSURE_TEXT}</p>`;
}

/** The Opinion label and byline at the top of an opinion piece; empty for anything else. */
export function opinionLabelHTML(publication: PublicationLike): string {
  if (!isOpinion(publication)) return '';
  const author = authorOf(publication);
  const by = author ? ` <span class="texture-opinion-byline" data-texture-opinion-byline>by ${escapeHTML(author)}</span>` : '';
  return `<p class="texture-opinion-label" data-texture-opinion-label><span class="texture-opinion-kind">Opinion</span>${by}</p>`;
}

/**
 * The line that closes a published page: the human-author notice for an
 * opinion piece, the AI disclosure for an Autopaper story, nothing otherwise.
 */
export function pieceDisclosureHTML(
  appContext: Record<string, unknown> | null | undefined,
  publication: PublicationLike,
): string {
  if (isOpinion(publication)) {
    const author = authorOf(publication);
    const text = author ? `${OPINION_NOTICE_TEXT} Author: ${escapeHTML(author)}.` : OPINION_NOTICE_TEXT;
    return `<p class="texture-opinion-notice" data-texture-opinion-notice>${text}</p>`;
  }
  if (publication?.kind === 'story') return aiDisclosureHTML({ autopaperStory: true });
  return aiDisclosureHTML(appContext);
}
