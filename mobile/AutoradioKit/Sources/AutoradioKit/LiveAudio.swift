import Foundation
import AVFoundation
import MediaPlayer

/// Converts any PCM buffer to mono Float32 at a fixed rate.
final class Resampler {
    private var conv: AVAudioConverter?
    private var inFmt: AVAudioFormat?
    let out: AVAudioFormat
    init(rate: Double) { out = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: rate, channels: 1, interleaved: false)! }
    func convert(_ b: AVAudioPCMBuffer) -> [Float] {
        if inFmt != b.format { inFmt = b.format; conv = AVAudioConverter(from: b.format, to: out) }
        guard let c = conv else { return [] }
        let cap = AVAudioFrameCount(Double(b.frameLength) * out.sampleRate / b.format.sampleRate) + 32
        guard let ob = AVAudioPCMBuffer(pcmFormat: out, frameCapacity: cap) else { return [] }
        var given = false
        var err: NSError?
        c.convert(to: ob, error: &err) { _, st in
            if given { st.pointee = .noDataNow; return nil }
            given = true; st.pointee = .haveData; return b
        }
        guard err == nil, ob.frameLength > 0, let p = ob.floatChannelData?[0] else { return [] }
        return Array(UnsafeBufferPointer(start: p, count: Int(ob.frameLength)))
    }
}

func monoBuffer(_ s: [Float], rate: Double) -> AVAudioPCMBuffer? {
    guard let f = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: rate, channels: 1, interleaved: false),
          let b = AVAudioPCMBuffer(pcmFormat: f, frameCapacity: AVAudioFrameCount(max(s.count, 1))) else { return nil }
    b.frameLength = AVAudioFrameCount(s.count)
    s.withUnsafeBufferPointer { if let base = $0.baseAddress { b.floatChannelData![0].update(from: base, count: s.count) } }
    return b
}

/// The one AVAudioEngine: mic tap -> bus.mic (16 kHz mono), two player nodes (programme, voice) -> mixer,
/// mixer tap -> bus.reference. It never touches AVAudioSession: the session owner claims it first.
public final class LiveAudio: @unchecked Sendable {
    public static let playRate = 44_100.0
    let engine = AVAudioEngine()
    let programmeNode = AVAudioPlayerNode()
    let voiceNode = AVAudioPlayerNode()
    let playFmt = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: playRate, channels: 1, interleaved: false)!
    private var micConv: Resampler?
    private var refConv: Resampler?
    private(set) public var running = false
    public private(set) var micActive = false
    public init() {
        engine.attach(programmeNode); engine.attach(voiceNode)
        engine.connect(programmeNode, to: engine.mainMixerNode, format: playFmt)
        engine.connect(voiceNode, to: engine.mainMixerNode, format: playFmt)
    }

    public static func requestMicPermission() async -> Bool {
        await withCheckedContinuation { c in AVAudioApplication.requestRecordPermission { c.resume(returning: $0) } }
    }

    public func start(bus: AudioBus, clock: LabClock, wantMic: Bool) throws {
        if running { return }
        if wantMic {
            let inFmt = engine.inputNode.outputFormat(forBus: 0)
            if inFmt.sampleRate > 0 {
                let rs = Resampler(rate: busSampleRate); micConv = rs
                engine.inputNode.installTap(onBus: 0, bufferSize: 1024, format: inFmt) { buf, _ in
                    let s = rs.convert(buf)
                    if !s.isEmpty { bus.pushMic(AudioFrame(samples: s, sampleRate: busSampleRate, t: clock.now)) }
                }
                micActive = true
            }
        }
        let mixFmt = engine.mainMixerNode.outputFormat(forBus: 0)
        let rr = Resampler(rate: busSampleRate); refConv = rr
        engine.mainMixerNode.installTap(onBus: 0, bufferSize: 2048, format: mixFmt) { buf, _ in
            let s = rr.convert(buf)
            if !s.isEmpty { bus.pushReference(AudioFrame(samples: s, sampleRate: busSampleRate, t: clock.now)) }
        }
        engine.prepare()
        try engine.start()
        running = true
    }

    public func stop() {
        guard running else { return }
        programmeNode.stop(); voiceNode.stop()
        if micActive { engine.inputNode.removeTap(onBus: 0) }
        engine.mainMixerNode.removeTap(onBus: 0)
        engine.stop(); running = false; micActive = false
    }

    /// Play voice frames now (acknowledgement, holding lines). Returns when the last frame has been heard.
    public func playVoice(_ frames: AsyncThrowingStream<AudioFrame, Error>) async {
        let rs = Resampler(rate: LiveAudio.playRate)
        voiceNode.play()
        var pending = 0
        let lock = NSLock()
        var waiter: CheckedContinuation<Void, Never>?
        var finished = false
        func settle() { lock.lock(); let w = (pending == 0 && finished) ? waiter : nil; if w != nil { waiter = nil }; lock.unlock(); w?.resume() }
        do {
            for try await f in frames {
                guard let inb = monoBuffer(f.samples, rate: f.sampleRate) else { continue }
                let s = rs.convert(inb)
                guard let ob = monoBuffer(s, rate: LiveAudio.playRate), ob.frameLength > 0 else { continue }
                lock.lock(); pending += 1; lock.unlock()
                voiceNode.scheduleBuffer(ob, completionCallbackType: .dataPlayedBack) { _ in
                    lock.lock(); pending -= 1; lock.unlock(); settle()
                }
            }
        } catch {}
        lock.lock(); finished = true; let done = pending == 0; lock.unlock()
        if !done { await withCheckedContinuation { c in lock.lock(); waiter = c; let d = pending == 0; if d { waiter = nil }; lock.unlock(); if d { c.resume() } } }
    }

    /// Schedule a buffer on the programme node and wait until it has played (or the node is stopped).
    func playProgramme(_ samples: [Float], rate: Double) async {
        guard let inb = monoBuffer(samples, rate: rate) else { return }
        let rs = Resampler(rate: LiveAudio.playRate)
        let s = rs.convert(inb)
        guard let ob = monoBuffer(s, rate: LiveAudio.playRate), ob.frameLength > 0 else { return }
        await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
            let once = Once<Void>(c)
            programmeNode.scheduleBuffer(ob, completionCallbackType: .dataPlayedBack) { _ in once.resume(()) }
            if !programmeNode.isPlaying { programmeNode.play() }
        }
    }
}

// MARK: Programme player

/// Plays the programme document: speech blocks through the Voice stage, clips from files. Implements the
/// engine's PlaybackControl (yield levels, speaking an acknowledgement, applying a revision).
public final class ProgrammePlayer: PlaybackControl, @unchecked Sendable {
    let audio: LiveAudio
    let registry: StageRegistry
    let log: EventLog
    private let lock = NSLock()
    private var configuration: LabConfiguration
    public private(set) var document: ProgrammeDocument
    public private(set) var blockIndex = 0
    public private(set) var level: PlayLevel = .play
    public private(set) var userPaused = true
    private var jumpTo: Int?
    private var loop: Task<Void, Never>?
    public var onChange: (@Sendable () -> Void)?
    public var dipVolume: Float = 0.25

    public init(audio: LiveAudio, registry: StageRegistry, log: EventLog, configuration: LabConfiguration, document: ProgrammeDocument) {
        self.audio = audio; self.registry = registry; self.log = log; self.configuration = configuration; self.document = document
        installRemoteCommands()
    }
    public func setConfiguration(_ c: LabConfiguration) { lock.lock(); configuration = c; lock.unlock() }
    public func setDocument(_ d: ProgrammeDocument) { document = d; blockIndex = 0 }

    // MARK: Transport

    public func play() {
        userPaused = false
        if level != .pause { audio.programmeNode.play() }
        if loop == nil { start(at: blockIndex) }
        publishNowPlaying(); onChange?()
    }
    public func pause() { userPaused = true; audio.programmeNode.pause(); publishNowPlaying(); onChange?() }
    public func next() { start(at: min(blockIndex + 1, document.blocks.count - 1)) }
    public func previous() { start(at: max(blockIndex - 1, 0)) }

    func start(at i: Int) {
        loop?.cancel()
        audio.programmeNode.stop()
        blockIndex = i
        loop = Task { [weak self] in
            guard let self else { return }
            var n = i
            while !Task.isCancelled, n < self.document.blocks.count {
                self.blockIndex = n; self.onChange?(); self.publishNowPlaying()
                if self.userPaused { self.audio.programmeNode.pause() } else if self.level != .pause { self.audio.programmeNode.play() }
                await self.playBlock(self.document.blocks[n])
                if Task.isCancelled { break }
                n += 1
            }
            if !Task.isCancelled { self.blockIndex = min(n, max(self.document.blocks.count - 1, 0)); self.loop = nil; self.onChange?() }
        }
    }

    func playBlock(_ b: ProgrammeBlock) async {
        switch b {
        case .speech(let t), .listenerPassage(let t, _):
            lock.lock(); let cfg = configuration; lock.unlock()
            guard let voice = try? registry.make(.voice, as: Voice.self, cfg) else { return }
            var samples: [Float] = []; var rate = 22050.0
            do { for try await f in voice.speak(AsyncStream { $0.yield(t); $0.finish() }) { samples += f.samples; rate = f.sampleRate } } catch {
                log.emit(.error(stage: "voice", message: "\(error)"))
            }
            if Task.isCancelled { return }
            await audio.playProgramme(samples, rate: rate)
        case .clip(let url, let start, let end, _):
            guard let u = FixtureResources.resolve(url), let all = try? AudioFiles.readMono(u, rate: 16_000) else {
                log.emit(.error(stage: "player", message: "clip not found: \(url)")); return
            }
            let a = Int(start * 16_000), z = min(all.count, end.map { Int($0 * 16_000) } ?? all.count)
            if a < z { await audio.playProgramme(Array(all[a..<z]), rate: 16_000) }
        }
    }

    // MARK: PlaybackControl

    public func position() async -> ProgramPosition {
        let blocks = document.blocks
        let lo = max(0, blockIndex - 1), hi = min(blocks.count - 1, blockIndex + 1)
        let ctx = blocks.isEmpty ? "" : blocks[lo...hi].compactMap { $0.spokenText }.joined(separator: " ")
        return ProgramPosition(programmeID: document.id, blockIndex: blockIndex, context: ctx)
    }
    public func setLevel(_ l: PlayLevel) async {
        level = l
        switch l {
        case .play: audio.programmeNode.volume = 1; if !userPaused { audio.programmeNode.play() }
        case .dip: audio.programmeNode.volume = dipVolume
        case .pause: audio.programmeNode.pause()
        }
        onChange?()
    }
    public func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async { await audio.playVoice(frames) }
    public func apply(_ r: Revision) async { document = document.applying(r); jumpTo = r.fromBlock; onChange?() }
    public func resume() async {
        if let j = jumpTo { jumpTo = nil; userPaused = false; start(at: min(j, max(document.blocks.count - 1, 0))) }
        else if !userPaused { audio.programmeNode.play() }
    }

    // MARK: Now Playing and remote commands

    private func installRemoteCommands() {
        let c = MPRemoteCommandCenter.shared()
        c.playCommand.addTarget { [weak self] _ in self?.play(); return .success }
        c.pauseCommand.addTarget { [weak self] _ in self?.pause(); return .success }
        c.togglePlayPauseCommand.addTarget { [weak self] _ in
            guard let self else { return .commandFailed }
            if self.userPaused { self.play() } else { self.pause() }; return .success
        }
        c.nextTrackCommand.addTarget { [weak self] _ in self?.next(); return .success }
        c.previousTrackCommand.addTarget { [weak self] _ in self?.previous(); return .success }
    }
    func publishNowPlaying() {
        let text = document.blocks.indices.contains(blockIndex) ? document.blocks[blockIndex].summary : ""
        MPNowPlayingInfoCenter.default().nowPlayingInfo = [
            MPMediaItemPropertyTitle: text, MPMediaItemPropertyArtist: "Autoradio lab",
            MPNowPlayingInfoPropertyPlaybackRate: userPaused ? 0.0 : 1.0,
        ]
    }
}

/// Where clip files are found. `fixture:<name>` resolves in `directory`, then the bundled fixture clip.
public enum FixtureResources {
    nonisolated(unsafe) public static var directory: URL?
    public static func resolve(_ ref: String) -> URL? {
        if ref.hasPrefix("http://") || ref.hasPrefix("https://") { return nil }  // remote clips: fetch to a cache first (not in lane K)
        if ref.hasPrefix("file://") { return URL(string: ref) }
        guard ref.hasPrefix("fixture:") else { return URL(fileURLWithPath: ref) }
        let name = String(ref.dropFirst("fixture:".count))
        if let d = directory, FileManager.default.fileExists(atPath: d.appendingPathComponent(name).path) { return d.appendingPathComponent(name) }
        return FixtureClip.materialise(name: name)
    }
}
