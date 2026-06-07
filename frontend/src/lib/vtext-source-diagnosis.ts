import { versionLabelForRevision } from './vtext-editor-state';

const SOURCE_STRUCTURE_DISPLAY_LIMIT = 24;

export function sourceDiagnosisSummary(diagnosis: any = null) {
  if (!diagnosis) return null;
  const revisions = Array.isArray(diagnosis.revisions) ? diagnosis.revisions : [];
  const structures = Array.isArray(diagnosis.revision_structures) ? diagnosis.revision_structures : [];
  const runs = Array.isArray(diagnosis.runs) ? diagnosis.runs : [];
  const latest = revisions[0] || null;
  const latestStructure = structures[0] || null;
  return {
    revisionCount: revisions.length || structures.length,
    runCount: runs.length,
    latestRevisionId: latest?.revision_id || latestStructure?.revision_id || '',
    latestVersion: latest
      ? versionLabelForRevision(latest, revisions.length - 1)
      : (typeof latestStructure?.version_number === 'number' ? `v${latestStructure.version_number}` : ''),
    latestAuthor: latest ? `${latest.author_kind || ''}:${latest.author_label || ''}` : '',
    errorCount: Array.isArray(diagnosis.error_matches) ? diagnosis.error_matches.length : 0,
    tableCount: structures.reduce((sum: number, item: any) => sum + (Number(item?.table_count) || 0), 0),
    sourceMarkerCount: structures.reduce((sum: number, item: any) => sum + (Number(item?.source_marker_count) || 0), 0),
  };
}

export function sourceStructureEvidence(diagnosis: any = null) {
  const structures = Array.isArray(diagnosis?.revision_structures) ? diagnosis.revision_structures : [];
  return structures.slice(0, SOURCE_STRUCTURE_DISPLAY_LIMIT).map((structure: any) => ({
    revisionID: structure?.revision_id || '',
    version: typeof structure?.version_number === 'number' ? `v${structure.version_number}` : '',
    contentHash: structure?.content_hash || '',
    headingCount: Number(structure?.heading_count) || 0,
    tableCount: Number(structure?.table_count) || 0,
    tableRowCount: Number(structure?.table_row_count) || 0,
    sourceMarkerCount: Number(structure?.source_marker_count) || 0,
    tables: Array.isArray(structure?.tables) ? structure.tables.slice(0, 4).map((table: any) => ({
      index: Number(table?.index) || 0,
      startLine: Number(table?.start_line) || 0,
      endLine: Number(table?.end_line) || 0,
      columnCount: Number(table?.column_count) || 0,
      rowCount: Number(table?.row_count) || 0,
      signature: table?.signature || '',
    })) : [],
  }));
}

function numberMetadataValue(metadata: any, key: string) {
  const value = metadata?.[key];
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
}

function stringMetadataValue(metadata: any, key: string) {
  return String(metadata?.[key] || '').trim();
}

export function revisionEditEvidence(revision: any) {
  const metadata = revision?.metadata || {};
  const contextMode = stringMetadataValue(metadata, 'vtext_context_mode');
  const operation = stringMetadataValue(metadata, 'vtext_edit_operation');
  const promptChars = numberMetadataValue(metadata, 'vtext_run_prompt_chars');
  const editCount = numberMetadataValue(metadata, 'vtext_edit_count');
  const deltaChars = numberMetadataValue(metadata, 'vtext_edit_delta_chars');
  const latencyMs = numberMetadataValue(metadata, 'vtext_run_latency_ms');
  if (!contextMode && !operation && promptChars === null && editCount === null && deltaChars === null && latencyMs === null) {
    return null;
  }
  return {
    revisionID: revision?.revision_id || '',
    version: typeof revision?.version_number === 'number' ? `v${revision.version_number}` : '',
    author: [revision?.author_kind, revision?.author_label].filter(Boolean).join(':'),
    contextMode,
    operation,
    promptChars,
    editCount,
    deltaChars,
    latencyMs,
  };
}

export function sourceEditEvidence(current: any = null, diagnosis: any = null) {
  const revisions = Array.isArray(diagnosis?.revisions) ? diagnosis.revisions : [];
  const candidates = [current, ...revisions].filter(Boolean);
  const seen = new Set();
  for (const revision of candidates) {
    const revisionID = revision?.revision_id || '';
    if (revisionID && seen.has(revisionID)) continue;
    if (revisionID) seen.add(revisionID);
    const evidence = revisionEditEvidence(revision);
    if (evidence) return evidence;
  }
  return null;
}
