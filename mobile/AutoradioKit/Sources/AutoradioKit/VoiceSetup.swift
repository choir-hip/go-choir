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
    case bounds, transcriber, voice, decision, responder

    /// Plain words for the listener.
    public var title: String {
        switch self {
        case .bounds: return "Knowing when you speak"
        case .transcriber: return "Hearing you"
        case .voice: return "The voice"
        case .decision: return "Quick decisions"
        case .responder:
            // The voice that talks while the desk works. Order is set by the Mac sample runs (2026-11-11, docs/reference/ON-DEVICE-MODELS.md):
            // Qwen3-0.6B Q8_0 first (the most page-grounded of the fast ones, and the file the decision role already loads, so its weights
            // cost nothing extra), then the others; LFM3 is last before the soft cue because its licence limits commercial use (read it) even
            // though it was the most faithful to the page. Setup times each on the phone; a candidate that is too slow or leaves too little
            // memory is passed over. The floor is the soft cue alone (no words).
            return [SetupCandidate(id: "qwen3-0.6b-q8_0", label: "A small language model", bytes: 639_446_688, minMemoryGB: 4,
                                   kitStageID: "local-ack", role: "responder", needsMB: 1000, minDecodeTokensPerSecond: 12),
                    SetupCandidate(id: "qwen3.5-0.8b-q4_k_m", label: "A newer small language model", bytes: 532_517_120, minMemoryGB: 4,
                                   kitStageID: "local-ack", role: "responder", needsMB: 800, minDecodeTokensPerSecond: 12),
                    SetupCandidate(id: "qwen3-0.6b-q4_k_m", label: "A small language model, light", bytes: 396_705_472, minMemoryGB: 4,
                                   kitStageID: "local-ack", role: "responder", needsMB: 600, minDecodeTokensPerSecond: 12),
                    SetupCandidate(id: "qwen3-1.7b-q4_k_m", label: "A larger small language model", bytes: 1_107_409_472, minMemoryGB: 6,
                                   kitStageID: "local-ack", role: "responder", needsMB: 1400, minDecodeTokensPerSecond: 20),
                    SetupCandidate(id: "lfm3-1.7b-qad-q4_0", label: "A larger language model", bytes: 1_049_615_680, minMemoryGB: 6,
                                   kitStageID: "local-ack", role: "responder", needsMB: 1500, minDecodeTokensPerSecond: 25),
                    SetupCandidate(id: "cue-only", label: "A soft sound and no words", bytes: 0, minMemoryGB: 0, kitStageID: "local-ack")]
        }
    }

    /// The ladder for this phone: from the first candidate that fits its memory, to the floor.
    public static func plan(for d: DeviceProfile, smaller: Bool = false) -> [SetupStage: [SetupCandidate]] {
        var out: [SetupStage: [SetupCandidate]] = [:]
        for s in SetupStage.allCases {
            let all = ladder(s)
            // The device's own reading of what is left when it has one: every candidate that does not fit is passed over (the floor
            // always stays). Physical memory is only the fallback guess, as a starting point of the ladder.
            var rest: [SetupCandidate]
            if let avail = d.availableMemoryMB {
                rest = all.filter { $0.isFloor || $0.needsMB == 0 || $0.needsMB <= avail - 600 }
            } else {
                rest = Array(all.drop(while: { $0.minMemoryGB > d.memoryGB }))
            }
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
    /// Measured on this device when the model is a language model: tokens per second reading the prompt, tokens per second writing.
    public var promptTokensPerSecond: Double?
    public var decodeTokensPerSecond: Double?
    /// Measured for a voice: ms from the text to the first sound.
    public var firstAudioMs: Double?
    /// What the process may still use after the model loaded (`os_proc_available_memory()`), and the phone's temperature then.
    public var availableMemoryMB: Double?
    public var thermal: String?
    public init(loadMs: Double, speed: Double, unit: String, memoryMB: Double, outputOK: Bool, note: String = "",
                promptTokensPerSecond: Double? = nil, decodeTokensPerSecond: Double? = nil, firstAudioMs: Double? = nil,
                availableMemoryMB: Double? = nil, thermal: String? = nil) {
        self.loadMs = loadMs; self.speed = speed; self.unit = unit; self.memoryMB = memoryMB; self.outputOK = outputOK; self.note = note
        self.promptTokensPerSecond = promptTokensPerSecond; self.decodeTokensPerSecond = decodeTokensPerSecond; self.firstAudioMs = firstAudioMs
        self.availableMemoryMB = availableMemoryMB; self.thermal = thermal
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

    /// The measured numbers in plain words, for the setup result screen.
    static func numbers(_ c: SetupCheck) -> String {
        var parts: [String] = []
        if !c.unit.isEmpty { parts.append(String(format: "%.2f ", c.speed) + c.unit) }
        if let p = c.promptTokensPerSecond { parts.append("reads about \(Int((p * 0.75).rounded())) words a second") }
        if let d = c.decodeTokensPerSecond { parts.append("writes about \(Int((d * 0.75).rounded())) words a second") }
        if let f = c.firstAudioMs { parts.append("first sound in \(Int(f.rounded())) ms") }
        parts.append(c.loadMs >= 1000 ? String(format: "loaded in %.1f s", c.loadMs / 1000) : String(format: "loaded in %.0f ms", c.loadMs))
        if c.memoryMB > 0 { parts.append(String(format: "%.0f MB in use", c.memoryMB)) }
        if let a = c.availableMemoryMB { parts.append(String(format: "%.0f MB left", a)) }
        if let t = c.thermal, t != "nominal", !t.isEmpty { parts.append("the phone is \(t == "fair" ? "warm" : "hot")") }
        return parts.joined(separator: " · ")
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


/// The real downloader. Pinned models (the small language model) go through the one shared ModelStore: HTTP Range resume and a
/// SHA-256 check against the pinned digest. Library-managed models (FluidAudio's Core ML bundles) are fetched, checksummed and
/// compiled by the app module's `ModelProvider`, which also runs the on-device check. With no provider installed (a build that
/// links no model libraries) the row says exactly that; it never says a model "is not registered" and never claims a check ran.
public final class LiveSetupBackend: SetupBackend, @unchecked Sendable {
    public typealias Fetch = @Sendable (SetupCandidate, @escaping @Sendable (Int64) -> Void) async throws -> Void
    public typealias Checker = @Sendable (SetupCandidate) async throws -> SetupCheck
    let store: ModelStore
    let fetch: Fetch?
    let checker: Checker?
    public init(store: ModelStore = .shared, fetch: Fetch? = nil, checker: Checker? = nil) { self.store = store; self.fetch = fetch; self.checker = checker }

    public func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
        if let fetch { try await fetch(c, onBytes); return }
        guard KnownModels.contains(c.id) else { throw LabError.badInput("setup asked for \(c.id), which is not in the model catalog") }
        if KnownModels.isPinned(c.id) {
            _ = try await store.ensure(c.id) { done, _ in onBytes(done) }      // download, resume, SHA-256
            return
        }
        guard let p = ModelProviders.current else { throw LabError.unavailable(ModelProviders.missingNote) }
        try await p.fetch(c.id, role: c.role, onBytes: onBytes)
    }

    public func check(_ c: SetupCandidate) async throws -> SetupCheck {
        if let checker { return try await checker(c) }
        if c.isFloor { return await Self.checkFloor(c) }
        guard let p = ModelProviders.current else {
            return SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: ModelProviders.missingNote)
        }
        var r = try await p.check(c.id, role: c.role)
        // What the device measured decides: too slow to be worth it, or too little memory left, is a failed check (setup moves on).
        if r.outputOK, let floor = c.minDecodeTokensPerSecond, let d = r.decodeTokensPerSecond, d < floor {
            r.outputOK = false
            r.note = "Too slow on this phone: it writes about \(Int((d * 0.75).rounded())) words a second."
        }
        if r.outputOK, let left = r.availableMemoryMB, left < 250 {
            r.outputOK = false
            r.note = "Too little memory would be left (\(Int(left)) MB)."
        }
        return r
    }

    /// The floor of each ladder needs no download. The level detector is checked for real (it must find speech in the bundled
    /// utterance); the others are the system's own and run when used, which the note says.
    static func checkFloor(_ c: SetupCandidate) async -> SetupCheck {
        if c.id == "energy" {
            let t0 = DispatchTime.now().uptimeNanoseconds
            guard let samples = FixtureTurn.micSamples16k() else {
                return SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: "The bundled utterance could not be read.")
            }
            let vad = EnergyVAD(StageParams())
            var started = false
            var i = 0
            while i < samples.count {
                let f = AudioFrame(samples: Array(samples[i..<min(samples.count, i + 512)]), sampleRate: 16000, t: Double(i) / 16000)
                if case .speechStarted = await vad.feed(f) { started = true }
                i += 512
            }
            let ms = Double(DispatchTime.now().uptimeNanoseconds - t0) / 1e6
            return SetupCheck(loadMs: 0, speed: ms / max(1, Double(samples.count) / 16), unit: "× real time", memoryMB: 0, outputOK: started,
                              note: started ? "" : "The level detector found no speech in the bundled utterance.")
        }
        return SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: true, note: "Built in; it runs when it is used.")
    }
}
