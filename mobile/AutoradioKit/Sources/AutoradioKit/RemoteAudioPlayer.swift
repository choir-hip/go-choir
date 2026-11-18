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
    /// Which start is current: a stop or a newer start moves it, and a start that waited through that gives up quietly.
    private var startToken = 0
    /// When the player itself last paused or resumed (a hold, a level change, the reader's pause): the observer reports that pause a moment later.
    private var ownPauseAt: DispatchTime = .init(uptimeNanoseconds: 0)
    private func ownPause() { ownPauseAt = .now() }

    /// The generation that ended (the item's own end or the clip's end).
    public var onEnded: ((Int) -> Void)?
    /// About once a second while something plays: the host saves the position.
    public var onTick: (() -> Void)?
    /// The system paused the player (an interruption, a route change, another app) while the reader wanted it playing: the host marks the
    /// queue paused, so the bar and the lock screen say what is true and the next Play plays.
    public var onSystemPause: (() -> Void)?
    /// Something worth a line in the event log (a failure, a system pause).
    public var note: ((String) -> Void)?
    private var controlObserver: NSKeyValueObservation?

    public init() {}

    public var isLoaded: Bool { player != nil }
    public var isPlaying: Bool { wantPlaying && level != .pause }

    public func play(url: URL, title: String, show: String, start: Double?, end: Double?, rate: Double, generation: Int) async -> Bool {
        stop()
        startToken &+= 1
        let token = startToken
        self.generation = generation; self.title = title; self.show = show; self.rate = PlaybackSpeed.clamp(rate); self.level = .play
        let it = AVPlayerItem(url: url)
        it.audioTimePitchAlgorithm = .timeDomain          // speech: keeps the pitch, no warble, 1/32x to 32x
        let p = AVPlayer(playerItem: it)
        p.automaticallyWaitsToMinimizeStalling = true
        p.defaultRate = Float(self.rate)
        player = p; item = it
        installCommands()
        controlObserver = p.observe(\.timeControlStatus, options: [.new]) { [weak self] pl, _ in
            guard pl.timeControlStatus == .paused else { return }
            Task { @MainActor in self?.checkSystemPause() }
        }
        let isReady = await ready(it)
        // A newer start (or a stop) took the player while this one waited: it touches nothing, and above all does not stop its replacement.
        guard token == startToken else { return false }
        guard isReady else {
            note?("remote: the stream did not become ready (\(it.error.map { "\($0)" } ?? "status \(it.status.rawValue)"))")
            stop(); return false
        }
        if let s = start, s > 0 { await p.seek(to: CMTime(seconds: s, preferredTimescale: 600), toleranceBefore: .zero, toleranceAfter: .zero) }
        if let e = end { setBoundary(e) }
        endObserver = NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: it, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.fire() }
        }
        tick = p.addPeriodicTimeObserver(forInterval: CMTime(seconds: 1, preferredTimescale: 1), queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.publish(); self?.onTick?() }
        }
        wantPlaying = true
        if !held { p.play() }
        publish()
        return true
    }

    /// The length of the file at `url`, read from its header without playing it; nil when it does not answer within `deadline`.
    public func duration(of url: URL, deadline: TimeInterval) async -> Double? {
        let shot = OneShot<Double?>()
        let asset = AVURLAsset(url: url)
        let load = Task { let d = try? await asset.load(.duration).seconds; shot.fire(d.flatMap { $0.isFinite && $0 > 0 ? $0 : nil }) }
        let timer = Task { try? await Task.sleep(nanoseconds: UInt64(max(0, deadline) * 1_000_000_000)); shot.fire(nil) }
        let r = await shot.wait()
        load.cancel(); timer.cancel()
        return r
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

    private var boundaryEnd: Double?
    private func setBoundary(_ end: Double) {
        clearBoundary()
        boundaryEnd = end
        guard let p = player else { return }
        boundary = p.addBoundaryTimeObserver(forTimes: [NSValue(time: CMTime(seconds: end, preferredTimescale: 600))], queue: .main) { [weak self] in
            MainActor.assumeIsolated { self?.fire() }
        }
    }
    private func clearBoundary() { if let b = boundary { player?.removeTimeObserver(b); boundary = nil }; boundaryEnd = nil }

    private func fire() { let g = generation; onEnded?(g) }

    public func pause() { ownPause(); wantPlaying = false; player?.pause(); publish() }
    public func resume() {
        guard let p = player else { return }
        wantPlaying = true
        if p.currentItem?.status == .failed { note?("remote: the item had failed (\(p.currentItem?.error.map { "\($0)" } ?? "unknown")); play does nothing until it is started again") }
        if level != .pause, !held { p.play() }
        publish()
    }

    /// The player paused and the reader did not ask for it: not at the end of the item, not for a hands-free turn.
    func checkSystemPause() {
        guard player != nil, wantPlaying, level != .pause, !held else { return }
        // A pause this player made itself (a voice line's hold, a session switch) is reported after it ended too: it is not the system's.
        if DispatchTime.now().uptimeNanoseconds &- ownPauseAt.uptimeNanoseconds < 1_500_000_000 { return }
        if let pos = position(), let d = pos.duration, pos.elapsed >= d - 0.75 { return }      // it reached its end
        if let e = boundaryEnd, (position()?.elapsed ?? 0) >= e - 0.25 { return }               // a clip reached its end
        wantPlaying = false
        note?("remote: paused by the system at \(Int(position()?.elapsed ?? 0)) s")
        publish()
        onSystemPause?()
    }

    public func stop() {
        startToken &+= 1
        clearBoundary()
        if let t = tick { player?.removeTimeObserver(t); tick = nil }
        if let o = endObserver { NotificationCenter.default.removeObserver(o); endObserver = nil }
        statusObserver?.invalidate(); statusObserver = nil
        controlObserver?.invalidate(); controlObserver = nil
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
        if wantPlaying, level != .pause, !held { player?.rate = Float(rate) }
        publish()
    }

    /// A hands-free turn: lowered, paused, or back. A pause for a turn does not forget that the reader wanted it playing.
    /// A voice line is speaking (the one voice at a time): remote audio is held, and goes on after it if it was playing.
    private var held = false
    public func setHeld(_ h: Bool) {
        guard h != held else { return }
        held = h
        ownPause()
        guard let p = player, wantPlaying else { return }
        if h { p.pause() } else if level != .pause { p.rate = Float(rate) }
        publish()
    }
    public var isHeld: Bool { held }
    func installPlayerForTest() { player = AVPlayer(); wantPlaying = true }
    var playerRateForTest: Float { player?.rate ?? 0 }

    public func setLevel(_ l: PlayLevel) {
        level = l
        ownPause()
        guard let p = player else { return }
        switch l {
        case .play: p.volume = 1; if wantPlaying, !held { p.rate = Float(rate) }
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
