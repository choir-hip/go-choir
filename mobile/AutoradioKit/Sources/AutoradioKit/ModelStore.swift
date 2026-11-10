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

/// Models are fetched on demand to Application Support, pinned by SHA-256, resumable (HTTP Range on a .part file),
/// and unloaded when the current configuration no longer needs them. Lane K ships it with no models registered.
public actor ModelStore {
    public let root: URL
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
