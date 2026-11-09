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
  /** Failures of queue jobs (also traced as `job-error:`); never swallowed. */
  readonly errors: unknown[] = [];

  private readonly tripListeners = new Set<(name: string, id: string, reason: string) => void>();

  constructor(
    private readonly now: () => number,
    private readonly watchdog: WatchdogOptions,
  ) {}

  /**
   * Resolves when every queued activation change has run. Never call it from
   * inside a queue job or an act-scope effect: the tail includes the running
   * job, so that wait is circular. Registration disposers therefore do not
   * wait; they enqueue their own cleanup and callers await settled().
   */
  async settled(): Promise<void> {
    // jobs may enqueue more jobs: wait until the tail stops moving
    for (let tail = this.queue; ; tail = this.queue) {
      await tail;
      if (tail === this.queue) return;
    }
  }

  private enqueue(job: () => Promise<void>): void {
    this.queue = this.queue.then(job).catch((err) => {
      this.errors.push(err);
      this.trace.push(`job-error:${err instanceof Error ? err.message : String(err)}`);
    });
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
    return scope.effect(() => {
      const at = r.providers.indexOf(p);
      if (at >= 0) r.providers.splice(at, 1);
      this.enqueue(async () => {
        await this.reconcile(name);
        this.prune(name);
      });
    });
  }

  consume(scope: Scope, names: string[], fn: ConsumeFn): Disposer {
    const c: ConsumerRec = { id: ++this.consumerSeq, names: [...names], fn, owner: scope, act: null };
    for (const n of names) this.rec(n).consumers.push(c);
    this.enqueue(() => this.syncConsumer(c));
    return scope.effect(() => {
      this.enqueue(async () => {
        await this.stopConsumer(c);
        for (const n of names) {
          const r = this.table.get(n);
          if (!r) continue;
          const at = r.consumers.indexOf(c);
          if (at >= 0) r.consumers.splice(at, 1);
          this.prune(n);
        }
      });
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
    return scope.effect(() => {
      const s = this.declined.get(slot);
      s?.delete(id);
      if (s && s.size === 0) this.declined.delete(slot);
      this.enqueue(() => this.reconcile(slot));
    });
  }

  private setPref(scope: Scope, map: Map<string, string>, slot: string, id: string): Disposer {
    const prev = map.get(slot);
    map.set(slot, id);
    this.enqueue(() => this.reconcile(slot));
    return scope.effect(() => {
      if (prev === undefined) map.delete(slot);
      else map.set(slot, prev);
      this.enqueue(() => this.reconcile(slot));
    });
  }

  /**
   * Hear every provider the kernel sets aside, whichever path did it: trip,
   * the watchdog, or a failed activation inside reconcile (L4: a restore of
   * the base package must not depend on who noticed). Returns the disposer.
   */
  onTrip(fn: (name: string, id: string, reason: string) => void): () => void {
    this.tripListeners.add(fn);
    return () => { this.tripListeners.delete(fn); };
  }

  private tripped(name: string, p: ProviderRec, reason: string): void {
    p.tripped = reason;
    for (const fn of this.tripListeners) {
      try { fn(name, p.spec.id, reason); } catch { /* a listener never breaks the kernel */ }
    }
  }

  /**
   * Take a provider out of the ranking at once, with the reason recorded (a
   * surface that failed to mount, a yanked release). Like a watchdog trip it
   * is permanent for this registration: the reason stays in describe() and
   * recovery is to dispose the registration and provide it again (V3).
   * Returns true when this call set it aside (false: unknown, or already).
   */
  trip(name: string, id: string, reason: string): boolean {
    const p = this.table.get(name)?.providers.find((x) => x.spec.id === id);
    if (!p || p.tripped) return false;
    this.tripped(name, p, reason || 'set aside');
    this.enqueue(() => this.reconcile(name));
    return true;
  }

  /**
   * Watchdog: record a throw or budget overrun against a provider (V3). N in
   * the window trip it. Returns true on the report that tripped it, so the
   * caller tells the owner once (decision 4: failover is silent with one
   * visible record).
   */
  reportFailure(name: string, id: string, reason: string): boolean {
    const p = this.table.get(name)?.providers.find((x) => x.spec.id === id);
    if (!p || p.tripped) return false;
    const t = this.now();
    p.failures = p.failures.filter((f) => t - f < this.watchdog.windowMs);
    p.failures.push(t);
    if (p.failures.length < this.watchdog.failures) return false;
    this.tripped(name, p, reason);
    this.enqueue(() => this.reconcile(name));
    return true;
  }

  /** Deterministic ranking (V2). Returns the winner for a slot or null. */
  rank(name: string): ProviderRec | null {
    const r = this.table.get(name);
    if (!r) return null;
    const ok = (p: ProviderRec) => p.healthy && !p.tripped && !p.owner.disposed;
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
          if (next.owner.disposed) {
            // the owner died during activate: its provide-effect owns the cleanup
            next.act = null;
            await act.dispose();
          } else {
            r.active = next;
          }
        } catch (err) {
          next.act = null;
          this.tripped(name, next, `activate failed: ${String(err)}`); // V7
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
