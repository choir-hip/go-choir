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
} = {}) {
  const cleanMarker = String(marker || '').trim();
  const cleanTitle = String(title || '').trim();
  const cleanExcerpt = String(excerpt || '').trim();
  const cleanURL = String(url || '').trim();
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
          },
        ],
        display: {
          inline_mode: 'embedded_excerpt',
          expanded_mode: 'source_card',
          open_surface: cleanURL ? 'browser' : 'source',
          default_collapsed: true,
        },
        evidence: {
          state: 'available',
          research_state: 'confirmed',
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
      },
    ],
  };
}
