// A heartbeat from shared code (docs/problems/phone-pages-stall-placard-2026-11-24.md): every five seconds one line, so a stall in this runtime shows as a gap.
// It runs as a resource asked again by a contract task (spine.contract `hbTimer`), so it goes through the same ordered request sequence as a page's read:
// read it WITH the native ledger (NativeOpLog.swift, `AUTORADIO native open ...`). The time is the app's clock (clock.ts), which only moves when native says (radioFacts); `n` counts.
import { clock } from './clock.ts';

let beats = 0;
export function heartbeat(): { n: number } {
  beats += 1;
  console.log(`AUTORADIO js heartbeat n=${beats} t=${clock.ms}`);
  return { n: beats };
}
