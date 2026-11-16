import Foundation

/// The run sheet player's engine in the app: spoken entries go through the existing programme player (ChoirApp), remote audio through
/// AVPlayer (RemoteAudioPlayer). Only one of them makes sound at a time: starting one silences the other.
@MainActor public final class ChoirPlaybackEngine: PlaybackEngine {
    let app: ChoirApp
    public let remote = RemoteAudioPlayer()
    var player: RunSheetPlayer { RunSheetPlayer.shared }

    public init(app: ChoirApp) { self.app = app }

    public func speak(documentID: String, entry: RunSheetEntry, paragraphs: [String]) async -> Bool {
        remote.stop()
        return await app.speakEntry(documentID: documentID, entry: entry, paragraphs: paragraphs)
    }

    public func playStory(route: String, title: String) async -> String? {
        remote.stop()
        return await app.playPaperStory(route: route)
    }

    public func restartSpoken() { app.runtime.player.begin(at: 0) }
    public func spokenPosition() -> (block: Int, seconds: Double) { app.runtime.player.storyPosition() }
    /// Silence the spoken entry without pausing the radio: an empty document replaces it and its loop ends (the listener's pause is untouched).
    public func stopSpoken() {
        app.runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
        app.runtime.player.start(at: 0)
    }

    public func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool {
        stopSpoken()                                    // the programme's voice stops (the listener's pause is not touched)
        await app.ensureEngine()                        // the engine's start claims the audio session
        return await remote.play(url: url, title: title, show: show, start: start, end: end, rate: BarTracker.shared.speed, generation: player.generation)
    }

    public func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { await remote.duration(of: url, deadline: deadline) }
    public func pauseRemote() { remote.pause() }
    public func resumeRemote() { remote.resume() }
    public func stopRemote() { remote.stop() }
    public func seekRemote(by seconds: Double) { remote.seek(by: seconds) }
    public func seekRemote(to seconds: Double) { remote.seek(to: seconds) }
    public func clearRemoteEnd() { remote.clearEnd() }
    public func remotePosition() -> (elapsed: Double, duration: Double?)? { remote.position() }
    public func setRemoteRate(_ r: Double) { remote.setRate(r) }
    public func setRemoteLevel(_ l: PlayLevel) { remote.setLevel(l) }
}
