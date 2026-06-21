import {
  mediaRefToSourceEntity,
  publicationBundleSourceEntities,
  sourceEntityID,
  sourceEntityTargetURL,
  sourceEntityTitle,
} from './texture-source-renderer';

export function revisionMediaSourceRefs(revision: any = null) {
  const refs = revision?.metadata?.media_source_refs;
  return Array.isArray(refs) ? refs : [];
}

export function revisionSourceEntities({
  revision = null,
  bundle = null,
  publishedRoutePath = '',
  appContext = {},
}: any = {}) {
  const publishedEntities = publicationBundleSourceEntities(bundle, publishedRoutePath, appContext);
  if (publishedEntities.length > 0) return publishedEntities;
  if (Array.isArray(revision?.source_entities) && revision.source_entities.length > 0) return revision.source_entities;
  const entities = revision?.metadata?.source_entities;
  if (Array.isArray(entities) && entities.length > 0) return entities;
  return revisionMediaSourceRefs(revision).map(mediaRefToSourceEntity).filter(Boolean);
}

export function revisionSourceGaps(revision: any = null) {
  const gaps = revision?.metadata?.source_gaps;
  return Array.isArray(gaps) ? gaps : [];
}

export function unresolvedCitationMarkers(content = '') {
  const sourceLinked = new Set<string>();
  for (const match of String(content || '').matchAll(/\[([^\]]+)\]\(source:[^)]+\)/g)) {
    sourceLinked.add(`[${match[1]}]`);
  }
  const markers = new Set<string>();
  for (const match of String(content || '').matchAll(/\[(\d+)\](?!\()/g)) {
    const marker = `[${match[1]}]`;
    if (!sourceLinked.has(marker)) markers.add(marker);
  }
  return [...markers];
}

export function sourceRepairCandidates(content = '', gaps: any[] = []) {
  const fromGaps = (Array.isArray(gaps) ? gaps : [])
    .map((gap) => String(gap?.marker || '').trim())
    .filter(Boolean);
  return [...new Set([...fromGaps, ...unresolvedCitationMarkers(content)])];
}

export function sourceReviewFormState(marker = '') {
  return {
    marker: marker || '',
    title: '',
    url: '',
    excerpt: '',
    relation: 'confirms',
    reason: '',
    status: '',
    error: '',
  };
}

export function selectedSourceEntity(sourceEntities: any[] = [], selectedSourceEntityID = '') {
  return sourceEntities.find((entity) => sourceEntityID(entity) === selectedSourceEntityID) || sourceEntities[0] || null;
}

export function sourceArtifactFormState(entity: any = null) {
  if (!entity) {
    return {
      selectedSourceEntityID: '',
      title: '',
      url: '',
      text: '',
      status: '',
      error: '',
    };
  }
  return {
    selectedSourceEntityID: sourceEntityID(entity),
    title: sourceEntityTitle(entity),
    url: sourceEntityTargetURL(entity),
    text: '',
    status: '',
    error: '',
  };
}
