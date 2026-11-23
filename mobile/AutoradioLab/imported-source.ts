// The imported documents as a source the retrieval runtime reads without waiting on the network (C-3 of the train review). No imports but a type: it sits outside the api/app cycle.
import type { Doc } from './search.ts';

/**
 * The source the retrieval runtime reads on every ask (retrieval-live.ts). Reading the folder is a signed-in network list (up to 120 s on a dead link), and a voice answer must never wait on it:
 * `docs()` answers at once with what the last read found, and starts ONE read in the background when none is running; a read that found a change says so (`changed`: the runtime indexes again).
 * A read that fails keeps what was held; one that lands after the account moved (`generation`) is dropped.
 */
export class ImportedSource {
  private held: Doc[] = [];
  private reading: Promise<void> | null = null;
  constructor(private load: () => Promise<Doc[]>, private generation: () => number, private changed: () => void) {}
  async docs(): Promise<Doc[]> {
    if (this.reading === null) {
      const g = this.generation();
      const p: Promise<void> = this.load().then((docs) => {
        if (g !== this.generation()) return;
        const same = docs.length === this.held.length && docs.every((d, i) => d.id === this.held[i].id && d.key === this.held[i].key);
        this.held = docs;
        if (!same) this.changed();
      }, () => undefined).finally(() => { if (this.reading === p) this.reading = null; });
      this.reading = p;
    }
    return this.held;
  }
  /** The titles last read (the launcher's rows). */
  get titles(): Doc[] { return this.held; }
  reset(): void { this.held = []; this.reading = null; }
}
