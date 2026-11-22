import type { Answer, Sources } from './app.contract.d.ts';
import { paperFront } from './paper.ts';
import { podcastEpisode, podcastHome, podcastPlay, podcastPlayhead, podcastQueue, podcastShow, podcastSubscribe, podcastUnsubscribe } from './podcast.ts';
import { clipAct, showClips, storyClips } from './clips.ts';
import { fileList, fileOpen, fileResolve, fileMkdir, fileDelete, fileRename, fileUpload } from './files.ts';
import { adoptNative, bindStorage, deviceStorage, markNotBaked, scoped, type World } from './api.ts';
import { settingsAccount, aboutPage } from './settings.ts';
import { settingsComputer, computerAct } from './computer.ts';
import { activityPage } from './activity.ts';
import { onboardingPage, onboardingAct, liveEnv as onboardingEnv } from './onboarding.ts';
import * as cards from './cards.ts';
import { aboutBuild } from './about.ts';
import { accountSignOut, accountEnding, enter, AccountClosedError } from './account.ts';
import { signInView, signInStart, signInPoll, signInCancel } from './signin.ts';
import * as pages from './pages.ts';
import { hostMode } from './host.ts';
import { accountState } from './account-state.ts';
import { readerDocShared, readerPrefetch, readerReload, readerRestore, readerStop } from './reader.ts';
import { info as conversationInfo } from './conversation.ts';
import { texturesListShared } from './textures.ts';
import * as themes from './themes.ts';
import { notebookShared } from './notebook-run.ts';
import { syncContent } from './content-run.ts';
import * as mail from './mail.ts';
import { barInfo } from './bar.ts';
import * as radioBridge from './radio-bridge.ts';
import { radioPage } from './radio-page.ts';
import { retrievalServe, paperSearch } from './retrieval-live.ts';
import * as imported from './imported.ts';
import { act as takesAct, pageFor as takesPageFor, newOverlay as newTakes, liveEnv as takesEnv } from './take-session.ts';
import * as picks from './edition-picks.ts';

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
net.fetch ${ORIGIN}
net.fetch https:
fs.read app:/data
fs.write app:/data
fs.read app:/cache
fs.write app:/cache
fs.read app:/tmp
fs.write app:/tmp
sqlite.open app:/data/accounts
secret.keep api-key
secret.keep key-id
secret.keep computer-id`;

const takesState = scoped(() => ({ overlay: undefined as ReturnType<typeof newTakes> | undefined }));      // the takes overlay belongs to the world (api.ts `scoped`)
const takes = (): ReturnType<typeof newTakes> => (takesState().overlay ??= newTakes());      // lazily: app.ts and take-session.ts import each other through api.ts, and a module-level call would run before take-session.ts is evaluated
export const sources: Sources = {
  settingsAccount: ([signedIn, tick]) => settingsAccount({ signedIn, tick }),
  // The About page's build rows (about.ts): the app's baked-in commit and the server's, from /health.
  aboutBuild: ([tick]) => aboutBuild({ tick }),
  // Settings: the Computer page (computer.ts), Activity (activity.ts) and About's app rows over the native facts (`settingsFacts`).
  settingsComputer: ([signedIn, tick]) => settingsComputer({ signedIn, tick }),
  computerAct: ([op, signedIn]) => computerAct(op, signedIn),
  activityPage: ([facts, signedIn, tick, state, updateAvailable, checked]) => activityPage(facts, signedIn, tick, state, updateAvailable, checked),
  aboutPage: ([facts, signedIn]) => aboutPage(facts, signedIn),
  // First run (onboarding.ts): the step machine over the voice side's facts (`voiceSetupFacts`); one send per button.
  onboardingPage: ([voice, tick, top], _s, storage) => onboardingPage(voice, tick, top, onboardingEnv(storage)),
  onboardingAct: ([op, arg, topics, top], _s, storage) => onboardingAct(op, arg, topics, top, onboardingEnv(storage)),
  // Sign-out: shared code leads (account.ts). The one native call is `{op: 'signOut'}` on the app module.
  // The deck's open pages (pages.ts): the shared set, read when the native package has none (Android).
  conversationInfo: ([raw]) => conversationInfo(raw),
  // The bar (bar.ts): the native facts and the conversation's entries in, the bar's state out; the tick (a timed state's end, a sign-out) asks again.
  barInfo: ([facts, run, entries, _tick]) => barInfo({ ...(facts as object), ...(run as object) }, entries),
  // The radio (radio-bridge.ts over radio-runtime.ts): the run sheet, its writer and the deck's journal are shared code; the deck is native (the one `deck` op).
  radioPulse: ([_deck, _note, _tick], _s, storage, native) => radioBridge.radioPulse(storage as never, native as never),
  radioWrite: ([_tick], _s, storage, native) => radioBridge.radioWrite(storage as never, native as never),
  radioHold: ([_tick], _s, storage, native) => radioBridge.radioHold(storage as never, native as never),
  queueView: ([_t], _s, storage, native) => radioBridge.queueView(storage as never, native as never),
  queueAdd: ([mode, kind, ref, title, local], _s, storage, native) => radioBridge.queueAdd(storage as never, native as never, mode, kind, ref, title, local),
  queueAct: ([op, arg], _s, storage, native) => radioBridge.queueAct(storage as never, native as never, op, arg),
  barRun: ([_t], _s, storage, native) => radioBridge.barRun(storage as never, native as never) as never,
  radioPageFacts: ([_t], _s, storage, native) => radioBridge.radioPageFacts(storage as never, native as never) as never,
  radioPageAct: ([op, arg, to], _s, storage, native) => radioBridge.radioPageAct(storage as never, native as never, op, arg, to),
  radioPlayPause: (_a, _s, storage, native) => radioBridge.radioPlayPause(storage as never, native as never),
  radioStart: (_a, _s, storage, native) => radioBridge.radioStart(storage as never, native as never),
  radioPlayStory: ([route, title], _s, storage, native) => radioBridge.radioPlayStory(storage as never, native as never, route, title),
  radioPlayAudio: ([op, id, url, title, show, start], _s, storage, native) => radioBridge.radioPlayAudio(storage as never, native as never, op, id, url, title, show, start),
  radioHandWords: ([op, id, title, at, text], _s, storage, native) => radioBridge.radioHandWords(storage as never, native as never, op, id, title, at, text),
  radioListen: ([address, title, text], _s, storage, native) => radioBridge.radioListen(storage as never, native as never, address, title, text),
  radioMove: ([move], _s, storage, native) => radioBridge.radioMove(storage as never, native as never, move),
  // The Autoradio page (radio-page.ts): the native facts, the bar's facts and the entries in; the page out.
  radioPage: ([facts, bar, entries]) => radioPage(facts, bar, entries),
  texturesList: ([at]) => texturesListShared(at),
  // The themes (themes.ts): the system's scheme and Reduce Motion come from `exactViewport()` (the first two arguments); the choice and the user's themes are kept under app:/data.
  theme: ([scheme, reduce], _s, storage) => themes.themeShared(scheme, reduce, storage),
  themeList: ([scheme, reduce], _s, storage) => themes.themeListShared(scheme, reduce, storage),
  themeExtras: ([scheme, reduce], _s, storage) => themes.themeExtrasShared(scheme, reduce, storage),
  themeCards: ([scheme, reduce], _s, storage) => themes.themeCardsShared(scheme, reduce, storage),
  themeMigrate: ([host], _s, storage) => themes.themeMigrate(host, storage),
  themeSet: ([id], _s, storage) => themes.themeSet(id, storage),
  schemeSet: ([mode], _s, storage) => themes.schemeSet(mode, storage),
  themeFromTexture: ([doc], _s, storage) => themes.themeFromTexture(doc, storage),
  themeRemove: ([id], _s, storage) => themes.themeRemove(id, storage),
  // The Takes section (take-session.ts): the recorder's snapshot (the native source takesRecorder: the voice) in, the page out; a press is takesAct. One overlay (the shared half of the session) per process.
  takesPage: ([rec, _tick], _s, storage) => takesPageFor(takesEnv(storage), takes(), rec),
  takesAct: ([op, arg], _s, storage) => takesAct(takesEnv(storage), takes(), op, arg),
  // The reader (reader.ts): the story or document in front, and what its page does to it.
  readerDocShared: ([address, acc, saved, _tick]) => readerDocShared(address, acc, saved),
  readerStopShared: ([address]) => readerStop(address),
  readerRestoreShared: ([address, version]) => readerRestore(address, version),
  readerReloadShared: ([address]) => readerReload(address),
  readerPrefetchShared: ([prev, next]) => readerPrefetch(prev, next),
  // My notebook (notebook-run.ts): the selected line, the Refresh count, whether the page is on top and the sign-out tick in; the page out. The content store's sync (content-run.ts) is started by a task.
  notebookPageShared: ([selected, tick, active, handle, _signedIn], _store, storage) => notebookShared({ selected, tick, active, handle }, storage),
  contentSync: ([key], _store, storage) => syncContent(key, storage),
  accountState: ([tick, host, tries], store, storage) => accountState(tick, host, tries, storage, store),
  signInView: ([_tick], store) => signInView(store),
  signInStart: (_a, store, storage) => signInStart(storage, store),
  signInPoll: (_a, store, storage) => signInPoll(storage, store),
  signInCancel: () => signInCancel(),
  accountEnding: (_a, store, storage) => accountEnding(storage, store),
  hostCaps: async ([nativeSeen, _again]) => ({ mode: await hostMode(1500, 800, nativeSeen) }),
  // The story cards and the launcher (cards.ts): the places, recent pages and local hits; "more of this" and the steering tap; what the paper has learned.
  launcherData: ([q, _pages, now], _s, storage) => cards.launcherData({ q, now }, storage),
  storyMore: ([address, surface, sheet, now], _s, storage) => cards.storyMore({ address, surface, sheet, now }, storage),
  learnedInfo: ([_tick], _s, storage) => cards.learnedInfo(storage),
  learnedReset: (_a, _s, storage) => cards.learnedReset(storage),
  pagesView: ([_tick, _requested], _store, storage) => pages.pagesView(storage),
  pageRequested: ([id, address, title], _store, storage) => pages.pageRequested(storage, id, address, title),
  pagesOpen: ([address, title], _store, storage) => pages.pagesOpen(storage, address, title),
  pagesClose: ([address], _store, storage) => pages.pagesClose(storage, address),
  pagesPin: ([address], _store, storage) => pages.pagesPin(storage, address),
  pagesReopen: (_a, _store, storage) => pages.pagesReopen(storage),
  accountSignOut: (_a, store, storage) => accountSignOut(storage, store),
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
  storyClips: ([address, tick]) => storyClips(address, tick),
  clipAct: ([op, id]) => clipAct(op, id),
  // Files (files.ts): the listener's computer's files. Positional arguments.
  fileList: ([path, mode, sort, desc, query, _acct], _s, storage) => fileList({ path, mode, sort, desc, query }, storage),
  fileOpen: ([address, _acct], _s, storage) => fileOpen({ address }, storage),
  fileResolve: ([address, title], _s, storage) => fileResolve({ address, title }, storage),
  fileMkdir: ([dir, name], _s, storage) => fileMkdir({ dir, name }, storage),
  fileDelete: ([path], _s, storage) => fileDelete({ path }, storage),
  fileRename: ([path, name], _s, storage) => fileRename({ path, name }, storage),
  fileUpload: ([from, name, dir], _s, storage) => fileUpload({ from, name, dir }, storage),
  // The edition picks (edition-picks.ts): the one reader and writer of app:/data/edition-topics.json. The last argument of each source is the signed-out tick (the page is asked again when the account changes).
  editionPicks: ([_focus, _signedOut], _s, storage) => picks.picksText(storage),
  editionTopics: ([_tick, talking, _signedOut], _s, storage) => picks.picksPage(storage, { talking }),
  editionToggle: ([label], _s, storage) => picks.toggle(storage, label),
  editionExpand: ([label], _s, storage) => picks.expand(storage, label),
  editionAdd: ([text], _s, storage) => picks.addTyped(storage, text),
  // Imported documents (imported.ts): PDFs and e-books that are files on the listener's computer, read through the Files client. The audio is native (importedFollow, importedHand).
  importedList: ([at, _acct], _s, storage) => imported.importedList(at, storage),
  importedDoc: ([address, _acct], _s, storage) => imported.importedDoc(address, storage),
  importedSection: ([address, index, _acct], _s, storage) => imported.importedSection(address, index, storage),
  importedFind: ([address, query], _s, storage) => imported.importedFind(address, query, storage),
  importedLocate: ([address, block], _s, storage) => imported.importedLocate(address, block, storage),
  importedPlace: ([address, section, block], _s, storage) => imported.importedPlace({ address, section, block }, storage),
  importedReadAloud: ([address, section, block], _s, storage) => imported.importedReadAloud({ address, section, block }, storage),
  importedQueue: ([address, section, block], _s, storage) => imported.importedQueue({ address, section, block }, storage),
  importedAdd: ([from, name], _s, storage) => imported.importedAdd({ from, name }, storage),
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
  // Retrieval (retrieval-live.ts): the voice turn's ask (native raises it, the answer goes back through one native op) and the launcher's paper-wide search.
  retrievalServe: ([id, words, pageTitle, pageAddress, now, facts], _s, storage) => retrievalServe({ id, words, pageTitle, pageAddress, now, facts }, storage),
  paperSearch: ([q, local]) => paperSearch(q, local),
  mailAsk: ([address, question]) => mail.mailAsk(address, question),
};
/** THE DEVICE SOURCES: the only sources that answer without an account (invariant I4). Each draws before sign-in or after the session ended, so each gets the DEVICE handle (api.ts `deviceStorage`): the theme choice and the
 * person's own theme files, and no other path. The three of the boundary itself (`accountState`, `accountEnding`, `accountSignOut`) take the host's storage. Every other source is gated: it is handed the storage of the open world, re-rooted under
 * the account's folder, and refused while the account is locked or the session has ended. A new source is gated until someone writes its line here and says why it needs no account. */
export const DEVICE: Partial<Record<keyof Sources, string>> = {
  accountState: 'the card: it asks the boundary for the state, never waits on anything, and says why pages refuse',
  accountEnding: 'Sign out, step one: the session ends and the ending mark is written, at once, so the card draws before the native half runs',
  accountSignOut: 'Sign out: the way out of every refusing state, so it can never wait behind the gate',
  signInView: 'the sign-in card\'s facts: whether a key is held and the flow\'s words; never the key or the device code',
  signInStart: 'Sign in: asks the server for a code; it needs no account and must work signed out',
  signInPoll: 'Sign in: one poll of the server; stores the key only while no key is held and the session has not ended',
  signInCancel: 'Sign in: stop waiting for the approval',
  hostCaps: 'a fact about this host (is there a native package), no account data',
  aboutBuild: 'the app\'s baked-in commit and the server\'s public /health, no account data',
  theme: 'device settings under app:/data/theme-choice.json; Texture themes only from the open account world, read only',
  themeList: 'the same device settings: the list of installed themes',
  themeExtras: 'the same device settings: the theme\'s sizes',
  themeCards: 'the same device settings: the theme cards',
  themeMigrate: 'the same device settings: the one-time carry-over of the theme choice',
  themeSet: 'the same device settings: choosing a theme',
  schemeSet: 'the same device settings: light or dark',
  themeRemove: 'the same device settings: removing an installed theme',
};

// `bindNative` hands the signed-in seam (api.ts) the one native reader of the stored key; it reads nothing here.
/** Run `p`, but end it (refused) the moment the world it began in closes: its own chain stays pending forever (api.ts, the world), and the page is told and asks again. */
const guarded = <T,>(w: World, p: Promise<T> | T): Promise<T> => new Promise<T>((res, rej) => { const off = w.watch(() => rej(new AccountClosedError('ended'))); Promise.resolve(p).then((v) => { off(); res(v); }, (e) => { off(); rej(e); }); });
/** THE GATE. Every source not in DEVICE asks the boundary first (account.ts `enter`) and throws AccountClosedError unless an account (or the anonymous world) is open: the page shows its existing failure state and the card says
 * why. It is handed the storage of ITS WORLD (api.ts `bindStorage`): every path re-rooted under the account's folder, every call inert once the world closes. The card and the sign-out take the host's storage. */
export const answer: Answer = async (source, args, store, storage, native) => {
  adoptNative(native as never, source);
  if (source === 'accountState' || source === 'accountSignOut' || source === 'accountEnding') return sources[source](args, store, storage, native);
  if (Object.hasOwn(DEVICE, source)) return sources[source](args, store, deviceStorage(storage), native);
  markNotBaked();      // reads Exact's host fact, which THROWS at the bake: the resource is left unbaked (an account's data must never be evaluated into the plan at build time, where there is no native module and no account)
  const b = await enter(storage, store);
  if (!b.readable) throw new AccountClosedError(b.state);
  const w = b.world;
  if (w.closed) throw new AccountClosedError('ended');      // a world that closed while the gate waited is refused BEFORE the source is called
  return guarded(w, sources[source](args, store, bindStorage(storage, w), native));
};
