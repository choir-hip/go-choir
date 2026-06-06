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

export const READER_ARTIFACT_STATES = {
  ready: 'reader_snapshot_ready',
  notPublicationSafe: 'not_publication_safe',
  boundedExcerptOnly: 'bounded_excerpt_only',
  importFailed: 'import_failed',
} as const;

export type SourceOpenPlanInput = {
  requestedOpenSurface?: unknown;
  targetKind?: unknown;
  sourceKind?: unknown;
  hasURL?: boolean;
};

export type SourceOpenPlan = {
  appId: string;
  openSurface: string;
  mode: string;
  liveOriginal: boolean;
  readerMode: boolean;
};

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

export function normalizeReaderArtifactState(value: unknown): string {
  const normalized = String(value || '').trim().toLowerCase().replace(/[\s-]+/g, '_');
  switch (normalized) {
    case READER_ARTIFACT_STATES.ready:
    case 'ready':
    case 'snapshot_ready':
      return READER_ARTIFACT_STATES.ready;
    case READER_ARTIFACT_STATES.notPublicationSafe:
    case 'publication_blocked':
    case 'not_safe_for_publication':
      return READER_ARTIFACT_STATES.notPublicationSafe;
    case READER_ARTIFACT_STATES.boundedExcerptOnly:
    case 'excerpt_only':
    case 'bounded_excerpt':
      return READER_ARTIFACT_STATES.boundedExcerptOnly;
    case READER_ARTIFACT_STATES.importFailed:
    case 'failed':
    case 'fetch_failed':
    case 'source_import_failed':
      return READER_ARTIFACT_STATES.importFailed;
    default:
      return '';
  }
}

export function readerArtifactStateLabel(value: unknown): string {
  switch (normalizeReaderArtifactState(value)) {
    case READER_ARTIFACT_STATES.ready:
      return 'Reader snapshot ready';
    case READER_ARTIFACT_STATES.notPublicationSafe:
      return 'Not publication safe';
    case READER_ARTIFACT_STATES.boundedExcerptOnly:
      return 'Bounded excerpt only';
    case READER_ARTIFACT_STATES.importFailed:
      return 'Source import failed';
    default:
      return '';
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

export function sourceOpenPlan(input: SourceOpenPlanInput = {}): SourceOpenPlan {
  const requested = normalizeSourceOpenSurface(input.requestedOpenSurface);
  const targetKind = String(input.targetKind || '').trim().toLowerCase();
  const sourceKind = String(input.sourceKind || '').trim().toLowerCase();
  const durableReaderTarget = targetKind === 'content_item' || targetKind === 'source_service_item' || !!input.hasURL;

  if (targetKind === 'published_vtext_span' || targetKind === 'publication_version') {
    return {
      appId: 'vtext',
      openSurface: requested || SOURCE_OPEN_SURFACES.vtext,
      mode: 'published_vtext',
      liveOriginal: false,
      readerMode: false,
    };
  }
  if (requested === SOURCE_OPEN_SURFACES.webLens) {
    return {
      appId: 'browser',
      openSurface: requested,
      mode: 'live_original',
      liveOriginal: true,
      readerMode: false,
    };
  }
  if (requested === SOURCE_OPEN_SURFACES.video || sourceKind === 'youtube_video') {
    return {
      appId: 'video',
      openSurface: requested || SOURCE_OPEN_SURFACES.video,
      mode: 'media',
      liveOriginal: false,
      readerMode: false,
    };
  }
  if (requested === SOURCE_OPEN_SURFACES.source || durableReaderTarget) {
    return {
      appId: 'content',
      openSurface: requested || SOURCE_OPEN_SURFACES.source,
      mode: 'source_reader',
      liveOriginal: false,
      readerMode: true,
    };
  }
  if (requested) {
    return {
      appId: requested,
      openSurface: requested,
      mode: requested,
      liveOriginal: false,
      readerMode: false,
    };
  }
  return {
    appId: 'content',
    openSurface: SOURCE_OPEN_SURFACES.source,
    mode: 'source_reader',
    liveOriginal: false,
    readerMode: true,
  };
}
