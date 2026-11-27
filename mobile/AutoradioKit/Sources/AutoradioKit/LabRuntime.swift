import Foundation
#if canImport(Darwin)
import Darwin
#endif

/// Resident memory of this process in bytes.
public func residentBytes() -> Int64 {
    var info = mach_task_basic_info()
    var count = mach_msg_type_number_t(MemoryLayout<mach_task_basic_info>.size / MemoryLayout<natural_t>.size)
    let r = withUnsafeMutablePointer(to: &info) {
        $0.withMemoryRebound(to: integer_t.self, capacity: Int(count)) { task_info(mach_task_self_, task_flavor_t(MACH_TASK_BASIC_INFO), $0, &count) }
    }
    return r == KERN_SUCCESS ? Int64(info.resident_size) : 0
}

/// Saved configurations as JSON files in a directory.
public final class ConfigStore: @unchecked Sendable {
    public let dir: URL
    public init(dir: URL) { self.dir = dir; try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true) }
    public func names() -> [String] {
        ((try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? []).filter { $0.hasSuffix(".json") }.map { String($0.dropLast(5)) }.sorted()
    }
    func url(_ name: String) -> URL { dir.appendingPathComponent(name.replacingOccurrences(of: "/", with: "_") + ".json") }
    public func save(_ c: LabConfiguration) throws { try c.write(to: url(c.name)) }
    public func load(_ name: String) throws -> LabConfiguration { try LabConfiguration.load(from: url(name)) }
}

/// Plain value types the host (Exact module, tests) can turn into JSON.
public struct LabSnapshot: Codable, Sendable {
    public var running = false, paused = true
    public var phase = "playing", level = "play"
    public var addressing = "", transcript = "", configName = ""
    public var blockIndex = 0, blockCount = 0
    public var blockText = ""
    public var residentMB = 0.0
    public var thermal = "nominal"
    public var message = ""
    public var micActive = false
}
public struct TurnRow: Codable, Sendable { public var id, transcript, configName, outcome: String; public var ackMs, revisionMs: Double }

/// Everything a host needs: the registry, the audio, the engine, kept turns, configurations.
/// A host (the Exact module) creates one, then calls `start`, `setStage`, `replay` ...
public final class LabRuntime: @unchecked Sendable {
    public let registry: StageRegistry
    let readiness: VoiceReadiness
    public let clock = LabClock()
    public let log: EventLog
    public let bus = AudioBus()
    public let audio = LiveAudio()
    public let player: ProgrammePlayer
    public let engine: TurnEngine
    public let recorder: TurnRecorder
    public let store: KeptTurnStore
    public let configs: ConfigStore
    /// The voice choice that survives a relaunch.
    public let voiceStore: VoiceChoiceStore
    public let session: AudioSessionOwner
    public let coordinator: SessionCoordinator
    public var onChange: (@Sendable () -> Void)?

    private let lock = NSLock()
    private var config: LabConfiguration
    private var snap = LabSnapshot()
    private var records: [String: TurnRecord] = [:]
    private var lastID: String?
    private var micTask: Task<Void, Never>?
    private var eventTask: Task<Void, Never>?
    private var started = false
    private let policyBox: PolicyBox

    /// `emptyProgramme`: the radio starts with nothing loaded (the phone and the Mac): the lab's fixture edition is for tests and the lab, and
    /// must never be spoken on a listener's device. Everything that plays is a run sheet entry.
    public init(supportDir: URL, session: AudioSessionOwner, registry: StageRegistry = .builtIn(), emptyProgramme: Bool = false, readiness: VoiceReadiness = Voices.readiness) {
        self.registry = registry; self.readiness = readiness
        let log = EventLog(clock: clock)
        self.log = log
        let voiceStore = VoiceChoiceStore(file: supportDir.appendingPathComponent("voice-choice.json"))
        self.voiceStore = voiceStore
        // The saved voice, or on a fresh install the bundled default. The model stages register after this runs, so the stage is not checked
        // here: `makeSpeaker` falls back to bundled Tiny Voca for a stage that is not registered.
        let cfg = StageRegistry.listenerConfiguration(voice: voiceStore.current(), registry: registry, readiness: readiness)
        self.config = cfg
        let policyBox = PolicyBox(cfg.audio)
        self.policyBox = policyBox
        let override = Locked<Bool?>(nil)
        self.overrideBox = override
        // Voice off is a playback session with no microphone; only the tap on the Voice control (or onboarding's own screen) opens it.
        let coord = SessionCoordinator(inner: session, log: log, audio: audio, policy: { policyBox.value },
                                       voiceOn: { override.value ?? HandsFreeSwitch.shared.isOn })
        self.coordinator = coord
        self.session = coord
        store = KeptTurnStore(root: supportDir.appendingPathComponent("turns"))
        configs = ConfigStore(dir: supportDir.appendingPathComponent("configs"))
        store.prune()                                  // the rule for what stays: the newest 20 turns, none older than 30 days, no sound (KeptTurnStore.prune)
        // The bundled kept turn is a development replay fixture (no product page reads it; the voice setup check reads FixtureTurn.micSamples16k() from the binary, not this folder):
        // it is written only where EXACT_DEVELOPMENT is defined (package Swift of a non-production build; the store archive has none).
        #if EXACT_DEVELOPMENT
        FixtureTurn.install(into: store)
        #endif
        let doc = emptyProgramme ? ProgrammeDocument(id: "none", title: "", blocks: []) : FixtureProgramme.document
        player = ProgrammePlayer(audio: audio, registry: registry, log: log, configuration: cfg, document: doc)
        recorder = TurnRecorder(bus: bus)
        let storeRef = store
        var envBox: LabRuntime?
        let env = EngineEnvironment(registry: registry, log: log, playback: player, session: self.session,
            onRecord: { rec in envBox?.keep(rec) },
            onOutcome: { id, o in storeRef.updateOutcome(id: id, o); envBox?.setOutcome(id, o) },
            frontPage: { envBox?.aimedPage() },
            pageText: { address in await envBox?.pageTextProvider?(address) ?? "" })
        engine = TurnEngine(env: env, configuration: cfg)
        envBox = self
        snap.configName = cfg.name
        // Model loads show in the trace; the one store is shared with voice setup.
        Task { await ModelStore.shared.attach(log: log) }
        // What the vector job waits for (EmbedDevice.state, read by shared code): speech on air, a voice line, or speech being made. Speech always wins the processor.
        RadioDemand.shared.add { [audio, player] in audio.voiceIsSpeaking || audio.programmeAudible || player.synthesisBusy }
        player.onChange = { [weak self] in self?.refresh() }
        // Play asks for the session first when the engine is down or the claim is doubted (after the background, an interruption).
        player.ignoreSystemPause = { [coordinator] in coordinator.changedWithin(2.5) }
        player.msSinceSessionChange = { [coordinator] in coordinator.msSinceSessionChange }
        player.onNeedsSession = { [coordinator] in Task { await coordinator.ensureReady(reason: "play") } }
        player.onPaused = { [coordinator] paused in Task { await coordinator.setPaused(paused) } }
        // The voice switch turned itself off (three quiet minutes) or was set elsewhere: the session drops the microphone.
        HandsFreeSwitch.shared.onVoiceChange = { [coordinator, audio] in
            Task {
                // Turning voice on is the moment to ask for the microphone (never before); a refusal leaves voice without a microphone.
                if !audio.virtual, MicAsk.shouldRequest(voiceOn: HandsFreeSwitch.shared.isOn, granted: LiveAudio.micPermissionGranted()) {
                    audio.permitMic(await LiveAudio.requestMicPermission())
                } else if HandsFreeSwitch.shared.isOn, LiveAudio.micPermissionGranted() { audio.permitMic(true) }
                await coordinator.reapply()
            }
        }
        // The bar and the player read the real position from the audio, and a speed picked there reaches the audio.
        let pl = player
        BarTracker.shared.positionSource = { [weak pl] in pl?.renderPosition() }
        BarTracker.shared.speedSink = { [weak pl] v in pl?.setRate(v) }
        coord.onChange = { [weak self] in self?.refresh() }
        eventTask = Task { [weak self] in
            guard let self else { return }
            for await te in self.log.stream() { self.absorb(te) }
        }
        NotificationCenter.default.addObserver(forName: ProcessInfo.thermalStateDidChangeNotification, object: nil, queue: nil) { [weak self] _ in
            guard let self else { return }
            self.log.emit(.thermal(Self.thermalName()))
            self.refresh()
        }
    }

    static func thermalName() -> String {
        switch ProcessInfo.processInfo.thermalState { case .nominal: return "nominal"; case .fair: return "fair"; case .serious: return "serious"; case .critical: return "critical"; @unknown default: return "unknown" }
    }

    // MARK: Lifecycle

    /// `virtual`: no audio hardware (the Exact agent, tests): playback waits instead of playing, mic frames are injected.
    public func start(virtual: Bool = false) async {
        guard !started else { return }
        // Load the small language model if its file is already on the phone (never downloads); the acknowledgement needs it loaded.
        Task.detached { [weak self] in
            await LocalLanguage.model?.warm()
            self?.warmResponder(address: nil)          // the page in front, once the model is loaded
        }
        if virtual { LabEnvironment.promptsAllowed = false; LabEnvironment.virtualAudio = true }
        do {
            // The coordinator decides what the claim is: a playback session while voice is off, voice processing from the start while it is on
            // (the radio's voice is not the listener's).
            try await session.claim(.talking, preferBuiltInMic: true, reason: "start")
            // Voice off is the default and never touches the microphone, so it never asks for it (owner: "it's off unless you turn it on";
            // the Mac's prompt also waits for a person, which hung Play). Voice on asks, here or when the listener turns it on (below).
            let voiceIsOn = coordinator.current != .playing
            let micOK = virtual ? true : (MicAsk.shouldRequest(voiceOn: voiceIsOn, granted: LiveAudio.micPermissionGranted()) ? await LiveAudio.requestMicPermission() : LiveAudio.micPermissionGranted())
            if !micOK { log.emit(.error(stage: "mic", message: "microphone permission denied; button/typing only")) }
            try audio.start(bus: bus, clock: clock, wantMic: micOK, mic: micOK && coordinator.current != .playing,
                            voiceProcessing: coordinator.current == .talking, virtual: virtual)
            if virtual { log.emit(.note("virtual audio: no hardware is opened; mic frames are injected")) }
            started = true
            LaunchTimings.shared.mark("engine"); LaunchTimings.shared.mark("session")
            let mic = bus.mic()
            let echo = audio.voiceProcessing
            let on = handsFreeOverride ?? HandsFreeSwitch.shared.isOn
            micTask = Task { [engine] in
                await engine.handsFreeConfigure(echoCancelled: echo)
                await engine.handsFreeSet(on)
                for await f in mic { await engine.ingestHandsFree(f) }      // hands-free: no button, the policy decides (HandsFree.swift); off: the engine looks at nothing
            }
            player.play()
            lock.lock(); snap.running = true; snap.message = ""; lock.unlock()
        } catch {
            let ns = error as NSError
            log.emit(.error(stage: "start", message: "\(ns.domain) \(ns.code): \(ns.localizedDescription)"))
            lock.lock(); snap.message = "start failed: \(error)"; lock.unlock()
        }
        refresh()
    }

    /// Set by the host: how to start the engine when text is to be spoken and it is not running (the host knows whether the
    /// engine is virtual). Without it the text becomes the loaded programme and plays when the radio is started.
    public var startRequest: (@Sendable () async -> Void)?

    /// Speak arbitrary text as a programme item (mail's "Read it to me"): it queues like a story, shows in now-playing, and the
    /// listener can cut into it like any speech. `now`: the listener pressed Listen on what is in front of them, so it replaces what
    /// is playing instead of waiting behind it. Returns false for text with nothing to say.
    @discardableResult
    public func speakText(address: String, text: String, title: String? = nil, now: Bool = false) async -> Bool {
        if !snapshot().running, let go = startRequest { await go() }
        let running = snapshot().running
        // Not running: the text is the loaded programme, played when the radio starts (the engine is not there to play it).
        return player.speakText(address: address, text: text, title: title, startNow: !running || now, play: running)
    }

    /// The session is claimed and active and the engine is up, taking the claim again when needed. Call before anything plays.
    @discardableResult
    public func ensureSessionReady(reason: String) async -> Bool {
        let ok = await coordinator.ensureReady(reason: reason)
        if !ok { print("AUTORADIO session could not be taken (\(reason)) \(await coordinator.failureDiagnostic())") }
        return ok
    }

    public func stop() {
        player.pause(reason: "runtime stop")
        micTask?.cancel(); micTask = nil
        audio.stop(); coordinator.release(); started = false
        lock.lock(); snap.running = false; lock.unlock()
        refresh()
    }

    /// Set by the host: the page for an address (nil: the page in front). The acknowledgement names its title and kind.
    public var frontPageResolver: (@Sendable (String?) -> FrontPage?)?
    /// Set by the host: the text of the page at an address, as far as the phone has it (the local stream may say something from it).
    public var pageTextProvider: (@Sendable (String) async -> String)?
    private var warmTask: Task<Void, Never>?
    private let warmKeys = Locked<(done: String, running: String)>(("", ""))
    /// Hands-free for one screen only (onboarding's "Talk to it" while the switch is off): not stored.
    private let overrideBox: Locked<Bool?>
    var handsFreeOverride: Bool? { overrideBox.value }
    static func warmKey(_ address: String, _ text: String) -> String { "\(address)|\(text.utf8.count)|\(text.hashValue)" }
    /// A page's text has arrived (or changed) while the page may already be in front: read it into the responder's cache unless that
    /// exact page and text has been read or is being read now.
    public func pageTextReady(address: String, text: String) {
        guard let front = frontPageResolver?(nil), front.address == address else { return }
        let key = Self.warmKey(address, text)
        let skip = warmKeys.read { $0.done == key || $0.running == key }
        if !skip { warmResponder(address: address) }
    }
    /// Read the page that is (or is about to be) in front into the responder's cache, in the background, so that at end of speech only
    /// the listener's words remain to be read. Cancels the reading of the previous page; what was read of it stays in the model's cache
    /// only as far as it is a prefix of the next prompt. No timer: it runs as long as the reading takes, and a turn preempts it.
    public func warmResponder(address: String?) {
        warmTask?.cancel()
        warmTask = Task.detached { [weak self] in
            guard let self, let model = LocalLanguage.model, model.isLoaded else { return }
            let front = self.frontPageResolver?(address)
            guard let front else { return }
            let text = await self.pageTextProvider?(front.address) ?? ""
            if Task.isCancelled { return }
            let prefix = AckPrompt.userPrefix(pageTitle: front.title, pageKind: front.kind, pageText: text)
            guard !prefix.isEmpty else { return }
            let key = Self.warmKey(front.address, text)
            self.warmKeys.write { $0.running = key }
            await model.prefill(system: AckPrompt.system(), userPrefix: prefix)
            if let r = model.lastPrefill() {
                if r.promptTokens >= r.prefixTokens { self.warmKeys.write { $0.done = key; $0.running = "" } } else { self.warmKeys.write { $0.running = "" } }
                self.log.emit(.localWarm(address: front.address, tokens: r.prefixTokens, ms: r.prefillMs, complete: r.promptTokens >= r.prefixTokens))
            }
        }
    }
    private let aimBox = Locked<String>("")
    /// The address of the page the listener aimed at (the bar's `aim`), kept for the spoken turn that follows a talk press.
    /// Empty: the page in front now.
    public var aimAddress: String { get { aimBox.value } set { aimBox.value = newValue } }
    func aimedPage() -> FrontPage? {
        let a = aimAddress
        return frontPageResolver?(a.isEmpty ? nil : a)
    }

    /// A typed question from the bar: starts the engine when it is not running (the host's `startRequest`), then runs one turn.
    /// `pageAddress` is the address of the page the bar was showing when the listener asked (empty: the page in front now).
    @discardableResult
    public func ask(_ text: String, pageAddress: String = "") async -> Bool {
        // A question that cannot be asked is kept, with the reason, so it is not lost with the cleared field: the card shows it with a retry.
        func refused(_ why: String) -> Bool {
            let id = "ask-" + UUID().uuidString.prefix(8).lowercased()
            Conversation.shared.begin(id: id, question: text)
            Conversation.shared.fail(id: id, note: why)
            return false
        }
        if !snapshot().running, let go = startRequest { await go() }
        guard snapshot().running else { return refused("The radio could not start, so it could not hear that. Try again.") }
        let front = pageAddress.isEmpty ? nil : frontPageResolver?(pageAddress)
        let started = await engine.typedTurn(text, front: front)
        if !started, !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { return refused(TurnGate.shared.allowed ? "The radio was busy with another question. Try again." : "Sign in first, then ask.") }
        return started
    }

    /// Hands-free on or off (one state, off at every launch): on, the microphone is open and the turn policy opens and closes turns; off,
    /// the engine hears nothing and an open turn is dropped now. `persist: false` is for one screen only (onboarding's "Talk to it").
    public func setHandsFree(_ on: Bool, persist: Bool = true) {
        if on, !TurnGate.shared.allowed { return }          // no key (signed out), or a screen holds turns back: the radio only listens, the microphone does not open
        if persist { overrideBox.write { $0 = nil }; HandsFreeSwitch.shared.set(on) } else { overrideBox.write { $0 = on } }
        // The session follows the switch first (mic and voice processing only while on); then the engine is told whether the radio's own voice is
        // cancelled from the microphone, which is what lets the listener's speech pause the programme at once.
        Task { [engine, coordinator, audio] in
            await coordinator.reapply()
            await engine.handsFreeConfigure(echoCancelled: audio.voiceProcessing)
            await engine.handsFreeSet(on)
            if on {
                print("AUTORADIO " + MicLog.voiceOnLine(claim: coordinator.describe(), rate: Int(audio.micTapRate), active: audio.micActive, state: audio.micState))
                if !audio.micActive, !audio.virtual { await reportMicWhenSettled() }
            }
        }
        refresh()
    }
    /// Voice was turned on and the microphone is not open: look again after the route settles, and say it is an error only when that is FINAL (failed after its retries, or not allowed); while a retry is pending the microphone may still open.
    /// The delays and the state read are seams for the test of this decision (HandsFreeToggleTests): the call site is what must not say an error for a pending retry.
    var micSettleNanos: UInt64 = 3_000_000_000, micPollNanos: UInt64 = 1_000_000_000
    var micStateNow: (() -> LiveAudio.MicState)?
    func reportMicWhenSettled() async {
        let state: () -> LiveAudio.MicState = { self.micStateNow?() ?? self.audio.micState }
        try? await Task.sleep(nanoseconds: micSettleNanos)
        let rate = Int(audio.micTapRate)
        print("AUTORADIO voice mic after the route settled rate=\(rate) tap=\(MicLog.tap(active: audio.micActive, rate: rate)) mic=\(state())")
        var waited = 0
        while state() == .pending, handsFreeIsOn, waited < 40 { if waited == 0 { print("AUTORADIO voice mic still trying") }; try? await Task.sleep(nanoseconds: micPollNanos); waited += 1 }
        if handsFreeIsOn, let say = MicLog.finalError(state(), rate: Int(audio.micTapRate)) { log.emit(.error(stage: "listen", message: say)) }
    }
    /// Sign-out, before the key goes (the gate is already shut: ChoirApp.signOut's first statement). The switch off and the engine's turn and listening ended, all synchronously; then the session moved off the microphone, bounded:
    /// a move that fails or does not answer in `seconds` is not waited for, and the microphone is closed the direct way (`stop`: the engine's input and the claim go) before the wait for shared code begins.
    public func closeVoiceForSignOut(seconds: Double = 2) async {
        overrideBox.write { $0 = nil }
        await MainActor.run { TakeSession.live?.discard() }      // a take in hand holds the microphone open and is the old account's (its give-back cannot turn hands-free on: HandsFreeSwitch.set refuses)
        HandsFreeSwitch.shared.set(false)                        // its own session move is a Task nobody waits for; the awaited one is below
        let ended = Locked<Bool>(false)
        let moved: Bool = await withCheckedContinuation { c in
            let once: @Sendable (Bool) -> Void = { ok in if ended.write({ let first = !$0; $0 = true; return first }) { c.resume(returning: ok) } }      // the first answer wins
            Task { [engine, coordinator] in await engine.endForSignOut(); once(await coordinator.reapply()) }
            Task { try? await Task.sleep(nanoseconds: UInt64(seconds * 1e9)); once(false) }
        }
        if !moved { stop() }
        refresh()
    }
    /// Sign-out's key step: it deletes the key it began with and nothing else (a new sign-in that landed meanwhile is kept, and so are its stores), then the gate's own sequence is over.
    public func endSignOut(deleteKey: () async -> Void) async {
        defer { TurnGate.shared.finishSignOut() }
        if TurnGate.shared.keyReplaced { log.emit(.note("sign-out: a new sign-in replaced the key meanwhile; it is kept")); return }
        await deleteKey()
        if TurnGate.shared.signedOutShut { log.emit(.note("sign-out: the key could not be deleted; turns stay refused until sign-out is tried again")) }
    }
    /// The stored state again, after a screen's own turn-on.
    /// Voice enrolment listens through the engine's microphone without the engine looking for turns (TurnEngine.enrolmentTap).
    public func setEnrolmentTap(_ tap: (@Sendable (AudioFrame) -> Void)?) async { await engine.setEnrolmentTap(tap) }
    public func restoreHandsFree() { setHandsFree(HandsFreeSwitch.shared.isOn) }
    public var handsFreeIsOn: Bool { handsFreeOverride ?? HandsFreeSwitch.shared.isOn }
    /// Discard the turn being talked (Cancel beside Send): stop the capture, drop the audio and words, call nothing.
    public func cancelTalk() { Task { await engine.cancelTurn() } }

    // MARK: Configuration

    private let engineHops = Locked<Task<Void, Never>?>(nil)
    /// Every configuration handed to the engine so far has been applied (tests).
    func settled() async { await engineHops.value?.value }
    public var configuration: LabConfiguration { lock.lock(); defer { lock.unlock() }; return config }
    public func setConfiguration(_ conf: LabConfiguration) {
        var c = conf
        // A configuration saved before the stock-phrase holder was removed names it; it now means the local model's holder.
        if c.setting(.holder)?.id == "fixed-lines" { c.set(.holder, StageSetting(id: LocalAckHolder.stageID)) }
        // The local holder now answers first from the saved stories (and says what it looks up when they hold nothing).
        if c.setting(.holder)?.id == LocalAckHolder.stageID { c.set(.holder, StageSetting(id: SavedAnswerHolder.stageID)) }
        lock.lock(); config = c; snap.configName = c.name; lock.unlock()
        let policyChanged = policyBox.value != c.audio
        policyBox.value = c.audio
        if policyChanged { Task { [coordinator] in await coordinator.reapply() } }
        player.setConfiguration(c)
        // Each hop waits for the one before it: tasks are not ordered, and the engine must end on the last configuration set.
        let applied = c
        engineHops.write { prev in let before = prev; prev = Task { [engine] in await before?.value; await engine.setConfiguration(applied) } }
        refresh()
    }
    public func setAudioPolicy(_ p: AudioPolicy) {
        var c = configuration
        c.audioPolicy = p
        setConfiguration(c)
    }
    /// After voice setup (the models just became ready): speech detection and recognition are chosen from the list again and applied together, with
    /// the composite's parameters, to the running app. Without this the listener built at launch stays until the next launch (review B1).
    public func applyListenerChoice() {
        let heard = ListenerPipeline.choose(registry: registry, readiness: readiness)
        var c = configuration
        c.set(.speechBounds, heard.bounds); c.set(.transcriber, heard.transcriber)
        setConfiguration(c)
        log.emit(.note(heard.summary))
    }
    /// A first-run setup finished: its choices become the running ones (the stages it set up, the listener from the list, the answering and decision models it fetched when none was ever chosen). A ROW's download (`rowDownload`)
    /// applies NOTHING: Download only fetches and verifies; only `useModel` changes what the radio uses (review 2026-11-24).
    public func applySetupChoices(_ s: VoiceSetup) {
        guard !s.rowDownload else { return }
        for (kind, id) in s.choices where kind != .speechBounds && kind != .transcriber && registry.stages(kind).contains(where: { $0.id == id }) { setStage(kind, id: id) }
        applyListenerChoice()
        if let r = s.record.items.first(where: { $0.stage == SetupStage.responder.rawValue }) { ResponderChoice.adopt(r.chosen) }
        if let d = s.record.items.first(where: { $0.stage == SetupStage.decision.rawValue }) { DecisionChoice.adopt(d.chosen) }
    }

    /// The person's Use on a model's row: it becomes the model the radio uses for that job, and the stage that runs it is set. A model that is not on the phone changes nothing (false).
    @discardableResult
    public func useModel(job: String, id: String, live: Bool, readiness: VoiceReadiness = Voices.readiness, defaults: UserDefaults = .standard) -> Bool {
        guard readiness.isReady(model: id) else { return false }
        switch job {
        case "responder":
            guard KnownModels.isPinned(id) else { return false }
            defaults.set(id, forKey: ResponderChoice.key)
            setStage(.holder, id: "local-ack")
        case "decision":
            guard DecisionChoice.resolve(saved: id) == id else { return false }
            defaults.set(id, forKey: DecisionChoice.key)
            setStage(.system1, id: "llama-qwen3-0.6b")
        case "voice":
            guard let o = VoiceCatalog.options.first(where: { $0.modelID == id }) else { return false }
            setVoice(o)
        case "transcriber": ModelUse.standard.set(live ? "live" : "final", id); applyListenerChoice()
        case "bounds": ModelUse.standard.set("bounds", id); applyListenerChoice()
        default: return false
        }
        return true
    }

    public func setStage(_ kind: StageKind, id: String) {
        var c = configuration
        c.set(kind, StageSetting(id: id))
        setConfiguration(c)
        if kind == .voice, let v = c.setting(.voice) { voiceStore.save(v) }       // a voice picked anywhere is the voice from now on
    }
    /// The listener picked a voice (the cards, setup): it speaks everything from now on and after a relaunch.
    public func setVoice(_ o: VoiceOption) {
        var c = configuration
        c.set(.voice, o.setting)
        setConfiguration(c)
        voiceStore.save(o.setting)
    }
    // MARK: Turns

    func keep(_ rec: TurnRecord) {
        let t0 = (rec.events.first?.t ?? 0) - 1.0
        let t1 = (rec.events.last?.t ?? 0) + 0.5
        let snapAudio = recorder.snapshot(from: t0, to: t1)
        lock.lock(); records[rec.id] = rec; lastID = rec.id; lock.unlock()
        let pos = ProgramPosition(programmeID: player.document.id, blockIndex: player.blockIndex, context: "")
        let ctx = TurnContextFile(position: pos, configuration: rec.configuration, retrieval: RetrievalBridge.shared.lastRecord)
        _ = try? store.save(record: rec, mic: snapAudio.mic, reference: snapAudio.reference, context: ctx, keepAudio: false)       // the words, not the sound (KeptTurnStore.prune states the rule)
        store.prune()
        refresh()
    }
    func setOutcome(_ id: String, _ o: Outcome) { lock.lock(); records[id]?.outcome = o; lock.unlock(); refresh() }

    public func turns() -> [TurnRow] {
        store.list().compactMap { k in
            guard let r = try? k.record() else { return nil }
            return TurnRow(id: k.name, transcript: r.summary.transcript ?? "", configName: r.configuration.name, outcome: r.outcome.rawValue,
                           ackMs: r.summary.turnEndToAckMs ?? 0, revisionMs: r.summary.turnEndToRevisionMs ?? 0)
        }
    }
    public func record(id: String?) -> TurnRecord? {
        if let id, let r = try? KeptTurn(folder: store.root.appendingPathComponent(id)).record() { return r }
        lock.lock(); defer { lock.unlock() }
        if let id, let r = records[id] { return r }
        if let l = lastID { return records[l] }
        return store.list().first.flatMap { try? $0.record() }
    }
    // MARK: Views for hosts

    public func snapshot() -> LabSnapshot {
        lock.lock(); var s = snap; lock.unlock()
        s.residentMB = Double(residentBytes()) / 1_048_576
        s.thermal = Self.thermalName()
        s.paused = player.userPaused
        s.level = player.level.rawValue
        s.blockIndex = player.blockIndex
        s.blockCount = player.document.blocks.count
        s.blockText = player.document.blocks.indices.contains(player.blockIndex) ? player.document.blocks[player.blockIndex].summary : ""
        s.addressing = configuration.setting(.addressing)?.id ?? ""
        s.micActive = audio.micActive
        return s
    }
    // MARK: Event absorption

    func absorb(_ te: TimedEvent) {
        lock.lock()
        switch te.event {
        case .phase(_, let to): snap.phase = to; if to == "candidate" { snap.transcript = "" }
        case .partialTranscript(let t), .finalTranscript(let t): snap.transcript = t
        case .error(_, let m): snap.message = m
        default: break
        }
        lock.unlock()
        onChange?()
    }
    func refresh() { onChange?() }

    public static func describe(_ e: LabEvent) -> String {
        switch e {
        case .phase(let a, let b): return "phase \(a) -> \(b)"
        case .speechStarted: return "speech started"
        case .speechEnded(let s): return String(format: "speech ended (%.1f s)", s)
        case .addressed(let b): return "addressed by \(b)"
        case .notATurn(let r): return "not a turn: \(r)"
        case .partialTranscript(let t): return "partial: \(t)"
        case .finalTranscript(let t): return "final: \(t)"
        case .turnEnded: return "turn ended"
        case .turnCancelled: return "turn cancelled by the listener"
        case .system1Answer(let q, let o, let s, let ms): return String(format: "system1 %@ = %@ (%.2f, %.0f ms)", q, o, s, ms)
        case .ackStarted(let ms): return String(format: "acknowledgement audio %.0f ms after turn end", ms)
        case .holdingSaid(let l): return "holding: \(l)"
        case .localAckReady(let t, let ms): return String(format: "local reply ready %.0f ms after turn end: ", ms) + t
        case .localAckSkipped(let r, let ms): return String(format: "no local reply (%.0f ms after turn end): ", ms) + r
        case .localSentenceStarted(let t, let ms): return String(format: "local sentence heard %.0f ms after turn end: ", ms) + t
        case .localCut(let n, let r, let ms): return String(format: "local stream cut after %d sentences, %.0f ms after turn end: ", n, ms) + r
        case .localPrefill(let n, let pre, let re, let ms): return String(format: "local prompt %d tokens (stable prefix %d, already cached %d), read in %.0f ms: ", n, pre, re, ms) + (pre > 0 && re >= pre ? "WARM cache" : "cold cache")
        case .localWarm(let a, let n, let ms, let done): return String(format: "page read into the responder ahead of time: %@, %d tokens, %.0f ms", a, n, ms) + (done ? "" : " (preempted)")
        case .deskSentenceStarted(let t, let ms): return String(format: "desk sentence heard %.0f ms after turn end: ", ms) + t
        case .deskDone(let st, let n, let a, let b): return "desk stream \(st), \(n) sentences, server first sentence \(a.map { String(format: "%.0f ms", $0) } ?? "-"), server done \(b.map { String(format: "%.0f ms", $0) } ?? "-")"
        case .transitionPlayed(let ms): return String(format: "hand-over sound %.0f ms after turn end", ms)
        case .cueStarted(let ms): return String(format: "non-verbal cue %.0f ms after turn end", ms)
        case .deskFirstToken(let ms): return String(format: "desk first token %.0f ms after turn end", ms)
        case .programmeAudioStart(let b): return "programme audio started, block \(b)"
        case .ackCancelled(let r, let a): return "acknowledgement stopped: \(r)\(a ? " (audible; the line finished)" : " (before any audio)")"
        case .revisionArrived(let id, let from, let n, let sum): return "revision \(id) from block \(from), \(n) blocks: \(sum)"
        case .playbackState(let s): return "playback \(s.rawValue)"
        case .yielded(let y): return "yield: \(y.rawValue)"
        case .stageTiming(let s, let a, let b): return String(format: "timing %@ %.0f ms", s, (b - a) * 1000)
        case .modelLoaded(let id, let ms, let b): return String(format: "model %@ loaded in %.0f ms, %lld MB", id, ms, b / 1_048_576)
        case .thermal(let t): return "thermal \(t)"
        case .audioSession(let c, let m, let n): return "audio session \(c)/\(m): \(n)"
        case .audioSwitch(let f, let t, let ms): return String(format: "audio switch %@ -> %@ in %.0f ms", f, t, ms)
        case .audioNotice(let k, let d): return "audio \(k): \(d)"
        case .configurationChanged(let n): return "configuration -> \(n)"
        case .note(let n): return n
        case .error(let s, let m): return "ERROR \(s): \(m)"
        }
    }
}

final class PolicyBox: @unchecked Sendable {
    private let l = NSLock(); private var v: AudioPolicy
    init(_ v: AudioPolicy) { self.v = v }
    var value: AudioPolicy { get { l.lock(); defer { l.unlock() }; return v } set { l.lock(); v = newValue; l.unlock() } }
}
