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
    guard rate.isFinite, rate > 0, let f = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: rate, channels: 1, interleaved: false),
          let b = AVAudioPCMBuffer(pcmFormat: f, frameCapacity: AVAudioFrameCount(max(s.count, 1))) else { return nil }
    b.frameLength = AVAudioFrameCount(s.count)
    s.withUnsafeBufferPointer { if let base = $0.baseAddress { b.floatChannelData![0].update(from: base, count: s.count) } }
    return b
}

/// The one AVAudioEngine: mic tap -> bus.mic (16 kHz mono), two player nodes (programme, voice) -> mixer,
/// mixer tap -> bus.reference. It never touches AVAudioSession: the session owner claims it first, and
/// after every claim change (or interruption, route change, media-services reset) the engine is restarted here.
///
/// `virtual` is the engine under the Exact agent: no hardware is opened, playback "plays" by waiting, and mic
/// frames come from whoever pushes them into the bus. Every node call goes through this class and does nothing
/// while the engine is down (a player node on a stopped engine raises an Objective-C exception).
public final class LiveAudio: @unchecked Sendable {
    public static let playRate = 44_100.0
    private(set) var engine = AVAudioEngine()
    private(set) var programmeNode = AVAudioPlayerNode()
    private(set) var voiceNode = AVAudioPlayerNode()
    /// Between the programme node and the mixer: changes the speed and keeps the pitch. The node's own render
    /// position is upstream of it, so it counts source audio and does not depend on the rate.
    private(set) var timePitch = AVAudioUnitTimePitch()
    let playFmt = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: playRate, channels: 1, interleaved: false)!
    private let lock = NSLock()
    private var bus: AudioBus?
    private var clock: LabClock?
    private var wantMic = false
    private var observer: NSObjectProtocol?
    /// Flags the audio thread, the notification queue, background tasks and the main thread all read: one lock (the repair of the
    /// app review; `running` was polled by `playProgramme` while `bring` and `takeDown` wrote it).
    private struct Flags { var running = false, micActive = false, down = false, nodeStopped = true, wantPlaying = false, restarting = false
        var lastRestart = 0.0; var volume: Float = 1 }
    private let flags = Locked(Flags())
    /// Continuations waiting for a buffer to finish playing. Any stop of the nodes or the engine resumes them all (a stopped
    /// player node does not call its `.dataPlayedBack` completions), so no wait outlives the audio it waits for.
    let waiters = Locked<[UUID: Once<Void>]>([:])
    private var restarting: Bool { get { flags.read { $0.restarting } } set { flags.write { $0.restarting = newValue } } }
    private var lastRestart: Double { get { flags.read { $0.lastRestart } } set { flags.write { $0.lastRestart = newValue } } }
    private var generation = 0                  // bumped by every engine stop that drops scheduled buffers
    private var playhead: Playhead?             // where the block being played stands (guarded by `lock`)
    private var nodeStopped: Bool {                // the programme node has no timeline (never started, stopped, engine down)
        get { flags.read { $0.nodeStopped } } set { flags.write { $0.nodeStopped = newValue } }
    }
    private var lastRender = 0
    private var rate = 1.0                      // programme speed, 1 = natural
    private var wantPlaying: Bool {                // the programme should be audible when the engine is up
        get { flags.read { $0.wantPlaying } } set { flags.write { $0.wantPlaying = newValue } }
    }
    private var volume: Float { get { flags.read { $0.volume } } set { flags.write { $0.volume = newValue } } }
    private var virtualPaused = true
    private var virtualVolume: Float = 1
    public private(set) var running: Bool { get { flags.read { $0.running } } set { flags.write { $0.running = newValue } } }
    public private(set) var micActive: Bool { get { flags.read { $0.micActive } } set { flags.write { $0.micActive = newValue } } }
    public private(set) var virtual = false
    public private(set) var voiceProcessing = false
    /// The engine was down (interruption, failed restart) and has not come back.
    public private(set) var down: Bool { get { flags.read { $0.down } } set { flags.write { $0.down = newValue } } }
    /// Told of anything worth an event: (kind, detail).
    public var onNotice: (@Sendable (String, String) -> Void)?

    public init() { wire() }

    private func wire() {
        engine.attach(programmeNode); engine.attach(voiceNode); engine.attach(timePitch)
        timePitch.rate = Float(rate); timePitch.pitch = 0
        engine.connect(programmeNode, to: timePitch, format: playFmt)
        engine.connect(timePitch, to: engine.mainMixerNode, format: playFmt)
        engine.connect(voiceNode, to: engine.mainMixerNode, format: playFmt)
    }

    public static func requestMicPermission() async -> Bool {
        await withCheckedContinuation { c in AVAudioApplication.requestRecordPermission { c.resume(returning: $0) } }
    }

    /// The programme node's current volume (what yielding set), and whether it is playing: for the trace and the proof.
    public var programmeVolume: Float { virtual ? virtualVolume : volume }
    public var programmeAudible: Bool { virtual ? !virtualPaused : (running && programmeNode.isPlaying) }

    // MARK: Start, stop, restart

    public func start(bus: AudioBus, clock: LabClock, wantMic: Bool, voiceProcessing: Bool = false, virtual: Bool = false) throws {
        if running { return }
        self.bus = bus; self.clock = clock; self.wantMic = wantMic; self.virtual = virtual
        if virtual { running = true; down = false; micActive = true; return }
        try bring(voiceProcessing: voiceProcessing)
        observe()
    }

    /// Stop the engine and drop its taps, remembering how far the programme node had rendered. Call before the
    /// session changes category (that stops a running engine anyway).
    public func quiesce() {
        guard running, !virtual else { return }
        lock.lock(); generation += 1; lock.unlock()
        freezePlayhead()
        takeDown()
    }

    /// Bring the engine back up after a category change; `voiceProcessing` for the talking state.
    public func resume(voiceProcessing vp: Bool) throws {
        guard !virtual, bus != nil else { return }
        try bring(voiceProcessing: vp)
    }

    /// Restart in place (route change, interruption end): the same voice processing setting.
    public func recover(reason: String) {
        guard !virtual, bus != nil, !restarting else { return }
        restarting = true; defer { restarting = false }
        if running { quiesce() }
        do { try bring(voiceProcessing: voiceProcessing); onNotice?("engine", "restarted: \(reason)") }
        catch { down = true; onNotice?("error", "engine restart failed (\(reason)): \(error)") }
    }

    /// Media services were reset: every engine object is invalid. Make new ones and start again.
    public func rebuild() {
        guard !virtual, bus != nil else { return }
        if let o = observer { NotificationCenter.default.removeObserver(o); observer = nil }
        lock.lock(); generation += 1; lock.unlock()
        freezePlayhead()
        running = false; micActive = false
        nodeStopped = true; releaseWaiters(); engine = AVAudioEngine(); programmeNode = AVAudioPlayerNode(); voiceNode = AVAudioPlayerNode(); timePitch = AVAudioUnitTimePitch()
        wire()
        do { try bring(voiceProcessing: voiceProcessing); observe(); onNotice?("engine", "rebuilt after media services reset") }
        catch { down = true; onNotice?("error", "engine rebuild failed: \(error)") }
    }

    public func stop() {
        if virtual { running = false; micActive = false; virtualPaused = true; return }
        defer { releaseWaiters() }
        guard running else { return }
        if let o = observer { NotificationCenter.default.removeObserver(o); observer = nil }
        wantPlaying = false
        programmeNode.stop(); voiceNode.stop(); nodeStopped = true
        takeDown()
    }

    /// Frames the programme node has rendered since it was last started. A paused node may report nothing, so the last
    /// value stands while the node is merely paused; a stopped node (skip, engine down) starts again from 0.
    private func renderedFrames() -> Int {
        if nodeStopped { lastRender = 0; return 0 }
        if let t = programmeNode.lastRenderTime, let p = programmeNode.playerTime(forNodeTime: t) { lastRender = max(0, Int(p.sampleTime)) }
        return lastRender
    }

    /// Remember how far the block had played, before a stop makes the node forget.
    private func freezePlayhead() {
        let r = renderedFrames()
        lock.lock(); playhead?.freeze(rendered: r); lock.unlock()
    }

    private func takeDown() {
        if micActive { engine.inputNode.removeTap(onBus: 0) }
        engine.mainMixerNode.removeTap(onBus: 0)
        programmeNode.stop(); voiceNode.stop(); nodeStopped = true
        engine.stop(); running = false; micActive = false
        releaseWaiters()
    }

    /// Resume every wait on a scheduled buffer. Each caller then looks at the generation and the playhead to see whether to
    /// carry on from the frames already rendered (the engine restarted) or to end (a stop).
    func releaseWaiters() {
        let all = waiters.write { w -> [Once<Void>] in let x = Array(w.values); w = [:]; return x }
        for o in all { o.resume(()) }
    }

    private func bring(voiceProcessing vp: Bool) throws {
        guard let bus, let clock else { return }
        voiceProcessing = false
        if wantMic {
            #if os(iOS)
            do { try engine.inputNode.setVoiceProcessingEnabled(vp); voiceProcessing = vp }
            catch { onNotice?("error", "voice processing \(vp ? "on" : "off") failed: \(error)") }
            #endif
            let inFmt = engine.inputNode.outputFormat(forBus: 0)
            if inFmt.sampleRate > 0 {
                let rs = Resampler(rate: busSampleRate)
                engine.inputNode.installTap(onBus: 0, bufferSize: 1024, format: inFmt) { buf, _ in
                    let s = rs.convert(buf)
                    if !s.isEmpty { bus.pushMic(AudioFrame(samples: s, sampleRate: busSampleRate, t: clock.now)) }
                }
                micActive = true
            } else { onNotice?("error", "input format has sample rate 0 (no microphone available)") }
        }
        let mixFmt = engine.mainMixerNode.outputFormat(forBus: 0)
        let rr = Resampler(rate: busSampleRate)
        engine.mainMixerNode.installTap(onBus: 0, bufferSize: 2048, format: mixFmt) { buf, _ in
            let s = rr.convert(buf)
            if !s.isEmpty { bus.pushReference(AudioFrame(samples: s, sampleRate: busSampleRate, t: clock.now)) }
        }
        engine.prepare()
        do { try engine.start() } catch {
            if micActive { engine.inputNode.removeTap(onBus: 0); micActive = false }
            engine.mainMixerNode.removeTap(onBus: 0)
            down = true
            throw error
        }
        running = true; down = false
        lastRestart = clock.now
        programmeNode.volume = volume
        if wantPlaying { programmeNode.play(); nodeStopped = false }
    }

    /// The engine stops itself when the route changes (AirPods in or out); restart it once, after the burst settles.
    private func observe() {
        if let o = observer { NotificationCenter.default.removeObserver(o) }
        observer = NotificationCenter.default.addObserver(forName: .AVAudioEngineConfigurationChange, object: engine, queue: nil) { [weak self] _ in
            guard let self, let clock = self.clock else { return }
            // Our own restart posts one too: ignore anything within a second of it.
            if self.restarting || clock.now - self.lastRestart < 1.0 { return }
            self.onNotice?("engine", "configuration change")
            DispatchQueue.global().asyncAfter(deadline: .now() + 0.3) { [weak self] in self?.recover(reason: "configuration change") }
        }
    }

    /// The system took the session (a call): stop pretending the engine runs.
    public func interrupted() {
        guard !virtual else { return }
        lock.lock(); generation += 1; lock.unlock()
        freezePlayhead()
        if running { takeDown() }
        down = true
        releaseWaiters()
    }

    // MARK: Programme node, guarded

    public func programmePlay() { wantPlaying = true; if virtual { virtualPaused = false } else if running { programmeNode.play(); nodeStopped = false } }
    public func programmePause() { wantPlaying = false; if virtual { virtualPaused = true } else if running { programmeNode.pause() } }
    public func programmeStop() { wantPlaying = false; if virtual { virtualPaused = true } else { programmeNode.stop(); nodeStopped = true; releaseWaiters() } }
    /// Speed of the programme, 0.5 to 2, pitch kept. Applies at once, also to what is already scheduled.
    public func setProgrammeRate(_ r: Double) {
        let c = r.isNaN ? 1 : min(2, max(0.5, r))
        lock.lock(); rate = c; lock.unlock()
        timePitch.rate = Float(c)
    }
    public var programmeRate: Double { lock.lock(); defer { lock.unlock() }; return rate }

    /// The position in the block `tag` (the block index), from the audio actually rendered; nil if that block is not the one
    /// last scheduled (nothing played yet, or another block).
    public func programmePosition(tag: Int) -> RenderPosition? {
        let r = virtual ? 0 : renderedFrames()
        lock.lock(); defer { lock.unlock() }
        guard let h = playhead, h.tag == tag else { return nil }
        return RenderPosition(index: tag, fraction: h.fraction(rendered: r), seconds: h.seconds(rendered: r), total: h.totalSeconds)
    }
    /// A block starts or is skipped: the old position answers for nothing.
    public func clearPlayhead() { lock.lock(); playhead = nil; lock.unlock() }

    public func setProgrammeVolume(_ v: Float) { volume = v; virtualVolume = v; if !virtual && running { programmeNode.volume = v } }

    // MARK: Playing

    /// Play voice frames now (acknowledgement, holding lines). Returns when the last frame has been heard.
    public func playVoice(_ frames: AsyncThrowingStream<AudioFrame, Error>) async {
        if virtual {
            var secs = 0.0
            do { for try await f in frames { secs += f.duration } } catch {}
            try? await Task.sleep(nanoseconds: UInt64(min(secs, 30) * 1e9))
            return
        }
        let rs = Resampler(rate: LiveAudio.playRate)
        if running { voiceNode.play() }
        let gate = NSLock()
        var pending = 0
        var finished = false
        var scheduledSeconds = 0.0
        let once = Once<Void>()
        let wid = UUID()
        waiters.write { $0[wid] = once }
        defer { waiters.write { $0[wid] = nil } }
        func settle() { gate.lock(); let fire = pending == 0 && finished; gate.unlock(); if fire { once.resume(()) } }
        do {
            for try await f in frames {
                guard running, let inb = monoBuffer(f.samples, rate: f.sampleRate) else { continue }
                let s = rs.convert(inb)
                guard let ob = monoBuffer(s, rate: LiveAudio.playRate), ob.frameLength > 0 else { continue }
                gate.lock(); pending += 1; scheduledSeconds += Double(ob.frameLength) / LiveAudio.playRate; gate.unlock()
                voiceNode.scheduleBuffer(ob, completionCallbackType: .dataPlayedBack) { _ in
                    gate.lock(); pending -= 1; gate.unlock(); settle()
                }
            }
        } catch {}
        gate.lock(); finished = true; let done = pending == 0; let secs = scheduledSeconds; gate.unlock()
        if done { return }
        // Every way the wait can end resumes it once: the last buffer played, a stop or restart (releaseWaiters), the caller
        // cancelled, or the audio taking far longer than it is long.
        let timer = Task { try? await Task.sleep(nanoseconds: UInt64((secs + 10) * 1e9)); once.resume(()) }
        await withTaskCancellationHandler {
            await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in once.arm(c) }
        } onCancel: { once.resume(()) }
        timer.cancel()
    }

    /// Schedule a buffer on the programme node and wait until it has played. If the engine is restarted meanwhile
    /// (a session switch, a route change), carry on from the frames already rendered once it is back.
    func playProgramme(_ samples: [Float], rate: Double, tag: Int = -1) async {
        if virtual {
            let total = Int(Double(samples.count) / rate * LiveAudio.playRate)
            lock.lock(); var h = Playhead(tag: tag, totalFrames: total); h.frozen = 0; playhead = h; lock.unlock()
            var played = 0.0                                      // source seconds
            let length = Double(samples.count) / rate
            while played < length, !Task.isCancelled {
                try? await Task.sleep(nanoseconds: 50_000_000)
                if !virtualPaused {
                    lock.lock(); let sp = self.rate; lock.unlock()
                    played = min(length, played + 0.05 * sp)
                    lock.lock(); if playhead?.tag == tag { playhead?.frozen = Int(played * LiveAudio.playRate) }; lock.unlock()
                }
            }
            return
        }
        guard let inb = monoBuffer(samples, rate: rate) else { return }
        let all = Resampler(rate: LiveAudio.playRate).convert(inb)
        lock.lock(); playhead = Playhead(tag: tag, totalFrames: all.count); lock.unlock()
        var offset = 0
        while offset < all.count, !Task.isCancelled {
            // Wait for the engine if it is down (a restart is under way); give up after 45 s.
            var waited = 0
            while !running, !Task.isCancelled, waited < 900 { try? await Task.sleep(nanoseconds: 50_000_000); waited += 1 }
            if !running || Task.isCancelled { return }
            lock.lock(); let gen = generation; lock.unlock()
            guard let ob = monoBuffer(Array(all[offset...]), rate: LiveAudio.playRate), ob.frameLength > 0 else { return }
            let base = renderedFrames()
            lock.lock(); if playhead?.tag == tag { playhead?.reschedule(from: offset, base: base) }; lock.unlock()
            // Resumed exactly once by: the buffer played, any stop or restart of the nodes or the engine (releaseWaiters), or the
            // task being cancelled. A stopped node never calls the completion, so without these the wait would hang for good.
            let once = Once<Void>()
            let wid = UUID()
            waiters.write { $0[wid] = once }
            await withTaskCancellationHandler {
                await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
                    once.arm(c)
                    programmeNode.scheduleBuffer(ob, completionCallbackType: .dataPlayedBack) { _ in once.resume(()) }
                    if wantPlaying, !programmeNode.isPlaying { programmeNode.play(); nodeStopped = false }
                }
            } onCancel: { once.resume(()) }
            waiters.write { $0[wid] = nil }
            lock.lock(); let now = generation; let frozen = playhead?.tag == tag ? playhead?.frozen : nil; lock.unlock()
            if now == gen { return }           // played through, or stopped by a transport change
            offset = frozen ?? offset          // the engine restarted mid-buffer: continue from what was rendered
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
    private let lock = NSLock()                 // guards `configuration` and `queue`
    private var configuration: LabConfiguration
    /// Everything the transport changes and the loop, the bar, the lock screen and the engine read, behind one lock (the repair of
    /// the app review: `document` was replaced from a revision while the loop indexed it). Never held across an await or a call out.
    private struct State {
        var document: ProgrammeDocument
        var blockIndex = 0
        var level: PlayLevel = .play
        var userPaused = true
        var jumpTo: Int?
        var rate = 1.0
        var loop: Task<Void, Never>?
        var epoch = 0                           // bumped by every start: a loop that is no longer the current one writes nothing
    }
    private let st: Locked<State>
    public var document: ProgrammeDocument { st.read { $0.document } }
    public var blockIndex: Int { st.read { $0.blockIndex } }
    public var level: PlayLevel { st.read { $0.level } }
    public var userPaused: Bool { st.read { $0.userPaused } }
    public var rate: Double { st.read { $0.rate } }
    public var onChange: (@Sendable () -> Void)?
    public var dipVolume: Float = 0.25
    /// Text the listener asked to hear (mail's "Read it to me"), behind the story now playing; each one a programme of its own.
    private var queue: [ProgrammeDocument] = []

    public init(audio: LiveAudio, registry: StageRegistry, log: EventLog, configuration: LabConfiguration, document: ProgrammeDocument) {
        self.audio = audio; self.registry = registry; self.log = log; self.configuration = configuration
        self.st = Locked(State(document: document))
        installRemoteCommands()
    }
    public func setConfiguration(_ c: LabConfiguration) { lock.lock(); configuration = c; lock.unlock() }
    public func setDocument(_ d: ProgrammeDocument) { st.write { $0.document = d; $0.blockIndex = 0 } }

    // MARK: Transport

    public func play() {
        let (lvl, hasLoop, at) = st.write { s -> (PlayLevel, Bool, Int) in s.userPaused = false; return (s.level, s.loop != nil, s.blockIndex) }
        if lvl != .pause { audio.programmePlay() }
        if !hasLoop { start(at: at) }
        publishNowPlaying(); onChange?()
    }
    public func pause() { st.write { $0.userPaused = true }; audio.programmePause(); publishNowPlaying(); onChange?() }
    public func next() {
        let (i, count) = st.read { ($0.blockIndex, $0.document.blocks.count) }
        if i >= count - 1, let n = dequeue() { st.write { $0.userPaused = false }; setDocument(n); start(at: 0); return }
        start(at: min(i + 1, count - 1))
    }
    public func previous() { start(at: max(blockIndex - 1, 0)) }

    func start(at i: Int) {
        let (old, my) = st.write { s -> (Task<Void, Never>?, Int) in
            let o = s.loop; s.epoch += 1; s.blockIndex = i; s.loop = nil; return (o, s.epoch)
        }
        old?.cancel()
        audio.programmeStop()
        audio.clearPlayhead()
        let task = Task { [weak self] in
            guard let self else { return }
            var n = i
            while !Task.isCancelled {
                // One look at the state per block: the block to play, and whether this loop is still the current one.
                let step = self.st.write { s -> (ProgrammeBlock, Bool, PlayLevel)? in
                    guard s.epoch == my, n < s.document.blocks.count else { return nil }
                    s.blockIndex = n
                    return (s.document.blocks[n], s.userPaused, s.level)
                }
                guard let (block, paused, lvl) = step else { break }
                self.audio.clearPlayhead(); self.onChange?(); self.publishNowPlaying()
                if paused { self.audio.programmePause() } else if lvl != .pause { self.audio.programmePlay() }
                await self.playBlock(block, index: n)
                if Task.isCancelled { break }
                n += 1
            }
            if !Task.isCancelled {
                let finished = self.st.write { s -> Bool in
                    guard s.epoch == my else { return false }
                    s.blockIndex = min(n, max(s.document.blocks.count - 1, 0)); s.loop = nil; return true
                }
                // The story is over: what the listener asked to hear comes next, like the next story.
                if finished { if let q = self.dequeue() { self.setDocument(q); self.start(at: 0) } else { self.onChange?() } }
            }
        }
        // Keep it unless a newer start has already replaced this one.
        let orphan = st.write { s -> Bool in if s.epoch == my { s.loop = task; return false } else { return true } }
        if orphan { task.cancel() }
    }

    // MARK: Speed

    /// Programme speed, 0.5 to 2, pitch kept (time-pitch in the audio engine). Applies to what is playing and to what follows.
    public func setRate(_ r: Double) {
        let c = r.isNaN ? 1 : min(2, max(0.5, r))
        st.write { $0.rate = c }; audio.setProgrammeRate(c); publishNowPlaying(); onChange?()
    }

    /// Where the player is in the block it is playing, from the audio rendered; nil when that block has no audio yet.
    public func renderPosition() -> RenderPosition? { audio.programmePosition(tag: blockIndex) }

    // MARK: Text to speak

    /// Speak arbitrary text as a programme item. It becomes a programme of its own, "text:<address>", paragraph by paragraph.
    /// Nothing playing (or `startNow`): it replaces the idle programme and plays. A story is playing: it waits its turn behind it,
    /// like the next story, and shows in now-playing when it starts. `play: false` only makes it the loaded programme (the engine is
    /// not running; it plays when the radio starts). Returns false for text with nothing to say.
    @discardableResult
    public func speakText(address: String, text: String, title: String? = nil, startNow: Bool = false, play: Bool = true) -> Bool {
        let paras = Array(ChoirText.paragraphs(text).prefix(200))
        guard !paras.isEmpty else { return false }
        let name = title ?? paras[0].split(separator: " ").prefix(6).joined(separator: " ")
        let doc = ProgrammeDocument(id: "text:\(address)", title: name, blocks: paras.map { .speech(text: $0) })
        if startNow || st.read({ $0.loop == nil }) {
            lock.lock(); queue.removeAll { $0.id == doc.id }; lock.unlock()
            let old = st.write { s -> Task<Void, Never>? in let o = s.loop; s.loop = nil; s.epoch += 1; return o }
            old?.cancel(); audio.programmeStop(); audio.clearPlayhead()
            setDocument(doc)
            if play { self.play() } else { publishNowPlaying(); onChange?() }
        } else {
            lock.lock()
            if let i = queue.firstIndex(where: { $0.id == doc.id }) { queue[i] = doc } else { queue.append(doc) }   // asked twice: once
            lock.unlock()
            onChange?()
        }
        return true
    }
    public var queuedTexts: [String] { lock.lock(); defer { lock.unlock() }; return queue.map(\.id) }
    private func dequeue() -> ProgrammeDocument? { lock.lock(); defer { lock.unlock() }; return queue.isEmpty ? nil : queue.removeFirst() }

    func playBlock(_ b: ProgrammeBlock) async { await playBlock(b, index: blockIndex) }

    /// `index` tags the audio so the position answers for this block even if the index moves on while it plays.
    func playBlock(_ b: ProgrammeBlock, index tag: Int) async {
        switch b {
        case .speech(let t), .listenerPassage(let t, _):
            lock.lock(); let cfg = configuration; lock.unlock()
            guard let voice = try? registry.make(.voice, as: Voice.self, cfg) else { return }
            var samples: [Float] = []; var rate = 22050.0
            do { for try await f in voice.speak(AsyncStream { $0.yield(t); $0.finish() }) { samples += f.samples; rate = f.sampleRate } } catch {
                log.emit(.error(stage: "voice", message: "\(error)"))
            }
            if Task.isCancelled { return }
            log.emit(.programmeAudioStart(block: tag))      // the trace reads "desk audio start" from the first of these after a revision
            await audio.playProgramme(samples, rate: rate, tag: tag)
        case .clip(let url, let start, let end, _):
            guard let u = FixtureResources.resolve(url), let all = try? AudioFiles.readMono(u, rate: 16_000) else {
                log.emit(.error(stage: "player", message: "clip not found: \(url)")); return
            }
            let a = Int(start * 16_000), z = min(all.count, end.map { Int($0 * 16_000) } ?? all.count)
            if a < z { log.emit(.programmeAudioStart(block: tag)); await audio.playProgramme(Array(all[a..<z]), rate: 16_000, tag: tag) }
        }
    }

    // MARK: PlaybackControl

    public func position() async -> ProgramPosition {
        let (blocks, idx, id) = st.read { ($0.document.blocks, $0.blockIndex, $0.document.id) }
        let lo = max(0, idx - 1), hi = min(blocks.count - 1, idx + 1)
        let ctx = blocks.isEmpty || lo > hi ? "" : blocks[lo...hi].compactMap { $0.spokenText }.joined(separator: " ")
        return ProgramPosition(programmeID: id, blockIndex: idx, context: ctx)
    }
    /// The line being spoken to the listener right now and how much of it has been heard (speech blocks only, audibly playing).
    public func spoken() async -> SpokenCut? {
        let (i, paused, lvl, doc) = st.read { ($0.blockIndex, $0.userPaused, $0.level, $0.document) }
        guard !paused, lvl != .pause, audio.programmeAudible, doc.blocks.indices.contains(i),
              let full = doc.blocks[i].spokenText, let p = audio.programmePosition(tag: i), p.fraction < 1 else { return nil }
        return SpokenCut(blockIndex: i, full: full, spoken: SpeechCut.text(of: full, upTo: p.fraction), fraction: p.fraction)
    }
    public func setLevel(_ l: PlayLevel) async {
        let paused = st.write { s -> Bool in s.level = l; return s.userPaused }
        switch l {
        case .play: audio.setProgrammeVolume(1); if !paused { audio.programmePlay() }
        case .dip: audio.setProgrammeVolume(dipVolume)
        case .pause: audio.programmePause()
        }
        onChange?()
    }
    public func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async { await audio.playVoice(frames) }
    public func apply(_ r: Revision) async { st.write { $0.document = $0.document.applying(r); $0.jumpTo = r.fromBlock }; onChange?() }
    public func resume() async {
        let jump = st.write { s -> Int? in
            guard let j = s.jumpTo else { return nil }
            s.jumpTo = nil; s.userPaused = false; return min(j, max(s.document.blocks.count - 1, 0))
        }
        if let j = jump { start(at: j) }
        else if !userPaused { audio.programmePlay() }
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
        let (doc, idx, paused, speed) = st.read { ($0.document, $0.blockIndex, $0.userPaused, $0.rate) }
        let text = doc.blocks.indices.contains(idx) ? doc.blocks[idx].summary : ""
        MPNowPlayingInfoCenter.default().nowPlayingInfo = [
            MPMediaItemPropertyTitle: text, MPMediaItemPropertyArtist: doc.title.isEmpty ? "Autoradio" : doc.title,
            MPNowPlayingInfoPropertyPlaybackRate: paused ? 0.0 : speed,
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
