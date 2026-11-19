import type { Answer, Sources } from './app.contract.d.ts';
import { paperFront } from './paper.ts';
import { podcastEpisode, podcastHome, podcastPlay, podcastPlayhead, podcastQueue, podcastShow, podcastSubscribe, podcastUnsubscribe } from './podcast.ts';
import { clipAct, showClips } from './clips.ts';
import { fileList, fileOpen, fileResolve, fileMkdir, fileDelete, fileRename, fileUpload } from './files.ts';
import { bindNative } from './api.ts';
import { settingsAccount } from './settings.ts';
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
net.fetch ${ORIGIN}
fs.read app:/data/podcasts-library.json
fs.write app:/data/podcasts-library.json
fs.read app:/data/podcast-feeds
fs.write app:/data/podcast-feeds
net.fetch https:
fs.read app:/data/recent-files.json
fs.write app:/data/recent-files.json
fs.read app:/cache/files
fs.write app:/cache/files
fs.read app:/tmp/picked
fs.read app:/data/mail
fs.write app:/data/mail
fs.read app:/cache
fs.write app:/cache
secret.keep mail-key`;

const sources: Sources = {
  settingsAccount: ([signedIn, demo, tick]) => settingsAccount({ signedIn, demo, tick }),
  podcastHome: ([q, address, tick], _store, storage) => podcastHome({ q, address, tick }, storage),
  podcastShow: ([address, tick, nowId, rev], _store, storage) => podcastShow({ address, tick, nowId, rev }, storage),
  podcastEpisode: ([address, find, tick, nowId, nowClip, nowStart, nowElapsed, nowDuration, rev], _store, storage) =>
    podcastEpisode({ address, find, tick, nowId, nowClip, nowStart, nowElapsed, nowDuration, rev }, storage),
  podcastSubscribe: ([feed], _store, storage) => podcastSubscribe(feed, storage),
  podcastUnsubscribe: ([feed], _store, storage) => podcastUnsubscribe(feed, storage),
  podcastPlay: ([address, mode, id, clip, elapsed, duration], _store, storage) => podcastPlay(address, mode, { id, clip, start: 0, elapsed, duration }, storage),
  podcastQueue: ([address], _store, storage) => podcastQueue(address, storage),
  podcastPlayhead: ([id, clip, elapsed, duration], _store, storage) => podcastPlayhead({ id, clip, elapsed, duration }, storage),
  showClips: ([address, tick]) => showClips(address, tick),
  clipAct: ([op, id]) => clipAct(op, id),
  // Files (files.ts): the listener's computer's files. Positional arguments, demo first.
  fileList: ([demo, path, mode, sort, desc, query], _s, storage) => fileList({ demo, path, mode, sort, desc, query }, storage),
  fileOpen: ([demo, address], _s, storage) => fileOpen({ demo, address }, storage),
  fileResolve: ([demo, address, title], _s, storage) => fileResolve({ demo, address, title }, storage),
  fileMkdir: ([demo, dir, name], _s, storage) => fileMkdir({ demo, dir, name }, storage),
  fileDelete: ([demo, path], _s, storage) => fileDelete({ demo, path }, storage),
  fileRename: ([demo, path, name], _s, storage) => fileRename({ demo, path, name }, storage),
  fileUpload: ([demo, from, name, dir], _s, storage) => fileUpload({ demo, from, name, dir }, storage),
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
