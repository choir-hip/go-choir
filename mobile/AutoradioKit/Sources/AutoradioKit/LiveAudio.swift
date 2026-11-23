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
    /// The microphone may be used at all (permission granted) and is wanted now (voice is on): the tap exists only while both hold.
    /// Samples handed to the programme node since the engine was made (what the output was given to play).
    private var samplesHanded = 0
    public var programmeSamplesHanded: Int { lock.lock(); defer { lock.unlock() }; return samplesHanded }
    /// One voice at a time. While a voice line (acknowledgement, holding line, answer) is being heard, the programme node is held: it is the lower
    /// rank. It goes on when the last voice line ends, if the programme is still wanted. Under the lock.
    private var voiceLines = 0
    private var heldForVoice = false
    /// True while a voice line is audible (tests and the on-air line read it).
    public var voiceIsSpeaking: Bool { lock.lock(); defer { lock.unlock() }; return voiceLines > 0 }
    /// True when the programme node is audible or may be heard now: wanted, and no voice line speaking.
    /// The first voice line began (true) or the last ended (false): other sound outside this engine (remote audio) is held with the programme.
    public var onVoiceHold: (@Sendable (Bool) -> Void)?
    private func voiceBegan(reason: String) {
        lock.lock(); voiceLines += 1; let first = voiceLines == 1; lock.unlock()
        guard first else { return }
        onVoiceHold?(true)
        if virtual { if !virtualPaused { lock.lock(); heldForVoice = true; lock.unlock(); virtualPaused = true } }
        else if running, programmeNode.isPlaying { lock.lock(); heldForVoice = true; lock.unlock(); programmeNode.pause() }
        if heldForVoice { print("AUTORADIO hold programme reason=\(reason)") }
    }
    /// The session is being reconfigured: the same hold as a voice line, so what is on air is paused (and remote audio held) and goes on after.
    public func holdForSessionSwitch(_ on: Bool) { if on { voiceBegan(reason: "session switch") } else { voiceEnded() } }
    private func voiceEnded() {
        lock.lock(); voiceLines = max(0, voiceLines - 1); let last = voiceLines == 0; let held = heldForVoice; if last { heldForVoice = false }; lock.unlock()
        if last { onVoiceHold?(false) }
        guard last, held, wantPlaying else { return }
        if virtual { virtualPaused = false } else { engineGate.lock(); if running { programmeNode.play(); nodeStopped = false }; engineGate.unlock() }
        print("AUTORADIO resume programme reason=voice line ended")
    }
    private var micPermitted = false
    private var micWanted = false
    /// Another app has the session, or this app is in the background: the microphone is not taken (and not fought for) until it is given back.
    private let suspendedBox = Locked<Bool>(false)
    public var micSuspended: Bool {
        get { suspendedBox.value }
        set {
            let was = suspendedBox.value
            suspendedBox.write { $0 = newValue }
            if was, !newValue { micMayRetry() }           // the app is back in front: the outcome may differ now
        }
    }
    /// Something that could change the outcome of opening the microphone happened (the permission changed, the route changed, the app came back to the front, the listener turned voice on
    /// again): the retries start from none. A coordinator state switch is NOT such an event: it resumes the engine with the count it has, so a microphone that cannot open does not cost
    /// six engine rebuilds (each one cutting the programme) on every switch.
    public func micMayRetry() { micLock.lock(); micRetries = 0; micGaveUp = false; micChain += 1; micLock.unlock() }          // a reset also lets go of any retry already waiting: one chain at a time
    /// The retry chain now alive: a retry timer fires only when it is still the latest scheduled (a reset or a newer schedule makes the older one a no-op), so the count of rebuilds a route storm can cause is the count of one chain.
    private var micChain = 0
    /// The retry state (`micRetries`, `micGaveUp`, `micChain`, `lastRoute`) is read and written from the main thread (a resume, the suspension setter), the notification queue (a route notice), the global queue (a retry timer) and the engine's thread (`settleInput`): one lock.
    private let micLock = NSLock()
    /// The input route as the system names it (the input and output ports; the engine's input rate off the phone), and the one last seen.
    var routeNow: () -> String = {
        #if os(iOS)
        let r = AVAudioSession.sharedInstance().currentRoute
        return (r.inputs + r.outputs).map { $0.uid }.joined(separator: "|")
        #else
        return ""
        #endif
    }
    private var lastRoute: String?
    /// A configuration-change notice arrived: the retries start again only if the ROUTE differs from the one seen last (a notice alone says nothing: one posts for a rebuild, a format change or a call).
    func routeChanged(_ route: String) {
        micLock.lock(); let changed = route != lastRoute; lastRoute = route; micLock.unlock()
        if changed { micMayRetry() }
    }
    /// The input rate of the tap now installed (0: none), for the log line of voice on.
    public private(set) var micTapRate = 0.0
    private var wantMic: Bool { micPermitted && micWanted && !micSuspended }
    /// The system's answer to "is the microphone allowed now" (a test replaces it).
    var micPermissionProbe: @Sendable () -> Bool = { LiveAudio.micPermissionGranted() }
    /// Test seam: the input format's sample rate by engine build number (the real input node is not asked).
    var inputRateForTest: ((Int) -> Double)?
    /// Seconds before the first retry of an input that reported rate 0; each retry doubles it.
    var micRetryBase = 0.4
    /// Where the microphone stands: `off` it is not wanted, `open` the tap runs, `pending` it is being opened or retried, `refused` it is wanted
    /// and not allowed, `failed` it is allowed and could not be opened after the retries.
    public enum MicState: Equatable, Sendable { case off, open, pending, refused, failed }
    public var micState: MicState {
        if micActive { return .open }
        if !micWanted || micSuspended { return .off }
        if !(micPermitted || micPermissionProbe()) { return .refused }
        micLock.lock(); defer { micLock.unlock() }
        return micGaveUp ? .failed : .pending
    }
    /// The retries of an allowed microphone that reported no input are used up.
    private var micGaveUp = false
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

    /// The listener's answer to the microphone question, when voice is turned on after the engine started.
    public func permitMic(_ ok: Bool) { micPermitted = ok }

    /// Whether the microphone is already allowed (no prompt).
    public static func micPermissionGranted() -> Bool { AVAudioApplication.shared.recordPermission == .granted }

    public static func requestMicPermission() async -> Bool {
        await withCheckedContinuation { c in AVAudioApplication.requestRecordPermission { c.resume(returning: $0) } }
    }

    /// The programme node's current volume (what yielding set), and whether it is playing: for the trace and the proof.
    public var programmeVolume: Float { virtual ? virtualVolume : volume }
    public var programmeAudible: Bool { virtual ? !virtualPaused : (running && programmeNode.isPlaying) }

    // MARK: Start, stop, restart

    /// `wantMic`: the microphone is permitted. `mic`: it is wanted now (voice is on); nil means as permitted. A playback-only session has no
    /// input: a tap on its input node makes the engine fail to start, and nothing is then ever heard.
    public func start(bus: AudioBus, clock: LabClock, wantMic: Bool, mic: Bool? = nil, voiceProcessing: Bool = false, virtual: Bool = false) throws {
        if running { return }
        self.bus = bus; self.clock = clock; self.micPermitted = wantMic; self.micWanted = mic ?? wantMic; self.virtual = virtual
        if virtual { running = true; down = false; micActive = true; return }
        engineGate.lock(); defer { engineGate.unlock() }
        try bring(voiceProcessing: voiceProcessing)
        observe()
    }

    /// Stop the engine and drop its taps, remembering how far the programme node had rendered. Call before the
    /// session changes category (that stops a running engine anyway).
    public func quiesce() {
        guard running, !virtual else { return }
        engineGate.lock(); defer { engineGate.unlock() }
        lock.lock(); generation += 1; lock.unlock()
        freezePlayhead()
        takeDown()
    }

    /// Bring the engine back up after a category change; `voiceProcessing` for the talking state.
    /// `mic`: the microphone tap is wanted (voice is on); without it the engine only plays.
    /// `fresh`: the session changed category (listening to playing, or a full re-claim), so the hardware format and the route are not those the
    /// engine's graph was built for: a new engine and nodes are made rather than the old ones restarted.
    public func resume(voiceProcessing vp: Bool, mic: Bool? = nil, fresh: Bool = false) throws {
        guard !virtual, bus != nil else { return }
        if let mic { if mic, !micWanted { micMayRetry() }; micWanted = mic }       // voice turned on (again) starts the count again; a switch that resumes with the mic already wanted does not
        engineGate.lock(); defer { engineGate.unlock() }
        try bring(voiceProcessing: vp, fresh: fresh)
    }

    /// Test seam: stands in for the engine's prepare and start (no hardware is opened). Throws to fail the start.
    var driveForTest: ((_ voiceProcessing: Bool, _ mic: Bool) throws -> Void)?
    /// How many engines have been made (the first included): the proof that a switch built a new one.
    private(set) var engineBuilds = 1
    /// The programme found the engine down when a block began: the host takes the session and the engine again (the event that shows the fault).
    /// The answer is whether the engine is up and wanted: false makes the block give up at once (the entry is held, never counted as heard).
    public var onNeedsEngine: (@Sendable () async -> Bool)?
    /// The programme is wanted audible now (Play was pressed and not undone by a pause, a stop or a hands-free pause).
    public var isPlayWanted: Bool { wantPlaying }
    /// One gate for everything that replaces, starts or stops the engine: a restart from the configuration-change observer cannot run
    /// through a replacement made by the coordinator.
    private let engineGate = NSRecursiveLock()
    /// The engine's input node was asked for since this engine was made: an engine that has had one fails to start under a playback-only category.
    private var inputTouched = false

    /// Plain facts for the log when sound cannot come out (flags and numbers only).
    public func diagnostic() -> String {
        if virtual { return "engine=virtual" }
        engineGate.lock(); defer { engineGate.unlock() }
        let f = engine.outputNode.outputFormat(forBus: 0)
        return "engine running=\(running) down=\(down) builds=\(engineBuilds) out_rate=\(Int(f.sampleRate)) out_ch=\(f.channelCount)"
    }
    /// The output format's sample rate now (0 when not known), for the category-change line.
    public func outputRate() -> Int { if virtual { return 0 }; engineGate.lock(); defer { engineGate.unlock() }; return Int(engine.outputNode.outputFormat(forBus: 0).sampleRate) }

    /// The engine should be making sound or ready to, and is not: it was taken down (interruption, failed restart) or never came back.
    public var needsRecovery: Bool { !virtual && bus != nil && (down || !running) }

    /// Restart in place (route change, interruption end): the same voice processing setting.
    public func recover(reason: String, fresh: Bool = false) {
        guard !virtual, bus != nil, !restarting else { return }
        engineGate.lock(); defer { engineGate.unlock() }
        restarting = true; defer { restarting = false }
        if running { quiesce() }
        do { try bring(voiceProcessing: voiceProcessing, fresh: fresh); onNotice?("engine", "restarted: \(reason)") }
        catch { down = true; Self.printFault("engine restart failed (\(reason))", error); onNotice?("error", "engine restart failed (\(reason)): \(error)") }
    }

    /// Media services were reset: every engine object is invalid. Make new ones and start again.
    public func rebuild() {
        guard !virtual, bus != nil else { return }
        engineGate.lock(); defer { engineGate.unlock() }
        if let o = observer { NotificationCenter.default.removeObserver(o); observer = nil }
        lock.lock(); generation += 1; lock.unlock()
        freezePlayhead()
        running = false; micActive = false
        nodeStopped = true; releaseWaiters(); replaceEngine()
        do { try bring(voiceProcessing: voiceProcessing); observe(); onNotice?("engine", "rebuilt after media services reset") }
        catch { down = true; Self.printFault("engine rebuild failed", error); onNotice?("error", "engine rebuild failed: \(error)") }
    }

    /// A new engine and nodes, wired as the old ones were. The caller starts it. Notifications of the old engine are dropped.
    private func replaceEngine() {
        if let o = observer { NotificationCenter.default.removeObserver(o); observer = nil }
        // Whoever waits on the old nodes carries on from the playhead (a changed generation), never "played through".
        lock.lock(); generation += 1; lock.unlock()
        releaseWaiters()
        let old = engine
        if old.isRunning { old.stop() }
        engine = AVAudioEngine(); programmeNode = AVAudioPlayerNode(); voiceNode = AVAudioPlayerNode(); timePitch = AVAudioUnitTimePitch()
        engineBuilds += 1; inputTouched = false
        wire()
        observe()
    }

    public func stop() {
        if virtual { running = false; micActive = false; virtualPaused = true; return }
        defer { releaseWaiters() }
        wantPlaying = false                              // also when the engine is down: a block must not ask for it back after a stop
        guard running else { return }
        engineGate.lock(); defer { engineGate.unlock() }
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
        voiceWaiterIDs.write { $0 = [] }
        for o in all { o.resume(()) }
    }
    /// The waiters of voice lines (a subset of `waiters`): a barge-in stops these only. The programme block's wait must not be resumed by it, or
    /// the block would return "played" with its remaining words unspoken.
    let voiceWaiterIDs = Locked<Set<UUID>>([])
    func releaseVoiceWaiters() {
        let all = waiters.write { w -> [Once<Void>] in
            let ids = voiceWaiterIDs.write { s -> Set<UUID> in let x = s; s = []; return x }
            var out: [Once<Void>] = []
            for id in ids { if let o = w[id] { out.append(o); w[id] = nil } }
            return out
        }
        for o in all { o.resume(()) }
    }

    /// A category change is about to be made: the engine's configuration-change notifications are not listened to until the engine is started again.
    public func detachObserver() {
        engineGate.lock(); defer { engineGate.unlock() }
        if let o = observer { NotificationCenter.default.removeObserver(o); observer = nil }
    }

    /// An error's domain and code on the console log (the phone cannot be attached), nothing else of it.
    static func printFault(_ what: String, _ error: Error) {
        let ns = error as NSError
        print("AUTORADIO \(what) \(ns.domain) \(ns.code)")
    }

    /// Start the engine for the state now wanted. A start that fails is tried once more on a new engine: an engine that has been through a
    /// category change can refuse to start although a new one would not.
    private func bring(voiceProcessing vp: Bool, fresh: Bool = false) throws {
        guard bus != nil, clock != nil else { return }
        // Voice processing belongs to the input node and stays with the engine object: a playback-only engine after a talking one is a
        // new engine, so nothing of the voice-processing unit is left in the output path. The same for an engine whose input node was used
        // (a capture session before) and for any switch of category (`fresh`).
        // The microphone may have been allowed after this engine started (onboarding asks for it on its own, with the radio already playing):
        // `micPermitted` was only ever set when the engine first started, so it is read from the system here. An engine made before the
        // grant is not trusted with the input node: a new one is built.
        var newlyAllowed = false
        if !virtual, !micPermitted, micPermissionProbe() {
            micPermitted = true; micMayRetry(); newlyAllowed = micWanted
            print("AUTORADIO voice mic allowed after the engine started; \(newlyAllowed ? "a new engine is built for it" : "not wanted now")")
        }
        let rebuilt = fresh || newlyAllowed || (!wantMic && (voiceProcessing || inputTouched))
        if rebuilt { replaceEngine() }
        defer { if !virtual, observer == nil { observe() } }
        do { try attempt(voiceProcessing: vp) }
        catch {
            Self.printFault("engine start failed (rebuilt=\(rebuilt))", error)
            replaceEngine()
            do { try attempt(voiceProcessing: vp) }
            catch { Self.printFault("engine start failed again on a new engine", error); throw error }
            print("AUTORADIO engine started on a new engine after a failed start")
        }
    }

    private func attempt(voiceProcessing vp: Bool) throws {
        guard let bus, let clock else { return }
        voiceProcessing = false
        if let drive = driveForTest {
            do { try drive(vp, wantMic) } catch { down = true; throw error }
            if wantMic { inputTouched = true; settleInput(rate: inputRateForTest?(engineBuilds) ?? 48000) }
            running = true; down = false; lastRestart = clock.now
            return
        }
        if wantMic {
            // Voice processing I/O (echo cancellation) is on the iPhone and on the Mac alike: AVAudioIONode.h declares
            // setVoiceProcessingEnabled(_:) for macOS 10.15 and iOS 13, with the same rule (the engine stopped, both IO nodes switch together).
            inputTouched = true
            #if os(iOS) || os(macOS)
            do { try engine.inputNode.setVoiceProcessingEnabled(vp); voiceProcessing = vp }
            catch { Self.printFault("voice processing \(vp ? "on" : "off") failed", error); onNotice?("error", "voice processing \(vp ? "on" : "off") failed: \(error)") }
            #endif
            let inFmt = engine.inputNode.outputFormat(forBus: 0)
            if inFmt.sampleRate > 0 {
                let rs = Resampler(rate: busSampleRate)
                engine.inputNode.installTap(onBus: 0, bufferSize: 1024, format: inFmt) { buf, _ in
                    let s = rs.convert(buf)
                    if !s.isEmpty { bus.pushMic(AudioFrame(samples: s, sampleRate: busSampleRate, t: clock.now)) }
                }
            }
            settleInput(rate: inFmt.sampleRate)
        }
        let mixFmt = engine.mainMixerNode.outputFormat(forBus: 0)
        let rr = Resampler(rate: busSampleRate)
        engine.mainMixerNode.installTap(onBus: 0, bufferSize: 2048, format: mixFmt) { buf, _ in
            let s = rr.convert(buf)
            if !s.isEmpty { bus.pushReference(AudioFrame(samples: s, sampleRate: busSampleRate, t: clock.now)) }
        }
        // A test affordance for the Mac proof, not a feature: with -AutoradioProofSilent YES (or that default set) the output is muted, and
        // the samples are still rendered and counted, so the proof does not play through the owner's speakers.
        if UserDefaults.standard.bool(forKey: "AutoradioProofSilent") { engine.mainMixerNode.outputVolume = 0 }
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
        if wantPlaying, !voiceIsSpeaking { programmeNode.play(); nodeStopped = false }
        onNotice?("engine", "up: mic \(micActive), voice processing \(voiceProcessing), programme wanted \(wantPlaying)")
    }

    /// A route that is still settling (Bluetooth A2DP to the hands-free profile) reports no input for a moment: the tap is tried again, a few times,
    /// instead of leaving the microphone silent for good.
    private var micRetries = 0
    /// The input format's rate is known: a real rate means the tap runs; 0 with the microphone allowed is retried on a new engine, a bounded number
    /// of times (the permission was checked before: with none, `wantMic` is false and this is never reached).
    private func settleInput(rate: Double) {
        if rate > 0 { micActive = true; micTapRate = rate; micLock.lock(); micRetries = 0; micGaveUp = false; micLock.unlock(); return }
        micActive = false; micTapRate = 0
        onNotice?("error", "input format has sample rate 0 (no microphone available)")
        scheduleMicRetry()
    }
    private func scheduleMicRetry() {
        guard !virtual else { return }
        micLock.lock()
        guard micRetries < 6 else {
            let said = micGaveUp, tried = micRetries
            micGaveUp = true; micLock.unlock()
            if said { return }                           // said once, not at every switch that resumes
            print("AUTORADIO voice mic gave up: the input still reported rate 0 after \(tried) retries on new engines")
            onNotice?("error", "the microphone could not be opened after \(tried) retries on new engines")
            return
        }
        micRetries += 1
        micChain += 1
        let n = micRetries, chain = micChain
        micLock.unlock()
        let wait = micRetryBase * Double(1 << (n - 1))
        DispatchQueue.global().asyncAfter(deadline: .now() + wait) { [weak self] in
            guard let self else { return }
            self.micLock.lock(); let current = chain == self.micChain; self.micLock.unlock()
            guard current, self.wantMic, !self.micActive, !self.restarting else { return }
            print("AUTORADIO voice mic retry \(n) of 6: the input rate was 0 with the microphone allowed; building a new engine")
            self.recover(reason: "microphone input not available yet", fresh: true)
        }
    }

    /// The engine stops itself when the route changes (AirPods in or out); restart it once, after the burst settles.
    private func observe() {
        if let o = observer { NotificationCenter.default.removeObserver(o) }
        observer = NotificationCenter.default.addObserver(forName: .AVAudioEngineConfigurationChange, object: engine, queue: nil) { [weak self] _ in
            guard let self, let clock = self.clock else { return }
            // Our own restart posts one too: ignore anything within a second of it.
            if self.restarting || clock.now - self.lastRestart < 1.0 { return }
            self.onNotice?("engine", "configuration change")
            self.routeChanged(self.routeNow())
            DispatchQueue.global().asyncAfter(deadline: .now() + 0.3) { [weak self] in self?.recover(reason: "configuration change") }
        }
    }

    /// The system took the session (a call): stop pretending the engine runs.
    public func interrupted() {
        guard !virtual else { return }
        engineGate.lock(); defer { engineGate.unlock() }
        lock.lock(); generation += 1; lock.unlock()
        freezePlayhead()
        if running { takeDown() }
        down = true
        releaseWaiters()
    }

    // MARK: Programme node, guarded

    public func programmePlay() {
        wantPlaying = true
        if voiceIsSpeaking { lock.lock(); heldForVoice = true; lock.unlock(); return }      // the voice line has the output; the programme goes on after it
        if virtual { virtualPaused = false } else { engineGate.lock(); if running { programmeNode.play(); nodeStopped = false }; engineGate.unlock() }
    }
    public func programmePause() { wantPlaying = false; if virtual { virtualPaused = true } else if running { programmeNode.pause() } }
    public func programmeStop() { wantPlaying = false; if virtual { virtualPaused = true } else { programmeNode.stop(); nodeStopped = true; releaseWaiters() } }
    /// Speed of the programme, 0.5 to 3, pitch kept (`PlaybackSpeed`). Applies at once, also to what is already scheduled. The time-pitch
    /// unit takes 1/32 to 32, so 3 is well inside it; every voice is played through this unit, so none has a lower ceiling.
    public func setProgrammeRate(_ r: Double) {
        let c = PlaybackSpeed.clamp(r)
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
    public func playVoice(_ frames: AsyncThrowingStream<AudioFrame, Error>, reason: String = "a voice line") async {
        voiceBegan(reason: reason)
        defer { voiceEnded() }
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
        voiceWaiterIDs.write { _ = $0.insert(wid) }
        defer { waiters.write { $0[wid] = nil }; voiceWaiterIDs.write { _ = $0.remove(wid) } }
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

    /// Stop the voice node at once and release whoever waits on it (the listener spoke over the radio).
    public func stopVoice() {
        guard !virtual, running else { return }
        voiceNode.stop()
        releaseVoiceWaiters()
    }

    /// Schedule a buffer on the programme node and wait until it has played. If the engine is restarted meanwhile
    /// (a session switch, a route change), carry on from the frames already rendered once it is back.
    /// True when the block was played (to its end, or until a transport change stopped it); false when the engine is down and could not be made
    /// ready (or the wait was cancelled while it was down): nothing was heard, and the caller must not count it as heard.
    @discardableResult
    func playProgramme(_ samples: [Float], rate: Double, tag: Int = -1) async -> Bool {
        lock.lock(); samplesHanded += samples.count; lock.unlock()
        if virtual {
            let total = Int(Double(samples.count) / rate * LiveAudio.playRate)
            lock.lock(); var h = Playhead(tag: tag, totalFrames: total); h.frozen = 0; playhead = h; lock.unlock()
            var played = 0.0                                      // source seconds
            let length = Double(samples.count) / rate
            // A block always ends: a virtual engine that stays paused gives up at ten times the block's own length (and at least a minute).
            let deadline = Date().addingTimeInterval(max(60, length * 10))
            while played < length, !Task.isCancelled, Date() < deadline {
                try? await Task.sleep(nanoseconds: 50_000_000)
                if !virtualPaused {
                    lock.lock(); let sp = self.rate; lock.unlock()
                    played = min(length, played + 0.05 * sp)
                    lock.lock(); if playhead?.tag == tag { playhead?.frozen = Int(played * LiveAudio.playRate) }; lock.unlock()
                }
            }
            return true
        }
        guard let inb = monoBuffer(samples, rate: rate) else { return true }
        let all = Resampler(rate: LiveAudio.playRate).convert(inb)
        lock.lock(); playhead = Playhead(tag: tag, totalFrames: all.count); lock.unlock()
        var offset = 0
        while offset < all.count, !Task.isCancelled {
            // The engine is down. A restart that is under way is waited for a moment; otherwise the host is asked to bring it back (it
            // refuses when the listener paused, stopped, or another app has the session). No sleeping to pass time: a block that cannot
            // sound says so at once.
            if !running, !Task.isCancelled {
                var w = 0
                while !running, restarting, !Task.isCancelled, w < 60 { try? await Task.sleep(nanoseconds: 50_000_000); w += 1 }
            }
            if !running, !Task.isCancelled {
                print("AUTORADIO block finds the engine down \(diagnostic())")
                let up = await onNeedsEngine?() ?? false
                if !up || !running { print("AUTORADIO block gives up: the engine is not available, nothing was heard"); return false }
            }
            if Task.isCancelled { return !running ? false : true }
            lock.lock(); let gen = generation; lock.unlock()
            guard let ob = monoBuffer(Array(all[offset...]), rate: LiveAudio.playRate), ob.frameLength > 0 else { return true }
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
                    engineGate.lock()
                    if running, wantPlaying, !voiceIsSpeaking, !programmeNode.isPlaying { programmeNode.play(); nodeStopped = false }
                    engineGate.unlock()
                }
            } onCancel: { once.resume(()) }
            waiters.write { $0[wid] = nil }
            lock.lock(); let now = generation; let frozen = playhead?.tag == tag ? playhead?.frozen : nil; lock.unlock()
            if now == gen { return true }      // played through, or stopped by a transport change
            offset = frozen ?? offset          // the engine restarted mid-buffer: continue from what was rendered
        }
        return true
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
        /// The pause was the system's (an interruption began), not the listener's: a held entry goes on by itself when the session is back.
        var systemPause = false
        var jumpTo: Int?
        /// Where the loop stopped when the programme ran dry: the first block that was never played. Play starts there when blocks have come since.
        var resumeAt: Int?
        var rate = 1.0
        var loop: Task<Void, Never>?
        var epoch = 0                           // bumped by every start: a loop that is no longer the current one writes nothing
    }
    private let st: Locked<State>
    public var document: ProgrammeDocument { st.read { $0.document } }
    public var blockIndex: Int { st.read { $0.blockIndex } }
    public var level: PlayLevel { st.read { $0.level } }
    public var userPaused: Bool { st.read { $0.userPaused } }
    /// Paused by the listener (or by a lost route): only their Play ends it. A system interruption's pause is not this.
    public var pausedByListener: Bool { st.read { $0.userPaused && !$0.systemPause } }
    public var rate: Double { st.read { $0.rate } }
    public var onChange: (@Sendable () -> Void)?
    /// The loop ran out of blocks and nothing is queued behind it: (the document's id, whether any block of this run made sound).
    /// A programme whose stretch is still arriving also runs dry between segments; the host decides what that means (autoplay).
    public var onRanDry: (@Sendable (String, Bool) -> Void)?
    /// A block could not sound because the engine is down and could not be made ready: (the document's id, the block that did not play). The loop
    /// has stopped at that block (`play()` starts there again); the run is not over and nothing was heard.
    public var onEngineLost: (@Sendable (String, Int) -> Void)?
    let engineLostFlag = Locked(false)
    /// The lock screen's next and previous: the host may take them as story moves (true: handled); else they skip a passage or segment.
    public var onNextTrack: (@Sendable () -> Bool)?
    public var onPreviousTrack: (@Sendable () -> Bool)?
    /// The lock screen's play, pause and toggle: the host may take them (true: handled) when something other than this player is playing (remote audio).
    public var onTransport: (@Sendable (String) -> Bool)?
    /// True when a system pause arriving now is the system reacting to the radio's own session change (a reclaim, a switch): it is not the listener's.
    public var ignoreSystemPause: (@Sendable () -> Bool)?
    public var msSinceSessionChange: (@Sendable () -> Int)?
    /// Play was pressed: the host makes sure the audio session is claimed and active and the engine is up before sound is expected (it takes
    /// the claim again when the session was doubted or the engine is down). Play does not wait for it: the node plays as the engine comes up.
    public var onNeedsSession: (@Sendable () -> Void)?
    /// The yield level changed (a hands-free turn lowered or paused the programme): remote audio follows it.
    public var onLevel: (@Sendable (PlayLevel) -> Void)?
    /// Where playback is, per segment (station P1): P2 observes it.
    public let playback = SegmentTracker()
    /// Tests only: runs in the loop after its last look at the document and before the write that ends it (the window of review finding R3).
    var beforeLoopTail: (@Sendable () -> Void)?
    /// A playback loop is running (it ended dry, or was never started, when false).
    var loopIsActive: Bool { st.read { $0.loop != nil } }
    public var dipVolume: Float = 0.25
    /// Text the listener asked to hear (mail's "Read it to me"), behind the story now playing; each one a programme of its own.
    private var queue: [ProgrammeDocument] = []

    public init(audio: LiveAudio, registry: StageRegistry, log: EventLog, configuration: LabConfiguration, document: ProgrammeDocument) {
        self.audio = audio; self.registry = registry; self.log = log; self.configuration = configuration
        self.st = Locked(State(document: document))
        playback.paused(true)                   // the player starts paused
        installRemoteCommands()
    }
    public func setConfiguration(_ c: LabConfiguration) { lock.lock(); configuration = c; lock.unlock() }
    /// Audio made before it was wanted, by the block's text (the host sets it from `OpeningCache.prepared`). Nil or no answer: the block is made.
    public var preparedSpeech: (@Sendable (String) -> PreparedAudio?)?
    /// The chosen voice, as the cache keys it: a changed voice is another key.
    public var voiceKey: String { lock.lock(); defer { lock.unlock() }; return String(describing: configuration.setting(.voice)) }
    /// Load the chosen voice by making one short line (never played, never timed into the launch line).
    public func warmVoice() async -> Bool {
        lock.lock(); let cfg = configuration; lock.unlock()
        var why = ""
        return await synthesize("Ready.", with: cfg, why: &why, timed: false) != nil
    }
    /// Make the audio of one block for the cache: the chosen voice only (a fall back to the system voice would be cached under the wrong name).
    public func synthesizeForCache(_ text: String) async -> PreparedAudio? {
        lock.lock(); let cfg = configuration; lock.unlock()
        var why = ""
        guard let m = await synthesize(text, with: cfg, why: &why, timed: false) else { return nil }
        return PreparedAudio(text: text, samples: m.samples, rate: m.rate)
    }
    /// Per-document state dies with the document: the block index, where the loop ran dry (`resumeAt`), and the tracker's open and
    /// closed segments.
    public func setDocument(_ d: ProgrammeDocument) {
        playback.documentReplaced()
        st.write { $0.document = d; $0.blockIndex = 0; $0.resumeAt = nil }
    }

    // MARK: Transport

    public func play() {
        let (lvl, hasLoop, at) = st.write { s -> (PlayLevel, Bool, Int) in
            s.userPaused = false; s.systemPause = false
            // Ran dry, and more has come since: the first unplayed block, not the last played one again. With nothing new, play replays the last block as it always did.
            let first = s.resumeAt.flatMap { $0 < s.document.blocks.count ? $0 : nil }
            return (s.level, s.loop != nil, first ?? s.blockIndex)
        }
        onNeedsSession?()
        if lvl != .pause { audio.programmePlay() }
        playback.paused(false)
        if !hasLoop { start(at: at) }
        publishNowPlaying(); onChange?()
    }
    /// `reason` names who asked: every pause is on the console, never silent.
    public func pause(reason: String = "listener") {
        print("AUTORADIO paused reason=\(reason)")
        st.write { $0.userPaused = true; $0.systemPause = reason == "system interruption" }; audio.programmePause(); playback.paused(true); publishNowPlaying(); onChange?()
    }
    public func next() {
        let (i, doc) = st.read { ($0.blockIndex, $0.document) }
        let count = doc.blocks.count
        if let si = doc.segmentIndex(containingBlock: i) {
            // A programme: skip is to the next segment. On the last one it does nothing (no wrap), unless a text is waiting behind it.
            if si + 1 < doc.segments.count { start(at: doc.segments[si + 1].firstBlock) }
            else if let n = dequeue() { st.write { $0.userPaused = false }; setDocument(n); start(at: 0) }
            return
        }
        if i >= count - 1, let n = dequeue() { st.write { $0.userPaused = false }; setDocument(n); start(at: 0); return }
        start(at: min(i + 1, count - 1))
    }
    public func previous() {
        let (i, doc) = st.read { ($0.blockIndex, $0.document) }
        if let si = doc.segmentIndex(containingBlock: i) {
            // The start of this segment, or the one before when the segment has barely begun (the first block, under 3 seconds in).
            let here = doc.segments[si].firstBlock
            let seconds = renderPosition()?.seconds ?? 0
            start(at: StoryCursor.previousRestartsCurrent(blockIndex: i - here, secondsIntoBlock: seconds) || si == 0 ? here : doc.segments[si - 1].firstBlock)
            return
        }
        start(at: max(i - 1, 0))
    }
    /// How far into the story the listener is: the block and the seconds in it (0 when nothing has been scheduled yet or the player is paused at the start).
    public func storyPosition() -> (block: Int, seconds: Double) { (blockIndex, renderPosition()?.seconds ?? 0) }
    /// The segment now playing; nil for a single article.
    public var currentSegmentIndex: Int? { st.read { $0.document.segmentIndex(containingBlock: $0.blockIndex) } }
    /// Go to the first block of segment `i`. Outside the list: nothing.
    public func jumpToSegment(_ i: Int) {
        guard let at = st.read({ s in s.document.segments.indices.contains(i) ? s.document.segments[i].firstBlock : nil }) else { return }
        start(at: at)
    }

    /// Say no more than the block being spoken now: what follows it in this document is cut off. A block is whole sentences, so no word is cut.
    public func endAfterCurrentBlock() {
        st.write { s in
            let keep = s.blockIndex + 1
            guard keep < s.document.blocks.count else { return }
            s.document.blocks = Array(s.document.blocks.prefix(keep))
            s.document.segments = s.document.segments.compactMap { seg in
                guard seg.firstBlock < keep else { return nil }
                var x = seg; x.blockCount = min(x.blockCount, keep - x.firstBlock); x.unitServerBlocks = Array(x.unitServerBlocks.prefix(x.blockCount)); return x
            }
        }
        onChange?()
    }

    /// Add a segment to the end of the programme being played, if `documentID` is still the document it was made for (a late segment of an
    /// older stretch is refused). Playback is not disturbed; when the programme had run dry and the listener has not paused, it carries on
    /// with the new segment. False: refused, or the segment was already there.
    @discardableResult
    public func appendSegment(_ p: PreparedSegment, toDocument documentID: String) -> Bool {
        let (ok, resumeAt) = st.write { s -> (Bool, Int?) in
            guard s.document.id == documentID else { return (false, nil) }
            let old = s.document.blocks.count
            guard s.document.append(p) else { return (false, nil) }
            return (true, s.loop == nil && !s.userPaused ? old : nil)
        }
        if let at = resumeAt { start(at: at) }
        if ok { onChange?(); publishNowPlaying() }
        return ok
    }

    /// Start playing from block `i` now. The listener asked for it (a story chosen, next, previous), so a pause is over: the pause flag is
    /// cleared before the loop first reads it. (`start(at:)` then `play()` raced: the loop could pause the audio after `play()` had started it,
    /// and nothing started it again.)
    func begin(at i: Int) {
        st.write { $0.userPaused = false; $0.systemPause = false }
        playback.paused(false)
        start(at: i)
        publishNowPlaying(); onChange?()
    }

    private var startedAt = Date()
    private var firstTag = 0
    func start(at i: Int) {
        lock.lock(); startedAt = Date(); firstTag = i; lock.unlock()
        let (old, my) = st.write { s -> (Task<Void, Never>?, Int) in
            let o = s.loop; s.epoch += 1; s.blockIndex = i; s.loop = nil; s.resumeAt = nil; return (o, s.epoch)
        }
        old?.cancel()
        engineLostFlag.value = false              // a loss noted by the loop just cancelled belongs to nothing now
        audio.programmeStop()
        audio.clearPlayhead()
        let task = Task { [weak self] in
            guard let self else { return }
            var n = i
            var anyPlayed = false                // a block of this run made sound: only then did the story "finish"
            outer: while !Task.isCancelled {
                while !Task.isCancelled {
                    // One look at the state per block: the block to play, and whether this loop is still the current one.
                    let step = self.st.write { s -> (ProgrammeBlock, Bool, PlayLevel, ProgrammeDocument)? in
                        guard s.epoch == my, n < s.document.blocks.count else { return nil }
                        s.blockIndex = n
                        return (s.document.blocks[n], s.userPaused, s.level, s.document)
                    }
                    guard let (block, paused, lvl, doc) = step else { break }
                    self.audio.clearPlayhead(); self.onChange?(); self.publishNowPlaying()
                    if paused { self.audio.programmePause() } else if lvl != .pause { self.audio.programmePlay() }
                    self.playback.unitBegan(n, in: doc)
                    let played = await self.playBlock(block, index: n)
                    if Task.isCancelled { break }
                    if self.engineLostFlag.write({ let v = $0; $0 = false; return v }) {
                        guard self.st.read({ $0.epoch == my }) else { break outer }       // a newer start has replaced this loop: the loss is not its to report
                        let id = self.st.write { s -> String in if s.epoch == my { s.blockIndex = n; s.resumeAt = n; s.loop = nil }; return s.document.id }
                        print("AUTORADIO engine lost at doc=\(id) block=\(n); the entry is held, nothing counted as heard")
                        self.onChange?()
                        self.onEngineLost?(id, n)
                        break outer
                    }
                    if played { anyPlayed = true }
                    self.playback.unitEnded(n, played: played, in: self.st.read { $0.document })
                    n += 1
                }
                if Task.isCancelled { break }
                self.beforeLoopTail?()
                // The end, decided in one write with the look at the document: a segment appended since the last read is played, not lost.
                let tail = self.st.write { s -> Int in
                    guard s.epoch == my else { return 0 }
                    if n < s.document.blocks.count, !s.userPaused { return 1 }
                    s.blockIndex = min(n, max(s.document.blocks.count - 1, 0)); s.resumeAt = n; s.loop = nil; return 2
                }
                if tail == 1 { continue outer }
                // The story is over: what the listener asked to hear comes next, like the next story.
                if tail == 2 {
                    if let q = self.dequeue() { self.setDocument(q); self.start(at: 0) }
                    else {
                        let id = self.st.read { $0.document.id }
                        self.onChange?()
                        // The empty document of a stop ends at once: nothing ran dry, and nothing is told.
                        if id != "none" { print("AUTORADIO ran dry doc=\(id) played=\(anyPlayed)"); self.onRanDry?(id, anyPlayed) }
                    }
                }
                break
            }
        }
        // Keep it unless a newer start has already replaced this one.
        let orphan = st.write { s -> Bool in if s.epoch == my { s.loop = task; return false } else { return true } }
        if orphan { task.cancel() }
    }

    // MARK: Speed

    /// Programme speed, 0.5 to 3, pitch kept (time-pitch in the audio engine). Applies to what is playing and to what follows.
    public func setRate(_ r: Double) {
        let c = PlaybackSpeed.clamp(r)
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
        let paras = Array(ChoirText.paragraphs(text, maxChars: ChoirText.speechChars).prefix(600))
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
    /// Forget what was queued behind the programme (sign-out: the previous account's texts must not play).
    public func clearQueue() { lock.lock(); queue.removeAll(); lock.unlock() }
    public var queuedTexts: [String] { lock.lock(); defer { lock.unlock() }; return queue.map(\.id) }
    private func dequeue() -> ProgrammeDocument? { lock.lock(); defer { lock.unlock() }; return queue.isEmpty ? nil : queue.removeFirst() }

    func playBlock(_ b: ProgrammeBlock) async { _ = await playBlock(b, index: blockIndex) }

    /// `index` tags the audio so the position answers for this block even if the index moves on while it plays.
    /// True when the block's audio was played (to its end, or until a transport change stopped it); false when it made no sound
    /// (no voice, a voice that failed, a clip that is not there): that is not a finish.
    @discardableResult
    func playBlock(_ b: ProgrammeBlock, index tag: Int) async -> Bool {
        switch b {
        case .speech(let t), .listenerPassage(let t, _):
            // The block after this one is synthesised while this one plays (`lookahead`), so a long story starts after its first short
            // block and then runs without a gap; the voice itself falls back to the system voice line by line (`makeSpeaker`).
            lock.lock(); let current = configuration; lock.unlock()
            let key = Self.lookaheadKey(t, current)
            let hit = ahead.write { a -> Task<Synthesized?, Never>? in
                guard let x = a, x.key == key else { return nil }
                a = nil; return x.task
            }
            let made: Synthesized?
            var source = "live"
            // Audio written ahead of the launch (OpeningCache): the block's samples are handed over instead of made; the rest of the path is the same.
            if let p = preparedSpeech?(t) { made = Synthesized(samples: p.samples, rate: p.rate); source = "cached" }
            else if let hit { made = await hit.value } else { made = await synthesize(t) }
            if Task.isCancelled { return false }
            guard let made else {
                if tag == firstTag { print("AUTORADIO on air \(st.read { $0.document.id }) FAILED \(speakFailure ?? "no sound")") }
                return false
            }
            if tag == firstTag {
                let ms = Int((Date().timeIntervalSince(startedAt)) * 1000)
                print("AUTORADIO on air \(st.read { $0.document.id }) voice=\(current.setting(.voice)?.id ?? "") samples=\(made.samples.count) first_audio_ms=\(ms)")
                LaunchTimings.shared.printLine(source: source)
            } else {
                print("AUTORADIO on air \(st.read { $0.document.id }) block=\(tag) samples=\(made.samples.count)")
            }
            if let ended = lastBlockEnd.value {
                let gap = Int(Date().timeIntervalSince(ended) * 1000)
                if gap > 2000 { print("AUTORADIO on air \(st.read { $0.document.id }) block=\(tag) gap_ms=\(gap) since the previous block ended") }
            }
            log.emit(.programmeAudioStart(block: tag))      // the trace reads "desk audio start" from the first of these after a revision
            startLookahead(after: tag)
            let sounded = await audio.playProgramme(made.samples, rate: made.rate, tag: tag)
            if !sounded { if !Task.isCancelled { engineLostFlag.value = true }; return false }
            lastBlockEnd.write { $0 = Date() }
            return true
        case .podcastClip:
            return false                                     // a podcast clip is a run sheet entry, played by the run sheet player
        case .clip(let url, let start, let end, _):
            guard let u = FixtureResources.resolve(url), let all = try? AudioFiles.readMono(u, rate: 16_000) else {
                log.emit(.error(stage: "player", message: "clip not found: \(url)")); return false
            }
            let a = Int(start * 16_000), z = min(all.count, end.map { Int($0 * 16_000) } ?? all.count)
            if a < z {
                log.emit(.programmeAudioStart(block: tag))
                let sounded = await audio.playProgramme(Array(all[a..<z]), rate: 16_000, tag: tag)
                if !sounded { if !Task.isCancelled { engineLostFlag.value = true }; return false }
                return true
            }
            return false
        }
    }

    // MARK: Synthesis and lookahead

    struct Synthesized: Sendable { var samples: [Float]; var rate: Double }
    /// When the last spoken block's audio ended (nil before the first), for the gap line.
    private let lastBlockEnd = Locked<Date?>(nil)
    private let ahead = Locked<(key: String, task: Task<Synthesized?, Never>)?>(nil)

    static func lookaheadKey(_ text: String, _ cfg: LabConfiguration) -> String {
        "\(String(describing: cfg.setting(.voice)))|\(text)"
    }

    /// Make the audio of one block with the configured voice (the neural voice, the system voice as its fall back). nil when nothing could be
    /// made; the failure is on the log (the trace's "voice" errors).
    func synthesize(_ text: String) async -> Synthesized? {
        lock.lock(); let cfg = configuration; lock.unlock()
        let want = cfg.setting(.voice)?.id ?? ""
        var failure = ""
        if let made = await synthesize(text, with: cfg, why: &failure) { setFailure(nil); return made }
        if Task.isCancelled { return nil }
        // The chosen voice made nothing: the system voice speaks instead, and the listener is told why.
        if VoiceCatalog.neuralStageIDs.contains(want) {   // a chosen on-device voice; any other id that cannot be made is an error, not a guess
            log.emit(.note("voice: \(want) made no sound (\(failure)); the system voice speaks"))
            var system = cfg
            system.set(.voice, StageSetting(id: VoiceCatalog.fallbackStageID))
            var second = ""
            if let made = await synthesize(text, with: system, why: &second) { setFailure(nil); return made }
            if !second.isEmpty { failure += "; the system voice: \(second)" }
        }
        setFailure(failure)
        return nil
    }

    private func setFailure(_ f: String?) { lock.lock(); speakFailureText = f; lock.unlock() }
    private var speakFailureText: String?
    /// Why the last block could not be spoken (nil: it was).
    public var speakFailure: String? { lock.lock(); defer { lock.unlock() }; return speakFailureText }

    /// True while a block of speech is being made (the vector job waits on it).
    public var synthesisBusy: Bool { synthDepth.read { $0 > 0 } }
    private let synthDepth = Locked(0)
    private func synthesize(_ text: String, with cfg: LabConfiguration, why: inout String, timed: Bool = true) async -> Synthesized? {
        synthDepth.write { $0 += 1 }; defer { synthDepth.write { $0 -= 1 } }
        let voice: Voice
        let t0 = Date()
        var loaded = t0
        defer { if timed { LaunchTimings.shared.mark("synth", ms: Date().timeIntervalSince(loaded) * 1000) } }
        do { voice = try registry.makeSpeaker(cfg, log: log); loaded = Date(); if timed { LaunchTimings.shared.mark("voice_load", ms: loaded.timeIntervalSince(t0) * 1000) } } catch {
            why = "the voice \(cfg.setting(.voice)?.id ?? "") could not be made: \(error)"
            log.emit(.error(stage: "voice", message: why)); return nil
        }
        var samples: [Float] = []; var rate = 22050.0
        var failed = false
        do { for try await f in voice.speak(AsyncStream { $0.yield(text); $0.finish() }) { samples += f.samples; rate = f.sampleRate } } catch {
            why = "\(error)"; log.emit(.error(stage: "voice", message: "\(error)")); failed = true
        }
        if Task.isCancelled { return nil }
        if samples.isEmpty && !failed { why = "the voice made no sound for this block"; log.emit(.error(stage: "voice", message: why)) }
        if failed || samples.isEmpty { return nil }
        return Synthesized(samples: samples, rate: rate)
    }

    /// Start making the next speech block now, so it is ready when this one ends.
    private func startLookahead(after tag: Int) {
        let next = st.read { s -> String? in
            let i = tag + 1
            guard s.document.blocks.indices.contains(i) else { return nil }
            return s.document.blocks[i].spokenText
        }
        guard let text = next, !text.isEmpty else { return }
        lock.lock(); let cfg = configuration; lock.unlock()
        let key = Self.lookaheadKey(text, cfg)
        let task = Task { [weak self] in await self?.synthesize(text) }
        let old = ahead.write { a -> Task<Synthesized?, Never>? in let o = a; a = (key, task); return o?.task }
        old?.cancel()
    }
    private func cancelLookahead() {
        let old = ahead.write { a -> Task<Synthesized?, Never>? in let o = a; a = nil; return o?.task }
        old?.cancel()
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
    /// What of the document is still to be said, from the start of the block being spoken (a block is a sentence or a short run of them).
    public func remainingText() -> String? {
        st.read { s -> String? in
            guard s.document.blocks.indices.contains(s.blockIndex) else { return nil }
            let t = s.document.blocks[s.blockIndex...].compactMap { $0.spokenText }.joined(separator: " ")
            return t.isEmpty ? nil : t
        }
    }
    public func setLevel(_ l: PlayLevel) async {
        let paused = st.write { s -> Bool in s.level = l; return s.userPaused }
        if l == .pause { cancelLookahead() }          // the listener spoke: the voice is wanted for the answer, not for the next block
        playback.cut(l == .pause)
        switch l {
        case .play: audio.setProgrammeVolume(1); if !paused { audio.programmePlay() }
        case .dip: audio.setProgrammeVolume(dipVolume)
        case .pause: audio.programmePause()
        }
        onChange?()
        onLevel?(l)
    }
    public func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async { await audio.playVoice(frames) }
    public func stopVoice() async { audio.stopVoice() }
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
        for cmd in [c.playCommand, c.pauseCommand, c.togglePlayPauseCommand, c.nextTrackCommand, c.previousTrackCommand] { cmd.isEnabled = true }
        c.playCommand.addTarget { [weak self] _ in if self?.onTransport?("play") == true { return .success }; self?.play(); return .success }
        c.pauseCommand.addTarget { [weak self] _ in
            let since = self?.msSinceSessionChange?() ?? -1
            if self?.ignoreSystemPause?() == true { print("AUTORADIO remote pause ignored ms_since_session_change=\(since)"); return .success }
            print("AUTORADIO remote pause accepted ms_since_session_change=\(since)")
            if self?.onTransport?("pause") == true { return .success }; self?.pause(reason: "remote command pause"); return .success }
        c.togglePlayPauseCommand.addTarget { [weak self] _ in
            guard let self else { return .commandFailed }
            if self.onTransport?("toggle") == true { return .success }
            if self.userPaused { self.play() } else { self.pause(reason: "remote command toggle") }; return .success
        }
        c.nextTrackCommand.addTarget { [weak self] _ in
            guard let self else { return .commandFailed }
            if self.onNextTrack?() != true { self.next() }
            return .success
        }
        c.previousTrackCommand.addTarget { [weak self] _ in
            guard let self else { return .commandFailed }
            if self.onPreviousTrack?() != true { self.previous() }
            return .success
        }
    }
    func publishNowPlaying() {
        let (doc, idx, paused, speed) = st.read { ($0.document, $0.blockIndex, $0.userPaused, $0.rate) }
        let np = ProgrammeNowPlaying.describe(document: doc, blockIndex: idx)
        MPNowPlayingInfoCenter.default().nowPlayingInfo = [
            MPMediaItemPropertyTitle: np.title, MPMediaItemPropertyArtist: np.artist,
            MPNowPlayingInfoPropertyPlaybackRate: paused ? 0.0 : speed,
        ]
        MPNowPlayingInfoCenter.default().playbackState = paused ? .paused : .playing
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

/// What the log and the first-run screen say about the microphone (pure, so a test can hold them to the truth).
public enum MicLog {
    /// "ok" only when the tap runs and has a real input rate.
    public static func tap(active: Bool, rate: Int) -> String { active && rate > 0 ? "ok" : "fail" }
    /// The "voice on" line: the tap word comes from the tap and the rate, never from anything else.
    public static func voiceOnLine(claim: String, rate: Int, active: Bool, state: LiveAudio.MicState) -> String {
        "voice on claim=\(claim) rate=\(rate) tap=\(tap(active: active, rate: rate)) mic=\(state)"
    }
    /// The error line for a microphone that is wanted and not open, only when that is FINAL (`failed`: the retries are used up; `refused`: not allowed). While `pending` a retry may still open it: no error (nil).
    public static func finalError(_ s: LiveAudio.MicState, rate: Int) -> String? {
        switch s {
        case .failed: return "the microphone could not be opened (input rate \(rate), failed after the retries)"
        case .refused: return "the microphone could not be opened (it is not allowed)"
        default: return nil
        }
    }
    /// What onboarding's microphone step must show when the microphone cannot be opened (nil: keep listening).
    public static func onboardingState(_ s: LiveAudio.MicState) -> String? {
        switch s { case .refused: return "refused"; case .failed: return "silent"; default: return nil }
    }
}

/// When the microphone permission is asked for: only with voice on, and only when it is not granted yet. Voice is off unless the listener
/// turns it on, and with it off the radio never uses the microphone, so it does not ask.
public enum MicAsk {
    public static func shouldRequest(voiceOn: Bool, granted: Bool) -> Bool { voiceOn && !granted }
}
