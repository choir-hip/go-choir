import Foundation
import CryptoKit

/// What a model IS on this phone, so that "ready" is derived from files and not remembered (review rounds 4 and 5).
/// `pin` names the exact file set (a bundle's commit and every file's size and digest; a GGUF's SHA-256; for a library-managed model, the library version and the id). `present` says every file of it is on disk now, by exact length.
/// A library-managed model's files are sealed in a MANIFEST when a check passes (`seal`) and unsealed when it is demoted or fetched again (`unseal`).
public struct ModelPin: Sendable {
    public let pin: String
    public let present: @Sendable () -> Bool
    /// Library-managed models: write the manifest of the files as they are now (trust on first consented fetch). false: nothing to seal, so the model cannot be ready.
    public let seal: (@Sendable () -> Bool)?
    public let unseal: (@Sendable () -> Void)?
    /// The files on disk against the last manifest or, after a demotion, the BASELINE (the last good manifest): intact, changed, or unreadable (a file that cannot be read now: not proof of anything); nil when there is nothing to compare with.
    public let proof: (@Sendable () -> ModelManifests.Proof?)?
    /// Delete the model's files and its manifest (damaged, or made for another version of the library).
    public let purge: (@Sendable () -> Void)?
    public init(pin: String, present: @escaping @Sendable () -> Bool, seal: (@Sendable () -> Bool)? = nil, unseal: (@Sendable () -> Void)? = nil, proof: (@Sendable () -> ModelManifests.Proof?)? = nil, purge: (@Sendable () -> Void)? = nil) { self.pin = pin; self.present = present; self.seal = seal; self.unseal = unseal; self.proof = proof; self.purge = purge }
}

/// Every file of a library-managed model: its relative path, exact length and SHA-256, written when the check that grants readiness passed.
public struct ModelManifest: Codable, Sendable {
    public struct Entry: Codable, Sendable, Equatable { public var path: String; public var bytes: Int64; public var sha256: String }
    public var pin: String
    public var files: [Entry]
}

public enum ModelManifests {
    static func file(_ id: String, root: URL) -> URL { root.appendingPathComponent("manifests", isDirectory: true).appendingPathComponent(id.replacingOccurrences(of: "/", with: "_") + ".json") }
    public static func read(_ id: String, root: URL = KnownModels.storeRoot) -> ModelManifest? {
        guard let d = try? Data(contentsOf: file(id, root: root)) else { return nil }
        return try? JSONDecoder().decode(ModelManifest.self, from: d)
    }
    public static func delete(_ id: String, root: URL = KnownModels.storeRoot) { try? FileManager.default.removeItem(at: file(id, root: root)) }
    /// The last good manifest, kept when a fetch demotes the model: the baseline a re-download's files are compared with.
    static func baselineFile(_ id: String, root: URL) -> URL { file(id, root: root).deletingPathExtension().appendingPathExtension("baseline.json") }
    public static func baseline(_ id: String, root: URL = KnownModels.storeRoot) -> ModelManifest? {
        guard let d = try? Data(contentsOf: baselineFile(id, root: root)) else { return nil }
        return try? JSONDecoder().decode(ModelManifest.self, from: d)
    }
    /// A demotion keeps the manifest as the baseline (it is no longer a claim that the files are there).
    public static func demote(_ id: String, root: URL = KnownModels.storeRoot) {
        let fm = FileManager.default, f = file(id, root: root), b = baselineFile(id, root: root)
        guard fm.fileExists(atPath: f.path) else { return }
        try? fm.removeItem(at: b); try? fm.moveItem(at: f, to: b)
    }
    public enum Proof: Sendable, Equatable { case intact, changed, unreadable }
    /// Digests against a manifest. A missing or resized file is `changed`; a file that exists and cannot be read now is `unreadable` (not proof of damage).
    public static func check(_ m: ModelManifest, base: URL) -> Proof { check(m, base: base, read: { try $0.read(upToCount: 1 << 20) }) }
    static func check(_ m: ModelManifest, base: URL, read: (FileHandle) throws -> Data?) -> Proof {
        guard !m.files.isEmpty else { return .changed }
        let fm = FileManager.default; var unreadable = false
        for e in m.files {
            let u = base.appendingPathComponent(e.path)
            guard let size = (try? fm.attributesOfItem(atPath: u.path)[.size]) as? NSNumber, size.int64Value == e.bytes else { return .changed }
            guard let h = sha256(u, read: read) else { unreadable = true; continue }
            if h != e.sha256 { return .changed }
        }
        return unreadable ? .unreadable : .intact
    }
    /// Walk the model's folders (relative to `base`) and record every regular file's path, length and SHA-256. nil when there is no file, or when ANY entry cannot be read or is not a plain file or folder (a link, an unreadable file): a seal over a subset is not a seal.
    public static func build(pin: String, base: URL, folders: [String]) -> ModelManifest? {
        let fm = FileManager.default; var entries: [ModelManifest.Entry] = []
        for f in folders {
            let dir = base.appendingPathComponent(f, isDirectory: true)
            guard fm.fileExists(atPath: dir.path) else { continue }
            guard let e = fm.enumerator(atPath: dir.path) else { return nil }             // relative paths, never resolved: no symlink arithmetic
            for case let rel as String in e {
                let u = dir.appendingPathComponent(rel)
                guard let attrs = try? fm.attributesOfItem(atPath: u.path), let type = attrs[.type] as? FileAttributeType else { return nil }
                if type == .typeDirectory { continue }
                guard type == .typeRegular, let size = attrs[.size] as? NSNumber, let h = sha256(u) else { return nil }
                entries.append(.init(path: f + "/" + rel, bytes: size.int64Value, sha256: h))
            }
        }
        return entries.isEmpty ? nil : ModelManifest(pin: pin, files: entries.sorted { $0.path < $1.path })
    }
    public static func write(_ m: ModelManifest, id: String, root: URL = KnownModels.storeRoot) -> Bool {
        let f = file(id, root: root)
        try? FileManager.default.createDirectory(at: f.deletingLastPathComponent(), withIntermediateDirectories: true)
        guard let d = try? JSONEncoder().encode(m) else { return false }
        return (try? d.write(to: f, options: .atomic)) != nil
    }
    /// Every file present with its exact length: what every launch checks.
    public static func present(_ m: ModelManifest, base: URL) -> Bool {
        let fm = FileManager.default
        return !m.files.isEmpty && m.files.allSatisfy { ((try? fm.attributesOfItem(atPath: base.appendingPathComponent($0.path).path)[.size]) as? NSNumber)?.int64Value == $0.bytes }
    }
    /// The digests: at the check that grants readiness, and again after a failed load or a length mismatch; not on every launch.
    public static func verifyHashes(_ m: ModelManifest, base: URL) -> Bool { check(m, base: base) == .intact }
    /// The digest of the whole file, or nil when it cannot be read to the END (it cannot be opened, or a read failed part-way): a partial digest is never returned, so an I/O error can never look like a changed file.
    static func sha256(_ u: URL, read: (FileHandle) throws -> Data? = { try $0.read(upToCount: 1 << 20) }) -> String? {
        guard let h = try? FileHandle(forReadingFrom: u) else { return nil }
        defer { try? h.close() }
        var hasher = SHA256()
        while true {
            do { guard let chunk = try read(h), !chunk.isEmpty else { break }; hasher.update(data: chunk) }
            catch { return nil }
        }
        return hasher.finalize().map { String(format: "%02x", $0) }.joined()
}
}

public enum ModelPins {
    /// The library whose own downloader fetches the managed models; a version bump is a pin change (the files may be other files).
    public static let libraryPin = "fluidaudio-0.18.3"

    /// nil: nothing pins this id, so it can never be ready. Bundles and GGUFs: present is the exact length of every file (their SHA-256 was checked at the fetch, by the store); library-managed models: the manifest.
    public static func current(_ id: String, root: URL = KnownModels.storeRoot) -> ModelPin? {
        let fm = FileManager.default
        if let b = KnownModels.bundles.first(where: { $0.id == id }) {
            let dir = root.appendingPathComponent(b.folder, isDirectory: true)
            return ModelPin(pin: "bundle:" + b.pinIdentity) {
                guard (try? String(contentsOf: dir.appendingPathComponent(ModelStore.pinFile), encoding: .utf8)) == b.pinIdentity else { return false }
                return b.files.allSatisfy { f in ((try? fm.attributesOfItem(atPath: dir.appendingPathComponent(f.path).path)[.size]) as? NSNumber)?.int64Value == f.bytes }
            }
        }
        if let s = KnownModels.pinned.first(where: { $0.id == id }) {
            let file = root.appendingPathComponent(s.fileName)
            return ModelPin(pin: "sha256:" + s.sha256.lowercased()) { ((try? fm.attributesOfItem(atPath: file.path)[.size]) as? NSNumber)?.int64Value == s.bytes }
        }
        if let m = KnownModels.managed.first(where: { $0.id == id }) { return managed(id, spec: m, root: root) }
        return nil
    }

    /// A library-managed model: no digest is published for its files, so they are pinned by the manifest of what the first consented fetch left (trust on first fetch). `root` holds the manifests; the library's own folders are under the spec's `cachePath`.
    public static func managed(_ id: String, spec m: ManagedModelSpec, root: URL, libraryPin: String = ModelPins.libraryPin) -> ModelPin? {
        guard let base = m.cachePath, let folders = m.cacheFolders, !folders.isEmpty else { return nil }
        let pin = "\(libraryPin)|\(id)"
        return ModelPin(pin: pin,
            present: { guard let man = ModelManifests.read(id, root: root), man.pin == pin else { return false }; return ModelManifests.present(man, base: base) },
            seal: { guard let man = ModelManifests.build(pin: pin, base: base, folders: folders) else { return false }; return ModelManifests.write(man, id: id, root: root) },
            unseal: { ModelManifests.demote(id, root: root) },
            proof: { guard let man = ModelManifests.read(id, root: root) ?? ModelManifests.baseline(id, root: root) else { return nil }; return ModelManifests.check(man, base: base) },      // digests of the files against what was sealed, whichever version sealed it
            purge: { for f in folders { try? FileManager.default.removeItem(at: base.appendingPathComponent(f, isDirectory: true)) }; ModelManifests.delete(id, root: root) })
    }
}

/// Where a library may be initialised. The libraries can fetch whenever they are initialised for files that are not there, so a stage initialises one only for a model that is READY by manifest, or while that model is being fetched or checked (the scope the provider sets).
public enum LibraryScope {
    @TaskLocal public static var checking: String?
    public static func require(_ id: String, readiness: VoiceReadiness = Voices.readiness) throws {
        if checking == id || readiness.isReady(model: id) { return }
        throw LabError.unavailable("\(id) is not on this phone (not ready): starting it would be a download, and downloads are the door's")
    }
}

/// THE deletion rule. Files are deleted only when (a) a digest read to the end proves them different from what was sealed (the manifest, or the baseline a fetch demoted it to), or (b) nothing has ever sealed them and the fetch that wrote them did not finish (a half pack: a library
/// that checks only presence would call it whole). Nothing else deletes: not an error, a timeout, a refusal, a cancelled wait, a failed load, an app update, a read error. Every caller goes through this one function.
public enum ModelDeletion {
    /// What the fetch that wrote the files did: it never began (refused, queued out, cancelled before it started: it wrote nothing), it began and did not finish, or it finished.
    public enum Fetch: Sendable { case notStarted, unfinished, finished }
    /// Apply the rule to this model; true when its files were deleted. The loaded manager goes first.
    @discardableResult
    public static func reconcile(model id: String, pin: ModelPin?, fetch: Fetch) async -> Bool {
        guard let pin, let purge = pin.purge else { return false }
        let proof = pin.proof?()
        let differs = proof == .changed
        let halfPack = proof == nil && fetch == .unfinished
        guard differs || halfPack else { return false }
        await ModelStore.shared.unload(id)
        purge()
        return true
    }
}

/// Every fetch of a library model: DEMOTE first (the manifest becomes the baseline), run the body, and let `ModelDeletion` decide what a failure leaves. The body calls `run.begin()` when it starts to write.
public enum ModelFetch {
    public final class Run: @unchecked Sendable {
        private let l = NSLock(); private var started = false
        public init() {}
        public func begin() { l.lock(); started = true; l.unlock() }
        var state: ModelDeletion.Fetch { l.lock(); defer { l.unlock() }; return started ? .unfinished : .notStarted }
    }
    public static func guarded<T>(model id: String, verifier: ReadinessVerifier = Voices.verifier, pins: (String) -> ModelPin? = { ModelPins.current($0) }, _ body: (Run) async throws -> T) async throws -> T {
        verifier.demote(model: id)
        let run = Run()
        do { return try await body(run) }
        catch {
            await ModelDeletion.reconcile(model: id, pin: pins(id), fetch: run.state)
            throw error
        }
    }
}
