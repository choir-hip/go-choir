// The front-end kernel. Five responsibilities and nothing else: the scope tree,
// effects (every registration returns a disposer, reversed LIFO and awaited),
// service resolution (ranked providers, consumers, Cordis unload order),
// pipelines and a read-only snapshot(). No proxy magic, no config tree, no HMR.
// The kernel imports only itself and knows no package, app or desk.
//
// Failure modes (the per-file lists carry the rest):
//  K1 a registration that leaks: none exists without a disposer, and the leak
//     differential (install, activate, exercise, uninstall) must leave
//     snapshot() equal to the baseline.
//  K2 the kernel grows an unlisted capability: responsibilities.json lists the
//     exported symbols and scripts/check-kernel.mjs fails on any difference.
//  K3 the kernel starts to know a package: check-kernel.mjs fails on any
//     package, app or desk name appearing in kernel source.

import { Pipelines } from './pipeline';
import { Scope } from './scope';
import { Services, type WatchdogOptions } from './service';
import { takeSnapshot, type Snapshot } from './snapshot';

export type { Disposer, DisposeReport } from './effect';
export type { Item, Handler, HandlerSpec, PipelineContext, PipelineResult } from './pipeline';
export type { ProviderSpec, ConsumeFn, Tier, WatchdogOptions } from './service';
export type { Snapshot } from './snapshot';
export { Scope } from './scope';
export { Services } from './service';
export { Pipelines } from './pipeline';

export interface KernelOptions {
  now?: () => number;
  watchdog?: WatchdogOptions;
}

export interface Kernel {
  readonly root: Scope;
  readonly services: Services;
  readonly pipelines: Pipelines;
  /** Fork a child scope of `parent` (default: the root). */
  scope(name: string, parent?: Scope): Scope;
  snapshot(): Snapshot;
}

export function createKernel(opts: KernelOptions = {}): Kernel {
  let ids = 0;
  const root = new Scope(0, 'root', null, () => undefined);
  const services = new Services(opts.now ?? (() => Date.now()), opts.watchdog ?? { failures: 3, windowMs: 10_000 });
  const pipelines = new Pipelines();
  return {
    root,
    services,
    pipelines,
    scope: (name, parent = root) => parent.fork(name, () => ++ids + 1_000_000),
    snapshot: () => takeSnapshot(root, services, pipelines),
  };
}
