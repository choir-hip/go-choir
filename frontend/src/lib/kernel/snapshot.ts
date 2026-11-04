// Read-only snapshot of the kernel: the scope tree, services with their
// providers and consumers, and pipelines with their handlers. Plain data,
// deep frozen, comparable with deepEqual (the leak differential).

import type { Pipelines } from './pipeline';
import type { Scope } from './scope';
import type { Services } from './service';

export type Snapshot = Readonly<Record<string, unknown>>;

function freeze<T>(v: T): T {
  if (v && typeof v === 'object') {
    for (const k of Object.keys(v as object)) freeze((v as Record<string, unknown>)[k]);
    Object.freeze(v);
  }
  return v;
}

export function takeSnapshot(root: Scope, services: Services, pipelines: Pipelines): Snapshot {
  const raw = { scopes: root.describe(), ...services.describe(), pipelines: pipelines.describe() };
  return freeze(JSON.parse(JSON.stringify(raw)));
}
