import Foundation

/// The queue's engine in the app: stories and spoken text go through the existing programme player (ChoirApp), remote audio through
/// AVPlayer (RemoteAudioPlayer). Only one of them makes sound at a time: starting one silences the other.
@MainActor public final class ChoirPlaybackEngine: PlaybackEngine {
    let app: ChoirApp
    var continuity: PlaybackContinuity?
    public let remote = RemoteAudioPlayer()
    var queue: PlayQueue { PlayQueue.shared }

    public init(app: ChoirApp) { self.app = app }

    public func playStory(route: String, title: String) async -> Bool {
        remote.stop()
        return await app.playStoryDirect(route: route)
    }

    public func speak(address: String, title: String, paragraphs: [String]) async -> Bool {
        remote.stop()
        let text = paragraphs.joined(separator: "\n\n")
        return await app.runtime.speakText(address: address, text: text, title: title.isEmpty ? nil : title, now: true)
    }

    public func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool {
        app.runtime.player.pause()                      // the programme's voice stops; the paper's list is not advanced by this
        if !app.runtime.snapshot().running { await app.runtime.start() }   // the engine's start claims the audio session
        else { await app.runtime.ensureSessionReady(reason: "remote audio") }
        return await remote.play(url: url, title: title, show: show, start: start, end: end, rate: BarTracker.shared.speed, generation: queue.generation)
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
