import Foundation

/// The one gate every side effect of a Test passes through (docs/problems/model-test-cancellation-2026-11-24.md). A Test is an obligation owned by the account generation it began in: when that account signs out, the microphone,
/// a model, the speaker and the page's line are no longer its to touch. `live` is read AT THE MOMENT OF the effect (the permission prompt answered, the engine opened, the model loaded, the first buffer scheduled), never only
/// at the awaits before it: a task's own cancellation is seen only where the code looks, and a sign-out cancels nothing in the audio engine or a library.
public struct TrialGate: Sendable {
    /// nil: a path with no Test owner (`alwaysLive`).
    public let generation: Int?
    public let why: String
    public init(generation: Int = AccountWipe.generation) { self.generation = generation; self.why = "" }
    private init(unowned why: String) { generation = nil; self.why = why }
    /// A path that is not an account's Test (the radio speaking its own line): written out at the call site WITH ITS REASON, so a gate is never merely absent. There is no nil gate anywhere: every effect entry takes one.
    public static func alwaysLive(because reason: String) -> TrialGate { precondition(!reason.isEmpty, "an always-live gate says why"); return TrialGate(unowned: reason) }
    /// Still the current account's, and the task still wanted (an always-live gate: always).
    public var live: Bool { guard let g = generation else { return true }; return g == AccountWipe.generation && !Task.isCancelled }
}
