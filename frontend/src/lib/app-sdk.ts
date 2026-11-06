// The platform API a package may use (docs/design/app-packages-replan-2026-10-24.md §3.1).
//
// A first-party package under src/packages/<name>/ may import `svelte`, its own
// files, and this file. `scripts/check-packages.mjs` refuses every other import
// at build time. Add a re-export here when a package needs a shell function;
// do not reach past it. This is the start of the "platform API range" a lock
// entry declares later.

// The SDK's version, which an installed package's requires_sdk is checked
// against, and the signal that the installed package set changed (the Library
// sends it after an install or uninstall, so the launcher updates in place).
export { SDK_VERSION, sdkSatisfies } from './sdk-version';
export { notifyPackagesChanged } from './apps/packages';

// Media and file helpers (the shell's auth-renewing fetch and its upload live
// behind these).
export {
  appTitle,
  apiFileURL,
  clampNumber,
  formatTime,
  loadContextContentItem,
  loadMediaPosition,
  loadMediaProgress,
  MEDIA_FILE_ROUTES,
  mediaRouteForFileName,
  loadRecentMedia,
  mediaSourceIdentity,
  recentMediaAppContext,
  rememberRecentMedia,
  resolveMediaSource,
  saveMediaPosition,
  uploadMediaFile,
  youtubeEmbedURL,
} from './media-utils.js';

// Live events (the synced computer state channel).
export { addLiveEventListener, currentDeviceId, isOwnLiveEvent, liveEventKind, liveEventPayload } from './live-events.js';

// Auth: the renewing fetch every /api call goes through, and its error when
// the session cannot be renewed.
export { AuthRequiredError, fetchWithRenewal } from './auth.js';

// The app registry: which app opens which file kind (Open with).
export { PACKAGE_MEDIA_ROUTES, getAppDefinition } from './apps/registry';
export type { PackageMediaRoute } from './apps/packages';

// Context menus: right-click, long press on touch, Shift+F10 and the Menu key.
export {
  closeContextMenu,
  contextMenu,
  isContextMenuOpen,
  openContextMenu,
  placeMenu,
  placeSubmenu,
} from './context-menu';
export type { ContextMenuEntry, ContextMenuItem, ContextMenuOptions, ContextMenuRequest, ContextMenuTarget } from './context-menu';

// Open with: the apps that open a file, and the file a window was launched on.
export { canOpenWith, extensionsOpenedBy, fileExtension, isTextFileName, openersForFile } from './open-with';
export type { FileOpener } from './open-with';
export { getLaunchFile, launchFileFromAppContext, launchFileStore } from './launch-file';
export type { LaunchFile } from './launch-file';

// Upload: the system picker, a list of files, or a drop, into a Files folder.
export {
  UploadError,
  acceptAttribute,
  createDropZone,
  dropZone,
  fileMatchesAccept,
  pickAndUploadFiles,
  pickFiles,
  uploadFiles,
} from './file-upload';
export type { DropZoneOptions, UploadFailure, UploadOptions } from './file-upload';

// Shell functions (1.1.0): what an app asks of the shell. They replace the
// events an app dispatched (authexpired, authrequired, launchapp, opentextfile,
// openmediafile, opentexture, contextchange, clearsavedwindows, keepwindowonly,
// resetdesktop, opencomputemonitor), which stay forwarded until 2.0.0.
export {
  buildInfo,
  closeOtherWindows,
  copyText,
  listWindows,
  openApp,
  openFile,
  requireAuth,
  resetDesktop,
  sessionExpired,
  logout,
  setAppContext,
  showDesktop,
  showOverview,
  submitPrompt,
  toast,
  windowAction,
} from './shell-services';
export type {
  AppContextChange,
  AuthRequest,
  OpenAppContext,
  PromptFocus,
  PromptOptions,
  ToastKind,
  ToastOptions,
  WindowAction,
  WindowSummary,
} from './shell-services';
export { createContentItem } from './content-items.js';

// Renderers shared by Source, Web Lens and Texture. Web Lens uses the source
// renderer and source-url; Source uses all three.
export { renderMarkdownBlocks } from './texture-markdown-renderer';
export {
  escapeHTML,
  renderInlineSourceRef,
  renderSourceEntityFacts,
  renderSourceTransclusionBody,
  sourceEntityKindLabel,
  sourceEntityOpenAppID,
  sourceEntityOpenPlan,
  sourceEntityTargetURL,
  sourceEntityTitle,
} from './texture-source-renderer';
export { browserOpenableSourceURL, storedSnapshotApplies, webLensAddress } from './source-url';

// Commands: a package registers one in code (a disposer comes back); data
// commands and keybindings go in the manifest's contributes. A package binds a
// key and never reads keys.
export { registerCommand } from './services/commands';
export type { CommandHandler, CommandRun, CommandScope, CommandSpec } from './services/commands';

// Theme (1.1.0): the applied theme, selecting one by id or config (built-in or
// a pack's), and following changes. Replaces the window choir-theme-change
// event and reading the theme preference. A pack contributes themes in its
// manifest (contributes.themes), as data.
export { getTheme, onThemeChange, setTheme } from './services/theme';
export type { ChoirTheme } from './theme';

// Window API (1.1.0, slice W1): raise or restore one window, find out whether an
// app is heavy, and suspend the heavy windows behind the active one. The window
// host (geometry, z-order, saved windows) stays the shell's; these are its
// public verbs. listWindows (above) answers what is open.
export { focusWindow as focus, isHeavyAppId, restoreWindow as restore, suspendBackgroundHeavyWindows } from './stores/desktop.js';

// Events and timers (1.1.0, slice U1): a package listens and schedules through
// these, which return disposers; the package boundary refuses the raw calls.
export { onDocumentEvent, onWindowEvent, repeatEvery } from './event-helpers';
export type { Disposer } from './event-helpers';

// Settings (1.3.0): a package's own settings, declared in its manifest
// (contributes.settings) and stored by the shell as preferences. Pass the
// package's name; an installed module is bound to its own name by the loader,
// so it can touch no other package's settings. The base Settings app renders
// the declared sections.
export { getSetting, onSetting, setSetting, settingSections } from './services/settings';
export type { PackageSettingsSection, SettingField, SettingResult, SettingsSection } from './services/settings';

// The open windows as a store, and which window has focus (1.4.0, slice C9).
export { windowList } from './window-list';
export type { WindowEntry } from './window-list';

// Autopaper (1.4.0, slice C8): the front page, its rank and layout, sections
// and routes of the Autopaper object graph.
export {
  AUTOPAPER_SECTIONS,
  FRONT_FIRST_PAGE_SIZE,
  FRONT_SECTION_ID,
  editionTime,
  fetchFrontPage,
  fetchOpinionPage,
  formatWhen,
  layoutFrontPage,
  loadSavedSection,
  loadSectionVisits,
  parsePublicationRoute,
  publicationRoute,
  rankFrontStories,
  recordSectionVisit,
  saveSection,
  sectionShares,
  storyPath,
  storyPathForRoute,
  storyTextureContext,
  usesSections,
} from './autopaper';
export type { FrontStory } from './autopaper';

// Source evidence labels and snapshot text (1.4.0, slices C10 and C11).
export {
  normalizeSourceEvidenceState,
  readerArtifactStateLabel,
  renderInlineMarkdown,
  sourceEntityExcerptText,
  sourceEntityReaderFallbackText,
  sourceEntityReaderSnapshotStatus,
  sourceEntityReaderSnapshotText,
  sourceEntitySnapshotText,
  sourceEntitySnapshotWarnings,
  sourceEvidenceResearchLabel,
  sourceEvidenceState,
  sourceEvidenceStateLabel,
} from './texture-source-renderer';

// Theme catalogue and validation (1.4.0, slice C12): what Settings lists and edits.
export {
  DEFAULT_THEME,
  listThemes,
  normalizeThemeConfig,
  onThemeRegistryChange,
  themeCSSVariables,
  validateThemeConfig,
} from './theme';

// The Choir mark's geometry (1.5.0, slice H3): a surface that draws the mark
// (the prompt bar's desk button) draws these paths in its own svg.
export { TETRA_MARK_PATHS, TETRA_MARK_VIEWBOX } from './tetramark';
