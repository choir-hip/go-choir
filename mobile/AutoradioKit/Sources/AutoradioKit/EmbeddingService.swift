import Foundation
import Network

// THE VECTORS OF THE PHONE'S CONTENT. Owner, 2026-11-17: "are we doing rag with bm25 and embeddings properly?" Until now the answer was no vector
// search at all. This service keeps one vector per item of the retrieval index (a long item as windows), made on the phone by EmbeddingGemma 2
// (docs/reference/EMBEDDINGGEMMA.md), stored in a flat float16 file beside the content store, and hands the retrieval a fused retriever once the
// model is there and the backlog is embedded. Until then retrieval is BM25 alone and nothing says otherwise.
//
// WAYS THIS CAN FAIL (each has a test in EmbeddingServiceTests):
//  1. The model is not on the phone: it is fetched once, on Wi-Fi only, checked by SHA-256 (ModelStore); off Wi-Fi nothing is fetched and retrieval stays BM25.
//  2. Speech is on air or being synthesised: the job waits (speech always wins). Low Power Mode or a serious thermal state: it waits.
//  3. The app is killed: at most about a minute of work is lost (the job saves every `savesEverySlices` slices and the app saves in the background); the next start embeds only what the file lacks (content keys).
//  4. Sign-out: the vectors, in memory and on disk, go with the content store (`SavedCorpus.reset`). The model file stays (it is not the account's).
//  5. The store changes while a pass runs: the pass is run again after it; nothing embeds twice.
//  6. The vector file is made by another engine or dimension: it is ignored and rebuilt.
//  7. Logs carry counts only: `AUTORADIO embed model=<ok|downloading|absent> items=<done>/<total> rate=<per s>`.

/// What the radio holds the processor for (one place the app and the tests say it): speech on air, speech being synthesised.
public final class RadioDemand: @unchecked Sendable {
    public static let shared = RadioDemand()
    private let lock = NSLock()
    private var sources: [@Sendable () -> Bool] = []
    private var remote = false
    public init() {}
    /// Remote audio (a podcast, an Autopaper clip) is playing: set by the host from the main actor, read from any thread.
    public func setRemotePlaying(_ on: Bool) { lock.lock(); remote = on; lock.unlock() }
    public func add(_ source: @escaping @Sendable () -> Bool) { lock.lock(); sources.append(source); lock.unlock() }
    public var busy: Bool { lock.lock(); let s = sources, r = remote; lock.unlock(); return r || s.contains { $0() } }
}

public struct DeviceConditions: Sendable, Equatable {
    public var lowPowerMode: Bool, thermalSerious: Bool, onWiFi: Bool
    public init(lowPowerMode: Bool, thermalSerious: Bool, onWiFi: Bool) { self.lowPowerMode = lowPowerMode; self.thermalSerious = thermalSerious; self.onWiFi = onWiFi }
}

/// Tells whether the phone is on Wi-Fi (or wired); the model is never fetched over cellular.
final class WiFiMonitor: @unchecked Sendable {
    static let shared = WiFiMonitor()
    private let monitor = NWPathMonitor()
    private let lock = NSLock()
    private var wifi = false
    var onChange: (@Sendable () -> Void)?
    private init() {
        monitor.pathUpdateHandler = { [weak self] p in
            guard let self else { return }
            let w = p.status == .satisfied && (p.usesInterfaceType(.wifi) || p.usesInterfaceType(.wiredEthernet)) && !p.isExpensive
            self.lock.lock(); let changed = self.wifi != w; self.wifi = w; self.lock.unlock()
            if changed, w { self.onChange?() }
        }
        monitor.start(queue: DispatchQueue(label: "embed.wifi", qos: .utility))
    }
    var onWiFi: Bool { lock.lock(); defer { lock.unlock() }; return wifi }
}

public final class EmbeddingService: @unchecked Sendable {
    public enum ModelState: String, Sendable { case ok, downloading, absent }
    public struct Progress: Sendable, Equatable { public var done = 0, total = 0, rate = 0.0 }

    public struct Environment: Sendable {
        public var conditions: @Sendable () -> DeviceConditions = {
            DeviceConditions(lowPowerMode: ProcessInfo.processInfo.isLowPowerModeEnabled,
                             thermalSerious: ProcessInfo.processInfo.thermalState.rawValue >= ProcessInfo.ThermalState.serious.rawValue, onWiFi: WiFiMonitor.shared.onWiFi)
        }
        public var radioBusy: @Sendable () -> Bool = { RadioDemand.shared.busy }
        public var modelPresent: @Sendable () async -> Bool = { await ModelStore.shared.localFileIfPresent(KnownModels.embeddingGemma2.id) != nil }
        public var download: @Sendable () async throws -> Void = { _ = try await ModelStore.shared.ensure(KnownModels.embeddingGemma2.id) }
        /// Loads the model (the models module sets `EmbeddingService.loader`; the kit has no llama.cpp).
        public var load: @Sendable () async throws -> any Embedder & QueryEmbedding = { try await EmbeddingService.loader() }
        public var say: @Sendable (String) -> Void = { fputs($0 + "\n", stderr) }
        public var vectorsDirectory: @Sendable () -> URL? = { nil }       // the vectors lived under the Swift content store; search is shared code (AutoradioLab/search.ts) and this service is the retrieval lane's to delete
        /// A wait before the first pass after `attach` (launch must not wait on this) and between passes (a sync puts many items).
        public var firstDelay: Duration = .seconds(15)
        public var settle: Duration = .seconds(3)
        public var budget = EmbeddingJob.Budget()
        public var dimension = 768
        public init() {}
    }

    nonisolated(unsafe) public static var loader: @Sendable () async throws -> any Embedder & QueryEmbedding = { throw EmbedderError.unavailable("no embedding runtime in this build") }
    public static let shared = EmbeddingService(Environment())

    private let env: Environment
    private let lock = NSLock()
    private var corpus: SavedCorpus?
    private var vectors: VectorStore?
    private var embedder: (any Embedder & QueryEmbedding)?
    private var runner: Task<Void, Never>?
    private var wanted = false, generation = 0, attachedOnce = false
    private var _state = ModelState.absent
    private var _progress = Progress()
    private var lastSaid = "", lastSaidAt = Date.distantPast
    public var modelState: ModelState { lock.lock(); defer { lock.unlock() }; return _state }
    public var progress: Progress { lock.lock(); defer { lock.unlock() }; return _progress }

    public init(_ env: Environment) {
        self.env = env
        // The job saves every few dozen slices; the app going to the background saves what is pending (the platform's own notification names).
        for n in ["UIApplicationDidEnterBackgroundNotification", "NSApplicationDidResignActiveNotification"] {
            NotificationCenter.default.addObserver(forName: Notification.Name(n), object: nil, queue: nil) { [weak self] _ in DispatchQueue.global(qos: .utility).async { self?.flush() } }
        }
    }

    /// Write the vectors now (a no-op when nothing changed).
    public func flush() { lock.lock(); let v = vectors; lock.unlock(); try? v?.save() }

    /// Wire to the corpus (and, in the app, the store): retrieval is BM25 now and becomes fused once the vectors are there.
    public func attach(corpus: SavedCorpus) {
        guard let dir = env.vectorsDirectory() else { return }
        let v = VectorStore(directory: dir, engine: KnownModels.embeddingEngine, dimension: env.dimension)
        lock.lock(); self.corpus = corpus; vectors = v; lock.unlock()
        let fused = FusedRetriever(index: corpus.retrieval, vectors: v,
                                   embedQuery: { [weak self] q in self?.currentEmbedder()?.embedQuery(q) },
                                   coverage: { [weak self] in
                                       guard let self else { return 0 }
                                       let p = self.progress; return p.total == 0 ? (v.count > 0 ? 1 : 0) : min(1, Double(v.count) / Double(p.total)) })
        fused.say = env.say
        corpus.retriever = fused
        corpus.resetHooks.append { [weak self] in self?.wipe() }
        WiFiMonitor.shared.onChange = { [weak self] in self?.kick() }
        lock.lock(); let first = !attachedOnce; attachedOnce = true; lock.unlock()
        kick(first: first)
    }

    private func currentEmbedder() -> (any Embedder & QueryEmbedding)? { lock.lock(); defer { lock.unlock() }; return embedder }

    /// Something changed (a store write, Wi-Fi came back): run a pass soon. Passes never overlap.
    public func kick(first: Bool = false) {
        lock.lock()
        wanted = true
        if runner == nil {
            let g = generation
            runner = Task.detached(priority: .utility) { [weak self] in await self?.loop(generation: g, first: first) }
        }
        lock.unlock()
    }
    /// For tests: returns when no pass is running or wanted.
    public func waitIdle() async {
        while true {
            lock.lock(); let r = runner; lock.unlock()
            guard let r else { return }
            await r.value
        }
    }

    private func loop(generation g: Int, first: Bool) async {
        var delay = first ? env.firstDelay : env.settle
        while true {
            try? await Task.sleep(for: delay); delay = env.settle
            lock.lock()
            if g != generation { runner = nil; lock.unlock(); return }
            guard wanted else { runner = nil; lock.unlock(); return }
            wanted = false; lock.unlock()
            await pass(generation: g)
        }
    }

    private func say(_ model: ModelState, force: Bool = false) {
        let p = progress
        let s = "AUTORADIO embed model=\(model.rawValue) items=\(p.done)/\(p.total) rate=\(String(format: "%.1f", p.rate))"
        lock.lock(); let due = force || Date().timeIntervalSince(lastSaidAt) >= 1; let same = s == lastSaid
        if !same && due { lastSaid = s; lastSaidAt = Date() }
        lock.unlock()
        if !same && due { env.say(s) }
    }
    private func set(_ m: ModelState) { lock.lock(); _state = m; lock.unlock() }

    /// The items to embed: every live item of the index, a long one as windows of about 180 words, newest first, undated last.
    public static func items(from index: RetrievalIndex) -> [EmbeddingJob.Item] {
        var out: [(date: Date?, item: EmbeddingJob.Item)] = []
        for it in index.snapshot() where !(it.title.isEmpty && it.text.isEmpty) {
            let wins = RetrievalText.passages(it.text, words: 180)
            if wins.count <= 1 { out.append((it.date, .init(id: it.id, key: Self.key(it.title, it.text), title: it.title, text: it.text))) }
            else { for (i, w) in wins.enumerated() { out.append((it.date, .init(id: "\(it.id)~w\(i + 1)", key: Self.key(it.title, w), title: it.title, text: w))) } }
        }
        out.sort { a, b in
            switch (a.date, b.date) {
            case let (x?, y?): return x != y ? x > y : a.item.id < b.item.id
            case (_?, nil): return true
            case (nil, _?): return false
            default: return a.item.id < b.item.id
            }
        }
        return out.map(\.item)
    }
    static func key(_ title: String, _ text: String) -> String {
        var h: UInt64 = 0xcbf29ce484222325
        for u in title.utf8 { h = (h ^ UInt64(u)) &* 0x100000001b3 }
        h = (h ^ 0x1f) &* 0x100000001b3
        for u in text.utf8 { h = (h ^ UInt64(u)) &* 0x100000001b3 }
        return String(h, radix: 16)
    }

    private func pass(generation g: Int) async {
        lock.lock(); let corpus = self.corpus, vectors = self.vectors; lock.unlock()
        guard let corpus, let vectors else { return }
        corpus.refresh()
        let items = Self.items(from: corpus.retrieval)
        lock.lock(); _progress.total = items.count; _progress.done = min(vectors.count, items.count); lock.unlock()
        guard !items.isEmpty else { return }
        // The model: on the phone, or fetched once on Wi-Fi.
        if currentEmbedder() == nil {
            if !(await env.modelPresent()) {
                guard env.conditions().onWiFi else { set(.absent); say(.absent, force: true); return }
                set(.downloading); say(.downloading, force: true)
                do { try await env.download() } catch { set(.absent); say(.absent, force: true); return }
            }
            do { let e = try await env.load(); lock.lock(); embedder = e; lock.unlock() } catch { set(.absent); say(.absent, force: true); return }
        }
        set(.ok); say(.ok, force: true)
        guard let e = currentEmbedder() else { return }
        let env = self.env
        let started = Date(), before = vectors.count
        let job = EmbeddingJob(store: vectors, embedder: e,
                               busy: { env.radioBusy() || env.conditions().lowPowerMode || env.conditions().thermalSerious }, budget: env.budget)
        _ = await job.run(items) { [weak self] p in
            guard let self else { return }
            let n = vectors.count
            self.lock.lock()
            self._progress.done = min(n, items.count)
            let dt = Date().timeIntervalSince(started); self._progress.rate = dt > 0 ? Double(max(0, n - before)) / dt : 0
            self.lock.unlock()
            self.say(.ok)
        }
        lock.lock(); _progress.done = min(vectors.count, items.count); lock.unlock()
        say(.ok, force: true)
        lock.lock(); let stale = g != generation; lock.unlock()
        if stale { vectors.wipe() }                  // a sign-out came while the last slice was running
    }

    /// Sign-out: the pass stops, the vectors go, retrieval is BM25 alone.
    public func wipe() {
        lock.lock(); generation += 1; wanted = false; let r = runner; runner = nil; let v = vectors; _progress = Progress(); lastSaid = ""; lock.unlock()
        r?.cancel()
        v?.wipe()
    }
}
