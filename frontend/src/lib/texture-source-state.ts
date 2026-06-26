import {
  publicationBundleSourceEntities,
  sourceEntityID,
} from './texture-source-renderer';

function objectValue(value: any): any {
  return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
}

function stringValue(value: any): string {
  return String(value || '').trim();
}

function firstString(...values: any[]): string {
  for (const value of values) {
    const normalized = stringValue(value);
    if (normalized) return normalized;
  }
  return '';
}

function isWebURL(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

function graphObjectKey(canonicalID: any, versionID: any = ''): string {
  const canonical = stringValue(canonicalID);
  const version = stringValue(versionID);
  if (!canonical) return '';
  return version ? `${canonical}\x00${version}` : canonical;
}

function graphEntityWrapperKey(wrapper: any): string {
  return graphObjectKey(wrapper?.canonical_id, wrapper?.version_id);
}

function graphRefEntityKey(ref: any): string {
  return graphObjectKey(ref?.source_entity_canonical_id, ref?.source_entity_version_id);
}

function targetIDKind(kind: string): boolean {
  const normalized = stringValue(kind).toLowerCase();
  return normalized === 'content_item' || normalized === 'source_service_item';
}

function sourceGraphEntityToLocal(wrapper: any, ref: any = null): any | null {
  const metadata = objectValue(wrapper?.metadata);
  const targetMetadata = objectValue(metadata.target);
  const displayMetadata = objectValue(metadata.display);
  const evidenceMetadata = objectValue(metadata.evidence);
  const provenanceMetadata = objectValue(metadata.provenance);
  const targetNestedMetadata = objectValue(targetMetadata.metadata);

  const sourceKind = firstString(metadata.source_kind, targetMetadata.kind, wrapper?.object_kind);
  const entityID = firstString(
    ref?.legacy_source_entity_id,
    wrapper?.legacy_source_entity_id,
    metadata.legacy_entity_id,
    metadata.legacy_source_entity_id,
    metadata.legacy_source_entity,
    wrapper?.canonical_id,
  );
  if (!entityID) return null;

  const targetKind = firstString(targetMetadata.target_kind, targetMetadata.kind, sourceKind);
  const targetIdentity = firstString(
    targetMetadata.identity,
    targetMetadata.uri,
    targetMetadata.url,
    targetMetadata.canonical_url,
    targetMetadata.id,
  );
  const sourceURL = firstString(
    targetMetadata.uri,
    targetMetadata.url,
    targetMetadata.canonical_url,
    targetNestedMetadata.canonical_url,
    targetNestedMetadata.url,
    isWebURL(targetIdentity) ? targetIdentity : '',
  );
  const targetID = firstString(
    targetMetadata.id,
    targetIDKind(targetKind) && !isWebURL(targetIdentity) ? targetIdentity : '',
  );
  const target = {
    ...targetMetadata,
    kind: targetKind,
    target_kind: firstString(targetMetadata.target_kind, targetKind),
    id: targetID,
  };
  if (sourceURL) {
    target.uri = target.uri || sourceURL;
    target.url = target.url || sourceURL;
    target.canonical_url = target.canonical_url || sourceURL;
  }
  if (targetKind === 'content_item' && !target.content_id) target.content_id = targetID;
  if (targetKind === 'source_service_item' && !target.item_id) target.item_id = targetID;

  const display = {
    ...displayMetadata,
    mode: firstString(displayMetadata.mode, displayMetadata.display_mode, displayMetadata.inline_mode),
    inline_mode: firstString(displayMetadata.inline_mode, displayMetadata.display_policy, displayMetadata.mode, displayMetadata.display_mode),
    title: firstString(displayMetadata.title, targetMetadata.title, entityID),
    label: firstString(displayMetadata.label, ref?.legacy_source_entity_id, wrapper?.legacy_source_entity_id, entityID),
    open_surface: firstString(displayMetadata.open_surface, evidenceMetadata.open_surface),
  };
  const readerSnapshot = objectValue(metadata.reader_snapshot);
  const body = stringValue(wrapper?.body);
  if (body && !readerSnapshot.text_content) readerSnapshot.text_content = body;
  if (sourceURL && !readerSnapshot.source_url) readerSnapshot.source_url = sourceURL;
  const readerSnapshotStatus = objectValue(metadata.reader_snapshot_status);
  if (body && !readerSnapshotStatus.state) readerSnapshotStatus.state = 'reader_snapshot_ready';

  const entity: any = {
    entity_id: entityID,
    source_entity_id: entityID,
    kind: sourceKind,
    label: display.label || display.title || entityID,
    target,
    selectors: Array.isArray(metadata.selectors) ? metadata.selectors : [],
    display,
    evidence: evidenceMetadata,
    provenance: provenanceMetadata,
    graph: {
      object_kind: wrapper?.object_kind || 'choir.source_entity',
      canonical_id: stringValue(wrapper?.canonical_id),
      version_id: stringValue(wrapper?.version_id),
      content_hash: stringValue(wrapper?.content_hash),
      owner_id: stringValue(wrapper?.owner_id),
      computer_id: stringValue(wrapper?.computer_id),
    },
  };
  if (Object.keys(readerSnapshot).length > 0) entity.reader_snapshot = readerSnapshot;
  if (Object.keys(readerSnapshotStatus).length > 0) entity.reader_snapshot_status = readerSnapshotStatus;
  if (ref) {
    entity.source_ref = {
      object_kind: ref.object_kind || 'choir.source_ref',
      canonical_id: stringValue(ref.canonical_id),
      version_id: stringValue(ref.version_id),
      body_node_id: stringValue(ref.body_node_id),
      body_node_path_hash: stringValue(ref.body_node_path_hash),
      display_mode: stringValue(ref.display_mode),
      citation_state: stringValue(ref.citation_state),
      source_entity_canonical_id: stringValue(ref.source_entity_canonical_id),
      source_entity_version_id: stringValue(ref.source_entity_version_id),
    };
  }
  return sourceEntityID(entity) ? entity : null;
}

function graphWrapperSourceEntities(revision: any): any[] {
  const wrappers = Array.isArray(revision?.source_entity_objects) ? revision.source_entity_objects : [];
  if (wrappers.length === 0) return [];

  const refs = Array.isArray(revision?.source_refs) ? revision.source_refs : [];
  const wrappersByKey = new Map<string, any>();
  for (const wrapper of wrappers) {
    const key = graphEntityWrapperKey(wrapper);
    if (key && !wrappersByKey.has(key)) wrappersByKey.set(key, wrapper);
    const canonicalKey = graphObjectKey(wrapper?.canonical_id);
    if (canonicalKey && !wrappersByKey.has(canonicalKey)) wrappersByKey.set(canonicalKey, wrapper);
  }

  const entities: any[] = [];
  const seen = new Set<string>();
  const append = (wrapper: any, ref: any = null) => {
    const entity = sourceGraphEntityToLocal(wrapper, ref);
    const id = sourceEntityID(entity);
    if (!id || seen.has(id)) return;
    seen.add(id);
    entities.push(entity);
  };

  for (const ref of refs) {
    const wrapper = wrappersByKey.get(graphRefEntityKey(ref));
    if (wrapper) append(wrapper, ref);
  }
  for (const wrapper of wrappers) append(wrapper);
  return entities;
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
  const graphEntities = graphWrapperSourceEntities(revision);
  if (graphEntities.length > 0) return graphEntities;
  return [];
}
