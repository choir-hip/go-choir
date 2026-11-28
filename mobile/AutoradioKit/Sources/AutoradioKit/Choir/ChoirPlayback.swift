import Foundation

/// The deck's engine in the app: spoken entries go through the existing programme player (ChoirApp), remote audio through
/// AVPlayer (RemoteAudioPlayer). Only one of them makes sound at a time: starting one silences the other.
@MainActor public final class ChoirPlaybackEngine: PlaybackEngine {
    let app: ChoirApp
    var continuity: PlaybackContinuity?
    public let remote = RemoteAudioPlayer()

    public init(app: ChoirApp) { self.app = app }

    public func speak(documentID: String, entryID: String, title: String, kind: String, paragraphs: [String], source: String) async -> Bool {
        remote.stop()
        return await app.speakSegment(documentID: documentID, entryID: entryID, title: title, kind: kind, paragraphs: paragraphs, source: source)
    }
    public func spokenPlace() -> DeckPlace { app.runtime.player.spokenPlace() }

    /// The server block being spoken, the last one, and the seconds into the entry (the words before this block at the speaking rate, plus the
    /// rendered seconds in it).
    public func spokenReport() -> (block: Int, final: Int, seconds: Double) {
        let p = app.runtime.player, d = p.document
        let idx = min(max(p.blockIndex, 0), max(d.blocks.count - 1, 0))
        let rate = BarTracker.wordsPerSecond * max(p.rate, 0.5)
        func words(_ b: ProgrammeBlock) -> Double { Double((b.spokenText ?? "").split(separator: " ").count) }
        let before = d.blocks.prefix(idx).reduce(0.0) { $0 + words($1) } / rate
        let inBlock = p.renderPosition()?.seconds ?? 0
        if let si = d.segmentIndex(containingBlock: idx) {
            let seg = d.segments[si]
            let k = idx - seg.firstBlock
            return (seg.unitServerBlocks.indices.contains(k) ? seg.unitServerBlocks[k] : 0, max(0, seg.serverBlockCount - 1), before + inBlock)
        }
        return (idx, max(0, d.blocks.count - 1), before + inBlock)
    }
    public func restartSpoken() { app.runtime.player.begin(at: 0) }
    /// The engine came back mid-entry: `play()` starts at the first block that did not play.
    public func resumeSpoken() { app.runtime.player.play() }
    public func spokenPosition() -> (block: Int, seconds: Double) { app.runtime.player.storyPosition() }
    /// Silence the spoken entry without pausing the radio: an empty document replaces it and its loop ends (the listener's pause is untouched).
    public func stopSpoken() {
        app.runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
        app.runtime.player.start(at: 0)
    }

    public func pauseSpoken() { app.runtime.player.pause() }
    /// Sound is reaching the output: the spoken block's rendered seconds are past zero, or remote audio is playing and its position moves.
    public func rendering() -> Bool {
        if remote.isPlaying, let p = remote.position(), p.elapsed > 0.05 { return true }
        if let r = app.runtime.player.renderPosition(), r.seconds > 0 { return true }
        return false
    }
    public func spokenRemainder() -> String? { app.runtime.player.remainingText() }
    public func speakFailure() -> String? { app.runtime.player.speakFailure }

    public func endSpokenAfterCurrentBlock() { app.runtime.player.endAfterCurrentBlock() }

    public func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool {
        stopSpoken()                                    // the programme's voice stops (the listener's pause is not touched)
        return await remote.play(url: url, title: title, show: show, start: start, end: end, rate: BarTracker.shared.speed, generation: 0)
    }

    public func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { await remote.duration(of: url, deadline: deadline) }
    public func pauseRemote() { remote.pause() }
    public func resumeRemote() {
        continuity?.readerTapped()
        let rt = app.runtime, r = remote
        Task { @MainActor in await rt.ensureSessionReady(reason: "resume remote audio"); r.resume() }     // the session first: it may be dead after the background
    }
    public func stopRemote() { remote.stop() }
    public func seekRemote(by seconds: Double) { remote.seek(by: seconds) }
    public func seekRemote(to seconds: Double) { remote.seek(to: seconds) }
    public func clearRemoteEnd() { remote.clearEnd() }
    public func remotePosition() -> (elapsed: Double, duration: Double?)? { remote.position() }
    public func setRemoteRate(_ r: Double) { remote.setRate(r) }
    public func setRemoteLevel(_ l: PlayLevel) { remote.setLevel(l) }
}
