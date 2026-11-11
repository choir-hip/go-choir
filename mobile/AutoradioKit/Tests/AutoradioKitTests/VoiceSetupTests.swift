import XCTest
@testable import AutoradioKit

// The failure list is at the top of VoiceSetup.swift (numbers below refer to it).
final class VoiceSetupTests: XCTestCase {
    func device(_ gb: Double, disk: Double = 40, wifi: Bool = true) -> DeviceProfile { DeviceProfile(chip: "test", memoryGB: gb, freeDiskGB: disk, onWiFi: wifi) }

    func script(_ models: [String: SimulatedSetupBackend.Model] = [:]) -> SimulatedSetupBackend.Script {
        SimulatedSetupBackend.Script(tickMs: 5, stallSeconds: nil, profile: device(8), models: models,
                                     check: SetupCheck(loadMs: 900, speed: 0.08, unit: "x real time", memoryMB: 300, outputOK: true))
    }

    func testPlanStartsWhereThePhoneFitsAndEndsOnAFloor() {
        // failure 1
        let big = SetupCatalog.plan(for: device(8))
        XCTAssertEqual(big[.transcriber]!.map(\.id), ["parakeet-tdt-0.6b-v2", "parakeet-eou-120m", "apple-speech"])
        let mid = SetupCatalog.plan(for: device(4))
        XCTAssertEqual(mid[.transcriber]!.first?.id, "parakeet-eou-120m")
        XCTAssertEqual(mid[.voice]!.first?.id, "kokoro-82m-ane")
        let small = SetupCatalog.plan(for: device(2))
        XCTAssertEqual(small[.transcriber]!.map(\.id), ["apple-speech"])
        XCTAssertEqual(small[.voice]!.map(\.id), ["system-voice"])
        for s in SetupStage.allCases { XCTAssertTrue(SetupCatalog.plan(for: device(0.5))[s]!.last!.isFloor, "every ladder ends in something that exists") }
        XCTAssertEqual(SetupCatalog.plan(for: device(8), smaller: true)[.transcriber]!.first?.id, "parakeet-eou-120m")
    }

    func testTotalIsSaidPlainly() {
        let big = SetupCatalog.plan(for: device(8))
        XCTAssertEqual(SetupCatalog.totalBytes(big), 2_000_000 + 480_000_000 + 350_000_000)
        XCTAssertEqual(SetupCatalog.totalLabel(bytes: SetupCatalog.totalBytes(big), onWiFi: true), "About 830 MB. On Wi-Fi this takes a few minutes.")
        XCTAssertTrue(SetupCatalog.totalLabel(bytes: 1_400_000_000, onWiFi: false).contains("1.4 GB"))
        XCTAssertTrue(SetupCatalog.totalLabel(bytes: 1_400_000_000, onWiFi: false).contains("mobile data"), "failure 7")
        XCTAssertTrue(SetupCatalog.totalLabel(bytes: 0, onWiFi: true).hasPrefix("Nothing to download"))
        XCTAssertEqual(SetupCatalog.sizeLabel(480_000_000), "480 MB"); XCTAssertEqual(SetupCatalog.sizeLabel(0), "Built in")
    }

    func testEverythingPassesAndTheRecordHoldsTheNumbers() async {
        let s = VoiceSetup(backend: SimulatedSetupBackend(script: script()), profile: device(8), stallSeconds: 5, patience: 5)
        await s.run()
        XCTAssertTrue(s.isFinished)
        XCTAssertTrue(s.rows.allSatisfy { $0.state == "ready" && !$0.isFallback }, "\(s.rows)")
        XCTAssertEqual(s.choices[.transcriber], "parakeet-tdt")
        XCTAssertEqual(s.choices[.system1], "choir-system1")
        let rec = s.record
        XCTAssertEqual(rec.items.count, 4)
        XCTAssertEqual(rec.items.first { $0.stage == "transcriber" }?.check?.speed, 0.08)
        XCTAssertTrue(s.rows.first { $0.stage == "transcriber" }!.numbers.contains("0.08 x real time"))
        XCTAssertNotNil(try? JSONEncoder().encode(rec))
    }

    func testStallIsReportedAndFallbackOnRequest() async throws {
        // failure 2
        let m = SimulatedSetupBackend.Model(chunks: 10, stallAt: 3, checkOK: nil, checkNote: nil, checkMs: nil)
        let s = VoiceSetup(backend: SimulatedSetupBackend(script: script(["parakeet-tdt-0.6b-v2": m])), profile: device(8), stallSeconds: 0.4, patience: 30)
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
        XCTAssertEqual(s.choices[.transcriber], "parakeet-eou", "the smaller listener took over")
        XCTAssertEqual(s.record.items.first { $0.stage == "transcriber" }?.fellBackFrom, ["parakeet-tdt-0.6b-v2"])
    }

    func testRetryAfterAStallResumesAndPasses() async throws {
        let m = SimulatedSetupBackend.Model(chunks: 6, stallAt: 2, checkOK: nil, checkNote: nil, checkMs: nil)   // stalls on the first attempt only
        let s = VoiceSetup(backend: SimulatedSetupBackend(script: script(["parakeet-tdt-0.6b-v2": m])), profile: device(8), stallSeconds: 0.3, patience: 30)
        let run = Task { await s.run() }
        for _ in 0..<100 {
            try await Task.sleep(nanoseconds: 50_000_000)
            if s.rows.first(where: { $0.stage == "transcriber" })?.state == "stalled" { s.retry(.transcriber); s.retry(.transcriber); break }   // failure 5: twice is once
        }
        await run.value
        let after = s.rows.first { $0.stage == "transcriber" }!
        XCTAssertEqual(after.state, "ready"); XCTAssertFalse(after.isFallback)
        XCTAssertEqual(s.choices[.transcriber], "parakeet-tdt")
    }

    func testNoAnswerTakesTheFallbackAfterPatience() async {
        // failure 2, 3
        let m = SimulatedSetupBackend.Model(chunks: 10, stallAt: 2, checkOK: nil, checkNote: nil, checkMs: nil)
        let s = VoiceSetup(backend: SimulatedSetupBackend(script: script(["kokoro-82m-ane": m])), profile: device(8), stallSeconds: 0.3, patience: 0.4)
        await s.run()
        XCTAssertEqual(s.choices[.voice], "pocket-tts")
        XCTAssertTrue(s.rows.first { $0.stage == "voice" }!.isFallback)
    }

    func testAModelThatFailsItsCheckIsReplacedWithoutAsking() async {
        // failure 4
        let m = SimulatedSetupBackend.Model(chunks: 3, stallAt: nil, checkOK: false, checkNote: "The voice made audio of the wrong length.", checkMs: nil)
        let s = VoiceSetup(backend: SimulatedSetupBackend(script: script(["kokoro-82m-ane": m])), profile: device(8), stallSeconds: 5, patience: 5)
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
        let s = VoiceSetup(backend: Offline(), profile: device(8), stallSeconds: 5, patience: 0.05, maxTries: 1)
        await s.run()
        XCTAssertEqual(s.choices[.transcriber], "apple-speech")
        XCTAssertEqual(s.choices[.voice], "apple-voice")
        XCTAssertEqual(s.choices[.speechBounds], "energy-bounds")
        XCTAssertEqual(s.choices[.system1], "choir-system1")
        XCTAssertTrue(s.rows.allSatisfy { $0.state == "ready" }, "\(s.rows)")
        XCTAssertTrue(s.summary.contains { $0.contains("(fallback)") })
    }

    func testScriptFixtureParses() throws {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("fixtures/demo/setup-script.json")
        let sc = try XCTUnwrap(SimulatedSetupBackend.parse(Data(contentsOf: url)))
        XCTAssertNotNil(sc.models["parakeet-tdt-0.6b-v2"]?.stallAt, "the demo script has one stall")
        XCTAssertEqual(sc.models["kokoro-82m-ane"]?.checkOK, false, "and one failed check")
    }
}
