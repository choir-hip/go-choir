import Foundation

/// The microphone and the engine's own output, compared: the listener speaking over the radio, heard without echo cancellation.
public final class SpeechOverRadioMonitor: @unchecked Sendable {
    private let lock = NSLock()
    private var detector = ReferenceSpeechDetector()
    public init() {}
    public func reference(_ f: AudioFrame) { lock.lock(); detector.feedReference(f.samples); lock.unlock() }
    public func mic(_ f: AudioFrame) -> ReferenceSpeechDetector.Event? { lock.lock(); defer { lock.unlock() }; return detector.feedMic(f.samples) }
}

extension TurnEngine {
    /// The detector heard the listener over the radio (no voice processing is on): the programme is held at once, and the session goes to
    /// voice processing while nothing is playing. If no turn comes of it, everything is given back.
    public func speechOverRadio(_ e: ReferenceSpeechDetector.Event) async {
        guard hf.enabled, turn == nil, hf.session == nil, level == .play else { return }
        print("AUTORADIO speech detected ratio=\(String(format: "%.1f", e.ratio)) ms=\(e.ms)")
        await setLevelTo(.pause, reason: .pause)
        do { try await env.session.claim(.talking, preferBuiltInMic: false, reason: "speech over the radio") } catch {}
        Task { [weak self] in
            try? await Task.sleep(nanoseconds: 3_000_000_000)
            await self?.speechOverRadioSettled()
        }
    }
    func speechOverRadioSettled() async {
        guard turn == nil, hf.session == nil, pipeline == nil, phase == .playing else { return }       // a turn is going: it owns the level and the session
        print("AUTORADIO speech over the radio was not a turn: playing on")
        await setLevelTo(.play, reason: .resume)
        do { try await env.session.claim(.listening, preferBuiltInMic: false, reason: "not a turn") } catch {}
    }
}
