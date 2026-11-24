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
    public init(pin: String, present: @escaping @Sendable () -> Bool, seal: (@Sendable () -> Bool)? = nil, unseal: (@Sendable () -> Void)? = nil) { self.pin = pin; self.present = present; self.seal = seal; self.unseal = unseal }
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
    /// Walk the model's folders (relative to `base`) and record every regular file's path, length and SHA-256. nil when there is no file.
    public static func build(pin: String, base: URL, folders: [String]) -> ModelManifest? {
        let fm = FileManager.default; var entries: [ModelManifest.Entry] = []
        let basePath = base.resolvingSymlinksInPath().path
        for f in folders {
            let dir = base.appendingPathComponent(f, isDirectory: true)
            guard let e = fm.enumerator(at: dir, includingPropertiesForKeys: [.isRegularFileKey, .fileSizeKey]) else { continue }
            for case let u as URL in e {
                guard (try? u.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile) == true, let size = (try? fm.attributesOfItem(atPath: u.path)[.size]) as? NSNumber, let h = sha256(u) else { continue }
                entries.append(.init(path: String(u.resolvingSymlinksInPath().path.dropFirst(basePath.count + 1)), bytes: size.int64Value, sha256: h))
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
        return m.files.allSatisfy { ((try? fm.attributesOfItem(atPath: base.appendingPathComponent($0.path).path)[.size]) as? NSNumber)?.int64Value == $0.bytes }
    }
    /// The digests: at the check that grants readiness, and again after a failed load or a length mismatch; not on every launch.
    public static func verifyHashes(_ m: ModelManifest, base: URL) -> Bool { m.files.allSatisfy { sha256(base.appendingPathComponent($0.path)) == $0.sha256 } }
    static func sha256(_ u: URL) -> String? {
        guard let h = try? FileHandle(forReadingFrom: u) else { return nil }
        defer { try? h.close() }
        var hasher = SHA256()
        while let chunk = try? h.read(upToCount: 1 << 20), !chunk.isEmpty { hasher.update(data: chunk) }
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
            unseal: { ModelManifests.delete(id, root: root) })
    }
}

/// Where a library may be initialised. The libraries can fetch whenever they are initialised for files that are not there, so a stage initialises one only for a model that is READY by manifest, or while that model is being fetched or checked (the scope the provider sets).
public enum LibraryScope {
    @TaskLocal public static var checking: String?
    public static func require(_ id: String, readiness: VoiceReadiness = Voices.readiness) throws {
        if checking == id || readiness.isReady(model: id) { return }
        throw LabError.unavailable("\(id) is not on this phone (not ready): loading it would be a download, and downloads are the door's")
    }
}
