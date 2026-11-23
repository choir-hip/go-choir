// The time shared code may know. A data source may not read the clock (`Date.now()` and `new Date()` are refused: the build says so), so the device's own time arrives as a FACT: the native `radioFacts` carries
// `at` (milliseconds) and radio-bridge.ts sets it here before it asks anything of the runtime. Nothing here ticks by itself; 0 means no native side has said yet.
export const clock = { ms: 0 };
export const now = (): Date => new Date(clock.ms);
