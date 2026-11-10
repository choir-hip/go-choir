// What the prompt bar is aimed at while a Texture window is in view (lane D,
// handoff 4.D). Texture states it; the shell never infers ownership.
//
// Ways it can fail (docs/evidence/prompt-bar-universal-input-failures-2026-11-10.md):
//  D2 an owned aim is stated for a published read-only story (the bar would
//     offer "edit" on a document the reader does not own);
//  D7 the revision shown is not the one the text answers: a historical view
//     must carry its revision id, the latest view must carry none;
//  a foreign aim without a pinned version (no id or no manifest hash) cannot
//     fork, so it is not stated.

export type PromptAim = {
  state: 'owned' | 'foreign';
  title: string;
  /** "Edit this story…" or "Reply to …": the placeholder. */
  verb: string;
  /** The short word on the pill: "Edit" or "Reply". */
  chip: string;
  /** "v3 of v7" when an older revision is in view, else "v7". */
  revisionLabel: string;
  /** The revision the text answers when it is not the head; empty answers the head. */
  baseRevisionId: string;
  /** A foreign aim's pinned published version, the fork's source. */
  source: { publication_version_id: string; manifest_hash: string } | null;
};

type AimInput = {
  authenticated: boolean;
  loading: boolean;
  docId: string;
  title: string;
  readOnlyPublished: boolean;
  revisions: Array<{ revision_id?: string }>;
  activeIndex: number;
  versionNumber: (index: number) => number;
  publishedRevision: number;
  publishedLatest: number;
  publishedVersionId: string;
  publishedManifestHash: string;
};

const short = (title: string) => {
  const t = String(title || '').trim();
  return t.length > 48 ? `${t.slice(0, 47)}…` : t;
};

export function promptAimFor(input: AimInput): PromptAim | null {
  if (!input.authenticated || input.loading) return null;
  if (input.readOnlyPublished) {
    if (!input.publishedVersionId || !input.publishedManifestHash) return null;
    const rev = input.publishedRevision;
    const older = rev > 0 && input.publishedLatest > rev;
    const title = short(input.title);
    return {
      state: 'foreign',
      title,
      verb: title ? `Reply to ${title}…` : 'Reply to this story…',
      chip: 'Reply',
      revisionLabel: rev > 0 ? (older ? `v${rev} of v${input.publishedLatest}` : `v${rev}`) : '',
      baseRevisionId: '',
      source: { publication_version_id: input.publishedVersionId, manifest_hash: input.publishedManifestHash },
    };
  }
  if (!input.docId) return null;
  const n = input.revisions.length;
  const idx = input.activeIndex >= 0 && input.activeIndex < n ? input.activeIndex : n - 1;
  const historical = n > 0 && idx !== n - 1;
  const current = idx >= 0 ? input.versionNumber(idx) : 0;
  const head = n > 0 ? input.versionNumber(n - 1) : 0;
  const title = short(input.title);
  return {
    state: 'owned',
    title,
    verb: title ? `Edit ${title}…` : 'Edit this story…',
    chip: 'Edit',
    revisionLabel: current > 0 ? (historical ? `v${current} of v${head}` : `v${current}`) : '',
    baseRevisionId: historical ? String(input.revisions[idx]?.revision_id || '') : '',
    source: null,
  };
}
