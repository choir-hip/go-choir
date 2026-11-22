// HARNESS ONLY (the proof build: the native side answers only on the simulator, so a device build never mirrors and this stops after one call). A plain launch has no driver, so shared code's
// `console.log` lines were unreadable and the radio's failure to start could not be seen. They are mirrored to the app's native console (`AUTORADIO js: <line>`), which the proof already reads
// from the app's own console.log file. Nothing here changes what the app does.
import { nativeLater } from './api.ts';

let installed = false;
let enabled: boolean | undefined;

export function installConsoleMirror(): void {
  if (installed) return;
  installed = true;
  const original = console.log.bind(console);
  console.log = (...args: unknown[]): void => {
    original(...args);
    if (enabled === false) return;
    const later = nativeLater();
    if (!later) return;
    try {
      const line = args.map((a) => (typeof a === 'string' ? a : JSON.stringify(a))).join(' ').slice(0, 400);
      later({ op: 'jsLog', line }).then((r) => { if (!(r && typeof r === 'object' && (r as { on?: unknown }).on === true)) enabled = false; }, () => undefined);
    } catch { /* not in an answer: this line is not mirrored */ }
  };
}
