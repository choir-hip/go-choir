import Foundation

/// context.json of a kept turn: what the stages saw besides the audio.
public struct TurnContextFile: Codable, Sendable {
    public var position: ProgramPosition
    public var preparedMaterial: String?
    public var configuration: LabConfiguration?
    /// Button presses, seconds from the start of mic.caf.
    public var inputs: [ScheduledInput]
    /// Words a fixture expects in the transcript (proof only).
    public var expectedWords: [String]?
    public init(position: ProgramPosition, preparedMaterial: String? = nil, configuration: LabConfiguration? = nil,
                inputs: [ScheduledInput] = [], expectedWords: [String]? = nil) {
        self.position = position; self.preparedMaterial = preparedMaterial; self.configuration = configuration
        self.inputs = inputs; self.expectedWords = expectedWords
    }
}

/// A kept turn on disk: mic.caf, reference.caf (optional), context.json, record.json.
public struct KeptTurn: Sendable {
    public let folder: URL
    public init(folder: URL) { self.folder = folder }
    public var micURL: URL { folder.appendingPathComponent("mic.caf") }
    public var referenceURL: URL { folder.appendingPathComponent("reference.caf") }
    public var contextURL: URL { folder.appendingPathComponent("context.json") }
    public var recordURL: URL { folder.appendingPathComponent("record.json") }
    public func context() throws -> TurnContextFile { try JSONDecoder().decode(TurnContextFile.self, from: Data(contentsOf: contextURL)) }
    public func record() throws -> TurnRecord { try TurnRecord.decode(Data(contentsOf: recordURL)) }
    public var name: String { folder.lastPathComponent }
}

public final class KeptTurnStore: @unchecked Sendable {
    public let root: URL
    public init(root: URL) { self.root = root; try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true) }
    public func list() -> [KeptTurn] {
        let items = (try? FileManager.default.contentsOfDirectory(at: root, includingPropertiesForKeys: nil)) ?? []
        return items.filter { FileManager.default.fileExists(atPath: $0.appendingPathComponent("mic.caf").path) }
            .sorted { $0.lastPathComponent > $1.lastPathComponent }.map(KeptTurn.init)
    }
    /// Keep a live turn: audio from the recorder, the context the stages saw, the record.
    public func save(record: TurnRecord, mic: [Float], reference: [Float]?, context: TurnContextFile) throws -> KeptTurn {
        let f = root.appendingPathComponent(record.id)
        try FileManager.default.createDirectory(at: f, withIntermediateDirectories: true)
        let k = KeptTurn(folder: f)
        try AudioFiles.writeMono(mic, to: k.micURL)
        if let r = reference, !r.isEmpty { try AudioFiles.writeMono(r, to: k.referenceURL) }
        let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
        try e.encode(context).write(to: k.contextURL)
        try record.encoded().write(to: k.recordURL)
        return k
    }
    public func updateOutcome(id: String, _ o: Outcome) {
        let k = KeptTurn(folder: root.appendingPathComponent(id))
        guard var r = try? k.record() else { return }
        r.outcome = o
        try? r.encoded().write(to: k.recordURL)
    }
}

/// Rolling copy of the last seconds of mic and reference, so a finished turn can be kept.
public final class TurnRecorder: @unchecked Sendable {
    private let lock = NSLock()
    private var mic: [AudioFrame] = [], ref: [AudioFrame] = []
    let keepSeconds: Double
    private var tasks: [Task<Void, Never>] = []
    public init(bus: AudioBus, keepSeconds: Double = 90) {
        self.keepSeconds = keepSeconds
        let m = bus.mic(), r = bus.reference()
        tasks = [Task { [weak self] in for await f in m { self?.add(f, mic: true) } },
                 Task { [weak self] in for await f in r { self?.add(f, mic: false) } }]
    }
    deinit { tasks.forEach { $0.cancel() } }
    private func add(_ f: AudioFrame, mic isMic: Bool) {
        lock.lock(); defer { lock.unlock() }
        if isMic { mic.append(f); trim(&mic) } else { ref.append(f); trim(&ref) }
    }
    private func trim(_ a: inout [AudioFrame]) { while let f = a.first, let l = a.last, l.t - f.t > keepSeconds { a.removeFirst() } }
    /// Samples between two times on the clock; the reference is padded to the same grid.
    public func snapshot(from t0: Double, to t1: Double) -> (mic: [Float], reference: [Float]?) {
        lock.lock(); defer { lock.unlock() }
        func cut(_ a: [AudioFrame]) -> [Float] { a.filter { $0.t + $0.duration >= t0 && $0.t <= t1 }.flatMap(\.samples) }
        let r = cut(ref)
        return (cut(mic), r.isEmpty ? nil : r)
    }
}

// MARK: Replay

public struct ReplayOptions: Sendable {
    public var realTime = false
    public var overallTimeout = 90.0
    public init(realTime: Bool = false) { self.realTime = realTime }
}

public enum Replayer {
    /// Run a kept turn through a configuration and return a new record. Never writes into the kept folder.
    public static func run(turn: KeptTurn, configuration: LabConfiguration, registry: StageRegistry = .builtIn(),
                           options: ReplayOptions = ReplayOptions()) async throws -> TurnRecord {
        let ctx = try turn.context()
        let mic = try AudioFiles.readMono(turn.micURL)
        let ref = FileManager.default.fileExists(atPath: turn.referenceURL.path) ? try AudioFiles.readMono(turn.referenceURL) : nil
        var inputs = ctx.inputs

        let clock = LabClock()
        let log = EventLog(clock: clock)
        let playback = SimulatedPlayback(position: ctx.position)
        let env = EngineEnvironment(registry: registry, log: log, playback: playback)
        let engine = TurnEngine(env: env, configuration: configuration)

        // Failure mode 1: a button-addressed configuration over a turn with no recorded presses.
        let addrID = configuration.setting(.addressing)?.id ?? ""
        if inputs.isEmpty && (addrID == "button" || addrID == "hybrid") {
            if let onset = firstOnset(mic) {
                let t0 = max(0, onset - 0.1)
                inputs = [ScheduledInput(t: t0, kind: .buttonDown), ScheduledInput(t: t0 + 0.1, kind: .buttonUp)]   // a tap
                log.emit(.note("replay: no recorded button; tapped at first speech onset \(String(format: "%.2f", onset)) s"), at: 0)
            }
        }

        let bus = AudioBus()
        bus.syncMic = { f in await engine.ingest(f) }
        let src = FileAudioSource(mic: mic, reference: ref, inputs: inputs, realTime: options.realTime)
        src.onInput = { i in
            switch i.kind { case .buttonDown: await engine.buttonDown(); case .buttonUp: await engine.buttonUp() }
        }
        log.emit(.note("replay of \(turn.name) with configuration \(configuration.name)"))
        try await src.start(into: bus, clock: clock)
        await src.finished()
        let idle: Void? = await withDeadline(options.overallTimeout) { await engine.waitUntilIdle() }
        if idle == nil { log.emit(.error(stage: "replay", message: "turn did not finish within \(options.overallTimeout)s")) }
        if !log.events.contains(where: { if case .turnEnded = $0.event { return true }; return false }) {
            log.emit(.note("no turn was taken from this audio under this configuration"))
        }
        return TurnRecord.fold(id: "replay-" + turn.name, configuration: configuration, device: .current, events: log.events)
    }

    static func firstOnset(_ mic: [Float]) -> Double? {
        let n = Int(0.032 * busSampleRate)
        var levels: [Double] = []
        var i = 0
        while i + n <= mic.count { levels.append(AudioFrame(samples: Array(mic[i..<i + n]), sampleRate: busSampleRate, t: 0).rmsDb); i += n }
        guard let floor = levels.prefix(10).min() else { return nil }
        let thr = max(-40, floor + 10)
        return levels.firstIndex { $0 > thr }.map { Double($0) * 0.032 }
    }
}
