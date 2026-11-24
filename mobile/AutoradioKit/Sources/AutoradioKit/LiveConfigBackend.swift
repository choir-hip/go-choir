import Foundation

/// The configurator's backend on a real device (or the Mac): the one shared store and the app module's provider. Nothing is bundled: every
/// weight comes through `ModelProviders.current`, and what a rejected configuration fetched is deleted from the store.
public final class LiveConfigBackend: ConfigBackend, @unchecked Sendable {
    let store: ModelStore
    let probeFn: @Sendable () -> DeviceProbe
    let rounds: Int
    /// Where the library-managed (FluidAudio) bundles live; the catalog's own folder unless a test gives another.
    let managedRoot: URL
    public init(store: ModelStore = .shared, probe: @escaping @Sendable () -> DeviceProbe = { DeviceProbe.current() }, rounds: Int = ConfigThresholds.standard.sustainedRounds,
                managedRoot: URL = KnownModels.fluidRoot) {
        self.store = store; self.probeFn = probe; self.rounds = rounds; self.managedRoot = managedRoot
    }
    /// The Core ML folders a managed (non-pinned) candidate owns on the phone.
    func managedFolders(_ c: ModelCandidate) -> [URL] {
        guard !KnownModels.isPinned(c.id), let spec = KnownModels.managed.first(where: { $0.id == c.id }), let root = spec.cachePath, root == KnownModels.fluidRoot || root == KnownModels.ttsRoot else { return [] }
        return (spec.cacheFolders ?? []).filter { !$0.isEmpty && !$0.contains("..") && !$0.hasPrefix("/") }.map { (root == KnownModels.ttsRoot ? root : managedRoot).appendingPathComponent($0, isDirectory: true) }
    }
    /// On the phone: a folder of this model holds something of ITS own (Pocket's folder holding only Phonon's pack is not Pocket on the phone).
    func managedOnDisk(_ c: ModelCandidate) -> Bool {
        guard let spec = KnownModels.managed.first(where: { $0.id == c.id }), let root = spec.cachePath else { return false }
        let base = root == KnownModels.ttsRoot ? root : managedRoot
        let excl = spec.excludedFolders ?? []
        return (spec.cacheFolders ?? []).contains { excl.isEmpty ? FileManager.default.fileExists(atPath: base.appendingPathComponent($0, isDirectory: true).path) : ModelFolders.holds($0, base: base, excluding: excl) }
    }
    public func probe() -> DeviceProbe { probeFn() }
    public func isOnDisk(_ c: ModelCandidate) async -> Bool {
        if let b = KnownModels.bundles.first(where: { $0.id == c.id }) { return await store.bundleFolderExists(b) }      // a folder of files in the store's own root
        guard KnownModels.isPinned(c.id) else { return managedOnDisk(c) }
        let f = await store.localFileIfPresent(c.id)
        return f != nil
    }
    func roleName(_ r: ConfigRole) -> String { r == .responder || r == .decision ? r.rawValue : "" }
    public func prepare(_ c: ModelCandidate, role: ConfigRole, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
        guard KnownModels.contains(c.id) else { throw LabError.badInput("the candidate \(c.id) is not in the model catalog") }
        guard let p = ModelProviders.current else { throw LabError.unavailable(ModelProviders.missingNote) }
        try await p.fetch(c.id, role: roleName(role), onBytes: onBytes)       // downloads (resuming, verified) and loads
    }
    public func unload(_ c: ModelCandidate, role: ConfigRole) async { await ModelProviders.current?.unload(c.id, role: roleName(role)) }
    public func delete(_ c: ModelCandidate) async {
        Voices.verifier.demote(model: c.id)          // its files are going: the radio must not prefer a voice that is not there
        if let b = KnownModels.bundles.first(where: { $0.id == c.id }) { await store.deleteBundle(b) }
        else if KnownModels.isPinned(c.id) { await store.delete(c.id) }
        else if let spec = KnownModels.managed.first(where: { $0.id == c.id }), let root = spec.cachePath {
            let base = root == KnownModels.ttsRoot ? root : managedRoot
            for rel in (spec.cacheFolders ?? []) where managedFolders(c).contains(base.appendingPathComponent(rel, isDirectory: true)) { ModelFolders.remove(rel, base: base, excluding: spec.excludedFolders ?? []) }
        }       // a rejected Core ML model's bundle (failure 6)
    }
    public func measure(_ cfg: Configuration, picks: [ConfigRole: ModelCandidate]) async throws -> ConfigMeasurement {
        guard let p = ModelProviders.current else { throw LabError.unavailable(ModelProviders.missingNote) }
        return try await p.measureConfiguration(cfg.picks, rounds: rounds)
    }
}
