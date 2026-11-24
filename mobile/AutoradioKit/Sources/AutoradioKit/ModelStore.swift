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

/// A model that is a folder of files in one Hugging Face repository, every file pinned by the repository's commit and by its own
/// SHA-256 (Fluid Speech Mini). The store fetches each file the way it fetches a single-file model, and the folder is whole only
/// when every file is there at its size (a file is moved into place only after its SHA-256 matched).
public struct ModelBundleSpec: Codable, Sendable {
    public struct File: Codable, Sendable, Equatable {
        public var path: String
        public var bytes: Int64
        public var sha256: String
        public init(path: String, bytes: Int64, sha256: String) { self.path = path; self.bytes = bytes; self.sha256 = sha256 }
    }
    public var id: String
    public var repo: String
    /// The repository's commit the sizes and digests were read at; every URL names it, so a later push to the repository changes nothing here.
    public var commit: String
    public var files: [File]
    /// Folder under the store root.
    public var folder: String
    public var license: String
    /// What the licence asks us to show the person, word for word.
    public var credit: String
    /// Files to copy after the download: (from, to), for names a loader expects that the repository does not use.
    public var copies: [[String]]
    public init(id: String, repo: String, commit: String, files: [File], folder: String, license: String, credit: String, copies: [[String]] = []) {
        self.id = id; self.repo = repo; self.commit = commit; self.files = files; self.folder = folder; self.license = license
        self.credit = credit; self.copies = copies
    }
    /// What a served list may ask a phone to fetch: no file over 4 GiB, no model over 8 GiB, no more than 256 files. The sum saturates instead of trapping.
    public static let maxFileBytes: Int64 = 4 << 30, maxModelBytes: Int64 = 8 << 30, maxFiles = 256
    public var totalBytes: Int64 { files.reduce(0) { let (n, o) = $0.addingReportingOverflow($1.bytes); return o ? Int64.max : n } }
    /// `owner/name` of a Hugging Face repository: letters, digits, dot, dash and underscore, no `.` or `..` part.
    public static func isValidRepository(_ r: String) -> Bool {
        let parts = r.split(separator: "/", omittingEmptySubsequences: false)
        return parts.count == 2 && parts.allSatisfy { !$0.isEmpty && $0 != "." && $0 != ".." && $0.unicodeScalars.allSatisfy { $0.isASCII && ($0.properties.isAlphabetic || ("0"..."9").contains($0) || "._-".unicodeScalars.contains($0)) } }
    }
    static func isHex(_ s: String, _ n: Int) -> Bool { s.count == n && s.allSatisfy { $0.isASCII && $0.isHexDigit && !$0.isUppercase } }
    /// A path from a list a server serves must stay inside the model's own folder: relative, no empty, `.` or `..` component, no backslash or NUL.
    public static func isSafeRelativePath(_ p: String) -> Bool {
        guard !p.isEmpty, !p.hasPrefix("/"), !p.contains("\\"), !p.unicodeScalars.contains("\0") else { return false }
        return p.split(separator: "/", omittingEmptySubsequences: false).allSatisfy { !$0.isEmpty && $0 != "." && $0 != ".." }
    }
    /// The folder, every file's path and every copy name stay inside the store root.
    public var pathsStayInside: Bool {
        Self.isSafeRelativePath(folder) && files.allSatisfy { Self.isSafeRelativePath($0.path) }
            && copies.allSatisfy { $0.count == 2 && Self.isSafeRelativePath($0[0]) && Self.isSafeRelativePath($0[1]) }
    }
    /// Everything the store needs to be able to fetch this model without trapping or leaving its folder: paths, repository, commit, digests and sizes.
    public var isWellFormed: Bool {
        pathsStayInside && Self.isValidRepository(repo) && Self.isHex(commit, 40) && files.count <= Self.maxFiles
            && files.allSatisfy { $0.bytes > 0 && $0.bytes <= Self.maxFileBytes && Self.isHex($0.sha256, 64) } && totalBytes <= Self.maxModelBytes
    }
    /// The address of a file. The path is percent-encoded here, so no served text can make the address invalid.
    public func url(for f: File) -> URL {
        let path = f.path.split(separator: "/", omittingEmptySubsequences: false).map { String($0).addingPercentEncoding(withAllowedCharacters: .urlPathAllowed)?.replacingOccurrences(of: "%2F", with: "/") ?? "" }.joined(separator: "/")
        return URL(string: "https://huggingface.co/\(repo)/resolve/\(commit)/\(path)") ?? URL(string: "https://huggingface.co/")!   // refused earlier by `isWellFormed`
    }
    /// What the folder on the phone must record to be this model: the commit and every file's path, size and digest, and the copies.
    var pinIdentity: String {
        let lines = [commit] + files.map { "\($0.path) \($0.bytes) \($0.sha256)" } + copies.map { $0.joined(separator: ">") }
        return SHA256.hash(data: Data(lines.joined(separator: "\n").utf8)).map { String(format: "%02x", $0) }.joined()
    }
    /// The id the store registers a file under.
    func fileID(_ f: File) -> String { id + "/" + f.path }
}

/// A model a library fetches and caches itself (FluidAudio's Core ML bundles): listed, measured and unloaded here.
public struct ManagedModelSpec: Codable, Sendable {
    public var id: String
    public var label: String
    public var source: String          // e.g. a Hugging Face repo
    public var approxBytes: Int64      // from the model card; the measured size is in the load report
    public var license: String
    public var cachePath: URL?         // where the library caches it, to measure bytes fetched
    /// The folders under `cachePath` that belong to this model alone (FluidAudio names them by its own rules). Deleting a rejected
    /// configuration's model removes exactly these, never `cachePath` itself, which other models share. nil: the kit does not delete it.
    public var cacheFolders: [String]?
    /// The credit its licence asks the app to show (CC BY 4.0 and the like); nil when the licence asks for none.
    public var credit: String?
    public init(id: String, label: String, source: String, approxBytes: Int64, license: String, cachePath: URL? = nil, cacheFolders: [String]? = nil, credit: String? = nil) {
        self.credit = credit
        self.id = id; self.label = label; self.source = source; self.approxBytes = approxBytes; self.license = license; self.cachePath = cachePath
        self.cacheFolders = cacheFolders
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
    private var log: EventLog?
    /// A test's own transport; the door still authorises (`DownloadDoor.session(for:injected:)`).
    let injectedSession: URLSession?
    let door: DownloadDoor
    /// The one store of the app: voice setup, the engine's stages and the proof all use it. It is created with the whole
    /// catalog (`KnownModels`) registered, so no model setup can ask for is ever "not registered".
    public static let shared = ModelStore(root: KnownModels.storeRoot, registerKnown: true)
    public init(root: URL, log: EventLog? = nil, registerKnown: Bool = false, session: URLSession? = nil, door: DownloadDoor = .shared) {
        self.root = root; self.log = log; self.injectedSession = session; self.door = door
        try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        if registerKnown {
            for s in KnownModels.pinned { specs[s.id] = s }
            for m in KnownModels.managed { managed[m.id] = m }
        }
    }
    /// The runtime attaches its event log so loads show in the trace.
    public func attach(log: EventLog) { self.log = log }
    public func isRegistered(_ id: String) -> Bool { specs[id] != nil || managed[id] != nil }

    /// Delete a pinned model's weights (the file and any partial download). The configurator removes what a rejected configuration fetched.
    public func delete(_ id: String) {
        guard let s = specs[id] else { return }
        let final = root.appendingPathComponent(s.fileName)
        try? FileManager.default.removeItem(at: final)
        try? FileManager.default.removeItem(at: final.appendingPathExtension("part"))
        loaded.remove(id)
        log?.emit(.note("model \(id) deleted"))
    }

    /// The model's file if it is on the phone already, whole (the file is moved into place only after its SHA-256 matched, so
    /// the size is enough here). Never fetches.
    public func localFileIfPresent(_ id: String) -> URL? {
        guard let s = specs[id] else { return nil }
        let final = root.appendingPathComponent(s.fileName)
        let size = ((try? FileManager.default.attributesOfItem(atPath: final.path)[.size]) as? NSNumber)?.int64Value ?? -1
        return size == s.bytes ? final : nil
    }
    /// A spec whose file name leaves the store root is never kept.
    public func register(_ s: ModelSpec) { if ModelBundleSpec.isSafeRelativePath(s.fileName) { specs[s.id] = s } }

    // MARK: Folder models (one repository, many files)

    /// The folder of a bundle model if every file is on the phone at its size and the copies are made. Never fetches.
    static let pinFile = ".pin"
    public func bundleDirectoryIfPresent(_ b: ModelBundleSpec) -> URL? {
        guard b.isWellFormed else { return nil }
        let dir = root.appendingPathComponent(b.folder, isDirectory: true)
        let fm = FileManager.default
        // The folder says which pin it holds (written last, removed first): a folder of another pin, or one left half replaced, is not this model.
        guard (try? String(contentsOf: dir.appendingPathComponent(Self.pinFile), encoding: .utf8)) == b.pinIdentity else { return nil }
        for f in b.files {
            let size = ((try? fm.attributesOfItem(atPath: dir.appendingPathComponent(f.path).path)[.size]) as? NSNumber)?.int64Value ?? -1
            if size != f.bytes { return nil }
        }
        for c in b.copies where c.count == 2 { if !fm.fileExists(atPath: dir.appendingPathComponent(c[1]).path) { return nil } }
        return dir
    }

    /// Fetch (or resume) every file of the bundle, each checked against its SHA-256, then make the copies. Called only after the person
    /// has agreed to the download (voice setup's consent); nothing else in the app calls it. `progress` receives bytes of the whole folder.
    public func ensureBundle(_ b: ModelBundleSpec, progress: (@Sendable (Int64, Int64) -> Void)? = nil) async throws -> URL {
        guard b.isWellFormed else { throw LabError.badInput("model \(b.id) names a path, a repository, a size or a digest that this phone will not fetch") }
        let session = try door.session(for: b.id, injected: injectedSession)        // the door: consent for this model's group, and a transport that cannot spend mobile data unless it was agreed
        let dir = root.appendingPathComponent(b.folder, isDirectory: true)
        let fm = FileManager.default
        try fm.createDirectory(at: dir, withIntermediateDirectories: true)
        try? fm.removeItem(at: dir.appendingPathComponent(Self.pinFile))          // from here until the copies are made the folder is not this model
        var before: Int64 = 0
        for f in b.files {
            let spec = ModelSpec(id: b.fileID(f), url: b.url(for: f), sha256: f.sha256, bytes: f.bytes, fileName: b.folder + "/" + f.path)
            specs[spec.id] = spec
            try fm.createDirectory(at: root.appendingPathComponent(spec.fileName).deletingLastPathComponent(), withIntermediateDirectories: true)
            let base = before, total = b.totalBytes
            _ = try await fetchFile(spec.id, session: session) { done, _ in progress?(base + done, total) }
            before += f.bytes
            progress?(before, b.totalBytes)
        }
        for c in b.copies where c.count == 2 {
            let from = dir.appendingPathComponent(c[0]), to = dir.appendingPathComponent(c[1])
            // Always replaced: after a pin bump the verified source is new and a copy kept from the old pin would be the old network under the new name.
            if fm.fileExists(atPath: to.path) { try fm.removeItem(at: to) }
            try fm.copyItem(at: from, to: to)
        }
        try Data(b.pinIdentity.utf8).write(to: dir.appendingPathComponent(Self.pinFile), options: .atomic)
        return dir
    }

    /// Whether any of a bundle's folder is on the phone (whole or part: a rejected configuration must remove either).
    public func bundleFolderExists(_ b: ModelBundleSpec) -> Bool {
        b.pathsStayInside && FileManager.default.fileExists(atPath: root.appendingPathComponent(b.folder, isDirectory: true).path)
    }
    /// Delete a bundle's folder (a rejected configuration, or the person removing the model).
    public func deleteBundle(_ b: ModelBundleSpec) {
        guard b.pathsStayInside else { return }
        try? FileManager.default.removeItem(at: root.appendingPathComponent(b.folder, isDirectory: true))
        loaded.remove(b.id)
    }
    public var registered: [ModelSpec] { Array(specs.values) }

    /// Fetch (or resume) and verify; returns the local file. `progress` receives bytes so far.
    public func ensure(_ id: String, progress: (@Sendable (Int64, Int64) -> Void)? = nil) async throws -> URL {
        guard specs[id] != nil else {
            if managed[id] != nil { throw LabError.badInput("model \(id) is fetched by its own library, not by the store") }
            throw LabError.badInput("model \(id) not registered")
        }
        let session = try door.session(for: id, injected: injectedSession)          // the door: consent for this model's group, and a transport that cannot spend mobile data unless it was agreed
        return try await fetchFile(id, session: session, progress: progress)
    }
    private func fetchFile(_ id: String, session: URLSession, progress: (@Sendable (Int64, Int64) -> Void)? = nil) async throws -> URL {
        guard let s = specs[id] else {
            if managed[id] != nil { throw LabError.badInput("model \(id) is fetched by its own library, not by the store") }
            throw LabError.badInput("model \(id) not registered")
        }
        guard ModelBundleSpec.isSafeRelativePath(s.fileName) else { throw LabError.badInput("model \(id) names a file outside the store") }
        let final = root.appendingPathComponent(s.fileName)
        if FileManager.default.fileExists(atPath: final.path), try sha(final) == s.sha256.lowercased(), size(final) == s.bytes { return final }
        let part = final.appendingPathExtension("part")
        var have = ((try? FileManager.default.attributesOfItem(atPath: part.path)[.size]) as? NSNumber)?.int64Value ?? 0
        // WAYS THIS CAN FAIL (round 3): the partial file is already the whole file (a Range request past the end is HTTP 416 on every
        // retry): check it and move it into place, or drop it and start over. The server may also answer 416 for a smaller partial.
        if have > 0, have >= s.bytes {
            if (try? sha(part)) == s.sha256.lowercased(), size(part) == s.bytes {
                try? FileManager.default.removeItem(at: final)
                try FileManager.default.moveItem(at: part, to: final)
                return final
            }
            try? FileManager.default.removeItem(at: part); have = 0
        }
        func request(from offset: Int64) async throws -> (URLSession.AsyncBytes, Int) {
            var req = URLRequest(url: s.url)
            if offset > 0 { req.setValue("bytes=\(offset)-", forHTTPHeaderField: "Range") }
            let (b, r) = try await session.bytes(for: req)
            return (b, (r as? HTTPURLResponse)?.statusCode ?? 0)
        }
        var (bytes, code) = try await request(from: have)
        if code == 416, have > 0 {                                       // the partial file is not usable: start from the first byte
            try? FileManager.default.removeItem(at: part); have = 0
            (bytes, code) = try await request(from: 0)
        }
        guard code == 200 || code == 206 else { throw LabError.http(code) }
        if code == 200 { try? FileManager.default.removeItem(at: part) }   // server ignored Range: restart
        if !FileManager.default.fileExists(atPath: part.path) { FileManager.default.createFile(atPath: part.path, contents: nil) }
        let h = try FileHandle(forWritingTo: part)
        try h.seekToEnd()
        var done = Int64(try h.offset())
        var buf = Data()
        for try await b in bytes {
            buf.append(b)
            if done + Int64(buf.count) > s.bytes { try? h.close(); try? FileManager.default.removeItem(at: part); throw LabError.badInput("\(id) is larger than the \(s.bytes) bytes its list says") }
            if buf.count >= 1 << 20 { try h.write(contentsOf: buf); done += Int64(buf.count); buf.removeAll(keepingCapacity: true); progress?(done, s.bytes) }
        }
        try h.write(contentsOf: buf); try h.close()
        guard try sha(part) == s.sha256.lowercased() else { try? FileManager.default.removeItem(at: part); throw LabError.badInput("sha256 mismatch for \(id)") }
        // The digest matched: the size the list gave must be the size of what was served too, or the folder would never look whole.
        guard size(part) == s.bytes else { try? FileManager.default.removeItem(at: part); throw LabError.badInput("size mismatch for \(id): the list says \(s.bytes) bytes") }
        try? FileManager.default.removeItem(at: final)
        try FileManager.default.moveItem(at: part, to: final)
        return final
    }

    func size(_ url: URL) -> Int64 { ((try? FileManager.default.attributesOfItem(atPath: url.path)[.size]) as? NSNumber)?.int64Value ?? -1 }

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

    public static func directoryBytes(_ url: URL) -> Int64 {
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
