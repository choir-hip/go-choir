export const SOURCE_EVIDENCE_STATES = {
  candidate: 'candidate',
  available: 'available',
  confirms: 'confirms',
  refutes: 'refutes',
  qualifies: 'qualifies',
  noSourceNeeded: 'no_source_needed',
  stale: 'stale',
  blockedByAccess: 'blocked_by_access',
  unavailable: 'unavailable',
} as const;

export const SOURCE_OPEN_SURFACES = {
  source: 'source',
  webLens: 'web_lens',
  vtext: 'vtext',
  video: 'video',
  image: 'image',
} as const;

export function normalizeSourceEvidenceState(value: unknown): string {
  const normalized = String(value || '').trim().toLowerCase().replace(/[\s-]+/g, '_');
  switch (normalized) {
    case SOURCE_EVIDENCE_STATES.candidate:
    case SOURCE_EVIDENCE_STATES.available:
    case SOURCE_EVIDENCE_STATES.confirms:
    case SOURCE_EVIDENCE_STATES.refutes:
    case SOURCE_EVIDENCE_STATES.qualifies:
    case SOURCE_EVIDENCE_STATES.noSourceNeeded:
    case SOURCE_EVIDENCE_STATES.stale:
    case SOURCE_EVIDENCE_STATES.blockedByAccess:
    case SOURCE_EVIDENCE_STATES.unavailable:
      return normalized;
    case 'pending':
    case 'needs_source':
    case 'source_needed':
      return SOURCE_EVIDENCE_STATES.candidate;
    case 'confirming':
    case 'confirmed':
    case 'represented':
    case 'owner_supplied':
      return SOURCE_EVIDENCE_STATES.confirms;
    case 'refuting':
    case 'refuted':
      return SOURCE_EVIDENCE_STATES.refutes;
    case 'qualifying':
    case 'qualified':
      return SOURCE_EVIDENCE_STATES.qualifies;
    case 'blocked':
    case 'blocked_access':
    case 'access_blocked':
      return SOURCE_EVIDENCE_STATES.blockedByAccess;
    case 'not_needed':
    case 'no_source':
      return SOURCE_EVIDENCE_STATES.noSourceNeeded;
    case 'error':
    case 'failed':
    case 'fetch_failed':
      return SOURCE_EVIDENCE_STATES.unavailable;
    default:
      return '';
  }
}

export function sourceEvidenceStateLabel(value: unknown): string {
  const state = normalizeSourceEvidenceState(value);
  switch (state) {
    case SOURCE_EVIDENCE_STATES.candidate:
      return 'Candidate source';
    case SOURCE_EVIDENCE_STATES.available:
      return 'Available source';
    case SOURCE_EVIDENCE_STATES.confirms:
      return 'Confirms claim';
    case SOURCE_EVIDENCE_STATES.refutes:
      return 'Refutes claim';
    case SOURCE_EVIDENCE_STATES.qualifies:
      return 'Qualifies claim';
    case SOURCE_EVIDENCE_STATES.noSourceNeeded:
      return 'No source needed';
    case SOURCE_EVIDENCE_STATES.stale:
      return 'Stale source';
    case SOURCE_EVIDENCE_STATES.blockedByAccess:
      return 'Blocked by access';
    case SOURCE_EVIDENCE_STATES.unavailable:
      return 'Unavailable source';
    default:
      return 'Evidence unclassified';
  }
}

export function normalizeSourceOpenSurface(value: unknown): string {
  const normalized = String(value || '').trim().toLowerCase().replace(/[\s-]+/g, '_');
  switch (normalized) {
    case '':
      return '';
    case SOURCE_OPEN_SURFACES.webLens:
    case 'weblens':
    case 'browser':
    case 'web':
    case 'live':
    case 'original':
    case 'live_original':
      return SOURCE_OPEN_SURFACES.webLens;
    case SOURCE_OPEN_SURFACES.source:
    case 'source_viewer':
    case 'source_reader':
    case 'reader':
    case 'content':
      return SOURCE_OPEN_SURFACES.source;
    case SOURCE_OPEN_SURFACES.vtext:
    case 'published_vtext':
    case 'publication_version':
    case 'published_vtext_span':
      return SOURCE_OPEN_SURFACES.vtext;
    case SOURCE_OPEN_SURFACES.video:
    case 'youtube':
    case 'youtube_video':
      return SOURCE_OPEN_SURFACES.video;
    case SOURCE_OPEN_SURFACES.image:
      return SOURCE_OPEN_SURFACES.image;
    default:
      return normalized;
  }
}
