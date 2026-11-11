import Foundation

/// The configurator's backend on a real device (or the Mac): the one shared store and the app module's provider. Nothing is bundled: every
/// weight comes through `ModelProviders.current`, and what a rejected configuration fetched is deleted from the store.
public final class LiveConfigBackend: ConfigBackend, @unchecked Sendable {
    let store: ModelStore
    let probeFn: @Sendable () -> DeviceProbe
    let rounds: Int
    public init(store: ModelStore = .shared, probe: @escaping @Sendable () -> DeviceProbe = { DeviceProbe.current() }, rounds: Int = ConfigThresholds.standard.sustainedRounds) {
        self.store = store; self.probeFn = probe; self.rounds = rounds
    }
    public func probe() -> DeviceProbe { probeFn() }
    public func isOnDisk(_ c: ModelCandidate) async -> Bool { KnownModels.isPinned(c.id) && (await store.localFileIfPresent(c.id)) != nil }
    func roleName(_ r: ConfigRole) -> String { r == .responder || r == .decision ? r.rawValue : "" }
    public func prepare(_ c: ModelCandidate, role: ConfigRole, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
        guard KnownModels.contains(c.id) else { throw LabError.badInput("the candidate \(c.id) is not in the model catalog") }
        guard let p = ModelProviders.current else { throw LabError.unavailable(ModelProviders.missingNote) }
        try await p.fetch(c.id, role: roleName(role), onBytes: onBytes)       // downloads (resuming, verified) and loads
    }
    public func unload(_ c: ModelCandidate, role: ConfigRole) async { await ModelProviders.current?.unload(c.id, role: roleName(role)) }
    public func delete(_ c: ModelCandidate) async { if KnownModels.isPinned(c.id) { await store.delete(c.id) } }
    public func measure(_ cfg: Configuration, picks: [ConfigRole: ModelCandidate]) async throws -> ConfigMeasurement {
        guard let p = ModelProviders.current else { throw LabError.unavailable(ModelProviders.missingNote) }
        return try await p.measureConfiguration(cfg.picks, rounds: rounds)
    }
}
