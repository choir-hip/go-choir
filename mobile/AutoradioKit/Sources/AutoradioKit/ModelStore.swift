import Foundation
import CryptoKit

public struct ModelSpec: Codable, Sendable {
    public var id: String
    public var url: URL
    public var sha256: String       // hex
    public var bytes: Int64
    public var fileName: String
    public init(id: String, url: URL, sha256: String, bytes: Int64, fileName: String) {
        self.id = id; self.url = url; self.sha256 = sha256; self.bytes = bytes; self.fileName = fileName
    }
}

/// A model a library fetches and caches itself (FluidAudio's Core ML bundles): listed, measured and unloaded here.
public struct ManagedModelSpec: Codable, Sendable {
    public var id: String
    public var label: String
    public var source: String          // e.g. a Hugging Face repo
    public var approxBytes: Int64      // from the model card; the measured size is in the load report
    public var license: String
    public var cachePath: URL?         // where the library caches it, to measure bytes fetched
    public init(id: String, label: String, source: String, approxBytes: Int64, license: String, cachePath: URL? = nil) {
        self.id = id; self.label = label; self.source = source; self.approxBytes = approxBytes; self.license = license; self.cachePath = cachePath
    }
}
public struct ModelLoadReport: Codable, Sendable {
    public var id: String
    public var loadMs: Double
    public var fetchedBytes: Int64
    public var cacheBytes: Int64
    public var residentBeforeMB: Double
    public var residentAfterMB: Double
    public var residentAfterUnloadMB: Double?
}

/// Models are fetched on demand to Application Support, pinned by SHA-256, resumable (HTTP Range on a .part file),
/// and unloaded when the current configuration no longer needs them. Lane K ships it with no models registered.
public actor ModelStore {
    public nonisolated let root: URL
    private var specs: [String: ModelSpec] = [:]
    private var unloaders: [String: @Sendable () -> Void] = [:]
    private var loaded: Set<String> = []
    private let log: EventLog?
    public init(root: URL, log: EventLog? = nil) {
        self.root = root; self.log = log
        try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }
    public func register(_ s: ModelSpec) { specs[s.id] = s }
    public var registered: [ModelSpec] { Array(specs.values) }

    /// Fetch (or resume) and verify; returns the local file. `progress` receives bytes so far.
    public func ensure(_ id: String, progress: (@Sendable (Int64, Int64) -> Void)? = nil) async throws -> URL {
        guard let s = specs[id] else { throw LabError.badInput("model \(id) not registered") }
        let final = root.appendingPathComponent(s.fileName)
        if FileManager.default.fileExists(atPath: final.path), try sha(final) == s.sha256.lowercased() { return final }
        let part = final.appendingPathExtension("part")
        let have = ((try? FileManager.default.attributesOfItem(atPath: part.path)[.size]) as? NSNumber)?.int64Value ?? 0
        var req = URLRequest(url: s.url)
        if have > 0 { req.setValue("bytes=\(have)-", forHTTPHeaderField: "Range") }
        let (bytes, resp) = try await URLSession.shared.bytes(for: req)
        let code = (resp as? HTTPURLResponse)?.statusCode ?? 0
        guard code == 200 || code == 206 else { throw LabError.http(code) }
        if code == 200 { try? FileManager.default.removeItem(at: part) }   // server ignored Range: restart
        if !FileManager.default.fileExists(atPath: part.path) { FileManager.default.createFile(atPath: part.path, contents: nil) }
        let h = try FileHandle(forWritingTo: part)
        try h.seekToEnd()
        var done = Int64(try h.offset())
        var buf = Data()
        for try await b in bytes {
            buf.append(b)
            if buf.count >= 1 << 20 { try h.write(contentsOf: buf); done += Int64(buf.count); buf.removeAll(keepingCapacity: true); progress?(done, s.bytes) }
        }
        try h.write(contentsOf: buf); try h.close()
        guard try sha(part) == s.sha256.lowercased() else { try? FileManager.default.removeItem(at: part); throw LabError.badInput("sha256 mismatch for \(id)") }
        try? FileManager.default.removeItem(at: final)
        try FileManager.default.moveItem(at: part, to: final)
        return final
    }

    func sha(_ url: URL) throws -> String {
        let f = try FileHandle(forReadingFrom: url); defer { try? f.close() }
        var h = SHA256()
        while let d = try f.read(upToCount: 1 << 20), !d.isEmpty { h.update(data: d) }
        return h.finalize().map { String(format: "%02x", $0) }.joined()
    }

    // MARK: Library-managed models
    // Some libraries (FluidAudio) fetch and cache their own Core ML bundles. Those are registered here so the lab lists
    // them, measures them (bytes fetched, load time, resident memory before/after, unload) and unloads the others first.

    private var managed: [String: ManagedModelSpec] = [:]
    private var reports: [String: ModelLoadReport] = [:]
    public func register(managed s: ManagedModelSpec) { managed[s.id] = s }
    public var managedSpecs: [ManagedModelSpec] { managed.values.sorted { $0.id < $1.id } }
    public var loadReports: [ModelLoadReport] { reports.values.sorted { $0.id < $1.id } }
    public var loadedIDs: Set<String> { loaded }

    /// Run `load` (which fetches if needed and loads, and returns what unloads it) and record what it cost.
    public func measuredLoad(_ id: String, _ load: () async throws -> (@Sendable () -> Void)) async throws -> ModelLoadReport {
        let spec = managed[id]
        let before = residentBytes(), cacheBefore = spec?.cachePath.map(Self.directoryBytes) ?? 0
        let t0 = DispatchTime.now().uptimeNanoseconds
        let unload = try await load()
        let ms = Double(DispatchTime.now().uptimeNanoseconds - t0) / 1e6
        let after = residentBytes(), cacheAfter = spec?.cachePath.map(Self.directoryBytes) ?? 0
        let r = ModelLoadReport(id: id, loadMs: ms, fetchedBytes: max(0, cacheAfter - cacheBefore), cacheBytes: cacheAfter,
                                residentBeforeMB: Double(before) / 1_048_576, residentAfterMB: Double(after) / 1_048_576)
        reports[id] = r
        didLoad(id, ms: ms, unload: unload)
        return r
    }
    /// Unload one model and record the resident memory afterwards.
    public func unload(_ id: String) {
        guard loaded.contains(id) else { return }
        unloaders[id]?(); unloaders[id] = nil; loaded.remove(id)
        if var r = reports[id] { r.residentAfterUnloadMB = Double(residentBytes()) / 1_048_576; reports[id] = r }
        log?.emit(.note("model \(id) unloaded, \(Int(Double(residentBytes()) / 1_048_576)) MB resident"))
    }
    /// Unload everything except `keep` (used before a model that needs the whole budget loads).
    public func unloadAll(except keep: Set<String>) { for id in loaded.subtracting(keep) { unload(id) } }

    static func directoryBytes(_ url: URL) -> Int64 {
        guard let e = FileManager.default.enumerator(at: url, includingPropertiesForKeys: [.totalFileAllocatedSizeKey, .isRegularFileKey]) else { return 0 }
        var n: Int64 = 0
        for case let u as URL in e {
            let v = try? u.resourceValues(forKeys: [.totalFileAllocatedSizeKey, .isRegularFileKey])
            if v?.isRegularFile == true { n += Int64(v?.totalFileAllocatedSize ?? 0) }
        }
        return n
    }

    /// A stage that loads a model reports it so the trace shows load time and memory.
    public func didLoad(_ id: String, ms: Double, unload: @escaping @Sendable () -> Void) {
        loaded.insert(id); unloaders[id] = unload
        log?.emit(.modelLoaded(id: id, ms: ms, residentBytes: residentBytes()))
    }
    /// Unload every loaded model the current configuration does not need.
    public func unloadUnneeded(needed: Set<String>) {
        for id in loaded.subtracting(needed) { unloaders[id]?(); unloaders[id] = nil; loaded.remove(id) }
    }
    public static func needed(by config: LabConfiguration, registry: StageRegistry) -> Set<String> {
        var out = Set<String>()
        for k in StageKind.allCases {
            if let s = config.setting(k), let d = registry.stages(k).first(where: { $0.id == s.id })?.descriptor { out.formUnion(d.needsModels) }
        }
        return out
    }
}
