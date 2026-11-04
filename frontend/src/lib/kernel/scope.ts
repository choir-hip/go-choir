// Scope tree: a package run is a scope; children die with their parent.
//
// Failure modes:
//  S1 child outlives parent: dispose() disposes children (newest first)
//     before the scope's own effects.
//  S2 effect registered on a disposed scope: throws.
//  S3 concurrent dispose(): both callers await the same completion.
//  S4 disposed scope stays in the tree: it is detached from its parent.

import { once, runLifo, type Disposer, type DisposeReport } from './effect';

export type ScopeState = 'active' | 'disposed';

export class Scope {
  readonly id: number;
  readonly name: string;
  readonly parent: Scope | null;
  private state: ScopeState = 'active';
  private readonly children: Scope[] = [];
  private readonly effects: Disposer[] = [];
  private disposing: Promise<DisposeReport> | null = null;
  private readonly onDetach: (scope: Scope) => void;

  constructor(id: number, name: string, parent: Scope | null, onDetach: (scope: Scope) => void) {
    this.id = id;
    this.name = name;
    this.parent = parent;
    this.onDetach = onDetach;
  }

  get disposed(): boolean {
    return this.state === 'disposed';
  }

  /** Register a disposer. Returns a disposer that removes and runs it early. */
  effect(fn: Disposer): Disposer {
    if (this.state !== 'active') throw new Error(`scope ${this.name} is disposed`);
    const wrapped = once(fn);
    this.effects.push(wrapped);
    return async () => {
      const at = this.effects.lastIndexOf(wrapped);
      if (at >= 0) this.effects.splice(at, 1);
      await wrapped();
    };
  }

  fork(name: string, makeId: () => number): Scope {
    if (this.state !== 'active') throw new Error(`scope ${this.name} is disposed`);
    const child = new Scope(makeId(), name, this, (s) => {
      const at = this.children.indexOf(s);
      if (at >= 0) this.children.splice(at, 1);
    });
    this.children.push(child);
    return child;
  }

  dispose(): Promise<DisposeReport> {
    if (this.disposing) return this.disposing;
    this.state = 'disposed';
    this.disposing = (async () => {
      const errors: unknown[] = [];
      while (this.children.length > 0) {
        const child = this.children[this.children.length - 1];
        const r = await child.dispose();
        errors.push(...r.errors);
        const at = this.children.indexOf(child);
        if (at >= 0) this.children.splice(at, 1);
      }
      const own = await runLifo(this.effects);
      errors.push(...own.errors);
      this.onDetach(this);
      return { errors };
    })();
    return this.disposing;
  }

  /** @internal read by snapshot only */
  describe(): { name: string; effects: number; children: ReturnType<Scope['describe']>[] } {
    return {
      name: this.name,
      effects: this.effects.length,
      children: this.children.map((c) => c.describe()),
    };
  }
}
