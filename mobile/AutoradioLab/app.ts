import type { Answer, Sources } from './app.contract.d.ts';
import { paperFront } from './paper.ts';
import { podcastEpisode, podcastHome, podcastPlay, podcastPlayhead, podcastQueue, podcastShow, podcastSubscribe, podcastUnsubscribe } from './podcast.ts';
import { clipAct, showClips, storyClips } from './clips.ts';
import { fileList, fileOpen, fileResolve, fileMkdir, fileDelete, fileRename, fileUpload } from './files.ts';
import { adoptNative } from './api.ts';
import { settingsAccount, aboutPage } from './settings.ts';
import { settingsComputer, computerAct } from './computer.ts';
import { activityPage } from './activity.ts';
import { onboardingPage, onboardingAct, liveEnv as onboardingEnv } from './onboarding.ts';
import * as cards from './cards.ts';
import { aboutBuild } from './about.ts';
import { accountSignOut, settleAccount } from './account.ts';
import { clearDemoLeftovers } from './demo-leftovers.ts';
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
import { barInfo, dismissAnswer } from './bar.ts';
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
fs.read app:/data/what-i-told-the-paper.json
fs.write app:/data/what-i-told-the-paper.json
fs.read app:/data/recent-pages.json
fs.write app:/data/recent-pages.json
fs.read app:/data/steer-queue.json
fs.write app:/data/steer-queue.json
fs.read app:/data/open-pages-shared.json
fs.write app:/data/open-pages-shared.json
fs.read app:/data/demo-leftovers-cleared
fs.write app:/data/demo-leftovers-cleared
fs.read app:/data/notebook-cache.md
fs.write app:/data/notebook-cache.md
fs.read app:/data/takes.json
fs.write app:/data/takes.json
fs.read app:/data/runsheet.json
fs.write app:/data/runsheet.json
fs.read app:/data/vectors
fs.write app:/data/vectors
fs.read app:/data/imported-places.json
fs.write app:/data/imported-places.json
fs.read app:/data/theme-choice.json
fs.write app:/data/theme-choice.json
fs.read app:/data/themes
fs.write app:/data/themes
fs.read app:/data/edition-topics.json
fs.write app:/data/edition-topics.json
fs.read app:/data/onboarding.json
fs.write app:/data/onboarding.json
sqlite.open app:/data/content.db`;

let takesOverlay: ReturnType<typeof newTakes> | undefined;
const takes = (): ReturnType<typeof newTakes> => (takesOverlay ??= newTakes());      // lazily: app.ts and take-session.ts import each other through api.ts, and a module-level call would run before take-session.ts is evaluated
const sources: Sources = {
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
  queueView: ([_t, _pos], _s, storage, native) => radioBridge.queueView(storage as never, native as never),
  queueAdd: ([mode, kind, ref, title, local], _s, storage, native) => radioBridge.queueAdd(storage as never, native as never, mode, kind, ref, title, local),
  queueAct: ([op, arg], _s, storage, native) => radioBridge.queueAct(storage as never, native as never, op, arg),
  barRun: ([_t, _pos], _s, storage, native) => radioBridge.barRun(storage as never, native as never) as never,
  radioPageFacts: ([_t, _pos], _s, storage, native) => radioBridge.radioPageFacts(storage as never, native as never) as never,
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
  notebookPageShared: ([selected, tick, active, signedOutTick, _signedIn], _store, storage) => notebookShared({ selected, tick, active, signedOutTick }, storage),
  contentSync: ([key], _store, storage) => syncContent(key, storage),
  accountState: ([tick, top, host]) => accountState(tick, top, host),
  hostCaps: async ([nativeSeen, _again]) => ({ mode: await hostMode(1500, 800, nativeSeen) }),
  // The story cards and the launcher (cards.ts): the places, recent pages and local hits; "more of this" and the steering tap; what the paper has learned.
  launcherData: ([q, _pages, now], _s, storage) => cards.launcherData({ q, now }, storage),
  storyMore: ([address, surface, sheet, now], _s, storage) => cards.storyMore({ address, surface, sheet, now }, storage),
  learnedInfo: ([_tick], _s, storage) => cards.learnedInfo(storage),
  learnedReset: (_a, _s, storage) => cards.learnedReset(storage),
  // The answer card's Close (bar.ts): the entry the card shows stays closed, whatever turns come after.
  answerDismissShared: ([id]) => { dismissAnswer(id); return { ok: true, message: '' }; },
  pagesView: ([_tick, _requested], _store, storage) => pages.pagesView(storage),
  pageRequested: ([id, address, title], _store, storage) => pages.pageRequested(storage, id, address, title),
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
// `bindNative` hands the signed-in seam (api.ts) the one native reader of the stored key; it reads nothing here.
export const answer: Answer = async (source, args, store, storage, native) => {
  adoptNative(native as never, source);
  // The first answer after a launch finishes a sign-out that was cut short, or wipes another account's data (account.ts settleAccount).
  if (source !== 'accountSignOut') await settleAccount(storage, store);
  // What demo mode (deleted 2026-11-20) left in the shared files of a phone that was in it (demo-leftovers.ts): one attempt per process, finished before the first source runs, so the
  // modules that keep the open pages and the recent files in memory load from the cleaned files. A failed write is retried at the next launch.
  if (source !== 'accountSignOut') await clearDemoLeftovers(storage);
  return sources[source](args, store, storage, native);
};
