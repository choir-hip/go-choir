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
    /// Files and records of ANOTHER pin (another version of the library) exist: remove them (the folders and those records) so this version's fetch starts clean. true when something was removed.
    public let retire: (@Sendable () -> Bool)?
    /// The person's Remove: every file of the model and every record of it, of every pin.
    public let erase: (@Sendable () -> Void)?
    /// Any file of the model is on the phone (whole or not): what Remove is offered for.
    public let onDisk: (@Sendable () -> Bool)?
    /// A fetch has begun and not returned: written when it begins, cleared when its body returns by any road. One that is found at the next begin means the process died mid-download: the set is partial and no library will complete it.
    public let pendingMark: (@Sendable () -> Void)?
    public let pendingClear: (@Sendable () -> Void)?
    public let hasPending: (@Sendable () -> Bool)?
    public init(pin: String, present: @escaping @Sendable () -> Bool, seal: (@Sendable () -> Bool)? = nil, unseal: (@Sendable () -> Void)? = nil, proof: (@Sendable () -> ModelManifests.Proof?)? = nil, purge: (@Sendable () -> Void)? = nil, retire: (@Sendable () -> Bool)? = nil, erase: (@Sendable () -> Void)? = nil, onDisk: (@Sendable () -> Bool)? = nil, pendingMark: (@Sendable () -> Void)? = nil, pendingClear: (@Sendable () -> Void)? = nil, hasPending: (@Sendable () -> Bool)? = nil) { self.pin = pin; self.present = present; self.seal = seal; self.unseal = unseal; self.proof = proof; self.purge = purge; self.retire = retire; self.erase = erase; self.onDisk = onDisk; self.pendingMark = pendingMark; self.pendingClear = pendingClear; self.hasPending = hasPending }
}

/// Every file of a library-managed model: its relative path, exact length and SHA-256, written when the check that grants readiness passed.
public struct ModelManifest: Codable, Sendable {
    public struct Entry: Codable, Sendable, Equatable { public var path: String; public var bytes: Int64; public var sha256: String }
    public var pin: String
    public var files: [Entry]
}

public enum ModelManifests {
    /// Records belong to ONE pin (one version of the library): the file name carries it, and nothing compares a file set with a record of another pin.
    static func tag(_ pin: String) -> String { String(pin.map { $0.isLetter || $0.isNumber || $0 == "." || $0 == "-" ? $0 : "_" }) }
    static func dir(_ root: URL) -> URL { root.appendingPathComponent("manifests", isDirectory: true) }
    static func file(_ id: String, pin: String, root: URL) -> URL { dir(root).appendingPathComponent(id.replacingOccurrences(of: "/", with: "_") + "@" + tag(pin) + ".json") }
    static func baselineFile(_ id: String, pin: String, root: URL) -> URL { dir(root).appendingPathComponent(id.replacingOccurrences(of: "/", with: "_") + "@" + tag(pin) + ".baseline.json") }
    public static func read(_ id: String, pin: String, root: URL = KnownModels.storeRoot) -> ModelManifest? {
        guard let d = try? Data(contentsOf: file(id, pin: pin, root: root)), let m = try? JSONDecoder().decode(ModelManifest.self, from: d), m.pin == pin else { return nil }
        return m
    }
    static func pendingFile(_ id: String, pin: String, root: URL) -> URL { dir(root).appendingPathComponent(id.replacingOccurrences(of: "/", with: "_") + "@" + tag(pin) + ".pending") }
    public static func markPending(_ id: String, pin: String, root: URL) { let f = pendingFile(id, pin: pin, root: root); try? FileManager.default.createDirectory(at: f.deletingLastPathComponent(), withIntermediateDirectories: true); try? Data("pending".utf8).write(to: f, options: .atomic) }
    public static func clearPending(_ id: String, pin: String, root: URL) { try? FileManager.default.removeItem(at: pendingFile(id, pin: pin, root: root)) }
    public static func hasPending(_ id: String, pin: String, root: URL) -> Bool { FileManager.default.fileExists(atPath: pendingFile(id, pin: pin, root: root).path) }
    public static func delete(_ id: String, pin: String, root: URL = KnownModels.storeRoot) { try? FileManager.default.removeItem(at: file(id, pin: pin, root: root)) }
    /// The last good manifest OF THIS PIN, kept when a fetch demotes the model: what a re-download's files are compared with.
    public static func baseline(_ id: String, pin: String, root: URL = KnownModels.storeRoot) -> ModelManifest? {
        guard let d = try? Data(contentsOf: baselineFile(id, pin: pin, root: root)), let m = try? JSONDecoder().decode(ModelManifest.self, from: d), m.pin == pin else { return nil }
        return m
    }
    public static func demote(_ id: String, pin: String, root: URL = KnownModels.storeRoot) {
        let fm = FileManager.default, f = file(id, pin: pin, root: root), b = baselineFile(id, pin: pin, root: root)
        guard fm.fileExists(atPath: f.path) else { return }
        try? fm.removeItem(at: b); try? fm.moveItem(at: f, to: b)
    }
    /// Every record (manifest or baseline) kept for this model, with the pin it describes (read from the record, so an older build's unnamed files are found too).
    public static func records(_ id: String, root: URL = KnownModels.storeRoot) -> [(url: URL, pin: String)] {
        let fm = FileManager.default, key = id.replacingOccurrences(of: "/", with: "_")
        let names = (try? fm.contentsOfDirectory(atPath: dir(root).path)) ?? []
        return names.filter { $0.hasPrefix(key + "@") || $0.hasPrefix(key + ".") }.compactMap { n in
            let u = dir(root).appendingPathComponent(n)
            guard let d = try? Data(contentsOf: u), let m = try? JSONDecoder().decode(ModelManifest.self, from: d) else { return nil }
            return (u, m.pin)
        }
    }
    public static func write(_ m: ModelManifest, id: String, root: URL = KnownModels.storeRoot) -> Bool {
        let f = file(id, pin: m.pin, root: root)
        try? FileManager.default.createDirectory(at: f.deletingLastPathComponent(), withIntermediateDirectories: true)
        guard let d = try? JSONEncoder().encode(m) else { return false }
        return (try? d.write(to: f, options: .atomic)) != nil
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
            return ModelPin(pin: "bundle:" + b.pinIdentity, present: {
                guard (try? String(contentsOf: dir.appendingPathComponent(ModelStore.pinFile), encoding: .utf8)) == b.pinIdentity else { return false }
                return b.files.allSatisfy { f in ((try? fm.attributesOfItem(atPath: dir.appendingPathComponent(f.path).path)[.size]) as? NSNumber)?.int64Value == f.bytes }
            }, erase: { try? FileManager.default.removeItem(at: dir) }, onDisk: { ModelStore.directoryBytes(dir) > 0 })
        }
        if let s = KnownModels.pinned.first(where: { $0.id == id }) {
            let file = root.appendingPathComponent(s.fileName)
            return ModelPin(pin: "sha256:" + s.sha256.lowercased(), present: { ((try? fm.attributesOfItem(atPath: file.path)[.size]) as? NSNumber)?.int64Value == s.bytes },
                            erase: { try? fm.removeItem(at: file); try? fm.removeItem(at: file.appendingPathExtension("part")) },
                            onDisk: { fm.fileExists(atPath: file.path) || fm.fileExists(atPath: file.appendingPathExtension("part").path) })
        }
        if let m = KnownModels.managed.first(where: { $0.id == id }) { return managed(id, spec: m, root: root) }
        return nil
    }

    /// A library-managed model: no digest is published for its files, so they are pinned by the manifest of what the first consented fetch left (trust on first fetch). `root` holds the manifests; the library's own folders are under the spec's `cachePath`.
    public static func managed(_ id: String, spec m: ManagedModelSpec, root: URL, libraryPin: String = ModelPins.libraryPin) -> ModelPin? {
        guard let base = m.cachePath, let folders = m.cacheFolders, !folders.isEmpty else { return nil }
        let pin = "\(libraryPin)|\(id)"
        return ModelPin(pin: pin,
            present: { guard let man = ModelManifests.read(id, pin: pin, root: root) else { return false }; return ModelManifests.present(man, base: base) },
            seal: { guard let man = ModelManifests.build(pin: pin, base: base, folders: folders) else { return false }; return ModelManifests.write(man, id: id, root: root) },
            unseal: { ModelManifests.demote(id, pin: pin, root: root) },
            proof: { guard let man = ModelManifests.read(id, pin: pin, root: root) ?? ModelManifests.baseline(id, pin: pin, root: root) else { return nil }; return ModelManifests.check(man, base: base) },          // this pin's own seal only: another version's is unknown, never damage
            purge: { for f in folders { try? FileManager.default.removeItem(at: base.appendingPathComponent(f, isDirectory: true)) }; ModelManifests.delete(id, pin: pin, root: root) },
            retire: {
                let old = ModelManifests.records(id, root: root).filter { $0.pin != pin }
                guard !old.isEmpty else { return false }
                for f in folders { try? FileManager.default.removeItem(at: base.appendingPathComponent(f, isDirectory: true)) }          // another version's files: this version's fetch starts clean
                for o in old { try? FileManager.default.removeItem(at: o.url) }
                return true
            },
            erase: { for f in folders { try? FileManager.default.removeItem(at: base.appendingPathComponent(f, isDirectory: true)) }; for r in ModelManifests.records(id, root: root) { try? FileManager.default.removeItem(at: r.url) }; ModelManifests.clearPending(id, pin: pin, root: root) },
            onDisk: { folders.contains { ModelStore.directoryBytes(base.appendingPathComponent($0, isDirectory: true)) > 0 } },
            pendingMark: { ModelManifests.markPending(id, pin: pin, root: root) },
            pendingClear: { ModelManifests.clearPending(id, pin: pin, root: root) },
            hasPending: { ModelManifests.hasPending(id, pin: pin, root: root) })
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
        let differs = proof == .changed && fetch != .notStarted                 // a fetch that never began deletes nothing, whatever the files look like
        let halfPack = proof == nil && fetch == .unfinished
        guard differs || halfPack else { return false }
        await ModelStore.shared.unload(id)
        purge()
        return true
    }
}

/// Every fetch of a library model runs through here. Inside the door, once admitted, the body calls `run.begin()`: that DEMOTES the model (a refused or queued-out fetch demotes nothing) and, when the files on the phone came from another version of the library, removes them so this
/// version starts clean. A failure then leaves what `ModelDeletion` decides. "Did not finish" is evidence: the door killed the window (`run.interrupt()`), or the error is a transport one; a library error from the load after the download finished is not.
public enum ModelFetch {
    public final class Run: @unchecked Sendable {
        private let l = NSLock(); private var started = false, interrupted = false
        private let model: String, verifier: ReadinessVerifier, pins: (String) -> ModelPin?
        init(model: String, verifier: ReadinessVerifier, pins: @escaping (String) -> ModelPin?) { self.model = model; self.verifier = verifier; self.pins = pins }
        public func begin() {
            l.lock(); let first = !started; started = true; l.unlock()
            guard first else { return }
            verifier.demote(model: model)
            if let p = pins(model) {
                _ = p.retire?()
                if p.hasPending?() == true { p.purge?() }                       // the last fetch died mid-download (the process was killed): the set is partial and a library that checks presence would skip it
                p.pendingMark?()
            }
        }
        /// The door killed the window: the registry was closed under the transfer.
        public func interrupt() { l.lock(); interrupted = true; l.unlock() }
        func state(for error: Error) -> ModelDeletion.Fetch {
            l.lock(); defer { l.unlock() }
            if !started { return .notStarted }
            return interrupted || isTransport(error) ? .unfinished : .finished
        }
        private func isTransport(_ e: Error) -> Bool {
            if e is CancellationError || e is URLError { return true }
            if let r = e as? DownloadDoor.Refusal, r == .timedOut { return true }
            return (e as NSError).domain == NSURLErrorDomain
        }
    }
    public static func guarded<T>(model id: String, verifier: ReadinessVerifier = Voices.verifier, pins: @escaping (String) -> ModelPin? = { ModelPins.current($0) }, _ body: (Run) async throws -> T) async throws -> T {
        let run = Run(model: id, verifier: verifier, pins: pins)
        do { let r = try await body(run); pins(id)?.pendingClear?(); return r }
        catch {
            await ModelDeletion.reconcile(model: id, pin: pins(id), fetch: run.state(for: error))
            pins(id)?.pendingClear?()                                          // the body returned (it did not die): whatever it left is judged by the rule, not by a record
            throw error
        }
    }
}

extension ModelDeletion {
    public enum Removal: Sendable, Equatable { case removed, inUse, notRemovable, busy }
    /// The person's Remove on a model whose files are on the phone: everything of it goes (files, manifest, baseline, pending, readiness), except the model in use. It runs under the door's slot (no fetch or check is mid-write) and reads what is in use, and whether a setup is running, AGAIN inside it.
    public static func remove(model id: String, pin: ModelPin?, inUse: @escaping @Sendable () -> Set<String>, busy: @escaping @Sendable () -> Bool = { false }, verifier: ReadinessVerifier = Voices.verifier) async -> Removal {
        if inUse().contains(id) { return .inUse }
        guard let erase = pin?.erase else { return .notRemovable }
        let done: Removal? = try? await DownloadDoor.shared.exclusive(model: id) {
            if inUse().contains(id) { return Removal.inUse }
            if busy() { return Removal.busy }
            await ModelStore.shared.unload(id)
            verifier.demote(model: id)
            erase()
            return Removal.removed
        }
        return done ?? .busy
    }
}

/// What the radio is using right now, DERIVED: the models the registry's descriptors of the configured stages need (the voice whatever its saved parameter, the decision model, the recogniser, the speech detector), the models of a composite transcriber's two halves, and the answering model.
public enum ModelInUse {
    public static func ids(config: LabConfiguration, registry: StageRegistry, responder: String?) -> Set<String> {
        var out = ModelStore.needed(by: config, registry: registry)
        if let st = config.setting(.transcriber), st.id == ListenerPipeline.liveThenFinalStageID {
            for key in ["live", "final"] {
                if case .string(let sid)? = st.params[key], let d = registry.stages(.transcriber).first(where: { $0.id == sid })?.descriptor { out.formUnion(d.needsModels) }
            }
        }
        if let r = responder, !r.isEmpty { out.insert(r) }
        return out
    }
}
