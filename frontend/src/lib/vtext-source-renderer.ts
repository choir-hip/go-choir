import { youtubeEmbedURL } from './media-utils.js';

export function escapeHTML(value: unknown): string {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

export function sourceEntityID(entity: any): string {
  const record = sourceEntityRecord(entity);
  return String(
    entity?.entity_id ||
    entity?.source_entity_id ||
    record?.entity_id ||
    record?.source_entity_id ||
    ''
  ).trim();
}

export function findSourceEntity(sourceEntities: any[] = [], entityID = ''): any | null {
  const normalized = String(entityID || '').trim();
  if (!normalized) return null;
  return sourceEntities.find((entity) => sourceEntityID(entity) === normalized) || null;
}

export function sourceEntityRecord(entity: any): any {
  if (entity?.entity && typeof entity.entity === 'object') return entity.entity;
  return entity || {};
}

export function sourceEntityTransclusion(entity: any): any | null {
  const record = sourceEntityRecord(entity);
  return entity?.transclusion || record?.transclusion || null;
}

export function selectorTextQuote(entity: any): string {
  const record = sourceEntityRecord(entity);
  const selectors = [
    ...(Array.isArray(entity?.selectors) ? entity.selectors : []),
    ...(Array.isArray(record?.selectors) ? record.selectors : []),
  ];
  for (const selector of selectors) {
    const text = String(selector?.text_quote || '').trim();
    if (text) return text;
  }
  return '';
}

export function sourceEntityExcerptText(entity: any): string {
  return sourceEntityTransclusion(entity)?.snapshot_text || selectorTextQuote(entity) || '';
}

export function sourceEntityReaderSnapshotText(entity: any): string {
  const record = sourceEntityRecord(entity);
  return String(
    entity?.reader_snapshot?.text_content ||
    entity?.published_source?.text_content ||
    record?.reader_snapshot?.text_content ||
    record?.published_source?.text_content ||
    ''
  ).trim();
}

export function sourceEntitySnapshotText(entity: any): string {
  return sourceEntityReaderSnapshotText(entity) || sourceEntityExcerptText(entity);
}

function normalizeExcerptText(value: unknown): string {
  return String(value || '').replace(/\s+/g, ' ').trim();
}

function boundedExcerpt(value: unknown, maxChars = 520): string {
  const text = normalizeExcerptText(value);
  if (text.length <= maxChars) return text;
  const sentenceMatches = text.match(/[^.!?]+[.!?]+(?:\s|$)/g) || [];
  let output = '';
  for (const sentence of sentenceMatches) {
    const next = normalizeExcerptText(`${output} ${sentence}`);
    if (next.length > maxChars) break;
    output = next;
    if (output.length >= Math.floor(maxChars * 0.65)) break;
  }
  if (output) return output;
  return `${text.slice(0, Math.max(0, maxChars - 1)).trimEnd()}…`;
}

export function sourceEntityInlineExcerptText(entity: any, maxChars = 520): string {
  const selectedExcerpt = sourceEntityExcerptText(entity);
  const readerSnapshot = sourceEntityReaderSnapshotText(entity);
  return boundedExcerpt(selectedExcerpt || readerSnapshot, maxChars);
}

export function sourceEntityReaderSnapshotStatus(entity: any): any {
  const record = sourceEntityRecord(entity);
  return entity?.reader_snapshot_status || record?.reader_snapshot_status || null;
}

export function sourceEntitySnapshotWarnings(entity: any): string[] {
  const status = sourceEntityReaderSnapshotStatus(entity);
  const warnings = Array.isArray(status?.warnings) ? status.warnings : [];
  return warnings
    .map((warning: unknown) => String(warning || '').trim())
    .filter(Boolean)
    .slice(0, 8);
}

export function sourceEntityDisplayPolicy(entity: any): string {
  const record = sourceEntityRecord(entity);
  const raw = String(
    entity?.display_policy ||
    entity?.display?.display_policy ||
    entity?.display?.inline_mode ||
    record?.display_policy ||
    record?.display?.display_policy ||
    record?.display?.inline_mode ||
    ''
  ).trim();
  if (raw === 'embedded_excerpt' || raw === 'embedded_preview' || raw === 'expanded' || raw === 'collapsed_citation') return raw;
  if (sourceEntityExcerptText(entity)) return 'embedded_excerpt';
  return 'collapsed_citation';
}

export function sourceEntityKindLabel(kind: unknown): string {
  const normalized = String(kind || '').replace(/_/g, ' ');
  return normalized || 'source';
}

export function sourceEntityTitle(entity: any): string {
  const record = sourceEntityRecord(entity);
  return entity?.label || record?.label || sourceEntityKindLabel(entity?.kind || record?.kind);
}

export function sourceEntityTargetURL(entity: any): string {
  const record = sourceEntityRecord(entity);
  return (
    entity?.target?.canonical_url ||
    entity?.target?.url ||
    entity?.canonical_url ||
    entity?.url ||
    record?.target?.canonical_url ||
    record?.target?.url ||
    record?.canonical_url ||
    record?.url ||
    ''
  );
}

export function sourceEntityTargetKind(entity: any): string {
  const record = sourceEntityRecord(entity);
  return String(
    entity?.target?.target_kind ||
    entity?.target_kind ||
    record?.target?.target_kind ||
    record?.target_kind ||
    ''
  ).trim();
}

function sourceEntityRequestedOpenSurface(entity: any): string {
  const record = sourceEntityRecord(entity);
  return String(entity?.display?.open_surface || record?.display?.open_surface || '').trim().toLowerCase();
}

export function sourceEntityOpenPlan(entity: any): any {
  const record = sourceEntityRecord(entity);
  const targetKind = sourceEntityTargetKind(entity);
  const requested = sourceEntityRequestedOpenSurface(entity);
  const kind = String(entity?.kind || record?.kind || '').trim().toLowerCase();
  const hasURL = !!sourceEntityTargetURL(entity);
  const durableReaderTarget = targetKind === 'content_item' || targetKind === 'source_service_item' || hasURL;

  if (targetKind === 'published_vtext_span' || targetKind === 'publication_version') {
    return { appId: 'vtext', openSurface: requested || 'vtext', mode: 'published_vtext', liveOriginal: false, readerMode: false };
  }
  if (requested === 'browser' || requested === 'web' || requested === 'web_lens' || requested === 'live' || requested === 'original') {
    return { appId: 'browser', openSurface: requested, mode: 'live_original', liveOriginal: true, readerMode: false };
  }
  if (requested === 'video' || kind === 'youtube_video') {
    return { appId: 'video', openSurface: requested || 'video', mode: 'media', liveOriginal: false, readerMode: false };
  }
  if (requested === 'source' || requested === 'content' || durableReaderTarget) {
    return { appId: 'content', openSurface: requested || 'source', mode: 'source_reader', liveOriginal: false, readerMode: true };
  }
  if (requested) {
    return { appId: requested, openSurface: requested, mode: requested, liveOriginal: false, readerMode: false };
  }
  return { appId: 'content', openSurface: 'source', mode: 'source_reader', liveOriginal: false, readerMode: true };
}

export function sourceEntityOpenAppID(entity: any): string {
  return sourceEntityOpenPlan(entity).appId;
}

export function matchingPublicationTransclusion(bundle: any, entityID = ''): any | null {
  const normalized = String(entityID || '').trim();
  if (!normalized) return null;
  const transclusions = Array.isArray(bundle?.transclusions) ? bundle.transclusions : [];
  return transclusions.find((item) => String(item?.source_entity_id || '') === normalized) || null;
}

export function publicationSourceEntityToLocal(record: any, context: { bundle?: any; routePath?: string; appContext?: any } = {}): any | null {
  if (!record) return null;
  const raw = record.entity && typeof record.entity === 'object' ? record.entity : {};
  const bundle = context.bundle || null;
  const entity = {
    ...raw,
    entity_id: raw.entity_id || record.source_entity_id || record.id || '',
    kind: raw.kind || record.kind || '',
    target: {
      ...(raw.target || {}),
      target_kind: raw.target?.target_kind || record.target_kind || '',
    },
    display: {
      ...(raw.display || {}),
      inline_mode: raw.display?.inline_mode || record.display_policy || 'collapsed_citation',
      open_surface: raw.display?.open_surface || record.open_surface || '',
    },
    transclusion: matchingPublicationTransclusion(bundle, raw.entity_id || record.source_entity_id || ''),
    publication_route_path: bundle?.route?.path || context.routePath || context.appContext?.publishedRoutePath || '',
  };
  if (!entity.target.item_id && record.target_kind === 'source_service_item') entity.target.item_id = record.target_id || '';
  if (!entity.target.content_id && record.target_kind === 'content_item') entity.target.content_id = record.target_id || '';
  if (!entity.target.publication_version_id && record.target_kind === 'published_vtext_span') {
    entity.target.publication_version_id = record.target_id || bundle?.version?.id || '';
  }
  return sourceEntityID(entity) ? entity : null;
}

export function publicationBundleSourceEntities(bundle: any = null, routePath = '', appContext: any = {}): any[] {
  const records = Array.isArray(bundle?.source_entities) ? bundle.source_entities : [];
  if (records.length === 0) return [];
  return records
    .map((record) => publicationSourceEntityToLocal(record, { bundle, routePath, appContext }))
    .filter(Boolean);
}

export function mediaRefToSourceEntity(ref: any): any | null {
  const kind = String(ref?.kind || '').toLowerCase();
  if (!kind) return null;
  const entityKind = kind === 'youtube' ? 'youtube_video' : kind;
  const canonical = ref?.canonical_url || ref?.url || '';
  return {
    entity_id: `${entityKind}:${canonical || ref?.content_id || ''}`,
    kind: entityKind,
    label: ref?.title || (kind === 'youtube' ? 'YouTube source' : 'Image source'),
    target: {
      target_kind: 'content_item',
      content_id: ref?.content_id || '',
      url: ref?.url || canonical,
      canonical_url: canonical,
    },
    display: {
      inline_mode: kind === 'youtube' || kind === 'image' ? 'embedded_preview' : 'collapsed_citation',
      expanded_mode: kind === 'youtube' ? 'media_player' : 'source_card',
      open_surface: ref?.app_hint || (kind === 'youtube' ? 'video' : kind),
      default_collapsed: true,
    },
    evidence: {
      state: ref?.content_id ? 'available' : 'pending',
      research_state: ref?.research_state || 'pending',
      transcript_content_id: ref?.transcript_content_id || '',
      transcript_availability: ref?.transcript_availability || '',
    },
    provenance: {
      created_by: 'importer',
      rights_scope: 'private_user_source',
      untrusted_source_text: true,
    },
  };
}

export function sourceEntityMedia(entity: any, { inline = false } = {}): string {
  const kind = String(entity?.kind || '').toLowerCase();
  const sourceURL = sourceEntityTargetURL(entity);
  const title = escapeHTML(sourceEntityTitle(entity));
  if (kind === 'youtube_video') {
    const embed = youtubeEmbedURL(sourceURL);
    if (embed) {
      if (inline) {
        return `<span class="vtext-source-video vtext-source-video--inline"><iframe src="${escapeHTML(embed)}" title="${title}" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe></span>`;
      }
      return `<div class="vtext-source-video"><iframe src="${escapeHTML(embed)}" title="${title}" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe></div>`;
    }
  }
  if (kind === 'image' && sourceURL) {
    if (inline) {
      return `<span class="vtext-source-image vtext-source-image--inline"><img src="${escapeHTML(sourceURL)}" alt="${title}" loading="lazy"></span>`;
    }
    return `<div class="vtext-source-image"><img src="${escapeHTML(sourceURL)}" alt="${title}" loading="lazy"></div>`;
  }
  return '';
}

export function sourceEntityExpansionSurface(entity: any): string {
  return sourceEntityMedia(entity, { inline: true }) ? 'media' : 'journal';
}

export function renderSourceEntityFacts(entity: any): string {
  const transcript = String(entity?.evidence?.transcript_availability || '').trim();
  const selectors = Array.isArray(entity?.selectors) ? entity.selectors : [];
  const supports = selectors
    .map((selector) => String(selector?.supports || selector?.label || '').trim())
    .filter(Boolean)
    .slice(0, 2);
  const facts = [];
  if (transcript) facts.push(`transcript ${transcript}`);
  for (const support of supports) facts.push(`supports ${support}`);
  return `
    ${facts.map((fact) => `<span>${escapeHTML(fact)}</span>`).join('')}
  `;
}

export function renderSourceTransclusionBody(entity: any, { compact = false } = {}): string {
  const snapshot = compact ? sourceEntityInlineExcerptText(entity, 360) : sourceEntityExcerptText(entity);
  const facts = renderSourceEntityFacts(entity);
  if (compact) {
    return `<span class="vtext-transclusion-body vtext-transclusion-body--compact" data-vtext-transclusion-body>
      ${snapshot ? `<span class="vtext-transclusion-quote">${renderInlineMarkdown(snapshot, [])}</span>` : ''}
      ${sourceEntityMedia(entity, { inline: true })}
      ${facts.trim() ? `<span class="vtext-source-facts">${facts}</span>` : ''}
    </span>`;
  }
  const media = sourceEntityMedia(entity);
  return `<div class="vtext-transclusion-body" data-vtext-transclusion-body>
    ${snapshot ? `<blockquote class="vtext-transclusion-quote">${renderInlineMarkdown(snapshot, [])}</blockquote>` : ''}
    ${media}
    ${facts.trim() ? `<div class="vtext-source-facts">${facts}</div>` : ''}
  </div>`;
}

export function renderInlineSourceRef(label: string, entityID: string, sourceEntities: any[] = []): string {
  const entity = findSourceEntity(sourceEntities, entityID);
  const displayLabel = label || entity?.label || 'source';
  if (!entity) {
    return `<span class="vtext-source-ref vtext-source-ref--missing" data-vtext-source-ref data-source-entity-id="${escapeHTML(entityID)}" data-source-label="${escapeHTML(displayLabel)}" contenteditable="false">${escapeHTML(displayLabel)}</span>`;
  }
  const title = sourceEntityTitle(entity);
  const marker = sourceEntities.indexOf(entity) + 1 || '';
  const expansionSurface = sourceEntityExpansionSurface(entity);
  return `<span class="vtext-source-ref" data-vtext-source-ref data-vtext-citation-transclusion data-source-entity-id="${escapeHTML(entityID)}" data-source-label="${escapeHTML(displayLabel)}" data-source-expansion-surface="${escapeHTML(expansionSurface)}" contenteditable="false" tabindex="0" role="button" aria-label="${escapeHTML(`Source: ${title}`)}">
    <span class="vtext-source-ref-label">${escapeHTML(marker || displayLabel)}</span>
    <span class="vtext-source-ref-popover" data-vtext-source-ref-popover data-vtext-inline-transclusion role="note">
      <strong>${escapeHTML(title)}</strong>
      ${renderSourceTransclusionBody(entity, { compact: true })}
      <button type="button" class="vtext-source-open" data-vtext-open-source data-source-entity-id="${escapeHTML(entityID)}">Open source</button>
    </span>
  </span>`;
}

export function renderInlineMarkdown(value: unknown, sourceEntities: any[] = []): string {
  let html = escapeHTML(value);
  html = html.replace(/\[([^\]]+)\]\(source:([^)]+)\)/g, (_match, label, entityID) =>
    renderInlineSourceRef(label, entityID, sourceEntities)
  );
  html = html.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>');
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>');
  html = html.replace(/`([^`\n]+)`/g, '<code>$1</code>');
  return html;
}
