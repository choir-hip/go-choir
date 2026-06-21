import {
  mediaRefToSourceEntity,
  publicationBundleSourceEntities,
} from './texture-source-renderer';

export function revisionMediaSourceRefs(revision: any = null) {
  const refs = revision?.metadata?.media_source_refs;
  return Array.isArray(refs) ? refs : [];
}

function revisionHasStructuredBody(revision: any = null): boolean {
  const bodyDoc = revision?.body_doc || revision?.bodyDoc;
  if (!bodyDoc) return false;
  if (typeof bodyDoc === 'object') return true;
  return String(bodyDoc || '').trim() !== '';
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
  if (revisionHasStructuredBody(revision)) return [];
  return revisionMediaSourceRefs(revision).map(mediaRefToSourceEntity).filter(Boolean);
}
