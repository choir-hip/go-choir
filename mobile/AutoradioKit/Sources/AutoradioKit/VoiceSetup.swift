import Foundation

// Voice setup = configuring the voice and audio models (plan section 2, "Voice setup"; owner, 2026-11-11).
// It is not an introduction to the product. It ends with a working set of models for THIS phone, one per stage, each
// checked on this device, with a fallback ladder per stage that always ends in something that exists.
//
// Ways it can fail (written before the code, per the repo testing rule). Each has a test in VoiceSetupTests.
//  1. A phone with little memory: the pick must start the ladder at the first model that fits, never at one that will
//     not load; the floor of every ladder (energy detector, Apple's recogniser, the system voice, System 1 through the
//     listener's computer) needs no download and is always present.
//  2. The downloader stalls silently for minutes (the models lane found this): a watchdog sees no bytes for
//     `stallSeconds`, cancels that transfer, marks the row "stalled" and offers "Try again" and the fallback; with no
//     answer after `patience` the fallback is taken. It never waits forever and never shows a bar that does not move.
//  3. "Try again" is tapped repeatedly or the transfer keeps failing: at most `maxTries` tries on one model, then the fallback.
//  4. A model downloads but does not pass its check (does not load within budget, wrong output, too slow): replaced by the
//     next on the ladder without asking; the row says so; the failure is kept in the record.
//  5. The listener taps "Try again" or the fallback twice, or after the row already moved on: ignored (one action per wait).
//  6. The user leaves the card, or the app closes mid-download: the downloads resume (ModelStore uses HTTP Range on a .part
//     file); nothing here restarts by itself after a crash (repo rule): the card shows "Finish voice setup" and the
//     listener starts it again.
//  7. Mobile data: the total is said before anything downloads and the card asks first when not on Wi-Fi.
//  8. Not enough free disk: the total is compared with free space before starting; the card says so and picks the smaller set.
//  9. Every model fails (offline): all four stages end on their floors, which exist, so the radio still works; the page
//     states the result in one line per stage, marking each fallback, with "Try again" beside it.
// 10. Checks on a device are unproven here: this file defines the check's shape and the record; `LiveSetupBackend`
//     (below) wires the real downloader but reports a check as not run unless a checker is injected.

public enum SetupStage: String, Codable, CaseIterable, Sendable {
    case bounds, transcriber, voice, decision

    /// Plain words for the listener.
    public var title: String {
        switch self {
        case .bounds: return "Knowing when you speak"
        case .transcriber: return "Hearing you"
        case .voice: return "The voice"
        case .decision: return "Quick decisions"
        }
    }
    /// The kit's stage kind this configures.
    public var kind: StageKind {
        switch self {
        case .bounds: return .speechBounds
        case .transcriber: return .transcriber
        case .voice: return .voice
        case .decision: return .system1
        }
    }
}

public struct SetupCandidate: Codable, Equatable, Sendable {
    /// The model id in the ModelStore catalog, or the floor's id.
    public var id: String
    /// Plain name: "Parakeet, the larger model".
    public var label: String
    public var bytes: Int64
    public var minMemoryGB: Double
    /// The kit stage id this becomes when it passes (`StageRegistry` id), "" when the stage is not switchable here.
    public var kitStageID: String
    public init(id: String, label: String, bytes: Int64, minMemoryGB: Double, kitStageID: String = "") {
        self.id = id; self.label = label; self.bytes = bytes; self.minMemoryGB = minMemoryGB; self.kitStageID = kitStageID
    }
    /// Needs no download: it is on the phone already.
    public var isFloor: Bool { bytes == 0 }
}

public struct DeviceProfile: Codable, Equatable, Sendable {
    public var chip: String
    public var memoryGB: Double
    public var freeDiskGB: Double
    public var onWiFi: Bool
    public init(chip: String, memoryGB: Double, freeDiskGB: Double, onWiFi: Bool) {
        self.chip = chip; self.memoryGB = memoryGB; self.freeDiskGB = freeDiskGB; self.onWiFi = onWiFi
    }

    /// This device. `onWiFi` is true unless the host knows better (no Network framework here); the card asks first
    /// when the host says it is not.
    public static func current(onWiFi: Bool = true) -> DeviceProfile {
        var size = 0
        sysctlbyname("hw.machine", nil, &size, nil, 0)
        var buf = [CChar](repeating: 0, count: max(size, 1))
        sysctlbyname("hw.machine", &buf, &size, nil, 0)
        let chip = String(decoding: buf.prefix(while: { $0 != 0 }).map { UInt8(bitPattern: $0) }, as: UTF8.self)
        let mem = Double(ProcessInfo.processInfo.physicalMemory) / 1_073_741_824
        let home = URL(fileURLWithPath: NSHomeDirectory())
        let free = ((try? home.resourceValues(forKeys: [.volumeAvailableCapacityForImportantUsageKey]))?.volumeAvailableCapacityForImportantUsage).map { Double($0) / 1_073_741_824 } ?? 0
        return DeviceProfile(chip: chip, memoryGB: mem, freeDiskGB: free, onWiFi: onWiFi)
    }
}

/// The model table for setup: per stage, a ladder from the recommended model to the floor. Sizes are the model cards'
/// figures (ModelHub.catalog), not measurements; setup replaces them with measured numbers in the record.
public enum SetupCatalog {
    public static func ladder(_ s: SetupStage) -> [SetupCandidate] {
        switch s {
        case .bounds:
            return [SetupCandidate(id: "silero-vad-v6.2.1", label: "Silero speech detector", bytes: 2_000_000, minMemoryGB: 1, kitStageID: "silero-vad"),
                    SetupCandidate(id: "energy", label: "Level detector built in", bytes: 0, minMemoryGB: 0, kitStageID: "energy-bounds")]
        case .transcriber:
            return [SetupCandidate(id: "parakeet-tdt-0.6b-v2", label: "Parakeet, the larger listener", bytes: 480_000_000, minMemoryGB: 6, kitStageID: "parakeet-tdt"),
                    SetupCandidate(id: "parakeet-eou-120m", label: "Parakeet, the smaller listener", bytes: 250_000_000, minMemoryGB: 3, kitStageID: "parakeet-eou"),
                    SetupCandidate(id: "apple-speech", label: "Apple's recogniser", bytes: 0, minMemoryGB: 0, kitStageID: "apple-speech")]
        case .voice:
            return [SetupCandidate(id: "kokoro-82m-ane", label: "Kokoro voice", bytes: 350_000_000, minMemoryGB: 4, kitStageID: "kokoro"),
                    SetupCandidate(id: "pocket-tts", label: "Pocket voice, the lighter one", bytes: 300_000_000, minMemoryGB: 3, kitStageID: "pocket-tts"),
                    SetupCandidate(id: "system-voice", label: "The system voice", bytes: 0, minMemoryGB: 0, kitStageID: "apple-voice")]
        case .decision:
            // No on-device decision model is proven yet: System 1 through the listener's computer is the default.
            return [SetupCandidate(id: "system1-computer", label: "Your computer", bytes: 0, minMemoryGB: 0, kitStageID: "choir-system1")]
        }
    }

    /// The ladder for this phone: from the first candidate that fits its memory, to the floor.
    public static func plan(for d: DeviceProfile, smaller: Bool = false) -> [SetupStage: [SetupCandidate]] {
        var out: [SetupStage: [SetupCandidate]] = [:]
        for s in SetupStage.allCases {
            let all = ladder(s)
            var rest = Array(all.drop(while: { $0.minMemoryGB > d.memoryGB }))
            if rest.isEmpty { rest = [all.last!] }                       // the floor always exists
            if smaller, rest.count > 1, !rest[0].isFloor { rest.removeFirst() }
            out[s] = rest
        }
        return out
    }

    /// What the page promises before anything downloads: the first model of every ladder.
    public static func totalBytes(_ plan: [SetupStage: [SetupCandidate]]) -> Int64 {
        plan.values.reduce(0) { $0 + ($1.first?.bytes ?? 0) }
    }

    public static func sizeLabel(_ bytes: Int64) -> String {
        if bytes <= 0 { return "Built in" }
        if bytes >= 1_000_000_000 { return String(format: "%.1f GB", Double(bytes) / 1e9) }
        return "\(max(1, Int((Double(bytes) / 1e6).rounded()))) MB"
    }

    /// "About 0.8 GB. On Wi-Fi this takes a few minutes."
    public static func totalLabel(bytes: Int64, onWiFi: Bool) -> String {
        if bytes <= 0 { return "Nothing to download. The radio will use what is already on this phone." }
        let size = bytes >= 1_000_000_000 ? String(format: "About %.1f GB.", Double(bytes) / 1e9) : "About \(max(1, Int((Double(bytes) / 1e6 / 10).rounded()) * 10)) MB."
        return onWiFi ? size + " On Wi-Fi this takes a few minutes." : size + " You are on mobile data, so I will ask before I start."
    }
}

// MARK: Checks, backend, rows

/// What one check on this device measured.
public struct SetupCheck: Codable, Equatable, Sendable {
    public var loadMs: Double
    /// Real-time factor for audio models; milliseconds per decision for decision models.
    public var speed: Double
    public var unit: String
    public var memoryMB: Double
    public var outputOK: Bool
    public var note: String
    public init(loadMs: Double, speed: Double, unit: String, memoryMB: Double, outputOK: Bool, note: String = "") {
        self.loadMs = loadMs; self.speed = speed; self.unit = unit; self.memoryMB = memoryMB; self.outputOK = outputOK; self.note = note
    }
}

public protocol SetupBackend: Sendable {
    /// Fetch (or resume) the model onto the phone. Call `onBytes` with the total bytes on disk whenever it grows.
    func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws
    /// Load the model and run the bundled clip and fixed phrase: loads within memory, speed, correct output.
    func check(_ c: SetupCandidate) async throws -> SetupCheck
}

public struct SetupRow: Codable, Equatable, Sendable {
    public var stage: String
    public var name: String
    public var model: String
    public var sizeLabel: String
    /// waiting | downloading | stalled | failed | preparing | ready
    public var state: String
    public var progress: Double
    public var statusLine: String
    public var isFallback: Bool
    public var canRetry: Bool
    public var canFallback: Bool
    public var numbers: String
}

public struct SetupRecord: Codable, Equatable, Sendable {
    public struct Item: Codable, Equatable, Sendable {
        public var stage: String
        public var chosen: String
        public var kitStageID: String
        public var fellBackFrom: [String]
        public var reasons: [String]
        public var check: SetupCheck?
    }
    public var device: DeviceProfile
    public var at: Double
    public var items: [Item]
}

// MARK: The runner

public final class VoiceSetup: @unchecked Sendable {
    public enum UserAction: Sendable { case retry, fallback }
    private enum Outcome { case ok, stalled, failed(String), cancelled }

    private let backend: SetupBackend
    public let profile: DeviceProfile
    private let stallSeconds: Double
    private let patience: Double
    private let maxTries: Int
    private let now: @Sendable () -> Double
    public var onChange: (@Sendable () -> Void)?

    private let lock = NSLock()
    private let plan: [SetupStage: [SetupCandidate]]
    private var waitGen: [SetupStage: Int] = [:]
    private var rowsByStage: [SetupStage: SetupRow] = [:]
    private var lastBytesAt: [SetupStage: Double] = [:]
    private var waiting: [SetupStage: CheckedContinuation<UserAction, Never>] = [:]
    private var early: [SetupStage: UserAction] = [:]
    private var items: [SetupStage: SetupRecord.Item] = [:]
    private var finished = false
    private var started = false

    public init(backend: SetupBackend, profile: DeviceProfile, smaller: Bool = false, stallSeconds: Double = 20, patience: Double = 45, maxTries: Int = 3,
                now: @escaping @Sendable () -> Double = { Date().timeIntervalSince1970 }) {
        self.backend = backend; self.profile = profile; self.stallSeconds = stallSeconds; self.patience = patience; self.maxTries = maxTries; self.now = now
        plan = SetupCatalog.plan(for: profile, smaller: smaller)
        for s in SetupStage.allCases { rowsByStage[s] = Self.freshRow(s, plan[s]![0]) }
    }

    static func freshRow(_ s: SetupStage, _ c: SetupCandidate) -> SetupRow {
        SetupRow(stage: s.rawValue, name: s.title, model: c.label, sizeLabel: SetupCatalog.sizeLabel(c.bytes), state: "waiting", progress: 0,
                 statusLine: c.isFloor ? "Waiting" : "Waiting to download", isFallback: false, canRetry: false, canFallback: false, numbers: "")
    }

    // MARK: Reading

    public var rows: [SetupRow] { lock.lock(); defer { lock.unlock() }; return SetupStage.allCases.compactMap { rowsByStage[$0] } }
    public var totalBytes: Int64 { lock.lock(); defer { lock.unlock() }; return SetupCatalog.totalBytes(plan) }
    public var totalLabel: String { SetupCatalog.totalLabel(bytes: totalBytes, onWiFi: profile.onWiFi) }
    public var isFinished: Bool { lock.lock(); defer { lock.unlock() }; return finished }
    public var isStarted: Bool { lock.lock(); defer { lock.unlock() }; return started }
    /// Whether free disk covers the first picks with a margin (failure 8).
    public var fitsOnDisk: Bool { Double(totalBytes) / 1_073_741_824 * 1.5 < max(profile.freeDiskGB, 0.0001) || profile.freeDiskGB == 0 }

    /// One line per stage, marking any fallback ("Hearing you: Apple's recogniser (fallback)").
    public var summary: [String] {
        rows.map { r in r.state == "ready" ? "\(r.name): \(r.model)\(r.isFallback ? " (fallback)" : "")" : "\(r.name): \(r.statusLine)" }
    }

    public var record: SetupRecord {
        lock.lock(); defer { lock.unlock() }
        return SetupRecord(device: profile, at: now(), items: SetupStage.allCases.compactMap { items[$0] })
    }

    /// stage -> kit stage id for the stages that passed (what the listener's configuration should select).
    public var choices: [StageKind: String] {
        lock.lock(); defer { lock.unlock() }
        var out: [StageKind: String] = [:]
        for (s, i) in items where !i.kitStageID.isEmpty { out[s.kind] = i.kitStageID }
        return out
    }

    // MARK: Running

    /// Run every stage at once and return when each has a model that passed its check (or its floor).
    public func run() async {
        lock.lock(); if started && !finished { lock.unlock(); return }; started = true; finished = false; lock.unlock()
        await withTaskGroup(of: Void.self) { g in
            for s in SetupStage.allCases { g.addTask { [self] in await runStage(s) } }
        }
        lock.lock(); finished = true; lock.unlock(); onChange?()
    }

    /// "Try again" beside a stalled or failed row.
    public func retry(_ s: SetupStage) {
        lock.lock(); let st = rowsByStage[s]?.state; lock.unlock()
        if st == "stalled" || st == "failed" { deliver(s, .retry); return }
        guard st == "ready", let first = plan[s]?.first else { return }
        // A finished row with "Try again": run that stage again from the top of its ladder.
        Task { [self] in
            lock.locked { items[s] = nil; finished = false }
            update(s) { $0 = Self.freshRow(s, first) }
            await runStage(s)
            lock.locked { finished = true }; onChange?()
        }
    }
    /// "Use the fallback" beside a stalled or failed row.
    public func useFallback(_ s: SetupStage) { deliver(s, .fallback) }

    private func deliver(_ s: SetupStage, _ a: UserAction) {
        lock.lock()
        if let c = waiting.removeValue(forKey: s) { waitGen[s, default: 0] += 1; lock.unlock(); c.resume(returning: a); return }   // failure 5: one action per wait
        let row = rowsByStage[s]
        if row?.state == "stalled" || row?.state == "failed" { early[s] = a }
        lock.unlock()
    }

    private func timeout(_ s: SetupStage, gen: Int) {
        lock.lock(); let live = waitGen[s, default: 0] == gen && waiting[s] != nil; lock.unlock()
        if live { deliver(s, .fallback) }
    }

    private func update(_ s: SetupStage, _ f: (inout SetupRow) -> Void) {
        lock.lock(); if var r = rowsByStage[s] { f(&r); rowsByStage[s] = r }; lock.unlock(); onChange?()
    }

    private func runStage(_ s: SetupStage) async {
        let ladder = plan[s]!
        var fellFrom: [String] = [], reasons: [String] = []
        for (i, cand) in ladder.enumerated() {
            let isLast = i == ladder.count - 1
            update(s) { r in
                let fresh = Self.freshRow(s, cand)
                r.model = fresh.model; r.sizeLabel = fresh.sizeLabel; r.progress = 0; r.isFallback = i > 0 || r.isFallback
                r.state = "waiting"; r.canRetry = false; r.canFallback = false; r.numbers = ""
            }
            var tries = 0
            var downloaded = cand.isFloor
            var giveUp = false
            while !downloaded && !giveUp {
                tries += 1
                update(s) { r in r.state = "downloading"; r.statusLine = "Downloading"; r.canRetry = false; r.canFallback = false }
                lock.locked { lastBytesAt[s] = now(); early[s] = nil }
                let outcome = await download(s, cand)
                switch outcome {
                case .ok: downloaded = true
                case .cancelled: return
                case .stalled, .failed:
                    var stalled = true
                    var why = "No progress for \(Int(stallSeconds)) seconds"
                    if case .failed(let m) = outcome { why = "The download failed (\(m))"; stalled = false }
                    if tries >= maxTries || isLast {
                        reasons.append(why); fellFrom.append(cand.id); giveUp = true
                    } else {
                        update(s) { r in r.state = stalled ? "stalled" : "failed"; r.statusLine = why; r.canRetry = true; r.canFallback = true }
                        let a = await waitForUser(s)
                        if a == .retry { continue }
                        reasons.append(why); fellFrom.append(cand.id); giveUp = true
                    }
                }
            }
            if !downloaded {
                if isLast { markFloorOnly(s, ladder, fellFrom, reasons); return }
                continue
            }
            // Check on this device.
            update(s) { r in r.state = "preparing"; r.progress = 1; r.statusLine = "Preparing for this phone"; r.canRetry = false; r.canFallback = false }
            let check: SetupCheck
            do { check = try await backend.check(cand) }
            catch { check = SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: "\(error)") }
            if check.outputOK {
                let numbers = Self.numbers(check)
                update(s) { r in
                    r.state = "ready"; r.progress = 1; r.numbers = numbers
                    r.statusLine = r.isFallback ? "Using \(cand.label). \(reasons.last ?? "")" : "Ready"
                    r.canRetry = r.isFallback; r.canFallback = false
                }
                let item = SetupRecord.Item(stage: s.rawValue, chosen: cand.id, kitStageID: cand.kitStageID, fellBackFrom: fellFrom, reasons: reasons, check: check)
                lock.locked { items[s] = item }
                onChange?()
                return
            }
            reasons.append(check.note.isEmpty ? "It did not pass its check on this phone" : check.note); fellFrom.append(cand.id)   // failure 4
            if isLast { markFloorOnly(s, ladder, fellFrom, reasons); return }
        }
    }

    /// Every candidate failed, the floor included: say so, and keep the floor as the choice (it exists).
    private func markFloorOnly(_ s: SetupStage, _ ladder: [SetupCandidate], _ from: [String], _ reasons: [String]) {
        let floor = ladder.last!
        update(s) { r in
            r.model = floor.label; r.sizeLabel = SetupCatalog.sizeLabel(floor.bytes); r.state = "ready"; r.progress = 1; r.isFallback = true
            r.statusLine = "Using \(floor.label). \(reasons.last ?? "")"; r.canRetry = true; r.canFallback = false
        }
        lock.locked { items[s] = SetupRecord.Item(stage: s.rawValue, chosen: floor.id, kitStageID: floor.kitStageID, fellBackFrom: from, reasons: reasons, check: nil) }
    }

    static func numbers(_ c: SetupCheck) -> String {
        let speed = c.unit.isEmpty ? "" : String(format: "%.2f ", c.speed) + c.unit
        let load = c.loadMs >= 1000 ? String(format: "loaded in %.1f s", c.loadMs / 1000) : String(format: "loaded in %.0f ms", c.loadMs)
        let mem = c.memoryMB > 0 ? String(format: "%.0f MB", c.memoryMB) : ""
        return [speed, load, mem].filter { !$0.isEmpty }.joined(separator: " · ")
    }

    private func download(_ s: SetupStage, _ cand: SetupCandidate) async -> Outcome {
        let backend = self.backend, stallSeconds = self.stallSeconds, now = self.now
        return await withTaskGroup(of: Outcome.self) { g in
            g.addTask { [self] in
                do {
                    try await backend.download(cand) { [self] bytes in
                        lock.lock(); lastBytesAt[s] = now(); lock.unlock()
                        let p = min(1, Double(bytes) / Double(max(cand.bytes, 1)))
                        update(s) { r in
                            r.progress = p
                            r.statusLine = "Downloading: \(Self.mb(bytes)) of \(Self.mb(cand.bytes))"
                            if r.state == "stalled" { r.state = "downloading" }
                        }
                    }
                    return .ok
                } catch is CancellationError { return .cancelled }
                catch { return Task.isCancelled ? .cancelled : .failed(Self.short("\(error)")) }
            }
            g.addTask { [self] in
                while !Task.isCancelled {
                    try? await Task.sleep(nanoseconds: 200_000_000)
                    let last = lock.locked { lastBytesAt[s] ?? now() }
                    if now() - last > stallSeconds { return .stalled }
                }
                return .cancelled
            }
            let first = await g.next() ?? .cancelled
            g.cancelAll()
            return first
        }
    }

    static func mb(_ b: Int64) -> String { b >= 1_000_000_000 ? String(format: "%.1f GB", Double(b) / 1e9) : "\(Int((Double(b) / 1e6).rounded())) MB" }
    static func short(_ s: String) -> String { String(s.prefix(60)) }

    private func waitForUser(_ s: SetupStage) async -> UserAction {
        if let a = lock.locked({ early.removeValue(forKey: s) }) { return a }
        let patience = self.patience
        return await withCheckedContinuation { (c: CheckedContinuation<UserAction, Never>) in
            lock.lock(); waiting[s] = c; let gen = waitGen[s, default: 0]; lock.unlock()
            Task { [weak self] in
                try? await Task.sleep(nanoseconds: UInt64(patience * 1e9))
                self?.timeout(s, gen: gen)                                    // failure 2: never waits forever
            }
        }
    }
}

extension NSLock {
    func locked<T>(_ body: () -> T) -> T { lock(); defer { unlock() }; return body() }
}

// MARK: Backends

/// The demo and the simulator proof: a scripted download and check (fixtures/demo/setup-script.json), with one stall and
/// one failed check so those screens are proven. No network, no model, no microphone.
public final class SimulatedSetupBackend: SetupBackend, @unchecked Sendable {
    public struct Model: Codable, Sendable {
        public var chunks: Int
        public var stallAt: Int?
        public var checkOK: Bool?
        public var checkNote: String?
        public var checkMs: Int?
    }
    public struct Script: Codable, Sendable {
        public var tickMs: Int
        public var stallSeconds: Double?
        public var profile: DeviceProfile
        public var models: [String: Model]
        public var check: SetupCheck
    }
    public let script: Script
    private let lock = NSLock()
    private var stalledOnce = Set<String>()
    public init(script: Script) { self.script = script }
    public static func parse(_ data: Data) -> Script? { try? JSONDecoder().decode(Script.self, from: data) }

    public func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
        let m = script.models[c.id] ?? Model(chunks: 4)
        let per = c.bytes / Int64(max(m.chunks, 1))
        for k in 1...max(m.chunks, 1) {
            try await Task.sleep(nanoseconds: UInt64(script.tickMs) * 1_000_000)
            onBytes(per * Int64(k))
            if let at = m.stallAt, k == at {
                let first: Bool = lock.locked { stalledOnce.insert(c.id).inserted }
                if first { while true { try await Task.sleep(nanoseconds: 1_000_000_000) } }     // silent stall until cancelled
            }
        }
        onBytes(c.bytes)
    }

    public func check(_ c: SetupCandidate) async throws -> SetupCheck {
        let m = script.models[c.id]
        try await Task.sleep(nanoseconds: UInt64(m?.checkMs ?? 700) * 1_000_000)
        var out = script.check
        if c.isFloor { out.loadMs = 12; out.memoryMB = 0 }
        if m?.checkOK == false { out.outputOK = false; out.note = m?.checkNote ?? "It did not pass its check on this phone." }
        else { out.outputOK = true }
        return out
    }
}

/// The real downloader: pinned models go through the ModelStore (HTTP Range resume, SHA-256). Library-managed models
/// (FluidAudio's Core ML bundles) are fetched by their own loader inside `fetch`, which the app module supplies, and
/// checked by `checker`, also supplied by the app module (it can import the model stages). With no checker the check
/// says it did not run, so the card never claims a model works when nothing ran it.
public final class LiveSetupBackend: SetupBackend, @unchecked Sendable {
    public typealias Fetch = @Sendable (SetupCandidate, @escaping @Sendable (Int64) -> Void) async throws -> Void
    public typealias Checker = @Sendable (SetupCandidate) async throws -> SetupCheck
    let store: ModelStore
    let fetch: Fetch?
    let checker: Checker?
    public init(store: ModelStore, fetch: Fetch? = nil, checker: Checker? = nil) { self.store = store; self.fetch = fetch; self.checker = checker }

    public func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
        if let fetch { try await fetch(c, onBytes); return }
        _ = try await store.ensure(c.id) { done, _ in onBytes(done) }
    }
    public func check(_ c: SetupCandidate) async throws -> SetupCheck {
        guard let checker else { return SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: "The check did not run on this phone.") }
        return try await checker(c)
    }
}
