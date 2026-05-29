export const demoFiles = [
  { name: 'Mission Briefs', type: 'directory', size: 0, modified: '2026-05-29T06:10:00Z' },
  { name: 'Media Library', type: 'directory', size: 0, modified: '2026-05-29T06:12:00Z' },
  { name: 'node-a-redesign-notes.md', type: 'file', size: 18432, modified: '2026-05-29T06:18:00Z' },
  { name: 'trace-hard-cutover.pdf', type: 'file', size: 934120, modified: '2026-05-29T06:20:00Z' },
  { name: 'london-salmon-window.png', type: 'file', size: 512044, modified: '2026-05-29T06:23:00Z' },
  { name: 'podcast-briefing.mp3', type: 'file', size: 8204450, modified: '2026-05-29T06:25:00Z' },
  { name: 'launch-review.epub', type: 'file', size: 1458892, modified: '2026-05-29T06:31:00Z' },
];

export const demoFolderEntries: Record<string, any[]> = {
  'Mission Briefs': [
    { name: 'hard-cutover-plan.md', type: 'file', size: 22108, modified: '2026-05-29T06:22:00Z' },
    { name: 'node-a-inventory.json', type: 'file', size: 6204, modified: '2026-05-29T06:24:00Z' },
  ],
  'Media Library': [
    { name: 'shell-motion-study.mp4', type: 'file', size: 18204450, modified: '2026-05-29T06:28:00Z' },
    { name: 'tetramark-reference.svg', type: 'file', size: 4032, modified: '2026-05-29T06:30:00Z' },
  ],
};

export const demoComputeStatus = {
  status: 'ok',
  current_computer: {
    current: true,
    role: 'primary',
    desktop_id: 'public-preview',
    state: 'active',
    warmness_class: 'warm',
    protection: 'Preview computer is local to this browser; durable state starts after sign-in.',
    reclaimable: false,
  },
  computers: [
    {
      current: true,
      role: 'primary',
      desktop_id: 'public-preview',
      state: 'active',
      warmness_class: 'warm',
      protection: 'Local preview',
    },
    {
      role: 'candidate',
      desktop_id: 'node-a-design-lab',
      state: 'paused',
      warmness_class: 'hibernated',
      protection: 'Candidate preview sample',
    },
  ],
  runtime: {
    reachable: true,
    runtime_health: 'preview',
    running_runs: 2,
  },
  capabilities: {
    wake_current_computer: false,
  },
  samples: [
    { label: 'CPU', value: 32 },
    { label: 'Memory', value: 58 },
    { label: 'I/O', value: 18 },
    { label: 'Restore', value: 41 },
  ],
  events: [
    'Window restore weight settled under threshold',
    'Trace preview stream completed cleanly',
    'VText local draft is unsaved',
  ],
};

export const demoTraceTrajectories = [
  {
    trajectory_id: 'demo-redesign-cutover',
    title: 'Node A redesign cutover',
    subtitle: 'Deploy lab, switch shell ontology, verify preview surfaces',
    state: 'running',
    live: true,
    agent_count: 4,
    delegation_count: 3,
    moment_count: 14,
    message_count: 48,
    finding_count: 7,
    search_attempt_count: 2,
    latest_activity_at: '2026-05-29T06:44:00Z',
  },
  {
    trajectory_id: 'demo-vtext-versioning',
    title: 'VText version progression',
    subtitle: 'Local draft to revision animation with save boundary',
    state: 'completed',
    live: false,
    agent_count: 3,
    delegation_count: 1,
    moment_count: 9,
    message_count: 24,
    finding_count: 4,
    search_attempt_count: 0,
    latest_activity_at: '2026-05-29T06:35:00Z',
  },
];

export const demoTraceSnapshot = {
  trajectory: demoTraceTrajectories[0],
  agents: [
    { agent_id: 'super', label: 'super', role: 'orchestrator', profile: 'foreground', state: 'running' },
    { agent_id: 'vsuper-node-a', label: 'Node A vsuper', role: 'candidate computer', profile: 'design lab', state: 'running' },
    { agent_id: 'verifier-visual', label: 'visual verifier', role: 'verifier', profile: 'Computer Use', state: 'pending' },
    { agent_id: 'researcher-fixtures', label: 'fixture researcher', role: 'researcher', profile: 'frontend-only', state: 'completed' },
  ],
  edges: [
    { from_agent_id: 'super', to_agent_id: 'vsuper-node-a', kind: 'delegation' },
    { from_agent_id: 'super', to_agent_id: 'verifier-visual', kind: 'verification' },
    { from_agent_id: 'vsuper-node-a', to_agent_id: 'researcher-fixtures', kind: 'data-flow' },
  ],
  moments: [
    { moment_id: 'm1', kind: 'inventory.recorded', state: 'completed', title: 'Node A pre-wipe facts recorded', created_at: '2026-05-29T06:47:00Z', loop_id: 'demo-redesign-cutover' },
    { moment_id: 'm2', kind: 'deploy.path.blocker', state: 'blocked', title: 'Branch CI needs Node A deploy authority', created_at: '2026-05-29T06:48:00Z', loop_id: 'demo-redesign-cutover' },
    { moment_id: 'm3', kind: 'frontend.prompt_surface', state: 'running', title: 'PromptSurface replaces the old shelf', created_at: '2026-05-29T06:55:00Z', loop_id: 'demo-redesign-cutover' },
    { moment_id: 'm4', kind: 'verification.visual', state: 'pending', title: 'Computer Use desktop/mobile pass', created_at: '2026-05-29T07:05:00Z', loop_id: 'demo-redesign-cutover' },
  ],
  search: {
    providers: [
      { provider: 'fixtures', status: 'ok', merged_count: 6 },
      { provider: 'live-private-trace', status: 'auth-required', merged_count: 0 },
    ],
  },
  acceptances: [
    {
      acceptance_id: 'demo-visual-review',
      target_mission_id: 'node-a-redesign-preview',
      state: 'synthesized',
      acceptance_level: 'visual-preview-level',
      authority_profile: 'logged-out local fixture',
      deployment_commit: 'frontend-demo',
      staging_url: 'local preview',
      loop_id: 'demo-redesign-cutover',
      trajectory_id: 'demo-redesign-cutover',
      checkpoints: [
        {
          kind: 'logged_out_preview',
          state: 'synthesized',
          evidence_ref_ids: ['demo-vtext-preview', 'demo-trace-preview'],
          details: {
            note: 'Fixture data is visual-only and is not backend proof.',
          },
        },
      ],
      evidence_refs: [
        {
          ref_id: 'demo-vtext-preview',
          kind: 'frontend-fixture',
          summary: 'VText version preview is visible before sign-in.',
        },
        {
          ref_id: 'demo-trace-preview',
          kind: 'frontend-fixture',
          summary: 'Trace trajectory and acceptance surfaces render with local data.',
        },
      ],
      rollback_refs: [],
      verifier_contracts: [
        {
          name: 'Computer Use visual pass',
          state: 'pending',
          purpose: 'Owner-facing review of the Node A design lab.',
        },
      ],
    },
  ],
};

export const demoVTextDocument = {
  doc_id: 'demo-vtext-redesign',
  title: 'Node A redesign morning review',
  content: [
    '# Node A redesign morning review',
    '',
    'The public preview is intentionally useful while logged out. It shows the shell, Desk, VText, Trace, Files, media, compute telemetry, and Email preview with frontend-only fixture data.',
    '',
    'Saving, publishing, importing, sending, activating, and provider-spend actions still request auth at the moment of action.',
  ].join('\n'),
  revisions: [
    { revision_id: 'v1', label: 'v1', title: 'Inventory', summary: 'Node A facts and deploy-path mismatch recorded.' },
    { revision_id: 'v2', label: 'v2', title: 'Shell Cutover', summary: 'PromptSurface and DeskSheet replace bottom-only UI.' },
    { revision_id: 'v3', label: 'v3', title: 'Reviewable Preview', summary: 'Fixture-backed surfaces are coherent enough for owner QA.' },
  ],
};

export const demoEmailMessages = [
  {
    id: 'demo-email-1',
    direction: 'inbound',
    from_address: 'review@choir.local',
    subject: 'Morning review checklist',
    snippet: 'Please verify PromptSurface top and bottom placement, three themes, Trace swimlanes, and auth prompts.',
    trust_status: 'public',
    received_at: '2026-05-29T06:42:00Z',
    has_attachments: false,
  },
  {
    id: 'demo-email-2',
    direction: 'draft',
    from_address: 'preview@choir-ip.com',
    subject: 'Draft: Node A visual cut notes',
    snippet: 'This is a preview draft. Sending requires sign-in and a real mailbox.',
    trust_status: 'draft',
    created_at: '2026-05-29T06:45:00Z',
    has_attachments: false,
  },
];

export const demoPodcastItems = [
  {
    content_id: 'demo-podcast-1',
    title: 'Design Systems Field Notes',
    source_url: 'https://example.com/design-systems.rss',
    media_type: 'application/rss+xml',
    app_hint: 'podcast',
    text_content: `<?xml version="1.0"?><rss><channel><title>Design Systems Field Notes</title><description>Operating-surface critiques and product taste reviews.</description><item><title>Hard cutovers that work</title><description>Why naming and deletion matter.</description><pubDate>Fri, 29 May 2026 06:00:00 GMT</pubDate><enclosure url="https://example.com/audio.mp3" type="audio/mpeg"/></item></channel></rss>`,
  },
];
