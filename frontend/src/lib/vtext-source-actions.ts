import {
  attachVTextSourceArtifacts,
  createContentItem,
  importContentURL,
  repairVTextSourceGaps,
} from './vtext.js';
import { buildSourceReviewPayload } from './vtext-source-review.js';
import {
  selectorTextQuote,
  sourceEntityID,
  sourceEntityTargetURL,
  sourceEntityTitle,
} from './vtext-source-renderer';

export function sourceContentItemPayload({
  entity,
  title = '',
  sourceURL = '',
  text = '',
}: {
  entity: any;
  title?: string;
  sourceURL?: string;
  text?: string;
}): any {
  const resolvedTitle = String(title || '').trim() || sourceEntityTitle(entity);
  const resolvedURL = String(sourceURL || '').trim() || sourceEntityTargetURL(entity);
  return {
    source_type: 'text',
    media_type: 'text/markdown',
    app_hint: 'content',
    title: resolvedTitle,
    source_url: resolvedURL,
    canonical_url: resolvedURL,
    text_content: text,
    metadata: {
      source_entity_id: sourceEntityID(entity),
      created_from: 'vtext_source_artifact_ui',
    },
    provenance: {
      rights_scope: 'public_source',
      publish_source_snapshot: true,
      untrusted_source_text: true,
    },
  };
}

export async function applySourceReview({
  docId,
  revisionID,
  authorLabel,
  marker,
  title,
  excerpt,
  url,
}: {
  docId: string;
  revisionID: string;
  authorLabel: string;
  marker: string;
  title: string;
  excerpt: string;
  url?: string;
}): Promise<any> {
  const payload = {
    ...buildSourceReviewPayload({
      marker,
      title,
      excerpt,
      url,
      revisionID,
    }),
    base_revision_id: revisionID,
    author_label: authorLabel,
  };
  return repairVTextSourceGaps(docId, payload);
}

export async function attachSourceContentItem({
  docId,
  revisionID,
  authorLabel,
  entity,
  contentItem,
}: {
  docId: string;
  revisionID: string;
  authorLabel: string;
  entity: any;
  contentItem: any;
}): Promise<any> {
  if (!docId || !revisionID || !sourceEntityID(entity) || !contentItem?.content_id) {
    throw new Error('Choose a source and readable content item first');
  }
  return attachVTextSourceArtifacts(docId, {
    base_revision_id: revisionID,
    author_label: authorLabel,
    attachments: [{
      entity_id: sourceEntityID(entity),
      content_id: contentItem.content_id,
      text_quote: selectorTextQuote(entity),
    }],
  });
}

export async function importSourceContentItem({
  entity,
  title = '',
  sourceURL = '',
}: {
  entity: any;
  title?: string;
  sourceURL?: string;
}): Promise<any> {
  const resolvedURL = String(sourceURL || '').trim() || sourceEntityTargetURL(entity);
  const resolvedTitle = String(title || '').trim() || sourceEntityTitle(entity);
  return importContentURL(resolvedURL, resolvedTitle);
}

export async function createSourceContentItem({
  entity,
  title = '',
  sourceURL = '',
  text,
}: {
  entity: any;
  title?: string;
  sourceURL?: string;
  text: string;
}): Promise<any> {
  return createContentItem(sourceContentItemPayload({
    entity,
    title,
    sourceURL,
    text,
  }));
}
