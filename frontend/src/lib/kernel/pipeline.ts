// Pipelines: a named waterfall of handlers over a list of items.
//
// Failure modes:
//  P1 a handler mutates its input: the input is a frozen copy; to change,
//     return a new list.
//  P2 a handler throws or is too slow: it is skipped, the report names it, and
//     the waterfall continues with the previous payload.
//  P3 a handler forges the origin of an item: the kernel stamps `origin` on
//     every item the handler produced that was not in its input.
//  P4 a handler re-enters its own pipeline: refused through the chain on ctx.
//  P5 handlers of a lower tier run late: order is policy, user, app, core, then
//     priority (high first), then install order.
//  P6 a handler outlives its package: use() returns a disposer on the scope.

import type { Disposer } from './effect';
import type { Scope } from './scope';
import type { Tier } from './service';
import { TIER_ORDER } from './service';

export type Item = Record<string, unknown>;

export interface PipelineContext {
  readonly chain: readonly string[];
  /** Run another pipeline from inside a handler; refused if it re-enters (P4). */
  run(name: string, items: readonly Item[]): Promise<PipelineResult>;
}

export type Handler = (items: readonly Item[], ctx: PipelineContext) => readonly Item[] | void | Promise<readonly Item[] | void>;

export interface HandlerSpec {
  id: string;
  tier?: Tier;
  priority?: number;
}

export interface PipelineResult {
  items: Item[];
  skipped: { handler: string; reason: string }[];
}

interface HandlerRec {
  spec: HandlerSpec;
  fn: Handler;
  seq: number;
}

interface PipelineRec {
  budgetMs: number;
  handlers: HandlerRec[];
}

function deepFreeze<T>(v: T): T {
  if (v && typeof v === 'object' && !Object.isFrozen(v)) {
    Object.freeze(v);
    for (const k of Object.keys(v as object)) deepFreeze((v as Record<string, unknown>)[k]);
  }
  return v;
}

export class Pipelines {
  private readonly table = new Map<string, PipelineRec>();
  private seq = 0;

  define(scope: Scope, name: string, opts: { budgetMs: number }): Disposer {
    if (this.table.has(name)) throw new Error(`pipeline ${name} is already defined`);
    this.table.set(name, { budgetMs: opts.budgetMs, handlers: [] });
    return scope.effect(() => {
      this.table.delete(name);
    });
  }

  use(scope: Scope, name: string, spec: HandlerSpec, fn: Handler): Disposer {
    const p = this.table.get(name);
    if (!p) throw new Error(`pipeline ${name} is not defined`);
    const rec: HandlerRec = { spec, fn, seq: ++this.seq };
    p.handlers.push(rec);
    return scope.effect(() => {
      const at = p.handlers.indexOf(rec);
      if (at >= 0) p.handlers.splice(at, 1);
    });
  }

  private ordered(p: PipelineRec): HandlerRec[] {
    const tierIx = (h: HandlerRec) => TIER_ORDER.indexOf(h.spec.tier ?? 'app');
    return [...p.handlers].sort(
      (a, b) => tierIx(a) - tierIx(b) || (b.spec.priority ?? 50) - (a.spec.priority ?? 50) || a.seq - b.seq,
    );
  }

  async run(name: string, items: readonly Item[], chain: readonly string[] = []): Promise<PipelineResult> {
    const p = this.table.get(name);
    if (!p) throw new Error(`pipeline ${name} is not defined`);
    if (chain.includes(name)) throw new Error(`pipeline ${name} re-entered`);
    const nextChain = [...chain, name];
    let current: Item[] = items.map((i) => ({ ...i }));
    const skipped: PipelineResult['skipped'] = [];
    for (const h of this.ordered(p)) {
      const input = deepFreeze(structuredClone(current)) as Item[];
      const ctx: PipelineContext = {
        chain: nextChain,
        run: (n, its) => this.run(n, its, nextChain),
      };
      let timer: ReturnType<typeof setTimeout> | undefined;
      try {
        const budget = new Promise<never>((_, rej) => {
          timer = setTimeout(() => rej(new Error('over budget')), p.budgetMs);
        });
        const out = await Promise.race([Promise.resolve(h.fn(input, ctx)), budget]);
        if (out === undefined) continue;
        const known = new Set<unknown>(input);
        current = out.map((it) => {
          if (known.has(it)) return { ...(it as Item) };
          return { ...(it as Item), origin: h.spec.id }; // P3
        });
      } catch (err) {
        skipped.push({ handler: h.spec.id, reason: err instanceof Error ? err.message : String(err) });
      } finally {
        if (timer !== undefined) clearTimeout(timer);
      }
    }
    return { items: current, skipped };
  }

  describe() {
    const out: Record<string, unknown> = {};
    for (const [name, p] of [...this.table.entries()].sort((a, b) => (a[0] < b[0] ? -1 : 1))) {
      out[name] = {
        budgetMs: p.budgetMs,
        handlers: this.ordered(p).map((h) => ({ id: h.spec.id, tier: h.spec.tier ?? 'app', priority: h.spec.priority ?? 50 })),
      };
    }
    return out;
  }
}
