import type { Answer, Sources } from './app.contract.d.ts';
import { paperFront } from './paper.ts';
import { bindNative } from './api.ts';
import * as mail from './mail.ts';

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
fs.read app:/data/mail
fs.write app:/data/mail
fs.read app:/cache
fs.write app:/cache
secret.keep mail-key
net.fetch ${ORIGIN}`;

const sources: Sources = {
  paperFront: ([view, edition, width, tick, focus, picks, demo], _store, storage) => paperFront({ view, edition, width, tick, focus, picks, demo }, storage),
  // Mail (mail.ts): the phone's copy is sealed under app:/data/mail with the key `mail-key` in the secret store; "Show original" is written under app:/cache/mail-*.html.
  mailBox: ([address, demo], store, storage) => mail.mailBox(address, demo, { store, storage }),
  mailFrame: ([address], store, storage) => mail.mailFrame(address, { store, storage }),
  mailSync: ([address, demo], store, storage) => mail.mailSync(address, demo, { store, storage }),
  mailMessage: ([address, demo, original], store, storage) => mail.mailMessage(address, demo, original, { store, storage }),
  mailCompose: ([address, demo], store, storage) => mail.mailCompose(address, demo, { store, storage }),
  mailSaveDraft: ([address, to, subject, body, demo]) => mail.mailSaveDraft(address, to, subject, body, demo),
  mailApprovalEmail: ([address, draft, demo]) => mail.mailApprovalEmail(address, draft, demo),
  mailMarkUnread: ([address, demo]) => mail.mailMarkUnread(address, demo),
};
// `bindNative` hands the signed-in seam (api.ts) the one native reader of the stored key; it reads nothing here.
export const answer: Answer = (source, args, store, storage, native) => { bindNative(native); return sources[source](args, store, storage); };
