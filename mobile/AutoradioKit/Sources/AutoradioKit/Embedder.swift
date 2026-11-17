import Foundation
import Accelerate

// On-device embeddings (docs/reference/EMBEDDINGGEMMA.md). The kit knows the interface, the prompt strings, the vector store and the
// background job; the model runs in AutoradioModels (llama.cpp, the official GGUF). Nothing here talks to a server.

/// The prompt strings of EmbeddingGemma 2, verbatim from the model card's config.json.
public enum EmbedPrompt {
    public static func document(title: String?, text: String) -> String {
        let t = (title ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        return "title: \(t.isEmpty ? "none" : t) | text: \(text)"
    }
    public static func query(_ q: String) -> String { "task: search result | query: \(q)" }
}

/// A model that turns finished prompt strings into unit vectors. `engine` names the model file and runtime; vectors made by different
/// engines are not interchangeable, so a store records the engine and starts again when it changes.
public protocol Embedder: Sendable {
    var engine: String { get }
    var dimension: Int { get }
    func embed(_ prompts: [String]) async throws -> [[Float]]
}

public enum EmbedderError: Error, Equatable { case unavailable(String) }

/// Cut a vector to its first `dim` values and make it unit length again (Matryoshka truncation).
public func truncatedUnit(_ v: [Float], dim: Int) -> [Float] {
    let h = Array(v.prefix(dim)); var s: Float = 0
    vDSP_svesq(h, 1, &s, vDSP_Length(h.count))
    let n = s.squareRoot()
    return n > 0 ? h.map { $0 / n } : h
}

public enum RankFusion {
    /// Reciprocal rank fusion (Cormack, Clarke and Buettcher, SIGIR 2009), k = 60, over ranked id lists (best first).
    public static func rrf(_ lists: [[String]], k: Double = 60) -> [String] {
        var s: [String: Double] = [:]
        for l in lists { for (i, id) in l.enumerated() { s[id, default: 0] += 1 / (k + Double(i + 1)) } }
        return s.sorted { $0.value != $1.value ? $0.value > $1.value : $0.key < $1.key }.map { $0.key }
    }
}

/// Unit vectors on disk as one flat float16 file (rows in order) beside a small JSON file of ids and content keys, searched by a brute-force
/// cosine scan (Accelerate). A row is added or replaced by id; the store is saved atomically, vectors first, so a kill leaves a store that
/// opens (rows the JSON names but the vector file lacks are dropped on open).
public final class VectorStore: @unchecked Sendable {
    public let directory: URL, engine: String, dimension: Int
    private let lock = NSLock()
    private var ids: [String] = [], keys: [String] = []
    private var vec: [Float16] = []
    private var slot: [String: Int] = [:]
    private var dirty = false
    private struct Meta: Codable { var engine: String; var dimension: Int; var ids: [String]; var keys: [String] }
    private var vectorsURL: URL { directory.appendingPathComponent("vectors.f16") }
    private var metaURL: URL { directory.appendingPathComponent("rows.json") }

    public init(directory: URL, engine: String, dimension: Int) {
        self.directory = directory; self.engine = engine; self.dimension = dimension
        Self.prepare(directory)
        guard let m = (try? Data(contentsOf: metaURL)).flatMap({ try? JSONDecoder().decode(Meta.self, from: $0) }),
              m.engine == engine, m.dimension == dimension, m.ids.count == m.keys.count,
              let d = try? Data(contentsOf: vectorsURL, options: .mappedIfSafe) else { return }
        let have = min(m.ids.count, d.count / 2 / dimension)
        vec = d.withUnsafeBytes { Array($0.bindMemory(to: Float16.self).prefix(have * dimension)) }
        ids = Array(m.ids.prefix(have)); keys = Array(m.keys.prefix(have))
        for (i, id) in ids.enumerated() { slot[id] = i }
    }

    /// The folder is made with the content store's protection class and left out of device backups (everything here is derived from the store).
    static func prepare(_ directory: URL) {
        try? FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try? FileManager.default.setAttributes([.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication], ofItemAtPath: directory.path)
        var v = URLResourceValues(); v.isExcludedFromBackup = true
        var d = directory; try? d.setResourceValues(v)
    }
    /// Sign-out: forget every vector, in memory and on disk.
    public func wipe() {
        lock.lock(); ids = []; keys = []; vec = []; slot = [:]; dirty = false; lock.unlock()
        try? FileManager.default.removeItem(at: directory)
        Self.prepare(directory)
    }
    public var count: Int { lock.lock(); defer { lock.unlock() }; return ids.count }
    /// The content key each stored id was made from (what the job compares to decide what to embed again).
    public func contentKeys() -> [String: String] { lock.lock(); defer { lock.unlock() }; return Dictionary(uniqueKeysWithValues: zip(ids, keys)) }

    public func upsert(id: String, key: String, vector: [Float]) {
        guard vector.count >= dimension else { return }
        let v = truncatedUnit(vector, dim: dimension).map { Float16($0) }
        lock.lock(); defer { lock.unlock() }
        if let i = slot[id] { vec.replaceSubrange(i * dimension..<(i + 1) * dimension, with: v); keys[i] = key }
        else { slot[id] = ids.count; ids.append(id); keys.append(key); vec.append(contentsOf: v) }
        dirty = true
    }
    public func remove(_ id: String) {
        lock.lock(); defer { lock.unlock() }
        guard let i = slot.removeValue(forKey: id) else { return }
        let last = ids.count - 1
        if i != last {
            vec.replaceSubrange(i * dimension..<(i + 1) * dimension, with: vec[last * dimension..<(last + 1) * dimension])
            ids[i] = ids[last]; keys[i] = keys[last]; slot[ids[i]] = i
        }
        vec.removeLast(dimension); ids.removeLast(); keys.removeLast(); dirty = true
    }
    public func removeAll(except keep: Set<String>) { for id in contentKeys().keys where !keep.contains(id) { remove(id) } }

    public func save() throws {
        lock.lock(); let v = vec, m = Meta(engine: engine, dimension: dimension, ids: ids, keys: keys); let was = dirty; dirty = false; lock.unlock()
        guard was else { return }
        try v.withUnsafeBytes { Data($0) }.write(to: vectorsURL, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
        try JSONEncoder().encode(m).write(to: metaURL, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    }

    /// The best `k` rows by cosine (the stored vectors and `query` are unit length, so a dot product), best first.
    public func search(_ query: [Float], k: Int = 50) -> [(id: String, score: Float)] {
        let q = truncatedUnit(query, dim: dimension)
        lock.lock(); defer { lock.unlock() }
        let rows = ids.count
        guard rows > 0, q.count == dimension else { return [] }
        let block = 4096
        var buf = [Float](repeating: 0, count: block * dimension), out = [Float](repeating: 0, count: block)
        var best: [(Int, Float)] = []; var floor: Float = -2
        var r = 0
        while r < rows {
            let m = min(block, rows - r)
            vec.withUnsafeBufferPointer { src in buf.withUnsafeMutableBufferPointer { b in for i in 0..<(m * dimension) { b[i] = Float(src[r * dimension + i]) } } }
            cblas_sgemv(CblasRowMajor, CblasNoTrans, Int32(m), Int32(dimension), 1, buf, Int32(dimension), q, 1, 0, &out, 1)
            for i in 0..<m where out[i] > floor { best.append((r + i, out[i])) }
            if best.count > 4 * k { best.sort { $0.1 > $1.1 }; best = Array(best.prefix(k)); floor = best.last!.1 }
            r += m
        }
        best.sort { $0.1 > $1.1 }
        return best.prefix(k).map { (id: ids[$0.0], score: $0.1) }
    }
}

/// The background embedding job: brings a store in line with the documents the app holds. It embeds only what is new or changed (by
/// content key), in small slices, saves after each slice so a kill loses at most one slice, and waits while the radio needs the
/// processor (`busy`, true while a voice line is on or the listener is speaking): speech always wins. It never starts by itself after a
/// crash; the caller starts it when stores change.
public actor EmbeddingJob {
    public struct Item: Sendable { public var id: String, key: String, title: String?, text: String
        public init(id: String, key: String, title: String? = nil, text: String) { self.id = id; self.key = key; self.title = title; self.text = text } }
    public struct Budget: Sendable {
        /// Passages embedded between two looks at `busy` and two saves.
        public var passagesPerSlice = 4
        /// A rest between slices, so the processor is not held for the whole backlog.
        public var restBetweenSlices: Duration = .milliseconds(150)
        /// How long to wait before looking at `busy` again.
        public var busyPoll: Duration = .milliseconds(250)
        /// Longest text sent to the model, in characters (about 512 tokens at 3.5 characters).
        public var maxChars = 1800
        public init() {}
    }
    public struct Progress: Sendable, Equatable { public var embedded = 0, skipped = 0, removed = 0, waitedForSpeech = 0, failed = 0, finished = false }

    private let store: VectorStore, embedder: Embedder, busy: @Sendable () -> Bool, budget: Budget
    public private(set) var progress = Progress()
    public init(store: VectorStore, embedder: Embedder, busy: @escaping @Sendable () -> Bool, budget: Budget = Budget()) {
        self.store = store; self.embedder = embedder; self.busy = busy; self.budget = budget
    }

    /// Embed what the store lacks. Returns when done or when the task is cancelled (what was embedded stays saved).
    @discardableResult
    public func run(_ items: [Item], removeOthers: Bool = true, onSlice: (@Sendable (Progress) -> Void)? = nil) async -> Progress {
        progress = Progress()
        let have = store.contentKeys()
        if removeOthers { let keep = Set(items.map(\.id)); progress.removed = have.keys.filter { !keep.contains($0) }.count; store.removeAll(except: keep) }
        let todo = items.filter { have[$0.id] != $0.key }
        progress.skipped = items.count - todo.count
        var i = 0
        while i < todo.count, !Task.isCancelled {
            if busy() { progress.waitedForSpeech += 1; try? await Task.sleep(for: budget.busyPoll); continue }
            let slice = Array(todo[i..<min(todo.count, i + budget.passagesPerSlice)])
            do {
                let vs = try await embedder.embed(slice.map { EmbedPrompt.document(title: $0.title, text: String($0.text.prefix(budget.maxChars))) })
                for (it, v) in zip(slice, vs) { store.upsert(id: it.id, key: it.key, vector: v); progress.embedded += 1 }
            } catch { progress.failed = todo.count - i; break }     // the model is not there or failed: stop, the caller sees `failed`
            try? store.save()
            i += slice.count
            onSlice?(progress)
            try? await Task.sleep(for: budget.restBetweenSlices)
        }
        try? store.save()
        progress.finished = i >= todo.count
        return progress
    }
}

/// An embedder that can also embed one short query at once, without waiting on an async hop (the retriever runs inside a synchronous call).
public protocol QueryEmbedding: Sendable { func embedQuery(_ text: String) -> [Float]? }
