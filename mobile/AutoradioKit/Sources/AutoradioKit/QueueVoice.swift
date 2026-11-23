import Foundation

// What the phone hands the desk about the play queue. There is no phrase matching here (owner, 2026-11-17: "we shouldn't have specific commands that work
// differently than others"): every turn's words go to the desk with its context, and the desk acts through the run sheet's ops (`queue_ops`, applied by
// shared code's run sheet, AutoradioLab/voice-queue.ts). The host sets these hooks once; an unset closure means "no queue here".

public enum QueueVoice {
    public struct Hooks: Sendable {
        /// `PlayQueue.shared.snapshot()` as the wire's `queue` object, JSON bytes (nil: no queue).
        public var snapshot: (@Sendable () async -> Data?)?
        /// The paper's current list, route and title, for the wire's `candidates` (up to 40).
        public var candidates: (@Sendable () -> [(route: String, title: String)])?
        /// The `queue_ops` event's JSON, as the server sent it, for `PlayQueue.shared.apply(serverOpsJSON:)`.
        public var applyServerOps: (@Sendable (Data) async -> Void)?
        public init() {}
    }
    private static let box = Locked<Hooks>(Hooks())
    public static var hooks: Hooks { get { box.value } set { box.value = newValue } }
}
