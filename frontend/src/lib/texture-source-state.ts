import {
  publicationBundleSourceEntities,
} from './texture-source-renderer';

export function revisionSourceEntities({
  revision = null,
  bundle = null,
  publishedRoutePath = '',
  appContext = {},
}: any = {}) {
  const publishedEntities = publicationBundleSourceEntities(bundle, publishedRoutePath, appContext);
  if (publishedEntities.length > 0) return publishedEntities;
  if (Array.isArray(revision?.source_entities) && revision.source_entities.length > 0) return revision.source_entities;
  return [];
}
