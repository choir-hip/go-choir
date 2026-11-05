export const previewComputeStatus = {
  status: 'preview',
  current_computer: {
    current: true,
    role: 'public-preview',
    desktop_id: 'public-preview',
    state: 'local',
    warmness_class: 'browser',
    protection: 'This preview is local to the browser. Private computer state starts after sign-in.',
    reclaimable: false,
  },
  computers: [
    {
      current: true,
      role: 'public-preview',
      desktop_id: 'public-preview',
      state: 'local',
      warmness_class: 'browser',
      protection: 'Local preview only',
    },
    {
      role: 'private-computer',
      desktop_id: 'sign-in-required',
      state: 'locked',
      warmness_class: 'auth-required',
      protection: 'Sign in to inspect or mutate your durable computer.',
    },
  ],
  runtime: {
    reachable: true,
    runtime_health: 'public-preview',
    running_runs: 0,
  },
  persistent_disk: {
    source: 'public-preview',
    used_bytes: 2 * 1024 * 1024 * 1024,
    total_bytes: 8 * 1024 * 1024 * 1024,
    avail_bytes: 6 * 1024 * 1024 * 1024,
    cap_bytes: 8 * 1024 * 1024 * 1024,
    used_percent: 25,
    warning: false,
    critical: false,
    default_cap_bytes: 8 * 1024 * 1024 * 1024,
  },
  capabilities: {
    wake_current_computer: false,
  },
  samples: [
    { label: 'CPU', value: 12 },
    { label: 'Memory', value: 28 },
    { label: 'I/O', value: 8 },
    { label: 'Queue', value: 0 },
  ],
  events: [
    'Public shell loaded locally',
    'Private compute telemetry is locked until sign-in',
    'Durable recovery controls are disabled in preview',
  ],
};

/**
 * The signed-out Texture document: "Introducing Choir".
 *
 * This is the third pane of the landing deck, restated. The deck says the
 * thing in two slides; this says all of it in prose, for the visitor who
 * scrolled to the machine and wants to read rather than be told.
 *
 * It used to be titled "What Choir Is" and opened with "a private,
 * Texture-centered computer for durable knowledge work" — accurate, and
 * meaningless to anyone not already briefed on the ontology. It named the
 * mechanism before the reader had a reason to want it.
 *
 * The shape now: the shared failure, the reframe, the machine, why you can
 * trust it, and what you can actually do. Jargon is introduced only after
 * the reader has a reason to want it.
 */
export const previewTextureDocument = {
  doc_id: 'preview-texture',
  title: 'Introducing Choir',
  content: [
    '# Introducing Choir',
    '',
    '### A new species — of computer',
    '',
    'Every AI session starts from zero. You re-explain the project, it re-guesses the rules, and the thread dies with the tab. That is fine for ten minutes of thinking and useless for three months of work.',
    '',
    '**Choir is not a chat. It is a computer.** A persistent machine made of many agents that coordinate over months instead of minutes. It keeps a versioned record of everything it does — every draft, every claim, every reversal — so the work survives you closing the tab.',
    '',
    '*Persistent. Versioned. Reversible. Self-improving.*',
    '',
    '## How it is put together',
    '',
    'Four desks run on one machine, and you sit above all of them.',
    '',
    '- **Texture** writes the durable documents.',
    '- **Management** decides what is worth running.',
    '- **Engineering** does the building, inside a sandbox it cannot escape.',
    '- **Research** goes and gets evidence from the world.',
    '',
    'You state the intent. They do the work, and they show you the receipts.',
    '',
    '## Why you can trust it',
    '',
    'Nothing moves that you cannot read, cite, or undo. Every state change is a typed event with evidence attached, and every one of them rolls back.',
    '',
    'Before the agents act, they commit to typed predictions. Afterwards those commitments are scored. The accumulated log is how the machine learns, and it is yours to inspect.',
    '',
    'That discipline applies to Choir itself. The computer proposes changes to its own environment, tools, and operating rules under the same evidence-and-approval rules as any other change. It is the essential capability, not a feature.',
    '',
    '## What you get',
    '',
    'A **web desktop** for durable writing, sources, files, and a repair console. A **native macOS app** wrapping the same computer. A **CLI** for agents and scripts.',
    '',
    'All three are projections of one persistent machine. Not three products that disagree.',
    '',
    '---',
    '',
    '*You are looking at a live local preview. Sign in to connect your own durable computer: your documents keep their revisions, sources stay attached, and the agents have somewhere to work.*',
  ].join('\n'),
  revisions: [
    {
      revision_id: 'v1',
      label: 'v1',
      title: 'A new species',
      summary: 'Why a session is the wrong unit of work, and what replaces it.',
    },
    {
      revision_id: 'v2',
      label: 'v2',
      title: 'The machine',
      summary: 'Four desks, one computer, you on top.',
    },
    {
      revision_id: 'v3',
      label: 'v3',
      title: 'The receipts',
      summary: 'Every change typed, evidenced, and reversible.',
    },
  ],
};
