// Ids are minted from Exact's random stream, which starts the same on every launch of an agent run (the simulator proof found the same ids come out of two launches, and the sheet kept from the
// first launch refused every op of the second as an id it had already applied). An id must differ between launches: the device's clock goes into it.
import { afterEach, describe, expect, test } from 'bun:test';
import { mintId } from './runsheet-entries.ts';
import { clock } from './clock.ts';

const real = crypto.getRandomValues.bind(crypto);
afterEach(() => { (crypto as { getRandomValues: unknown }).getRandomValues = real; clock.ms = 0; });
/** A launch: the random stream starts over (all zeros here, the worst case), the clock says when. */
function launch(at: number): string[] {
  (crypto as { getRandomValues: unknown }).getRandomValues = (b: Uint8Array) => { b.fill(0); return b; };
  clock.ms = at;
  return [mintId('p-'), mintId('l-')];
}

describe('minted ids', () => {
  test('two launches with the same random stream mint different ids', () => {
    const a = launch(1_700_000_000_000), b = launch(1_700_000_060_000);
    expect(a[0]).not.toBe(b[0]);
    expect(a[1]).not.toBe(b[1]);
  });
  test('the shape stays: the prefix and twenty-four hex characters', () => {
    clock.ms = 1_700_000_000_000;
    expect(mintId('p-')).toMatch(/^p-[0-9a-f]{24}$/);
    expect(mintId('')).toMatch(/^[0-9a-f]{24}$/);
  });
});
