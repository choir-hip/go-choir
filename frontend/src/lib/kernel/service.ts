// Service resolution: named contracts, ranked providers, consumers that are
// active while every injected service has a provider.
//
// Failure modes:
//  V1 swap leaves a consumer holding the old provider: the order is consumers
//     deactivate, old provider deactivates, new provider activates, consumers
//     activate again (Cordis unload order).
//  V2 a claim outranks the default by assertion: priority above 50 counts only
//     with the rank.high grant.
//  V3 a failing provider keeps serving: N failures inside a window un-rank it
//     with the reason recorded; the next candidate takes over (failover is a
//     pointer move: losers stay loaded and inactive).
//  V4 no healthy candidate: the service has no active provider and consumers
//     stay inactive (the host shows its last-resort surface; not this code).
//  V5 registration entries linger after uninstall: the table drops an entry
//     when it has no contract, provider or consumer.
//  V6 overlapping changes interleave swaps: all changes run on one serial queue.
//  V7 a hook throws while activating: the provider stays inactive and the
//     failure is recorded as a failure against it.

import type { Disposer } from './effect';
import type { Scope } from './scope';

export type Tier = 'policy' | 'user' | 'app' | 'core';
export const TIER_ORDER: Tier[] = ['policy', 'user', 'app', 'core'];

export interface ProviderSpec<T = unknown> {
  id: string;
  impl: T;
  tier?: Tier;
  priority?: number; // 0..100
  base?: boolean; // the shipped default for the slot
  grants?: string[];
  healthy?: boolean;
  installedAt?: number;
  /** Runs when this provider becomes the active one; effects go on `act`. */
  activate?: (act: Scope) => void | Promise<void>;
}

export interface ConsumeFn {
  (services: Record<string, unknown>, act: Scope): void | Promise<void>;
}

interface ProviderRec {
  spec: ProviderSpec;
  owner: Scope;
  seq: number;
  healthy: boolean;
  tripped: string | null;
  failures: number[];
  act: Scope | null;
}

interface ConsumerRec {
  id: number;
  names: string[];
  fn: ConsumeFn;
  owner: Scope;
  act: Scope | null;
}

interface ServiceRec {
  abi: string | null;
  providers: ProviderRec[];
  active: ProviderRec | null;
  consumers: ConsumerRec[];
}

export interface WatchdogOptions {
  failures: number;
  windowMs: number;
}

export class Services {
  private readonly table = new Map<string, ServiceRec>();
  private readonly policy = new Map<string, string>();
  private readonly selection = new Map<string, string>();
  private readonly declined = new Map<string, Set<string>>();
  private queue: Promise<void> = Promise.resolve();
  private seq = 0;
  private consumerSeq = 0;
  readonly trace: string[] = [];

  constructor(
    private readonly now: () => number,
    private readonly watchdog: WatchdogOptions,
  ) {}

  /** Resolves when every queued activation change has run. */
  settled(): Promise<void> {
    return this.queue;
  }

  private enqueue(job: () => Promise<void>): void {
    this.queue = this.queue.then(job).catch(() => undefined);
  }

  private rec(name: string): ServiceRec {
    let r = this.table.get(name);
    if (!r) {
      r = { abi: null, providers: [], active: null, consumers: [] };
      this.table.set(name, r);
    }
    return r;
  }

  private prune(name: string): void {
    const r = this.table.get(name);
    if (r && r.abi === null && r.providers.length === 0 && r.consumers.length === 0) {
      this.table.delete(name);
    }
  }

  define(scope: Scope, name: string, abi: string): Disposer {
    const r = this.rec(name);
    if (r.abi !== null && r.abi !== abi) throw new Error(`service ${name} already defined as ${r.abi}`);
    if (r.abi === abi) throw new Error(`service ${name} is already defined`);
    r.abi = abi;
    return scope.effect(() => {
      r.abi = null;
      this.prune(name);
    });
  }

  provide<T>(scope: Scope, name: string, spec: ProviderSpec<T>): Disposer {
    const r = this.rec(name);
    if (r.providers.some((p) => p.spec.id === spec.id)) {
      this.prune(name);
      throw new Error(`provider ${spec.id} already provides ${name}`);
    }
    const p: ProviderRec = {
      spec: spec as ProviderSpec,
      owner: scope,
      seq: spec.installedAt ?? ++this.seq,
      healthy: spec.healthy ?? true,
      tripped: null,
      failures: [],
      act: null,
    };
    r.providers.push(p);
    this.enqueue(() => this.reconcile(name));
    return scope.effect(async () => {
      const at = r.providers.indexOf(p);
      if (at >= 0) r.providers.splice(at, 1);
      this.enqueue(() => this.reconcile(name));
      await this.settled();
      this.prune(name);
    });
  }

  consume(scope: Scope, names: string[], fn: ConsumeFn): Disposer {
    const c: ConsumerRec = { id: ++this.consumerSeq, names: [...names], fn, owner: scope, act: null };
    for (const n of names) this.rec(n).consumers.push(c);
    this.enqueue(() => this.syncConsumer(c));
    return scope.effect(async () => {
      this.enqueue(async () => {
        await this.stopConsumer(c);
        for (const n of names) {
          const r = this.table.get(n);
          if (r) r.consumers.splice(r.consumers.indexOf(c), 1);
        }
      });
      await this.settled();
      for (const n of names) this.prune(n);
    });
  }

  /** Current implementation, or undefined with no active provider. */
  resolve<T = unknown>(name: string): T | undefined {
    return this.table.get(name)?.active?.spec.impl as T | undefined;
  }

  setPolicy(scope: Scope, slot: string, id: string): Disposer {
    return this.setPref(scope, this.policy, slot, id);
  }

  select(scope: Scope, slot: string, id: string): Disposer {
    return this.setPref(scope, this.selection, slot, id);
  }

  decline(scope: Scope, slot: string, id: string): Disposer {
    let set = this.declined.get(slot);
    if (!set) this.declined.set(slot, (set = new Set()));
    set.add(id);
    this.enqueue(() => this.reconcile(slot));
    return scope.effect(async () => {
      const s = this.declined.get(slot);
      s?.delete(id);
      if (s && s.size === 0) this.declined.delete(slot);
      this.enqueue(() => this.reconcile(slot));
      await this.settled();
    });
  }

  private setPref(scope: Scope, map: Map<string, string>, slot: string, id: string): Disposer {
    const prev = map.get(slot);
    map.set(slot, id);
    this.enqueue(() => this.reconcile(slot));
    return scope.effect(async () => {
      if (prev === undefined) map.delete(slot);
      else map.set(slot, prev);
      this.enqueue(() => this.reconcile(slot));
      await this.settled();
    });
  }

  setHealthy(name: string, id: string, healthy: boolean): void {
    const p = this.table.get(name)?.providers.find((x) => x.spec.id === id);
    if (!p) return;
    p.healthy = healthy;
    this.enqueue(() => this.reconcile(name));
  }

  /** Watchdog: record a throw or budget overrun against a provider (V3). */
  reportFailure(name: string, id: string, reason: string): void {
    const p = this.table.get(name)?.providers.find((x) => x.spec.id === id);
    if (!p || p.tripped) return;
    const t = this.now();
    p.failures = p.failures.filter((f) => t - f < this.watchdog.windowMs);
    p.failures.push(t);
    if (p.failures.length >= this.watchdog.failures) {
      p.tripped = reason;
      this.enqueue(() => this.reconcile(name));
    }
  }

  /** Deterministic ranking (V2). Returns the winner for a slot or null. */
  rank(name: string): ProviderRec | null {
    const r = this.table.get(name);
    if (!r) return null;
    const ok = (p: ProviderRec) => p.healthy && !p.tripped;
    const live = r.providers.filter(ok);
    const byId = (id: string | undefined) => live.find((p) => p.spec.id === id);
    const pol = byId(this.policy.get(name));
    if (pol) return pol;
    const declined = this.declined.get(name) ?? new Set<string>();
    const sel = byId(this.selection.get(name));
    if (sel && !declined.has(sel.spec.id)) return sel;
    const eff = (p: ProviderRec) => {
      const pr = Math.max(0, Math.min(100, p.spec.priority ?? 50));
      return pr > 50 && !(p.spec.grants ?? []).includes('rank.high') ? 50 : pr;
    };
    const cands = live
      .filter((p) => !declined.has(p.spec.id))
      .sort((a, b) => eff(b) - eff(a) || b.seq - a.seq || (a.spec.id < b.spec.id ? -1 : 1));
    if (cands.length > 0) return cands[0];
    return live.find((p) => p.spec.base) ?? null;
  }

  private ready(c: ConsumerRec): boolean {
    return c.names.every((n) => this.table.get(n)?.active != null);
  }

  private async stopConsumer(c: ConsumerRec): Promise<void> {
    const act = c.act;
    c.act = null;
    if (act) {
      this.trace.push(`consumer-off:${c.id}`);
      await act.dispose();
    }
  }

  private async startConsumer(c: ConsumerRec): Promise<void> {
    if (c.act || c.owner.disposed) return;
    const act = c.owner.fork(`consumer:${c.id}`, () => ++this.seq);
    c.act = act;
    const services: Record<string, unknown> = {};
    for (const n of c.names) services[n] = this.table.get(n)?.active?.spec.impl;
    this.trace.push(`consumer-on:${c.id}`);
    try {
      await c.fn(services, act);
    } catch {
      /* a failing consumer stays inactive; its partial effects are reversed */
      c.act = null;
      await act.dispose();
    }
  }

  private async syncConsumer(c: ConsumerRec): Promise<void> {
    if (this.ready(c)) await this.startConsumer(c);
    else await this.stopConsumer(c);
  }

  /** Make the active provider match the ranking, in the V1 order. */
  private async reconcile(name: string): Promise<void> {
    const r = this.table.get(name);
    if (!r) return;
    const next = this.rank(name);
    const prev = r.active;
    if (next === prev) return;
    const consumers = [...r.consumers].reverse();
    for (const c of consumers) await this.stopConsumer(c);
    if (prev) {
      r.active = null;
      this.trace.push(`provider-off:${prev.spec.id}`);
      const act = prev.act;
      prev.act = null;
      if (act) await act.dispose();
    }
    if (next) {
      const act = next.owner.disposed ? null : next.owner.fork(`provider:${next.spec.id}`, () => ++this.seq);
      if (act) {
        next.act = act;
        this.trace.push(`provider-on:${next.spec.id}`);
        try {
          await next.spec.activate?.(act);
          r.active = next;
        } catch (err) {
          next.act = null;
          next.tripped = `activate failed: ${String(err)}`; // V7
          await act.dispose();
          this.enqueue(() => this.reconcile(name));
        }
      }
    }
    for (const c of r.consumers) await this.syncConsumer(c);
  }

  describe() {
    const out: Record<string, unknown> = {};
    for (const [name, r] of [...this.table.entries()].sort((a, b) => (a[0] < b[0] ? -1 : 1))) {
      out[name] = {
        abi: r.abi,
        active: r.active?.spec.id ?? null,
        providers: r.providers.map((p) => ({
          id: p.spec.id,
          tier: p.spec.tier ?? 'app',
          priority: p.spec.priority ?? 50,
          healthy: p.healthy && !p.tripped,
          tripped: p.tripped,
        })),
        consumers: r.consumers.map((c) => ({ id: c.id, names: c.names, active: c.act !== null })),
      };
    }
    return {
      services: out,
      policy: Object.fromEntries([...this.policy.entries()].sort()),
      selection: Object.fromEntries([...this.selection.entries()].sort()),
      declined: Object.fromEntries(
        [...this.declined.entries()].sort().map(([k, v]) => [k, [...v].sort()]),
      ),
    };
  }
}
