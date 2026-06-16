import {
  READER_ARTIFACT_STATES,
  SOURCE_CONTRACT_SCHEMA,
  SOURCE_EVIDENCE_STATES,
  SOURCE_OPEN_SURFACES,
  SOURCE_SELECTOR_KINDS,
} from './source-contract.generated';

export {
  READER_ARTIFACT_STATES,
  SOURCE_CONTRACT_SCHEMA,
  SOURCE_EVIDENCE_STATES,
  SOURCE_OPEN_SURFACES,
  SOURCE_SELECTOR_KINDS,
};

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

function normalizeContractToken(value: unknown): string {
  return String(value || '').trim().toLowerCase().replace(/[\s-]+/g, '_');
}

function canonicalFromSchema(entries: Record<string, { aliases?: readonly string[] }>, value: unknown): string {
  const normalized = normalizeContractToken(value);
  for (const [canonical, spec] of Object.entries(entries)) {
    if (normalized === canonical) return canonical;
    if ((spec.aliases || []).some((alias) => normalizeContractToken(alias) === normalized)) return canonical;
  }
  return '';
}

export function normalizeSourceEvidenceState(value: unknown): string {
  return canonicalFromSchema(SOURCE_CONTRACT_SCHEMA.evidence_states, value);
}

export function sourceEvidenceStateLabel(value: unknown): string {
  const state = normalizeSourceEvidenceState(value);
  return SOURCE_CONTRACT_SCHEMA.evidence_states[state]?.label || 'Evidence unclassified';
}

export function normalizeReaderArtifactState(value: unknown): string {
  return canonicalFromSchema(SOURCE_CONTRACT_SCHEMA.reader_artifact_states, value);
}

export function readerArtifactStateLabel(value: unknown): string {
  const state = normalizeReaderArtifactState(value);
  return SOURCE_CONTRACT_SCHEMA.reader_artifact_states[state]?.label || '';
}

export function normalizeSourceSelectorKind(value: unknown): string {
  const normalized = normalizeContractToken(value);
  if (!normalized) return SOURCE_SELECTOR_KINDS.wholeResource;
  return canonicalFromSchema(SOURCE_CONTRACT_SCHEMA.selector_kinds, value) || normalized;
}

export function normalizeSourceSelector(selector: any): any | null {
  if (!selector || typeof selector !== 'object') return null;
  return {
    ...selector,
    selector_kind: normalizeSourceSelectorKind(selector.selector_kind),
  };
}

export function sourceSelectorList(value: any): any[] {
  if (!value) return [];
  if (Array.isArray(value)) {
    return value.flatMap((selector) => sourceSelectorList(selector));
  }
  if (typeof value !== 'object') return [];
  const selector = normalizeSourceSelector(value);
  if (!selector) return [];
  if (selector.selector_kind === SOURCE_SELECTOR_KINDS.selectorSet) {
    const selectors = Array.isArray(selector.selectors) ? selector.selectors : [];
    return selectors.flatMap((nested) => sourceSelectorList(nested));
  }
  return [selector];
}

export function normalizeSourceOpenSurface(value: unknown): string {
  const normalized = normalizeContractToken(value);
  if (!normalized) return '';
  return canonicalFromSchema(SOURCE_CONTRACT_SCHEMA.open_surfaces, value) || normalized;
}

export function sourceOpenPlan(input: SourceOpenPlanInput = {}): SourceOpenPlan {
  const requested = normalizeSourceOpenSurface(input.requestedOpenSurface);
  const targetKind = String(input.targetKind || '').trim().toLowerCase();
  const sourceKind = String(input.sourceKind || '').trim().toLowerCase();
  const durableReaderTarget = targetKind === 'content_item' || targetKind === 'source_service_item' || !!input.hasURL;

  if (targetKind === 'published_vtext_span' || targetKind === 'publication_version') {
    return {
      appId: 'texture',
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
