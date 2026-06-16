function safeSourceIDPart(value) {
  const normalized = String(value || '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');
  return normalized || 'source';
}

export function sourceReviewEntityID({ marker = '', title = '', revisionID = '' } = {}) {
  const markerPart = safeSourceIDPart(String(marker || '').replace(/[\[\]]/g, ''));
  const titlePart = safeSourceIDPart(title).slice(0, 48);
  const revisionPart = safeSourceIDPart(revisionID).slice(0, 12);
  return ['src_review', markerPart, titlePart, revisionPart].filter(Boolean).join('_');
}

export function buildSourceReviewPayload({
  marker = '',
  title = '',
  excerpt = '',
  url = '',
  revisionID = '',
  relation = 'confirms',
  reason = '',
} = {}) {
  const cleanMarker = String(marker || '').trim();
  const cleanTitle = String(title || '').trim();
  const cleanExcerpt = String(excerpt || '').trim();
  const cleanURL = String(url || '').trim();
  const cleanRelation = normalizeSourceReviewRelation(relation);
  const cleanReason = String(reason || '').trim();
  if (cleanRelation === 'no_source_needed') {
    return {
      base_revision_id: revisionID,
      citation_resolutions: [
        {
          marker: cleanMarker,
          action: 'no_source_needed',
          reason: cleanReason,
        },
      ],
    };
  }
  const entityID = sourceReviewEntityID({
    marker: cleanMarker,
    title: cleanTitle,
    revisionID,
  });
  const target = cleanURL
    ? {
      target_kind: 'url',
      url: cleanURL,
      canonical_url: cleanURL,
    }
    : {
      target_kind: 'source_service_item',
      item_id: entityID,
    };

  return {
    base_revision_id: revisionID,
    source_entities: [
      {
        entity_id: entityID,
        kind: cleanURL ? 'web_source' : 'source_service_item',
        label: cleanTitle,
        target,
        selectors: [
          {
            selector_kind: 'text_quote',
            text_quote: cleanExcerpt,
            relation: cleanRelation,
            supports: sourceReviewRelationLabel(cleanRelation),
          },
        ],
        display: {
          inline_mode: 'embedded_excerpt',
          expanded_mode: 'source_card',
          open_surface: 'source',
          default_collapsed: true,
        },
        evidence: {
          state: cleanRelation,
          research_state: 'owner_supplied',
          relation: cleanRelation,
        },
        provenance: {
          created_by: 'source_review_panel',
          rights_scope: 'public_source',
          untrusted_source_text: true,
        },
      },
    ],
    citation_resolutions: [
      {
        marker: cleanMarker,
        entity_id: entityID,
        action: 'link_source',
      },
    ],
  };
}

export function normalizeSourceReviewRelation(value) {
  const normalized = String(value || '').trim().toLowerCase();
  if (['refutes', 'refuting', 'refute'].includes(normalized)) return 'refutes';
  if (['qualifies', 'qualifying', 'qualify'].includes(normalized)) return 'qualifies';
  if (['no_source_needed', 'no-source-needed', 'no_source', 'omit', 'remove'].includes(normalized)) return 'no_source_needed';
  return 'confirms';
}

function sourceReviewRelationLabel(relation) {
  switch (relation) {
    case 'refutes':
      return 'refutes claim';
    case 'qualifies':
      return 'qualifies claim';
    default:
      return 'confirms claim';
  }
}
