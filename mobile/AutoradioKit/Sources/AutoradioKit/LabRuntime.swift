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
public struct AudioInfo: Codable, Sendable {
    public var state = "none", policy = "switchOnAddress", engine = "off", route = "", message = ""
    public var voiceProcessing = false, audible = false
    public var volume = 1.0, lastSwitchMs = 0.0
    public var policies: [String] = AudioPolicy.allCases.map(\.rawValue)
    public var ackTiming = "immediate"
    public var ackTimings: [String] = AckTiming.allCases.map(\.rawValue)
}
public struct StageRow: Codable, Sendable { public var kind, kindLabel, id, name, summary: String; public var selected: Bool; public var network: Bool; public var models: String }
public struct ParamRow: Codable, Sendable { public var kind, stage, name, label, value, hint: String }
public struct TimingRow: Codable, Sendable { public var stage: String; public var startMs, durMs: Double }
public struct TraceView: Codable, Sendable {
    public var id = "", transcript = "", configName = "", outcome = "", addressedBy = ""
    public var totalMs = 0.0, ackMs = 0.0, revisionMs = 0.0, audioSwitchMs = 0.0
    public var timings: [TimingRow] = []
    public var lines: [String] = []
}
public struct TurnRow: Codable, Sendable { public var id, transcript, configName, outcome: String; public var ackMs, revisionMs: Double }

/// Everything a host needs: the registry, the audio, the engine, kept turns, configurations.
/// A host (the Exact module) creates one, then calls `start`, `talkDown`, `talkUp`, `setStage`, `replay` ...
public final class LabRuntime: @unchecked Sendable {
    public let registry: StageRegistry
    public let clock = LabClock()
    public let log: EventLog
    public let bus = AudioBus()
    public let audio = LiveAudio()
    public let player: ProgrammePlayer
    public let engine: TurnEngine
    public let recorder: TurnRecorder
    public let store: KeptTurnStore
    public let configs: ConfigStore
    public let session: AudioSessionOwner
    public let coordinator: SessionCoordinator
    public var onChange: (@Sendable () -> Void)?

    private let lock = NSLock()
    private var config: LabConfiguration
    private var snap = LabSnapshot()
    private var tail: [String] = []
    private var records: [String: TurnRecord] = [:]
    private var lastID: String?
    private var micTask: Task<Void, Never>?
    private var eventTask: Task<Void, Never>?
    private var started = false
    private var lastSwitchMs = 0.0
    private let policyBox: PolicyBox
    private var injecting = false

    public init(supportDir: URL, session: AudioSessionOwner, registry: StageRegistry = .builtIn()) {
        self.registry = registry
        let log = EventLog(clock: clock)
        self.log = log
        let cfg = StageRegistry.listenerConfiguration()
        self.config = cfg
        let policyBox = PolicyBox(cfg.audio)
        self.policyBox = policyBox
        let coord = SessionCoordinator(inner: session, log: log, audio: audio, policy: { policyBox.value })
        self.coordinator = coord
        self.session = coord
        store = KeptTurnStore(root: supportDir.appendingPathComponent("turns"))
        configs = ConfigStore(dir: supportDir.appendingPathComponent("configs"))
        FixtureTurn.install(into: store)
        let doc = FixtureProgramme.document
        player = ProgrammePlayer(audio: audio, registry: registry, log: log, configuration: cfg, document: doc)
        recorder = TurnRecorder(bus: bus)
        let storeRef = store
        var envBox: LabRuntime?
        let env = EngineEnvironment(registry: registry, log: log, playback: player, session: self.session,
            onRecord: { rec in envBox?.keep(rec) },
            onOutcome: { id, o in storeRef.updateOutcome(id: id, o); envBox?.setOutcome(id, o) })
        engine = TurnEngine(env: env, configuration: cfg)
        envBox = self
        snap.configName = cfg.name
        player.onChange = { [weak self] in self?.refresh() }
        // The bar and the player read the real position from the audio, and a speed picked there reaches the audio.
        let pl = player
        BarTracker.shared.positionSource = { [weak pl] in pl?.renderPosition() }
        BarTracker.shared.speedSink = { [weak pl] v in pl?.setRate(v) }
        // Mail's "Read it to me" speaks through the engine.
        MailReading.shared.handler = { [weak self] address, text in Task { await self?.speakText(address: address, text: text) } }
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
        if virtual { LabEnvironment.promptsAllowed = false; LabEnvironment.virtualAudio = true }
        do {
            try await session.claim(.listening, preferBuiltInMic: true, reason: "start")
            let micOK = virtual ? true : await LiveAudio.requestMicPermission()
            if !micOK { log.emit(.error(stage: "mic", message: "microphone permission denied; button/typing only")) }
            try audio.start(bus: bus, clock: clock, wantMic: micOK, voiceProcessing: coordinator.current == .talking, virtual: virtual)
            if virtual { log.emit(.note("virtual audio: no hardware is opened; mic frames are injected")) }
            started = true
            let mic = bus.mic()
            micTask = Task { [engine] in for await f in mic { await engine.ingest(f) } }
            player.play()
            lock.lock(); snap.running = true; snap.message = ""; lock.unlock()
        } catch {
            log.emit(.error(stage: "start", message: "\(error)"))
            lock.lock(); snap.message = "start failed: \(error)"; lock.unlock()
        }
        refresh()
    }

    /// Set by the host: how to start the engine when text is to be spoken and it is not running (the host knows whether the
    /// engine is virtual). Without it the text becomes the loaded programme and plays when the radio is started.
    public var startRequest: (@Sendable () async -> Void)?

    /// Speak arbitrary text as a programme item (mail's "Read it to me"): it queues like a story, shows in now-playing, and the
    /// listener can cut into it like any speech. Returns false for text with nothing to say.
    @discardableResult
    public func speakText(address: String, text: String, title: String? = nil) async -> Bool {
        if !snapshot().running, let go = startRequest { await go() }
        let running = snapshot().running
        // Not running: the text is the loaded programme, played when the radio starts (the engine is not there to play it).
        return player.speakText(address: address, text: text, title: title, startNow: !running, play: running)
    }

    public func stop() {
        player.pause()
        micTask?.cancel(); micTask = nil
        audio.stop(); coordinator.release(); started = false
        lock.lock(); snap.running = false; lock.unlock()
        refresh()
    }

    /// A typed question from the bar: starts the engine when it is not running (the host's `startRequest`), then runs one turn.
    @discardableResult
    public func ask(_ text: String) async -> Bool {
        if !snapshot().running, let go = startRequest { await go() }
        guard snapshot().running else { return false }
        return await engine.typedTurn(text)
    }

    public func talkDown() { Task { await engine.buttonDown() } }
    public func talkUp() { Task { await engine.buttonUp() } }

    /// Drive the live path with the bundled kept turn: press, feed its microphone audio in real time as if it came from
    /// the hardware, release. Works with real or virtual audio; the turn lands in Trace like a spoken one.
    public func injectFixtureTurn() async {
        guard started, !injecting else { return }
        injecting = true; defer { injecting = false }
        guard let url = FixtureTurn.materialise(), let mic = try? AudioFiles.readMono(url) else {
            log.emit(.error(stage: "inject", message: "no fixture turn audio")); return
        }
        log.emit(.note("injecting the fixture turn (\(mic.count / 16) ms) through the live engine"))
        // Press the button only when the configuration addresses by button; in open mode the audio alone must do it.
        let useButton = ["button", "hybrid"].contains(configuration.setting(.addressing)?.id ?? "")
        if useButton { await engine.buttonDown() }
        let n = 512   // 32 ms frames, paced in real time
        var i = 0
        while i < mic.count {
            let f = AudioFrame(samples: Array(mic[i..<min(i + n, mic.count)]), sampleRate: busSampleRate, t: clock.now)
            bus.pushMic(f)
            i += n
            try? await Task.sleep(nanoseconds: 32_000_000)
        }
        // trailing silence so the VAD closes, then the button
        for _ in 0..<20 {
            bus.pushMic(AudioFrame(samples: [Float](repeating: 0, count: n), sampleRate: busSampleRate, t: clock.now))
            try? await Task.sleep(nanoseconds: 32_000_000)
        }
        if useButton { await engine.buttonUp() }
    }

    // MARK: Configuration

    public var configuration: LabConfiguration { lock.lock(); defer { lock.unlock() }; return config }
    public func setConfiguration(_ c: LabConfiguration) {
        lock.lock(); config = c; snap.configName = c.name; lock.unlock()
        let policyChanged = policyBox.value != c.audio
        policyBox.value = c.audio
        if policyChanged { Task { [coordinator] in await coordinator.reapply() } }
        player.setConfiguration(c)
        Task { await engine.setConfiguration(c) }
        refresh()
    }
    public func setAudioPolicy(_ p: AudioPolicy) {
        var c = configuration
        c.audioPolicy = p
        setConfiguration(c)
    }
    public func setAckTiming(_ t: AckTiming) {
        var c = configuration
        c.ackTiming = t
        setConfiguration(c)
    }
    public func setStage(_ kind: StageKind, id: String) {
        var c = configuration
        c.set(kind, StageSetting(id: id))
        setConfiguration(c)
    }
    public func setParam(_ kind: StageKind, name: String, value: String) {
        var c = configuration
        guard var s = c.setting(kind) else { return }
        let spec = registry.stages(kind).first { $0.id == s.id }?.descriptor.params.first { $0.name == name }
        switch spec?.kind {
        case .number, .int: if let d = Double(value) { s.params[name] = .number(d) }
        case .bool: s.params[name] = .bool(value == "true" || value == "1" || value == "yes")
        default: s.params[name] = .string(value)
        }
        c.set(kind, s)
        setConfiguration(c)
    }
    public func saveConfiguration(name: String) throws {
        var c = configuration; c.name = name
        try configs.save(c); setConfiguration(c)
    }
    public func loadConfiguration(name: String) throws { setConfiguration(try configs.load(name)) }
    public func loadConfiguration(url: URL) async throws { setConfiguration(try await LabConfiguration.fetch(url)) }

    // MARK: Turns

    func keep(_ rec: TurnRecord) {
        let t0 = (rec.events.first?.t ?? 0) - 1.0
        let t1 = (rec.events.last?.t ?? 0) + 0.5
        let snapAudio = recorder.snapshot(from: t0, to: t1)
        lock.lock(); records[rec.id] = rec; lastID = rec.id; lock.unlock()
        let pos = ProgramPosition(programmeID: player.document.id, blockIndex: player.blockIndex, context: "")
        let ctx = TurnContextFile(position: pos, configuration: rec.configuration)
        _ = try? store.save(record: rec, mic: snapAudio.mic, reference: snapAudio.reference, context: ctx)
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
    public func trace(id: String?) -> TraceView {
        guard let r = record(id: id) else { return TraceView() }
        let t0 = r.events.first?.t ?? 0
        var v = TraceView(id: r.id, transcript: r.summary.transcript ?? "", configName: r.configuration.name, outcome: r.outcome.rawValue,
                          addressedBy: r.summary.addressedBy ?? "", totalMs: ((r.events.last?.t ?? t0) - t0) * 1000,
                          ackMs: r.summary.turnEndToAckMs ?? 0, revisionMs: r.summary.turnEndToRevisionMs ?? 0)
        v.audioSwitchMs = r.summary.audioSwitchMs ?? 0
        v.timings = r.summary.timings.map { TimingRow(stage: $0.stage, startMs: ($0.startedAt - t0) * 1000, durMs: $0.ms) }
        v.lines = r.events.filter { if case .partialTranscript = $0.event { return false }; if case .stageTiming = $0.event { return false }; return true }
            .map { String(format: "%7.0f ms  ", ($0.t - t0) * 1000) + Self.describe($0.event) }
        return v
    }

    /// Write a turn record (JSON) to Documents/AutoradioExports/<id>.json and return its URL. Pull it off a phone with
    /// `xcrun devicectl device copy from --device <udid> --domain-type appDataContainer --domain-identifier <bundle id>
    /// --source Documents/AutoradioExports/<id>.json --destination .`
    public func export(id: String?) -> URL? {
        guard let r = record(id: id), let data = try? r.encoded() else { return nil }
        let dir = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("AutoradioExports")
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let url = dir.appendingPathComponent(r.id + ".json")
        do { try data.write(to: url, options: .atomic); return url } catch { log.emit(.error(stage: "export", message: "\(error)")); return nil }
    }

    /// Run a kept turn through the current (or another) configuration; returns the new record's id.
    public func replay(id: String, configuration: LabConfiguration? = nil) async throws -> TurnRecord {
        let k = KeptTurn(folder: store.root.appendingPathComponent(id))
        let rec = try await Replayer.run(turn: k, configuration: configuration ?? self.configuration, registry: registry)
        lock.lock(); records[rec.id] = rec; lastID = rec.id; lock.unlock()
        refresh()
        return rec
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
    public func audioInfo() -> AudioInfo {
        var a = AudioInfo()
        a.state = coordinator.current?.rawValue ?? "none"
        a.policy = configuration.audio.rawValue
        a.ackTiming = configuration.ack.rawValue
        a.engine = audio.virtual && audio.running ? "virtual" : audio.running ? "up" : audio.down ? "down" : "off"
        a.route = coordinator.describe()
        a.voiceProcessing = audio.voiceProcessing
        a.audible = audio.programmeAudible
        a.volume = Double(audio.programmeVolume)
        lock.lock(); a.lastSwitchMs = lastSwitchMs; a.message = snap.message; lock.unlock()
        return a
    }
    public func stageRows() -> [StageRow] {
        let c = configuration
        return registry.all.map { r in
            StageRow(kind: r.kind.rawValue, kindLabel: Self.kindLabel(r.kind), id: r.id, name: r.descriptor.displayName, summary: r.descriptor.summary,
                     selected: c.setting(r.kind)?.id == r.id, network: r.descriptor.needsNetwork, models: r.descriptor.needsModels.joined(separator: ", "))
        }
    }
    public func paramRows() -> [ParamRow] {
        let c = configuration
        var out: [ParamRow] = []
        for k in StageKind.allCases {
            guard let s = c.setting(k), let d = registry.stages(k).first(where: { $0.id == s.id })?.descriptor else { continue }
            let p = registry.params(k, s)
            for spec in d.params {
                let hint = [spec.min.map { "min \($0)" }, spec.max.map { "max \($0)" }, spec.note].compactMap { $0 }.joined(separator: ", ")
                out.append(ParamRow(kind: k.rawValue, stage: s.id, name: spec.name, label: spec.label, value: (p.values[spec.name] ?? spec.defaultValue).description, hint: hint))
            }
        }
        return out
    }
    public func eventLines() -> [String] { lock.lock(); defer { lock.unlock() }; return tail }

    static func kindLabel(_ k: StageKind) -> String {
        switch k {
        case .micSource: return "Microphone"; case .addressing: return "Addressing"; case .speechBounds: return "Speech bounds"
        case .speakerCheck: return "Speaker check"; case .transcriber: return "Transcription"; case .yieldPolicy: return "Yielding"
        case .system1: return "System 1"; case .holder: return "Holding"; case .voice: return "Voice"; case .desk: return "System 2 (desk)"
        case .programme: return "Programme"
        }
    }

    // MARK: Event absorption

    func absorb(_ te: TimedEvent) {
        lock.lock()
        switch te.event {
        case .phase(_, let to): snap.phase = to; if to == "candidate" { snap.transcript = "" }
        case .partialTranscript(let t), .finalTranscript(let t): snap.transcript = t
        case .error(_, let m): snap.message = m
        case .audioSwitch(let from, _, let ms) where from != "none": lastSwitchMs = ms
        default: break
        }
        if case .partialTranscript = te.event {} else {
            tail.append(String(format: "%.2f ", te.t.truncatingRemainder(dividingBy: 10000)) + Self.describe(te.event))
            if tail.count > 60 { tail.removeFirst() }
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
        case .system1Answer(let q, let o, let s, let ms): return String(format: "system1 %@ = %@ (%.2f, %.0f ms)", q, o, s, ms)
        case .ackStarted(let ms): return String(format: "acknowledgement audio %.0f ms after turn end", ms)
        case .holdingSaid(let l): return "holding: \(l)"
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
