import Foundation
import AVFoundation
import MediaPlayer

/// Remote audio (a podcast episode or a clip of one) played by AVPlayer: it streams over http(s), seeks truly (the server's range
/// requests), and lets the system keep it going in the background (the app's `UIBackgroundModes: audio`, from app.json). The audio
/// session is the host's (the Kit never touches AVAudioSession): the radio engine's start claims it before any remote audio plays.
/// Speed is AVPlayer's own rate with the time-domain pitch algorithm, which keeps a voice natural from 0.5x to 3x.
/// This file is the thin, device-only part; the queue's logic is tested against a fake `PlaybackEngine` (PlayQueueTests).
@MainActor public final class RemoteAudioPlayer {
    private var player: AVPlayer?
    private var item: AVPlayerItem?
    private var tick: Any?
    private var boundary: Any?
    private var endObserver: NSObjectProtocol?
    private var statusObserver: NSKeyValueObservation?
    private var title = "", show = ""
    private var rate = 1.0
    private var level: PlayLevel = .play
    private var wantPlaying = false
    private var commandsInstalled = false
    private var generation = 0

    /// The generation that ended (the item's own end or the clip's end).
    public var onEnded: ((Int) -> Void)?
    /// About once a second while something plays: the host saves the position.
    public var onTick: (() -> Void)?

    public init() {}

    public var isLoaded: Bool { player != nil }
    public var isPlaying: Bool { wantPlaying && level != .pause }

    public func play(url: URL, title: String, show: String, start: Double?, end: Double?, rate: Double, generation: Int) async -> Bool {
        stop()
        self.generation = generation; self.title = title; self.show = show; self.rate = PlaybackSpeed.clamp(rate); self.level = .play
        let it = AVPlayerItem(url: url)
        it.audioTimePitchAlgorithm = .timeDomain          // speech: keeps the pitch, no warble, 1/32x to 32x
        let p = AVPlayer(playerItem: it)
        p.automaticallyWaitsToMinimizeStalling = true
        p.defaultRate = Float(self.rate)
        player = p; item = it
        installCommands()
        guard await ready(it) else { stop(); return false }
        if let s = start, s > 0 { await p.seek(to: CMTime(seconds: s, preferredTimescale: 600), toleranceBefore: .zero, toleranceAfter: .zero) }
        if let e = end { setBoundary(e) }
        endObserver = NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: it, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.fire() }
        }
        tick = p.addPeriodicTimeObserver(forInterval: CMTime(seconds: 1, preferredTimescale: 1), queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.publish(); self?.onTick?() }
        }
        wantPlaying = true
        p.play()
        publish()
        return true
    }

    /// Ready to play, failed, or no answer in 20 seconds (a stream that never starts is a failure, not a hang).
    private func ready(_ it: AVPlayerItem) async -> Bool {
        if it.status == .readyToPlay { return true }
        if it.status == .failed { return false }
        return await withCheckedContinuation { (c: CheckedContinuation<Bool, Never>) in
            let done = Locked(false)
            func finish(_ ok: Bool) { if done.write({ let was = $0; $0 = true; return !was }) { c.resume(returning: ok) } }
            statusObserver = it.observe(\.status, options: [.new]) { item, _ in
                if item.status == .readyToPlay { finish(true) } else if item.status == .failed { finish(false) }
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 20) { finish(false) }
        }
    }

    private func setBoundary(_ end: Double) {
        clearBoundary()
        guard let p = player else { return }
        boundary = p.addBoundaryTimeObserver(forTimes: [NSValue(time: CMTime(seconds: end, preferredTimescale: 600))], queue: .main) { [weak self] in
            MainActor.assumeIsolated { self?.fire() }
        }
    }
    private func clearBoundary() { if let b = boundary { player?.removeTimeObserver(b); boundary = nil } }

    private func fire() { let g = generation; onEnded?(g) }

    public func pause() { wantPlaying = false; player?.pause(); publish() }
    public func resume() { guard let p = player else { return }; wantPlaying = true; if level != .pause { p.play() }; publish() }

    public func stop() {
        clearBoundary()
        if let t = tick { player?.removeTimeObserver(t); tick = nil }
        if let o = endObserver { NotificationCenter.default.removeObserver(o); endObserver = nil }
        statusObserver?.invalidate(); statusObserver = nil
        player?.pause(); player = nil; item = nil; wantPlaying = false
        MPNowPlayingInfoCenter.default().nowPlayingInfo = nil
        setCommands(enabled: false)
    }

    public func seek(by s: Double) { seek(to: (position()?.elapsed ?? 0) + s) }
    public func seek(to s: Double) {
        guard let p = player else { return }
        var t = max(0, s)
        if let d = position()?.duration, d > 0 { t = min(t, d) }
        p.seek(to: CMTime(seconds: t, preferredTimescale: 600), toleranceBefore: .zero, toleranceAfter: .zero) { [weak self] _ in
            Task { @MainActor in self?.publish() }
        }
    }
    public func clearEnd() { clearBoundary() }

    public func position() -> (elapsed: Double, duration: Double?)? {
        guard let p = player, let it = item else { return nil }
        let e = p.currentTime().seconds
        let d = it.duration.seconds
        return (e.isFinite ? e : 0, d.isFinite && d > 0 ? d : nil)
    }

    public func setRate(_ r: Double) {
        rate = PlaybackSpeed.clamp(r)
        player?.defaultRate = Float(rate)
        if wantPlaying, level != .pause { player?.rate = Float(rate) }
        publish()
    }

    /// A hands-free turn: lowered, paused, or back. A pause for a turn does not forget that the reader wanted it playing.
    public func setLevel(_ l: PlayLevel) {
        level = l
        guard let p = player else { return }
        switch l {
        case .play: p.volume = 1; if wantPlaying { p.rate = Float(rate) }
        case .dip: p.volume = 0.25
        case .pause: p.pause()
        }
        publish()
    }

    // MARK: Lock screen

    private func publish() {
        guard player != nil else { return }
        let pos = position()
        var info: [String: Any] = [
            MPMediaItemPropertyTitle: title, MPMediaItemPropertyArtist: show,
            MPNowPlayingInfoPropertyElapsedPlaybackTime: pos?.elapsed ?? 0,
            MPNowPlayingInfoPropertyPlaybackRate: isPlaying ? rate : 0.0,
            MPNowPlayingInfoPropertyDefaultPlaybackRate: rate,
        ]
        if let d = pos?.duration { info[MPMediaItemPropertyPlaybackDuration] = d }
        MPNowPlayingInfoCenter.default().nowPlayingInfo = info
    }

    private func installCommands() {
        if !commandsInstalled {
            commandsInstalled = true
            let c = MPRemoteCommandCenter.shared()
            c.skipForwardCommand.preferredIntervals = [15]
            c.skipBackwardCommand.preferredIntervals = [15]
            c.skipForwardCommand.addTarget { [weak self] _ in MainActor.assumeIsolated { guard let s = self, s.isLoaded else { return .noActionableNowPlayingItem }; s.seek(by: 15); return .success } }
            c.skipBackwardCommand.addTarget { [weak self] _ in MainActor.assumeIsolated { guard let s = self, s.isLoaded else { return .noActionableNowPlayingItem }; s.seek(by: -15); return .success } }
            c.changePlaybackPositionCommand.addTarget { [weak self] e in
                MainActor.assumeIsolated {
                    guard let s = self, s.isLoaded, let e = e as? MPChangePlaybackPositionCommandEvent else { return .noActionableNowPlayingItem }
                    s.seek(to: e.positionTime); return .success
                }
            }
        }
        setCommands(enabled: true)
    }
    private func setCommands(enabled: Bool) {
        let c = MPRemoteCommandCenter.shared()
        c.skipForwardCommand.isEnabled = enabled; c.skipBackwardCommand.isEnabled = enabled; c.changePlaybackPositionCommand.isEnabled = enabled
    }
}
