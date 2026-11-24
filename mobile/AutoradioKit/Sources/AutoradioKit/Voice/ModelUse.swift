import Foundation

/// The model the person chose to USE for a job on Settings > Voice and models, kept on the phone (owner, 2026-11-24). Only the jobs whose model the radio otherwise picks by rank are kept here:
/// "bounds" (knowing when you speak), "live" (words as you speak) and "final" (the transcript). The answering and quick-decision models have their own saved choice (`ResponderChoice`, `DecisionChoice`)
/// and the voice is `VoiceStore`'s. A choice names a model; whether it is on the phone is the readiness record's word, so a choice for a model that was removed changes nothing (`ListenerPipeline.choose`).
public struct ModelUse: Sendable {
    public static let jobs: Set<String> = ["bounds", "live", "final"]
    static let key = "AutoradioModelUse"
    public static let standard = ModelUse(defaults: .standard)
    nonisolated(unsafe) private let defaults: UserDefaults
    public init(defaults: UserDefaults) { self.defaults = defaults }
    public var all: [String: String] {
        guard let d = defaults.dictionary(forKey: Self.key) as? [String: String] else { return [:] }
        return d.filter { Self.jobs.contains($0.key) && !$0.value.isEmpty }
    }
    /// An unknown job is not kept; an empty id clears the job (the list's rank decides again).
    public func set(_ job: String, _ id: String) {
        guard Self.jobs.contains(job) else { return }
        var d = all
        if id.isEmpty { d[job] = nil } else { d[job] = id }
        defaults.set(d, forKey: Self.key)
    }
}
