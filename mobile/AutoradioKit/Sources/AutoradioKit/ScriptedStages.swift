import Foundation

/// A transcriber that hears the same words every time. For the proof on a simulator (no speech authorization,
/// no microphone) and for measuring the stages after it without a recognizer's variance.
public final class ScriptedTranscriber: Transcriber, @unchecked Sendable {
    public static let stageID = "scripted"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Scripted (fixed words)", summary: "Returns the configured words at the end of every turn; hears nothing.",
                        params: [.text("text", "Words", "tell me more about the park"), .number("delay", "Delay before the words (s)", 0.1, 0, 5)])
    }
    let text: String, delay: Double
    public init(_ p: StageParams) { text = p.string("text", "tell me more about the park"); delay = p.double("delay", 0.1) }
    public func begin() async throws -> TranscriptionSession { Session(text, delay) }

    final class Session: TranscriptionSession, @unchecked Sendable {
        let text: String, delay: Double
        let partials = AsyncStream<String> { $0.finish() }
        init(_ t: String, _ d: Double) { text = t; delay = d }
        func append(_ frame: AudioFrame) {}
        func finish() async throws -> String {
            try? await Task.sleep(nanoseconds: UInt64(delay * 1e9))
            return text
        }
        func cancel() {}
    }
}
