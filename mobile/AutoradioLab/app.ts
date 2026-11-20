import type { Answer, Sources } from './app.contract.d.ts';
import { paperFront } from './paper.ts';
import { podcastEpisode, podcastHome, podcastPlay, podcastPlayhead, podcastQueue, podcastShow, podcastSubscribe, podcastUnsubscribe } from './podcast.ts';
import { clipAct, showClips } from './clips.ts';
import { fileList, fileOpen, fileResolve, fileMkdir, fileDelete, fileRename, fileUpload } from './files.ts';
import { adoptNative } from './api.ts';
import { settingsAccount } from './settings.ts';
import { accountSignOut, settleAccount } from './account.ts';
import * as pages from './pages.ts';
import { hostMode } from './host.ts';
import { accountState } from './account-state.ts';
import { readerDocShared } from './reader.ts';
import { info as conversationInfo } from './conversation.ts';
import * as textureEdit from './texture-edit.ts';
import { texturesListShared } from './textures.ts';
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
fs.write app:/data/takes.json
fs.read app:/data/texture-pending.json
fs.write app:/data/texture-pending.json`;

const editDeps = (demo: boolean, storage: unknown, native: unknown): textureEdit.Deps => ({ demo, storage: storage as never, native: nativeOps(native) });
const nativeOps = (native: unknown): textureEdit.Deps['native'] => { const n = native as { later?: (r: Record<string, unknown>) => Promise<unknown> } | null | undefined; return n && n.later ? (r) => n.later!(r) : undefined; };

const sources: Sources = {
  settingsAccount: ([signedIn, tick]) => settingsAccount({ signedIn, tick }),
  // Sign-out: shared code leads (account.ts). The one native call is `{op: 'signOut'}` on the app module.
  // The deck's open pages (pages.ts): the shared set, read when the native package has none (Android).
  conversationInfo: ([raw]) => conversationInfo(raw),
  // Editing your own Texture document (texture-edit.ts). The one native call after a save is `{op: 'textureSaved'}` (the Swift reader and content store catch up).
  textureEditInfo: ([address, demo, visibility], _s, storage, native) => textureEdit.textureEditInfo(address, editDeps(demo, storage, native), visibility === 'hidden'),
  textureEdit: ([address, markdown, demo], _s, storage, native) => textureEdit.textureEdit(address, markdown, editDeps(demo, storage, native)),
  textureFlush: ([address, demo], _s, storage, native) => textureEdit.textureFlush(address, editDeps(demo, storage, native)),
  texturesList: ([at]) => texturesListShared(at),
  readerDocShared: ([address, active]) => (active ? readerDocShared(address) : readerDocShared('')),
  notebookPageShared: ([selected, steerQueued, active, _tick, _signedIn], _store, storage) => (active ? notebookPage({ selected, steerQueued }, storage) : notebookIdlePage({ doc: null, learning: null, selected: '', steerQueued: 0 })),
  accountState: ([tick, top, host]) => accountState(tick, top, host),
  hostCaps: async ([nativeSeen, _again]) => ({ mode: await hostMode(1500, 800, nativeSeen) }),
  pagesView: ([_tick], _store, storage) => pages.pagesView(storage),
  pagesOpen: ([address, title], _store, storage) => pages.pagesOpen(storage, address, title),
  pagesClose: ([address], _store, storage) => pages.pagesClose(storage, address),
  pagesPin: ([address], _store, storage) => pages.pagesPin(storage, address),
  pagesReopen: (_a, _store, storage) => pages.pagesReopen(storage),
  accountSignOut: (_a, store, storage, native) => accountSignOut(storage, store, native as never),
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
  // Files (files.ts): the listener's computer's files. Positional arguments.
  fileList: ([path, mode, sort, desc, query, _acct], _s, storage) => fileList({ path, mode, sort, desc, query }, storage),
  fileOpen: ([address, _acct], _s, storage) => fileOpen({ address }, storage),
  fileResolve: ([address, title], _s, storage) => fileResolve({ address, title }, storage),
  fileMkdir: ([dir, name], _s, storage) => fileMkdir({ dir, name }, storage),
  fileDelete: ([path], _s, storage) => fileDelete({ path }, storage),
  fileRename: ([path, name], _s, storage) => fileRename({ path, name }, storage),
  fileUpload: ([from, name, dir], _s, storage) => fileUpload({ from, name, dir }, storage),
  paperFront: ([view, width, tick, focus, picks], _store, storage) => paperFront({ view, width, tick, focus, picks }, storage),
  // Mail (mail.ts): the phone's copy is sealed under app:/data/mail by the platform cipher (the native pair mailSeal/mailOpen, mail-seal.ts); "Show original" is written under app:/cache/mail-*.html.
  mailBox: ([address, _acct], _store, storage) => mail.mailBox(address, { storage }),
  mailFrame: ([address], _store, storage) => mail.mailFrame(address, { storage }),
  mailSync: ([address], _store, storage) => mail.mailSync(address, { storage }),
  mailMessage: ([address, original, _acct], _store, storage) => mail.mailMessage(address, original, { storage }),
  mailCompose: ([address], _store, storage) => mail.mailCompose(address, { storage }),
  mailSaveDraft: ([address, to, subject, body]) => mail.mailSaveDraft(address, to, subject, body),
  mailApprovalEmail: ([address, draft]) => mail.mailApprovalEmail(address, draft),
  mailMarkUnread: ([address]) => mail.mailMarkUnread(address),
};
// `bindNative` hands the signed-in seam (api.ts) the one native reader of the stored key; it reads nothing here.
export const answer: Answer = async (source, args, store, storage, native) => {
  adoptNative(native as never, source);
  // The first answer after a launch finishes a sign-out that was cut short, or wipes another account's data (account.ts settleAccount).
  if (source !== 'accountSignOut') await settleAccount(storage, store);
  return sources[source](args, store, storage, native);
};
