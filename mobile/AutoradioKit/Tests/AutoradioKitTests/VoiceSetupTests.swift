import XCTest
@testable import AutoradioKit

// The failure list is at the top of VoiceSetup.swift (numbers below refer to it).
final class VoiceSetupTests: XCTestCase {
    func device(_ gb: Double, disk: Double = 40, wifi: Bool = true) -> DeviceProfile { DeviceProfile(chip: "test", memoryGB: gb, freeDiskGB: disk, onWiFi: wifi) }

    func script(_ models: [String: ScriptedSetupBackend.Model] = [:]) -> ScriptedSetupBackend.Script {
        ScriptedSetupBackend.Script(tickMs: 5, stallSeconds: nil, profile: device(8), models: models,
                                     check: SetupCheck(loadMs: 900, speed: 0.08, unit: "x real time", memoryMB: 300, outputOK: true))
    }

    func testPlanStartsWhereThePhoneFitsAndEndsOnAFloor() {
        // failure 1
        let big = SetupCatalog.plan(for: device(8))
        XCTAssertEqual(big[.transcriber]!.map(\.id), ["fluid-speech-pro", "fluid-speech-mini", "parakeet-tdt-0.6b-v2", "parakeet-eou-120m", "apple-speech"])
        let six = SetupCatalog.plan(for: device(5.6))       // an iPhone 14 Pro Max or 15 reads about 5.6 GB: Mini, not Pro
        XCTAssertEqual(six[.transcriber]!.first?.id, "fluid-speech-mini", "Pro is recommended from the 8 GB class (about 7.4 GB read) up")
        XCTAssertEqual(SetupCatalog.plan(for: device(7.4))[.transcriber]!.first?.id, "fluid-speech-pro")
        XCTAssertEqual(SetupCatalog.plan(for: device(11.4))[.transcriber]!.first?.id, "fluid-speech-pro")
        var sixFree = device(5.6); sixFree.availableMemoryMB = 4000      // plenty free, still not Pro: the phone is too small for it
        XCTAssertEqual(SetupCatalog.plan(for: sixFree)[.transcriber]!.first?.id, "fluid-speech-mini")
        let mid = SetupCatalog.plan(for: device(4))
        XCTAssertEqual(mid[.transcriber]!.first?.id, "fluid-speech-mini")
        XCTAssertEqual(SetupCatalog.plan(for: device(3))[.transcriber]!.first?.id, "parakeet-eou-120m", "a 3 GB phone starts at the smaller listener")
        XCTAssertEqual(mid[.voice]!.first?.id, "kokoro-82m-ane")
        let small = SetupCatalog.plan(for: device(2))
        XCTAssertEqual(small[.transcriber]!.map(\.id), ["apple-speech"])
        XCTAssertEqual(small[.voice]!.map(\.id), ["system-voice"])
        for s in SetupStage.allCases { XCTAssertTrue(SetupCatalog.plan(for: device(0.5))[s]!.last!.isFloor, "every ladder ends in something that exists") }
        XCTAssertEqual(SetupCatalog.plan(for: device(8), smaller: true)[.transcriber]!.first?.id, "fluid-speech-mini")
    }

    func testTotalIsSaidPlainly() {
        let big = SetupCatalog.plan(for: device(8))
        XCTAssertEqual(SetupCatalog.totalBytes(big), 2_000_000 + KnownModels.fluidSpeechPro.totalBytes + 250_000_000 + 350_000_000 + 639_446_688, "the decision and the responder share one file: counted once")
        XCTAssertEqual(SetupCatalog.totalLabel(bytes: SetupCatalog.totalBytes(big), onWiFi: true), "About 1.9 GB. On Wi-Fi this takes a few minutes.")
        XCTAssertTrue(SetupCatalog.totalLabel(bytes: 1_400_000_000, onWiFi: false).contains("1.4 GB"))
        XCTAssertTrue(SetupCatalog.totalLabel(bytes: 1_400_000_000, onWiFi: false).contains("mobile data"), "failure 7")
        XCTAssertTrue(SetupCatalog.totalLabel(bytes: 0, onWiFi: true).hasPrefix("Nothing to download"))
        XCTAssertEqual(SetupCatalog.sizeLabel(480_000_000), "480 MB"); XCTAssertEqual(SetupCatalog.sizeLabel(0), "Built in")
    }

    func testEverythingPassesAndTheRecordHoldsTheNumbers() async {
        let s = VoiceSetup(backend: ScriptedSetupBackend(script: script()), profile: device(8), stallSeconds: 5, patience: 5, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))
        await s.run()
        XCTAssertTrue(s.isFinished)
        XCTAssertTrue(s.rows.allSatisfy { $0.state == "ready" && !$0.isFallback }, "\(s.rows)")
        XCTAssertEqual(s.choices[.transcriber], "live-then-final", "Fluid Speech Mini with live words comes first")
        XCTAssertEqual(s.choices[.system1], "llama-qwen3-0.6b", "the small language model on the phone comes first")
        let rec = s.record
        XCTAssertEqual(rec.items.count, 5)
        XCTAssertEqual(rec.items.first { $0.stage == "transcriber" }?.check?.speed, 0.08)
        XCTAssertTrue(s.rows.first { $0.stage == "transcriber" }!.numbers.contains("0.08 x real time"))
        XCTAssertNotNil(try? JSONEncoder().encode(rec))
    }

    func testStallIsReportedAndFallbackOnRequest() async throws {
        // failure 2
        let m = ScriptedSetupBackend.Model(chunks: 10, stallAt: 3, checkOK: nil, checkNote: nil, checkMs: nil)
        let s = VoiceSetup(backend: ScriptedSetupBackend(script: script(["fluid-speech-mini": m])), profile: device(5.6), stallSeconds: 0.4, patience: 30, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))   // a 6 GB phone: Mini is first
        let run = Task { await s.run() }
        var stalled = false
        for _ in 0..<100 {
            try await Task.sleep(nanoseconds: 50_000_000)
            if s.rows.first(where: { $0.stage == "transcriber" })?.state == "stalled" { stalled = true; break }
        }
        XCTAssertTrue(stalled, "a row that makes no progress says so")
        let row = s.rows.first { $0.stage == "transcriber" }!
        XCTAssertTrue(row.canRetry && row.canFallback); XCTAssertTrue(row.statusLine.contains("No progress"))
        s.useFallback(.transcriber)
        await run.value
        let after = s.rows.first { $0.stage == "transcriber" }!
        XCTAssertEqual(after.state, "ready"); XCTAssertTrue(after.isFallback)
        XCTAssertEqual(s.choices[.transcriber], "parakeet-tdt", "the next listener on the ladder took over")
        XCTAssertEqual(s.record.items.first { $0.stage == "transcriber" }?.fellBackFrom, ["fluid-speech-mini"])
    }

    func testRetryAfterAStallResumesAndPasses() async throws {
        let m = ScriptedSetupBackend.Model(chunks: 6, stallAt: 2, checkOK: nil, checkNote: nil, checkMs: nil)   // stalls on the first attempt only
        let s = VoiceSetup(backend: ScriptedSetupBackend(script: script(["fluid-speech-mini": m])), profile: device(8), stallSeconds: 0.3, patience: 30, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))
        let run = Task { await s.run() }
        for _ in 0..<100 {
            try await Task.sleep(nanoseconds: 50_000_000)
            if s.rows.first(where: { $0.stage == "transcriber" })?.state == "stalled" { s.retry(.transcriber); s.retry(.transcriber); break }   // failure 5: twice is once
        }
        await run.value
        let after = s.rows.first { $0.stage == "transcriber" }!
        XCTAssertEqual(after.state, "ready"); XCTAssertFalse(after.isFallback)
        XCTAssertEqual(s.choices[.transcriber], "live-then-final")
    }

    func testNoAnswerTakesTheFallbackAfterPatience() async {
        // failure 2, 3
        let m = ScriptedSetupBackend.Model(chunks: 10, stallAt: 2, checkOK: nil, checkNote: nil, checkMs: nil)
        let s = VoiceSetup(backend: ScriptedSetupBackend(script: script(["kokoro-82m-ane": m])), profile: device(8), stallSeconds: 0.3, patience: 0.4, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))
        await s.run()
        XCTAssertEqual(s.choices[.voice], "pocket-tts")
        XCTAssertTrue(s.rows.first { $0.stage == "voice" }!.isFallback)
    }

    func testAModelThatFailsItsCheckIsReplacedWithoutAsking() async {
        // failure 4
        let m = ScriptedSetupBackend.Model(chunks: 3, stallAt: nil, checkOK: false, checkNote: "The voice made audio of the wrong length.", checkMs: nil)
        let s = VoiceSetup(backend: ScriptedSetupBackend(script: script(["kokoro-82m-ane": m])), profile: device(8), stallSeconds: 5, patience: 5, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))
        await s.run()
        let row = s.rows.first { $0.stage == "voice" }!
        XCTAssertEqual(row.state, "ready"); XCTAssertTrue(row.isFallback)
        XCTAssertTrue(row.statusLine.contains("wrong length"), row.statusLine)
        XCTAssertEqual(s.record.items.first { $0.stage == "voice" }?.reasons, ["The voice made audio of the wrong length."])
        XCTAssertTrue(s.summary.contains("The voice: Pocket voice, the lighter one (fallback)"), "\(s.summary)")
    }

    struct Offline: SetupBackend {
        func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws { throw URLError(.notConnectedToInternet) }
        func check(_ c: SetupCandidate) async throws -> SetupCheck { SetupCheck(loadMs: 5, speed: 0.1, unit: "x real time", memoryMB: 0, outputOK: true) }
    }

    func testOfflineEndsOnTheFloorsWhichExist() async {
        // failure 9
        let s = VoiceSetup(backend: Offline(), profile: device(8), stallSeconds: 5, patience: 0.05, maxTries: 1, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))
        await s.run()
        XCTAssertEqual(s.choices[.transcriber], "apple-speech")
        XCTAssertEqual(s.choices[.voice], "avspeech")
        XCTAssertEqual(s.choices[.speechBounds], "energy-vad")
        XCTAssertEqual(s.choices[.system1], "choir-system1")
        XCTAssertTrue(s.rows.allSatisfy { $0.state == "ready" }, "\(s.rows)")
        XCTAssertTrue(s.summary.contains { $0.contains("(fallback)") })
    }
}

/// A scripted download and check for the tests, with a stall and a failed check on demand. No network, no model, no microphone.
final class ScriptedSetupBackend: SetupBackend, @unchecked Sendable {
    struct Model: Codable, Sendable {
        var chunks: Int
        var stallAt: Int?
        var checkOK: Bool?
        var checkNote: String?
        var checkMs: Int?
    }
    struct Script: Codable, Sendable {
        var tickMs: Int
        var stallSeconds: Double?
        var profile: DeviceProfile
        var models: [String: Model]
        var check: SetupCheck
    }
    let script: Script
    private let lock = NSLock()
    private var stalledOnce = Set<String>()
    init(script: Script) { self.script = script }
    static func parse(_ data: Data) -> Script? { try? JSONDecoder().decode(Script.self, from: data) }

    func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
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

    func check(_ c: SetupCandidate) async throws -> SetupCheck {
        let m = script.models[c.id]
        try await Task.sleep(nanoseconds: UInt64(m?.checkMs ?? 700) * 1_000_000)
        var out = script.check
        if c.isFloor { out.loadMs = 12; out.memoryMB = 0 }
        if m?.checkOK == false { out.outputOK = false; out.note = m?.checkNote ?? "It did not pass its check on this phone." }
        else { out.outputOK = true }
        return out
    }
}

// ONE readiness record, written only after the on-device check passed, read by the page and by the engine (review round 2, B2).
final class SetupReadinessTests: XCTestCase {
    func device(_ gb: Double) -> DeviceProfile { DeviceProfile(chip: "test", memoryGB: gb, freeDiskGB: 40, onWiFi: true) }
    struct Backend: SetupBackend {
        let rejects: Set<String>
        func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws {}
        func check(_ c: SetupCandidate) async throws -> SetupCheck {
            SetupCheck(loadMs: 5, speed: 0.1, unit: "x real time", memoryMB: 0, outputOK: !rejects.contains(c.id), note: rejects.contains(c.id) ? "Too little memory would be left (200 MB)" : "")
        }
    }
    func testR1AModelTheCheckRejectedIsNotReady_AndTheOneThatPassedIs() async {
        let ready = MemoryVoiceReadiness(); ready.markReady(model: "fluid-speech-mini")           // a leftover mark (an older build)
        let s = VoiceSetup(backend: Backend(rejects: ["fluid-speech-mini"]), profile: device(8), stallSeconds: 5, patience: 0.05, maxTries: 1, verifier: ReadinessVerifier(record: ready))
        await s.run(only: [.transcriber])
        XCTAssertFalse(ready.isReady(model: "fluid-speech-mini"), "it failed its check on this phone: the listener must not choose it")
        let chosen = s.record.items.first { $0.stage == "transcriber" }?.chosen
        XCTAssertNotEqual(chosen, "fluid-speech-mini"); XCTAssertTrue(ready.isReady(model: chosen ?? "?") || chosen == "apple-speech")
    }
    func testR2APassingModelAndItsCompanionsAreReadyOnlyAfterTheCheck() async {
        let ready = MemoryVoiceReadiness()
        let s = VoiceSetup(backend: Backend(rejects: []), profile: device(8), stallSeconds: 5, patience: 0.05, maxTries: 1, verifier: ReadinessVerifier(record: ready))
        XCTAssertFalse(ready.isReady(model: "fluid-speech-mini"), "nothing is ready before setup ran")
        await s.run(only: [.transcriber])
        XCTAssertTrue(ready.isReady(model: "fluid-speech-mini"))
        for c in ListenerPipeline.companions(of: "fluid-speech-mini") { XCTAssertTrue(ready.isReady(model: c), "the live-words model comes with it") }
    }
    func testR3ThePageReadsTheSameRecord() {
        let ready = MemoryVoiceReadiness(); ready.markReady(model: "fluid-speech-mini")
        let choices = ModelOffer.readyChoices(isReady: { ready.isReady(model: $0) })
        XCTAssertEqual(choices["transcriber"], "fluid-speech-mini"); XCTAssertNil(choices["responder"])
        XCTAssertTrue(ModelOffer.readyStages(choices: choices).contains("transcriber"))
        ready.markNotReady(model: "fluid-speech-mini")
        XCTAssertNil(ModelOffer.readyChoices(isReady: { ready.isReady(model: $0) })["transcriber"], "what the engine will not use the page does not call ready")
    }
}

final class QueuedRowTests: XCTestCase {
    struct Queued: SetupBackend {
        func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws { onBytes(-1); try await Task.sleep(nanoseconds: 80_000_000); onBytes(-1); onBytes(5_000_000) }
        func check(_ c: SetupCandidate) async throws -> SetupCheck { SetupCheck(loadMs: 1, speed: 0.1, unit: "x", memoryMB: 0, outputOK: true) }
    }
    // Round 7: a fetch queued behind another says so; it is not "Downloading 0 MB", and the wait is not a stall.
    func testAQueuedFetchSaysItIsWaitingAndIsNotAStall() async {
        let s = VoiceSetup(backend: Queued(), profile: DeviceProfile(chip: "t", memoryGB: 8, freeDiskGB: 40, onWiFi: true), stallSeconds: 0.05, patience: 0.05, maxTries: 1, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))
        let t = Task { await s.run(only: [.bounds]) }
        try? await Task.sleep(nanoseconds: 40_000_000)
        XCTAssertTrue(s.rows.first { $0.stage == "bounds" }?.statusLine.contains("Waiting for the other download") == true, "\(s.rows.first { $0.stage == "bounds" }?.statusLine ?? "")")
        await t.value
        XCTAssertEqual(s.rows.first { $0.stage == "bounds" }?.state, "ready", "queued heartbeats kept the stall watchdog quiet")
    }
}

