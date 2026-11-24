import Foundation

/// What a model IS on this phone, so that "ready" can be derived from it instead of remembered (review round 4: readiness as an id set was itself the bypass: after an app update with the record intact, a library was initialised for files that were not there).
/// `pin` names the exact file set (a bundle's commit and every file's size and digest; a GGUF's SHA-256; for a library-managed model, the library version, its id and size); `present` says every file of it is on disk now.
public struct ModelPin: Sendable {
    public let pin: String
    public let present: @Sendable () -> Bool
    public init(pin: String, present: @escaping @Sendable () -> Bool) { self.pin = pin; self.present = present }
}

public enum ModelPins {
    /// The library whose own downloader fetches the managed models; a version bump is a pin change (the files may be other files).
    public static let libraryPin = "fluidaudio-0.18.3"

    /// nil: nothing pins this id, so it can never be ready.
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
        if let m = KnownModels.managed.first(where: { $0.id == id }), let base = m.cachePath, let folders = m.cacheFolders, !folders.isEmpty {
            return ModelPin(pin: "\(libraryPin)|\(id)|\(m.approxBytes)") {
                var total: Int64 = 0
                for f in folders { total += ModelStore.directoryBytes(base.appendingPathComponent(f, isDirectory: true)) }
                return total * 10 >= m.approxBytes * 9          // within a tenth of the size the model card gives: the library's own folders hold it
            }
        }
        return nil
    }
}
