import {
  publicURLForPublishResult as derivePublicURLForPublishResult,
  truncateText,
} from './vtext-editor-state';

export function titleForPublishedBundle(bundle: any = null) {
  return bundle?.publication?.title || 'Published Texture';
}

export function publicURLForPublishResult(result: any = null, origin = '') {
  return derivePublicURLForPublishResult(result, origin);
}

export function currentPublicationRoute({
  publishResult = null,
  publishedBundle = null,
  publishedRoutePath = '',
  appContext = {},
}: any = {}) {
  return publishResult?.route_path ||
    publishedBundle?.route?.path ||
    publishedRoutePath ||
    appContext?.publishedRoutePath ||
    '';
}

export function buildPublishedTransclusionRef(
  bundle: any = null,
  { publishedRoutePath = '', appContext = {} }: any = {},
) {
  if (!bundle?.publication?.id || !bundle?.version?.id) return null;
  const firstSpan = bundle.retrieval?.spans?.[0] || null;
  const firstBlock = bundle.artifact?.render_model?.[0] || null;
  const selector = firstSpan?.selector || {
    kind: 'document',
    route_path: bundle.route?.path || publishedRoutePath || appContext?.publishedRoutePath || '',
  };
  return {
    source_kind: firstSpan?.id ? 'published_vtext_span' : 'publication_version',
    publication_id: bundle.publication.id,
    publication_version_id: bundle.version.id,
    span_id: firstSpan?.id || firstBlock?.span_id || '',
    content_hash: bundle.version?.content_hash || '',
    selector,
    snapshot_text: truncateText(firstSpan?.snippet || firstBlock?.text || bundle.artifact?.content || '', 720),
  };
}

export function derivativeContentForPublished(bundle: any = null) {
  const title = titleForPublishedBundle(bundle);
  const source = String(bundle?.artifact?.content || '').trim();
  const quoted = (source || 'Blank published Texture.')
    .split(/\r?\n/)
    .map((line) => `> ${line}`)
    .join('\n');
  return `# My version of ${title}\n\n${quoted}\n\n## Notes\n\n`;
}

export function publishedCitationPayload(ref: any = null, bundle: any = null) {
  if (!ref) return [];
  return [{
    kind: 'published_vtext_span',
    title: titleForPublishedBundle(bundle),
    publication_id: ref.publication_id,
    publication_version_id: ref.publication_version_id,
    span_id: ref.span_id,
    content_hash: ref.content_hash,
    selector: ref.selector,
  }];
}
