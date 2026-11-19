import type { Answer, Sources } from './app.contract.d.ts';
import { paperFront } from './paper.ts';
import { fileList, fileOpen, fileResolve, fileMkdir, fileDelete, fileRename, fileUpload } from './files.ts';

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
fs.read app:/data/recent-files.json
fs.write app:/data/recent-files.json
fs.read app:/cache/files
fs.write app:/cache/files
fs.read app:/tmp/picked
net.fetch ${ORIGIN}`;

const sources: Sources = {
  // Files (files.ts): the listener's computer's files. Positional arguments, demo first.
  fileList: ([demo, path, mode, sort, desc, query], _s, storage) => fileList({ demo, path, mode, sort, desc, query }, storage),
  fileOpen: ([demo, address], _s, storage) => fileOpen({ demo, address }, storage),
  fileResolve: ([demo, address, title], _s, storage) => fileResolve({ demo, address, title }, storage),
  fileMkdir: ([demo, dir, name], _s, storage) => fileMkdir({ demo, dir, name }, storage),
  fileDelete: ([demo, path], _s, storage) => fileDelete({ demo, path }, storage),
  fileRename: ([demo, path, name], _s, storage) => fileRename({ demo, path, name }, storage),
  fileUpload: ([demo, from, name, dir], _s, storage) => fileUpload({ demo, from, name, dir }, storage),
  paperFront: ([view, edition, width, tick, focus, picks, demo], _store, storage) => paperFront({ view, edition, width, tick, focus, picks, demo }, storage),
};
export const answer: Answer = (source, args, store, storage) => sources[source](args, store, storage);
