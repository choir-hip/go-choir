// Model provenance of a Texture document
// (docs/problems/model-provenance-unwired-2026-11-05.md): the models that
// actually served the runs of its trajectory, by desk role, from the
// computer's model.served record (GET /api/texture/documents/{id}/models),
// and pinned into each published version's manifest (model_provenance, the
// public projection: no run ids, document ids or policy sources). It
// is what ran, never the model policy's prediction.

export type ProvenanceModel = {
  provider?: string;
  model?: string;
  reported_model?: string;
  calls?: number;
  runs?: number;
  input_tokens?: number;
  output_tokens?: number;
  via?: string[];
  source?: string;
  delegate?: boolean;
};

export type ProvenanceSwitch = {
  run_id?: string;
  role?: string;
  from_provider?: string;
  from_model?: string;
  to_provider?: string;
  to_model?: string;
  reason?: string;
};

export type ModelProvenance = {
  schema?: string;
  runs?: number;
  runs_served?: number;
  roles?: { role?: string; models?: ProvenanceModel[] }[];
  switches?: ProvenanceSwitch[];
};

export type ProvenanceRow = {
  role: string;
  label: string;
  models: { key: string; name: string; detail: string; delegate: boolean }[];
};

// Desk order of the roles; an unknown role sorts after these, by name.
const ROLE_ORDER = ['conductor', 'texture', 'research', 'research_child', 'management', 'management_task', 'engineering', 'verifier', 'system1'];

const ROLE_LABELS: Record<string, string> = {
  conductor: 'Conductor',
  texture: 'Texture',
  research: 'Research',
  research_child: 'Research children',
  management: 'Management',
  management_task: 'Management tasks',
  engineering: 'Engineering',
  verifier: 'Verifier',
  system1: 'System 1 judge',
};

const VIA_LABELS: Record<string, string> = {
  selected: '',
  selected_unrecorded: 'chosen, before call logging',
  model_failover: 'pool fallback',
  provider_quota: 'quota failover',
  provider_quota_retry: 'quota failover',
  'claude-delegate': '',
};

export function roleLabel(role: string): string {
  return ROLE_LABELS[role] || role.replace(/_/g, ' ');
}

function viaLabel(via: string): string {
  return via in VIA_LABELS ? VIA_LABELS[via] : via.replace(/_/g, ' ');
}

function count(n: unknown): number {
  const v = Number(n);
  return Number.isFinite(v) && v > 0 ? Math.floor(v) : 0;
}

function tokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${Math.round(n / 1_000)}k`;
  return String(n);
}

export function modelName(m: ProvenanceModel): string {
  const model = String(m?.model || 'unknown model');
  if (m?.delegate) return `Claude delegate (${model})`;
  return m?.provider ? `${m.provider} / ${model}` : model;
}

export function modelDetail(m: ProvenanceModel): string {
  const parts: string[] = [];
  const calls = count(m?.calls);
  if (calls) parts.push(`${calls} ${m?.delegate ? 'delegate run' : 'call'}${calls === 1 ? '' : 's'}`);
  const reported = String(m?.reported_model || '');
  if (reported && reported !== m?.model) parts.push(`reported as ${reported}`);
  const tin = count(m?.input_tokens);
  const tout = count(m?.output_tokens);
  if (tin || tout) parts.push(`${tokens(tin)} in / ${tokens(tout)} out`);
  // A model with recorded calls needs no "before call logging" note: a mix
  // of recorded and older runs shows the recorded counts only.
  const via = (Array.isArray(m?.via) ? m.via : [])
    .filter((v) => !(v === 'selected_unrecorded' && calls > 0))
    .map(viaLabel)
    .filter(Boolean);
  if (via.length) parts.push(via.join(', '));
  return parts.join(' · ');
}

/** Rows for display: roles in desk order, each with its served models. */
export function modelProvenanceRows(provenance: ModelProvenance | null | undefined): ProvenanceRow[] {
  const roles = Array.isArray(provenance?.roles) ? provenance!.roles! : [];
  const rows = roles
    .map((r) => {
      const role = String(r?.role || 'unknown');
      const models = (Array.isArray(r?.models) ? r.models : []).map((m) => ({
        key: `${m?.delegate ? 'delegate' : m?.provider || ''}/${m?.model || ''}`,
        name: modelName(m),
        detail: modelDetail(m),
        delegate: Boolean(m?.delegate),
      }));
      return { role, label: roleLabel(role), models };
    })
    .filter((row) => row.models.length > 0);
  const rank = (role: string) => {
    const i = ROLE_ORDER.indexOf(role);
    return i < 0 ? ROLE_ORDER.length : i;
  };
  rows.sort((a, b) => rank(a.role) - rank(b.role) || a.role.localeCompare(b.role));
  return rows;
}

/** One line per mid-run switch: role, from, to and why. */
export function modelSwitchLines(provenance: ModelProvenance | null | undefined): string[] {
  const switches = Array.isArray(provenance?.switches) ? provenance!.switches! : [];
  return switches.map((s) => {
    const from = s?.from_model ? `${s.from_provider ? `${s.from_provider} / ` : ''}${s.from_model}` : 'unknown';
    const to = s?.to_model ? `${s.to_provider ? `${s.to_provider} / ` : ''}${s.to_model}` : 'unknown';
    const why = viaLabel(String(s?.reason || '')) || String(s?.reason || '');
    return `${roleLabel(String(s?.role || 'unknown'))}: ${from} → ${to}${why ? ` (${why})` : ''}`;
  });
}

function escapeHTML(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

/** The "Models" section of the published Sources panel; empty when the version carries none. */
export function renderModelProvenanceHTML(provenance: ModelProvenance | null | undefined): string {
  const rows = modelProvenanceRows(provenance);
  if (!rows.length) return '';
  const items = rows
    .map(
      (row) =>
        `<div class="texture-model-provenance-role" data-texture-model-role="${escapeHTML(row.role)}">` +
        `<dt>${escapeHTML(row.label)}</dt>` +
        row.models
          .map(
            (m) =>
              `<dd data-texture-model="${escapeHTML(m.key)}"${m.delegate ? ' data-texture-model-delegate' : ''}>` +
              `<span class="texture-model-name">${escapeHTML(m.name)}</span>` +
              (m.detail ? ` <span class="texture-model-detail">${escapeHTML(m.detail)}</span>` : '') +
              `</dd>`,
          )
          .join('') +
        `</div>`,
    )
    .join('');
  const switches = modelSwitchLines(provenance);
  const switchHTML = switches.length
    ? `<p class="texture-model-switches-heading">Model switches</p><ul class="texture-model-switches">${switches
        .map((line) => `<li data-texture-model-switch>${escapeHTML(line)}</li>`)
        .join('')}</ul>`
    : '';
  return (
    `<section class="texture-model-provenance" data-texture-model-provenance contenteditable="false">` +
    `<h2 class="texture-model-provenance-heading">Models</h2>` +
    `<dl>${items}</dl>${switchHTML}</section>`
  );
}

/** The model provenance a published bundle's version carries, if any. */
export function publishedModelProvenance(bundle: any): ModelProvenance | null {
  let manifest = bundle?.payload?.manifest;
  if (typeof manifest === 'string') {
    try {
      manifest = JSON.parse(manifest);
    } catch {
      return null;
    }
  }
  const provenance = manifest?.model_provenance;
  return provenance && typeof provenance === 'object' ? provenance : null;
}
