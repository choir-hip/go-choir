import Foundation

/// The one gate every side effect of a Test passes through (docs/problems/model-test-cancellation-2026-11-24.md). A Test is an obligation owned by the account generation it began in: when that account signs out, the microphone,
/// a model, the speaker and the page's line are no longer its to touch. `live` is read AT THE MOMENT OF the effect (the permission prompt answered, the engine opened, the model loaded, the first buffer scheduled), never only
/// at the awaits before it: a task's own cancellation is seen only where the code looks, and a sign-out cancels nothing in the audio engine or a library.
public struct TrialGate: Sendable {
    public let generation: Int
    public init(generation: Int = AccountWipe.generation) { self.generation = generation }
    /// Still the current account's, and the task still wanted.
    public var live: Bool { generation == AccountWipe.generation && !Task.isCancelled }
}
