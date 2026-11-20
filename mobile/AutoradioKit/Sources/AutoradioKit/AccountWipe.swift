import Foundation

// One wipe for everything of the account that sits on the phone. Sign-out calls `run()` once; each store has a `wipe()` (memory and files) and is
// registered here by name. The order of sign-out (ChoirApp.signOut):
//  1. playback stops; the player reports the entry on air as `left` (a pending op);
//  2. the run sheet gets up to three seconds to post its pending ops with the OLD key still held (RunSheetStore.settlePending); whatever is not
//     answered by then is DROPPED by the wipe, never sent later: pending ops are never posted under a later account;
//  3. the local key goes, then this wipe runs (ChoirSignIn.signOut);
//  4. the revoke is sent in the background with the old key held only for that one call.
// The run sheet poster is dropped by the wipe and rebuilt at the next sign-in (ChoirApp).

public final class AccountWipe: @unchecked Sendable {
    private let lock = NSLock()
    private var items: [(name: String, run: @Sendable () async -> Void)] = []
    public init() {}

    // The ONE account generation. `run()` bumps it first. Every writer of an account store captures it when its work starts and checks it before
    // each put, flush, save or markSynced; a write whose captured generation is no longer the current one is dropped.
    private static let genLock = NSLock()
    nonisolated(unsafe) private static var _generation = 0
    public static var generation: Int { genLock.lock(); defer { genLock.unlock() }; return _generation }
    @discardableResult
    public static func bumpGeneration() -> Int { genLock.lock(); defer { genLock.unlock() }; _generation += 1; return _generation }

    public var names: [String] { lock.lock(); defer { lock.unlock() }; return items.map(\.name) }

    /// A store's wipe, by name. A second registration under a name replaces the first.
    public func register(_ name: String, _ run: @escaping @Sendable () async -> Void) {
        lock.lock(); items.removeAll { $0.name == name }; items.append((name, run)); lock.unlock()
    }

    public func run() async {
        Self.bumpGeneration()
        lock.lock(); let all = items; lock.unlock()
        for i in all { await i.run() }
    }

    /// The stores the Kit owns. Every parameter defaults to the app's own; a test passes its own.
    @MainActor
    func registerStandard(
        turnHistory: TurnHistory = .shared,
        notebookCache: NotebookCache = .shared,
        legacyTopicsFile: URL? = EditionPicksLegacy.file,
        topicsMarker: URL? = TakeFiles.url("notebook-topics-written"),
        notebook: NotebookStore = .shared,
        steer: SteerQueue = SteerQueue.shared,
        takeOwn: TakeOwn = .shared,
        takeMutes: TakeMutes = .shared,
        takeStore: TakeStore = .shared,
        tempDirectory: URL = FileManager.default.temporaryDirectory,
        runSheet: RunSheetStore = .shared,
        openingCache: OpeningCache? = nil,
        clips: ClipLocator = .shared,
        texture: TextureEditor = .shared,
        environment: ChoirEnvironment = .shared,
        content: ContentStore = .shared,
        corpus: SavedCorpus = .shared,
        mailReading: MailReading = .shared,
        conversation: Conversation = .shared,
        acks: AckHistory = .shared,
        bar: BarTracker = .shared,
        reader: ReaderStore = .shared,
        keptTurns: KeptTurnStore? = nil,
        voice: SpeakerGate = .shared,
        heldRest: HeldRest = .shared,
        deck: Deck = .shared
    ) {
        register("turn-history") { turnHistory.clear() }
        register("notebook") {
            notebookCache.wipe()
            if let f = legacyTopicsFile { try? FileManager.default.removeItem(at: f) }       // the old build's picks file (nothing writes it now); the proof in edition-picks.ts decides about it if this delete fails
            if let f = topicsMarker { try? FileManager.default.removeItem(at: f) }
            await MainActor.run { notebook.wipe() }
        }
        register("steer") { steer.wipe() }
        register("takes") {
            takeOwn.wipe(); takeMutes.wipe(); takeStore.wipe()
            Self.removeTakeScratch(in: tempDirectory)
        }
        register("run-sheet") { await MainActor.run { runSheet.wipe() } }
        register("deck") { await MainActor.run { deck.wipe() } }       // the item on air, the held items and the reason too, not only the journal
        if let openingCache { register("opening") { openingCache.wipe() } }
        register("clips") { clips.wipe() }
        // "podcasts": the library and cached feeds are files of shared code now; podcast.ts `wipe(storage)` is what sign-out must call (left to the lead).
        register("texture") { await MainActor.run { texture.wipe() } }
        register("playing") { environment.wipePlaying() }
        register("mail-reading") { mailReading.wipe() }
        register("spoken-words") { conversation.clear(); acks.clear(); bar.wipe() }
        // The native copy of the edition picks and the words waiting for shared code (the file itself is shared code's, wiped by account.ts).
        register("edition-picks") { EditionPicksMirror.shared.reset(); EditionTalk.shared.clear() }
        register("reader") { reader.wipe() }
        // The enrolled voice is app-private (Application Support/MikeVoice, not a system setting) and the listener's own: it goes at sign-out.
        register("voice-profile") { voice.forget() }
        if let keptTurns { register("kept-turns") { keptTurns.wipe() } }
        register("held-rest") { heldRest.wipe() }
        register("content") { ContentSync.wipeAccount(store: content, corpus: corpus) }
    }

    /// For the app target's stores that are files (one-line registrations in AutoradioModule): delete these paths.
    public static func removeFiles(_ urls: [URL]) { for u in urls { try? FileManager.default.removeItem(at: u) } }
    /// Delete what is inside a folder and keep the folder.
    public static func removeContents(of dir: URL) {
        for n in (try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? [] { try? FileManager.default.removeItem(at: dir.appendingPathComponent(n)) }
    }

    /// Recordings the listener abandoned (`takes/`) and the take being played (`take-play-*.m4a`).
    static func removeTakeScratch(in tmp: URL) {
        try? FileManager.default.removeItem(at: tmp.appendingPathComponent("takes", isDirectory: true))
        for n in (try? FileManager.default.contentsOfDirectory(atPath: tmp.path)) ?? [] where n.hasPrefix("take-play-") {
            try? FileManager.default.removeItem(at: tmp.appendingPathComponent(n))
        }
    }
}
