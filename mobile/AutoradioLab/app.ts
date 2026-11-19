import type { Answer, Sources } from './app.contract.d.ts';
import { paperFront } from './paper.ts';
import { podcastEpisode, podcastHome, podcastPlay, podcastPlayhead, podcastQueue, podcastShow, podcastSubscribe, podcastUnsubscribe } from './podcast.ts';
import { clipAct, showClips } from './clips.ts';
import { fileList, fileOpen, fileResolve, fileMkdir, fileDelete, fileRename, fileUpload } from './files.ts';
import { adoptNative, setDemo } from './api.ts';
import { settingsAccount } from './settings.ts';
import { accountSignOut, settleAccount } from './account.ts';
import * as pages from './pages.ts';
import { hostMode } from './host.ts';
import { readerDocShared } from './reader.ts';
import { notebookPage, page as notebookIdlePage } from './notebook.ts';
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
fs.read app:/data/signout-pending
fs.write app:/data/signout-pending
fs.read app:/data/account-stamp
fs.write app:/data/account-stamp
fs.read app:/data/open-pages-shared.json
fs.write app:/data/open-pages-shared.json
fs.read app:/data/notebook-cache.md
fs.write app:/data/notebook-cache.md
fs.read app:/data/takes.json
fs.write app:/data/takes.json`;

const sources: Sources = {
  settingsAccount: ([signedIn, demo, tick]) => settingsAccount({ signedIn, demo, tick }),
  // Sign-out: shared code leads (account.ts). The one native call is `{op: 'signOut'}` on the app module.
  // The deck's open pages (pages.ts): the shared set, read when the native package has none (Android).
  readerDocShared: ([address, active]) => (active ? readerDocShared(address) : readerDocShared('')),
  notebookPageShared: ([selected, steerQueued, active, _tick, _signedIn], _store, storage) => (active ? notebookPage({ selected, steerQueued }, storage) : notebookIdlePage({ doc: null, learning: null, selected: '', steerQueued: 0 })),
  hostCaps: async ([nativeSeen]) => ({ mode: await hostMode(4000, 1500, nativeSeen) }),
  pagesView: ([_tick], _store, storage) => pages.pagesView(storage),
  pagesOpen: ([address, title], _store, storage) => pages.pagesOpen(storage, address, title),
  pagesClose: ([address], _store, storage) => pages.pagesClose(storage, address),
  pagesPin: ([address], _store, storage) => pages.pagesPin(storage, address),
  pagesReopen: (_a, _store, storage) => pages.pagesReopen(storage),
  accountSignOut: ([demo], store, storage, native) => accountSignOut({ demo }, storage, store, native as never),
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
  // Mail (mail.ts): the phone's copy is sealed under app:/data/mail by the platform cipher (the native pair mailSeal/mailOpen, mail-seal.ts); "Show original" is written under app:/cache/mail-*.html.
  mailBox: ([address, demo], _store, storage) => mail.mailBox(address, demo, { storage }),
  mailFrame: ([address, demo], _store, storage) => mail.mailFrame(address, demo, { storage }),
  mailSync: ([address, demo], _store, storage) => mail.mailSync(address, demo, { storage }),
  mailMessage: ([address, demo, original], _store, storage) => mail.mailMessage(address, demo, original, { storage }),
  mailCompose: ([address, demo], _store, storage) => mail.mailCompose(address, demo, { storage }),
  mailSaveDraft: ([address, to, subject, body, demo]) => mail.mailSaveDraft(address, to, subject, body, demo),
  mailApprovalEmail: ([address, draft, demo]) => mail.mailApprovalEmail(address, draft, demo),
  mailMarkUnread: ([address, demo]) => mail.mailMarkUnread(address, demo),
};
// `bindNative` hands the signed-in seam (api.ts) the one native reader of the stored key; it reads nothing here.
// Where the demo flag sits in a source's arguments (the sources that carry it). The signed-in seam is told at every answer; once in demo it stays
// in demo until the process ends (api.ts `setDemo`), and the native reader answers "not found" in demo for the sources that carry no flag.
export const DEMO_AT: Record<string, number> = {
  settingsAccount: 1, accountSignOut: 0, mailBox: 1, mailFrame: 1, mailSync: 1, mailMessage: 1, mailCompose: 1, mailSaveDraft: 4, mailApprovalEmail: 2, mailMarkUnread: 1,
  fileList: 0, fileOpen: 0, fileResolve: 0, fileMkdir: 0, fileDelete: 0, fileRename: 0, fileUpload: 0, paperFront: 6,
};
export const answer: Answer = async (source, args, store, storage, native) => {
  adoptNative(native as never, source);
  const at = DEMO_AT[source];
  if (at !== undefined && (args as unknown[])[at] === true) setDemo(true);
  // The first answer after a launch finishes a sign-out that was cut short, or wipes another account's data (account.ts settleAccount).
  if (source !== 'accountSignOut') await settleAccount(storage, store);
  return sources[source](args, store, storage, native);
};
