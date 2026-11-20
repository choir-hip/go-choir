// The Exact side of the lab: AutoradioKit (copied in as Kit/*.swift by scripts/sync-exact-package.sh) behind
// the @choir/autoradio interface. Exact owns the audio session; the kit only asks this adapter for a claim.
import AVFoundation
import ExactPackage
import Foundation

/// Adapts Exact's session claims to the kit's AudioSessionOwner. One standing capture claim at a time:
/// listening (mode default, A2DP, speaker, no voice processing) or talking (voiceChat, HFP, speaker). A change of
/// state is release, then claim, because the first claim's mode wins in Exact (AudioSession.swift:238-262). Options
/// are always explicit: an empty list silently means defaultToSpeaker + allowBluetoothHFP (NativeHatches.swift:239-243).
#if os(iOS)
final class ExactSessionOwner: AudioSessionOwner, @unchecked Sendable {
    private let context: ExactPackageContext
    private var claim: ExactSubscription?
    private var held: AudioState?
    private let note: (String) -> Void
    private let lock = NSLock()
    private var handler: (@Sendable (String, String) -> Void)?
    init(context: ExactPackageContext, note: @escaping (String) -> Void) { self.context = context; self.note = note }

    static func spec(_ s: AudioState) -> (mode: String, options: [String], expected: AVAudioSession.Mode) {
        switch s {
        case .playing: return ("speech-playback", [], .spokenAudio)   // a playback session, no microphone (Exact's speech claim: .playback, spokenAudio, runs in the background)
        case .listening: return ("default", ["defaultToSpeaker", "allowBluetoothA2DP"], .default)
        case .talking: return ("voiceChat", ["defaultToSpeaker", "allowBluetoothHFP"], .voiceChat)
        }
    }

    func observe(_ h: @escaping @Sendable (String, String) -> Void) { lock.lock(); handler = h; lock.unlock() }

    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        let ok: Bool = await MainActor.run {
            if claim != nil, held == state { return true }
            if claim != nil { print("AUTORADIO released reason=state change \(held?.rawValue ?? "?") -> \(state.rawValue)") }
            claim?.cancel(); claim = nil; held = nil          // release first: the first claim's mode wins
            let spec = Self.spec(state)
            claim = context.audioSession.claim(mode: spec.mode, options: spec.options, background: true, events: { [weak self] e in
                guard let self else { return }
                self.lock.lock(); let h = self.handler; self.lock.unlock()
                h?(e, self.describe())
            })
            guard claim != nil else { return false }
            held = state
            let s = AVAudioSession.sharedInstance()
            note("claimed \(state.rawValue) (\(reason)): \(describe())")
            if s.mode != spec.expected && !context.agent { note("MODE MISMATCH: asked \(spec.mode), session has \(s.mode.rawValue)") }
            return true
        }
        if !ok {
            // Exact answers 0 without the system's reason: ask the session once, so the notice and the log carry the real code.
            var code = ""
            do { try AVAudioSession.sharedInstance().setActive(true) } catch { let ns = error as NSError; code = " (setActive: \(ns.domain) \(ns.code))" }
            throw LabError.unavailable("Exact refused the audio session claim for \(state.rawValue)\(code)")
        }
    }
    func holds(_ state: AudioState) -> Bool { lock.lock(); defer { lock.unlock() }; return claim != nil && held == state }
    /// Assert the session again without giving the claim up (the claim is Exact's; this only asks the system to be active).
    func reassert() async throws { try await MainActor.run { try AVAudioSession.sharedInstance().setActive(true) } }
    func release() {
        let drop = { [self] in claim?.cancel(); claim = nil; held = nil; note("released") }
        if Thread.isMainThread { drop() } else { DispatchQueue.main.async(execute: drop) }       // in order with the claim that follows
    }
    func describe() -> String {
        let s = AVAudioSession.sharedInstance()
        let cat = s.category.rawValue.replacingOccurrences(of: "AVAudioSessionCategory", with: "")
        let mode = s.mode.rawValue.replacingOccurrences(of: "AVAudioSessionMode", with: "")
        return "cat=\(cat) mode=\(mode) in=\(s.currentRoute.inputs.map { $0.portType.rawValue }) out=\(s.currentRoute.outputs.map { $0.portType.rawValue })"
    }
}

#endif

/// A handler for one source or send name. Lanes register theirs from their own AutoradioModule+<Lane>.swift.
public typealias AutoradioHandler = ([String: Any], @escaping ([String: Any]) -> Void) -> Void

public final class AutoradioModule: ExactPackage {
    // Internal (not private) so the per-lane extension files can reach them.
    let context: ExactPackageContext
    /// The folder on disk that a page reads as `app:/cache/files`. Not `context.file(...)`: that names a file and answers nil for a path that is
    /// an existing folder, so on every launch after the first the copy went to a folder no page can read, and a PDF, a picture or a recording
    /// opened from Files or the Reader drew a blank frame (owner, 2026-11-18).
    var filesCacheDisk: URL {
        let u = context.cache.appendingPathComponent("files", isDirectory: true)
        try? FileManager.default.createDirectory(at: u, withIntermediateDirectories: true)
        return u
    }
    var runtime: LabRuntime!
    var owner: PlatformSessionOwner!
    var choir: ChoirApp!
    /// The spine's shared kit objects: every lane reads these and none replaces them.
    var pages: OpenPages!
    var signals: Signals!
    /// How many times the app has resigned active (the editor's flush signal, AutoradioModule+TextureEdit.swift).
    public var textureResigns = 0
    var themes: ThemeStore!
    let pageSources = PageSources()
    /// name -> handler. Looked up first in `later`; the integrator's switch below answers the rest.
    var handlers: [String: AutoradioHandler] = [:]
    private var dirty = Set<String>()
    private var scheduled = false
    private var turnArg: [String: Any] = [:]
    /// The page the bar opened in place (a URL typed in the bar); lane autoradio-queue.
    var barWebURL = ""

    /// The one handle the app module (modules/apple/ModelsEntry.swift) uses to run the native half of sign-out, which shared code leads
    /// (AutoradioLab/account.ts). Weak: the module owns its life; this is set at init and read on the main thread.
    nonisolated(unsafe) public static weak var current: AutoradioModule?

    /// The native half of sign-out: playback stops, the Swift run sheet posts what it holds under the old key, the key goes, the Swift
    /// stores are wiped, the revoke is sent in the background (ChoirApp.signOut). Answers when that has finished.
    /// `keepKey`: the account CHANGED (a different key is stored): only the old account's Swift stores go, the key stays. Otherwise the whole sign-out.
    public func signOutForShared(keepKey: Bool = false) async { if keepKey { await choir?.accountWipe.run() } else { await choir?.signOut() } }

    public init(context: ExactPackageContext) {
        self.context = context
        Self.current = self
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        #if os(macOS)
        ConsoleFile.install(directory: support)
        owner = PlatformSessionOwner(context: context) { [weak self] s in self?.context.log("AUTORADIO \(s)"); print("AUTORADIO \(s)") }
        #else
        ConsoleFile.install(directory: support)
        owner = PlatformSessionOwner(context: context) { [weak self] s in self?.context.log("AUTORADIO \(s)"); NSLog("AUTORADIO %@", s); ConsoleFile.write("AUTORADIO \(s)") }
        #endif
        runtime = LabRuntime(supportDir: support, session: owner, emptyProgramme: true)
        runtime.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["choirPlayer", "choirStories", "choirAuth", "barState", "radioView"]) } }
        HandsFreeSwitch.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["barState"], now: true) } }
        // The listener path: sign-in (Keychain), the front page, the real stages.
        let appKeys: KeyStore = KeychainKeyStore()
        Demo.clearLeftovers(support: support)   // a phone that was in demo mode (removed 2026-11-20): once
        #if targetEnvironment(simulator)
        // Real-data proof only (compiled out of device builds): a `proof-key` file in Documents becomes the stored key, then is deleted.
        if let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first {
            ProofKey.importIfPresent(documents: docs, into: appKeys)
        }
        #endif
        choir = ChoirApp(runtime: runtime, keys: appKeys, openURL: { url in Platform.open(url) })
        choir.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["choirPlayer", "choirStories", "choirAuth", "barState", "radioView"]) } }
        // The spine's objects.
        signals = FileSignals(file: support.appendingPathComponent("signals.jsonl"))
        pages = OpenPages(file: support.appendingPathComponent("open-pages.json"), signals: signals)
        // The phone's own content store (Kit/ContentStore.swift): one source for the index, the offline pages and "play that".
        // Nothing here touches the store on the launch path: it opens in a background task, and a store that cannot open leaves the app as it was.
        ContentSync.enabled = true
        Task.detached(priority: .utility) {
            if ContentBoot.start() { ContentSync.shared.audioBusy = { await MainActor.run { RunSheetPlayer.shared.isRemote } } }
            if ContentStore.shared.usable {                // vectors for the same items: BM25 alone until the model is there and the backlog embedded
                Task { @MainActor in          // remote audio on air: the player is main-actor state, so its flag is copied across every two seconds
                    while true { RadioDemand.shared.setRemotePlaying(RunSheetPlayer.shared.isRemote && !RunSheetPlayer.shared.paused); try? await Task.sleep(nanoseconds: 2_000_000_000) }
                }
                EmbeddingService.shared.attach(corpus: .shared, store: .shared)
            }
            try? await Task.sleep(nanoseconds: 30_000_000_000)
            SavedCorpus.shared.refresh()          // the index warm-up, long after the front page has drawn
        }
        ReaderStore.shared.offline = { ContentStore.shared.readerDoc(address: $0) }
        PublicationTextSource.offline = { ContentStore.shared.readerDoc(address: $0) }
        // Sign-out wipes through the Kit's AccountWipe (ChoirApp registers every Kit store, ContentSync.wipeAccount among them); the stores below live in this target.
        // Kit methods called: Signals.wipe(), OpenPages.wipe(), AccountWipe.removeFiles(_:), AccountWipe.removeContents(of:), AccountWipe.register(_:_:).
        let filesCache = filesCacheDisk
        // The folder the old lookup fell back to: copies no page could read. Removed once; nothing writes there now.
        try? FileManager.default.removeItem(at: FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio/files"))
        choir.accountWipe.register("signals") { [signals] in signals?.wipe() }
        choir.accountWipe.register("open-pages") { [pages] in pages?.wipe() }
        choir.accountWipe.register("app-files") {
            AccountWipe.removeFiles(["onboarding.json", "voice-setup.json", "recent-files.json"].map { support.appendingPathComponent($0) })
            AccountWipe.removeContents(of: support.appendingPathComponent("books")); AccountWipe.removeContents(of: filesCache)
        }
        SavedCorpus.shared.setSource("open-pages") { [pages] in RetrievalSources.openPages(pages?.list() ?? []) }
        pages.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["openPages"]) } }
        themes = ThemeStore(userDir: support.appendingPathComponent("themes"), systemScheme: { Self.systemScheme() })
        pageSources.register(NotYetPageSource())
        pageSources.register(PublicationTextSource())
        // Registration: the spine, then one line per lane (each defined in that lane's own file).
        registerSpine()
        registerBar()
        registerPaper()
        registerReader()
        registerCards()
        registerMail()
        registerSettings()
        registerOnboarding()
        registerMark()
        registerVoice()
        registerQueue()          // lane autoradio-queue
        registerDeck()           // the deck: shared code commands it and reads its journal (Deck.swift)
        registerRadioPage()      // the Autoradio page
        registerTakes()          // takes: record, review, publish (station L7)
        registerNotebook()       // my notebook (station L6)
        registerBooks()   // lane autoradio-reader
        registerTalk()
        registerTextureEdit()   // the editor's background signal (shared code flushes; AutoradioModule+TextureEdit.swift)
        registerPodcasts()      // lane autoradio-podcasts
        registerJoins()         // lane autoradio-joins
        Task { [choir] in await choir?.refresh() }
    }

    /// The system's appearance, read from the first window (main thread).
    static func systemScheme() -> String { Platform.systemScheme() }

    /// Register a handler for a source or send name.
    func handle(_ name: String, _ h: @escaping AutoradioHandler) { handlers[name] = h }

    /// Coalesce change notifications to a few per second. `names` are source names without the "autoradio." prefix.
    /// `now`: the change goes out on the next turn of the main loop instead of after the 0.2 s coalescing wait (the page list: a swipe
    /// must not wait for it; the open-page list changes once per gesture, so there is no storm to coalesce).
    func touch(_ names: [String], now: Bool = false) {
        // `dirty` is only touched on the main thread: a background caller (found by a real-data run: readServerBuild called this from a
        // cooperative thread and crashed in Set.insert, EXC_BAD_ACCESS) is moved over first.
        if !Thread.isMainThread { DispatchQueue.main.async { [weak self] in self?.touch(names, now: now) }; return }
        dirty.formUnion(names)
        if names.contains(where: { $0 == "radioView" || $0 == "queueView" || $0 == "barState" }) { dirty.insert("radioPage") }   // the Autoradio page reads all three
        if now {
            DispatchQueue.main.async { [weak self] in
                guard let self else { return }
                let d = self.dirty; self.dirty = []
                for n in d { self.context.changed("autoradio.\(n)") }
            }
            return
        }
        guard !scheduled else { return }
        scheduled = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
            guard let self else { return }
            self.scheduled = false
            let d = self.dirty; self.dirty = []
            for n in d { self.context.changed("autoradio.\(n)") }
        }
    }

    func json<T: Encodable>(_ v: T) -> [String: Any] {
        guard let d = try? JSONEncoder().encode(v), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] else { return [:] }
        return o
    }

    /// The app came back to the foreground. Voice is not touched: it stays as the listener left it (off unless they turned it on). Main thread.
    public func foreground(_ shown: Bool) {
        if shown { touch(["barState"], now: true) }
    }

    public func later(_ name: String, _ args: [String: Any], preferred: Any?, reply: @escaping ([String: Any]) -> Void) {
        let state = { [unowned self] in self.json(self.runtime.snapshot()) }
        if let h = handlers[name] { h(args, reply); return }
        switch name {
        case "choirAuth": reply(json(choir.authView()))
        case "choirPlayer": reply(json(choir.playerView()))
        case "choirStories": reply(["rows": choir.storyRows().map { json($0) }])
        case "choirSignIn": Task { [choir] in _ = await choir?.signIn.begin() }; reply(state())
        case "choirCancelSignIn": choir.signIn.cancel(); reply(state())
        case "choirOpenApproval":
            if let u = URL(string: choir.authView().verificationURL) { Platform.open(u) }
            reply(state())
        case "choirRefresh": Task { [choir] in await choir?.refresh() }; reply(state())
        case "choirPlayPause": Task { [choir] in await choir?.playPause() }; reply(state())
        case "choirTalkCancel": runtime.cancelTalk(); reply(state())      // Cancel beside Send while listening: discard, call nothing
        case "choirPlay": let r = args["route"] as? String ?? "", t = args["title"] as? String ?? ""; Task { [choir] in await choir?.play(route: r, title: t) }; reply(state())
        default: reply(["error": "NotSupportedError"])
        }
    }
}
