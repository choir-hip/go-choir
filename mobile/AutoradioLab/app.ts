import type { Answer, Sources } from './app.contract.d.ts';
import { paperFront } from './paper.ts';

// The app's data module: one line per source it answers, each in a module of its own (paper.ts, api.ts).
// A source this file does not name is the Swift package's (the voice, and the pages not yet moved); a name is
// answered here or there, never both.
// The build reads `grants` without running the module, and an imported constant does not count: the server's origin is
// this one constant (api.ts imports it), and the offline copy's path is the literal in the grant (paper.ts CACHE_FILE
// must equal it; paper.test.ts checks).
export const ORIGIN = 'https://choir.news';
export const grants = `device.microphone purpose.microphone
device.microphone-background purpose.recordingInBackground
device.speech-recognition purpose.speech
fs.read app:/data/exports
fs.write app:/data/exports
fs.read app:/data/paper-front.json
fs.write app:/data/paper-front.json
net.fetch ${ORIGIN}`;

const sources: Sources = {
  paperFront: ([view, edition, width, tick, focus, picks, demo], _store, storage) => paperFront({ view, edition, width, tick, focus, picks, demo }, storage),
};
export const answer: Answer = (source, args, store, storage) => sources[source](args, store, storage);
