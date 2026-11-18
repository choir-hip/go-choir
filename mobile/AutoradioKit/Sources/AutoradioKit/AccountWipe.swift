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

    public var names: [String] { lock.lock(); defer { lock.unlock() }; return items.map(\.name) }

    /// A store's wipe, by name. A second registration under a name replaces the first.
    public func register(_ name: String, _ run: @escaping @Sendable () async -> Void) {
        lock.lock(); items.removeAll { $0.name == name }; items.append((name, run)); lock.unlock()
    }

    public func run() async {
        lock.lock(); let all = items; lock.unlock()
        for i in all { await i.run() }
    }

    /// The stores the Kit owns. Every parameter defaults to the app's own; a test passes its own.
    @MainActor
    func registerStandard(
        turnHistory: TurnHistory = .shared,
        notebookCache: NotebookCache = .shared,
        topicsFile: URL? = EditionText.topicsFile,
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
        podcasts: PodcastService = .shared,
        texture: TextureEditor = .shared,
        environment: ChoirEnvironment = .shared,
        content: ContentStore = .shared,
        corpus: SavedCorpus = .shared
    ) {
        register("turn-history") { turnHistory.clear() }
        register("notebook") {
            notebookCache.wipe()
            if let f = topicsFile { try? FileManager.default.removeItem(at: f) }
            if let f = topicsMarker { try? FileManager.default.removeItem(at: f) }
            await MainActor.run { notebook.wipe() }
        }
        register("steer") { steer.wipe() }
        register("takes") {
            takeOwn.wipe(); takeMutes.wipe(); takeStore.wipe()
            Self.removeTakeScratch(in: tempDirectory)
        }
        register("run-sheet") { await MainActor.run { runSheet.wipe() } }
        if let openingCache { register("opening") { openingCache.wipe() } }
        register("clips") { clips.wipe() }
        register("podcasts") { podcasts.wipe() }
        register("texture") { await MainActor.run { texture.wipe() } }
        register("playing") { environment.wipePlaying() }
        register("content") { ContentSync.wipeAccount(store: content, corpus: corpus) }
    }

    /// Recordings the listener abandoned (`takes/`) and the take being played (`take-play-*.m4a`).
    static func removeTakeScratch(in tmp: URL) {
        try? FileManager.default.removeItem(at: tmp.appendingPathComponent("takes", isDirectory: true))
        for n in (try? FileManager.default.contentsOfDirectory(atPath: tmp.path)) ?? [] where n.hasPrefix("take-play-") {
            try? FileManager.default.removeItem(at: tmp.appendingPathComponent(n))
        }
    }
}
