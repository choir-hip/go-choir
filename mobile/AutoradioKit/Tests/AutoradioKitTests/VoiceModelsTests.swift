import XCTest
@testable import AutoradioKit

// The failure list is at the top of KnownModels.swift (numbers below refer to it) and VoiceSetup.swift.
// The first phone walk: "The download failed (bad input: model silero-vad-v6.2.1 not registered)" for every model, and
// "Quick decisions: Using Your computer. The check did not run on this phone." Cause, traced from the code: onboarding built
// `LiveSetupBackend(store: ModelStore(root:))`, a store nothing was registered into, with no fetch and no checker, so
// `store.ensure(id)` threw for every id and `check` returned "did not run". These tests fail on that build.

final class FakeProvider: ModelProvider, @unchecked Sendable {
    private let lock = NSLock()
    private(set) var fetched: [String] = [], checked: [String] = []
    var failCheck: Set<String> = []
    var prompt: [String: Double] = [:], decode: [String: Double] = [:], leftMB: [String: Double] = [:]
    func fetch(_ id: String, role: String, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
        lock.lock(); fetched.append(id); lock.unlock()
        onBytes(1_000_000); onBytes(2_000_000)
    }
    func check(_ id: String, role: String) async throws -> SetupCheck {
        lock.lock(); checked.append(id); lock.unlock()
        let ok = !failCheck.contains(id)
        return SetupCheck(loadMs: 800, speed: 0.1, unit: "of real time", memoryMB: 300, outputOK: ok, note: ok ? "" : "wrong output",
                          promptTokensPerSecond: prompt[id], decodeTokensPerSecond: decode[id], availableMemoryMB: leftMB[id])
    }
}

final class VoiceModelsTests: XCTestCase {
    override func setUp() { ModelProviders.current = nil }
    override func tearDown() { ModelProviders.current = nil }

    func device(_ gb: Double) -> DeviceProfile { DeviceProfile(chip: "test", memoryGB: gb, freeDiskGB: 40, onWiFi: true) }

    // failure 1: every id setup can ask for is in the catalog
    func testEverySetupCandidateIsInTheCatalog() {
        for st in SetupStage.allCases {
            for c in SetupCatalog.ladder(st) where !c.isFloor {
                XCTAssertTrue(KnownModels.contains(c.id), "\(st.rawValue): setup asks for \(c.id), which the catalog lacks")
            }
        }
    }

    func testCatalogHasNoDuplicatesAndPinnedChecksumsAreWellFormed() {
        XCTAssertEqual(Set(KnownModels.ids).count, KnownModels.ids.count)           // failure 2
        for p in KnownModels.pinned {                                                 // failure 3
            XCTAssertEqual(p.sha256.count, 64, p.id)
            XCTAssertTrue(p.sha256.allSatisfy { $0.isHexDigit }, p.id)
            XCTAssertGreaterThan(p.bytes, 0)
            XCTAssertTrue(KnownModels.contains(p.id), "a pinned model is also in the catalog")
        }
    }

    func testSetupSizesMatchTheCatalog() {
        for st in SetupStage.allCases {
            for c in SetupCatalog.ladder(st) where !c.isFloor {
                let spec = KnownModels.managed.first { $0.id == c.id }
                XCTAssertEqual(c.bytes, spec?.approxBytes, "\(c.id): the size setup promises is the catalog's")
            }
        }
    }

    func testTheSharedStoreHasTheWholeCatalogFromTheStart() async {
        for id in KnownModels.ids {
            let reg = await ModelStore.shared.isRegistered(id)
            XCTAssertTrue(reg, "\(id) is not registered in the shared store")
        }
    }

    func testAStoreNeverSaysNotRegisteredForACatalogModel() async {
        let store = ModelStore(root: FileManager.default.temporaryDirectory.appendingPathComponent("vm-\(UUID().uuidString)"), registerKnown: true)
        for m in KnownModels.managed where !KnownModels.isPinned(m.id) {
            do { _ = try await store.ensure(m.id); XCTFail("a library-managed model is not fetched by the store") }
            catch { XCTAssertFalse("\(error)".contains("not registered"), "\(m.id): \(error)") }
        }
        // And the old shape of the bug is still an error, so it cannot pass silently.
        let empty = ModelStore(root: FileManager.default.temporaryDirectory.appendingPathComponent("vm-\(UUID().uuidString)"))
        do { _ = try await empty.ensure("silero-vad-v6.2.1"); XCTFail() } catch { XCTAssertTrue("\(error)".contains("not registered")) }
    }

    // The live backend, without and with the app module's provider.
    func testWithoutLibrariesTheRowSaysSoPlainly() async throws {
        let b = LiveSetupBackend(store: .shared)
        let silero = try XCTUnwrap(SetupCatalog.ladder(.bounds).first)
        do { try await b.download(silero) { _ in }; XCTFail("nothing can fetch it") }
        catch { XCTAssertTrue("\(error)".contains("no on-device model libraries"), "\(error)"); XCTAssertFalse("\(error)".contains("not registered")) }
        let c = try await b.check(silero)
        XCTAssertFalse(c.outputOK)
        XCTAssertEqual(c.note, ModelProviders.missingNote)
        XCTAssertFalse(c.note.contains("did not run"))
    }

    func testWithTheProviderEveryNonFloorModelFetchesAndChecks() async throws {
        let p = FakeProvider(); ModelProviders.current = p
        let b = LiveSetupBackend(store: ModelStore(root: FileManager.default.temporaryDirectory.appendingPathComponent("vm-\(UUID().uuidString)"), registerKnown: true))
        for st in SetupStage.allCases {
            for c in SetupCatalog.ladder(st) where !c.isFloor && !KnownModels.isPinned(c.id) {
                let last = Locked<Int64>(0)
                try await b.download(c) { last.value = $0 }
                XCTAssertEqual(last.value, 2_000_000, "\(c.id): progress reached the row")
                let r = try await b.check(c)
                XCTAssertTrue(r.outputOK, c.id)
            }
        }
        XCTAssertEqual(Set(p.fetched), Set(p.checked))
        XCTAssertFalse(p.fetched.isEmpty)
    }

    func testAnIdOutsideTheCatalogIsNamedNotHidden() async {
        let b = LiveSetupBackend(store: .shared)
        let odd = SetupCandidate(id: "no-such-model", label: "x", bytes: 5, minMemoryGB: 0)
        do { try await b.download(odd) { _ in }; XCTFail() } catch { XCTAssertTrue("\(error)".contains("not in the model catalog"), "\(error)") }
    }

    // The floors run checks that mean something.
    func testTheLevelDetectorFloorFindsSpeechInTheBundledUtterance() async throws {
        let b = LiveSetupBackend(store: .shared)
        let floor = try XCTUnwrap(SetupCatalog.ladder(.bounds).last)
        let c = try await b.check(floor)
        XCTAssertTrue(c.outputOK, c.note)
    }

    func testTheOtherFloorsSayTheyAreBuiltInNotThatTheCheckDidNotRun() async throws {
        let b = LiveSetupBackend(store: .shared)
        for st in [SetupStage.transcriber, .voice, .decision] {
            let floor = try XCTUnwrap(SetupCatalog.ladder(st).last)
            let c = try await b.check(floor)
            XCTAssertTrue(c.outputOK, st.rawValue)
            XCTAssertTrue(c.note.contains("Built in"), c.note)
        }
    }

    // The whole setup, live backend, fake provider: the models are what is chosen, with the numbers from the check.
    func testSetupChoosesTheRealModelsWhenTheyPassAndFallsBackWhenOneFails() async {
        let p = FakeProvider(); p.failCheck = ["kokoro-82m-ane"]
        ModelProviders.current = p
        let store = ModelStore(root: FileManager.default.temporaryDirectory.appendingPathComponent("vm-\(UUID().uuidString)"), registerKnown: true)
        // The pinned model would download 639 MB: the test's fetch hook stands in for the store's HTTP, nothing else.
        let backend = LiveSetupBackend(store: store, fetch: { c, onBytes in
            if KnownModels.isPinned(c.id) { onBytes(c.bytes); return }
            try await p.fetch(c.id, role: c.role, onBytes: onBytes)
        })
        let s = VoiceSetup(backend: backend, profile: device(8), stallSeconds: 5, patience: 5)
        await s.run()
        XCTAssertEqual(s.choices[.speechBounds], "silero-vad")
        XCTAssertEqual(s.choices[.transcriber], "parakeet-tdt")
        XCTAssertEqual(s.choices[.voice], "pocket-tts", "Kokoro failed its check, the next on the ladder is used")
        XCTAssertEqual(s.choices[.system1], "llama-qwen3-0.6b")
        XCTAssertEqual(s.choices[.holder], "local-ack")
        XCTAssertEqual(s.record.items.first { $0.stage == "responder" }?.chosen, "qwen3-0.6b-q4_k_m", "the lighter quantisation is tried first")
        XCTAssertFalse(s.rows.contains { $0.statusLine.contains("did not run") || $0.statusLine.contains("not registered") }, "\(s.rows)")
        XCTAssertTrue(s.rows.first { $0.stage == "voice" }!.statusLine.contains("wrong output"))
    }

    func testTheDecisionLadderStartsWithTheLocalModelAndEndsAtTheComputer() {
        let ids = SetupCatalog.plan(for: device(8))[.decision]!.map(\.id)
        XCTAssertEqual(ids, ["qwen3-0.6b-q8_0", "system1-computer"])
        XCTAssertEqual(SetupCatalog.plan(for: device(3))[.decision]!.map(\.id), ["system1-computer"], "a phone with little memory starts at the floor")
    }

    func testTheResponderIsItsOwnStageWithQ4First() {
        let ids = SetupCatalog.plan(for: device(8))[.responder]!.map(\.id)
        XCTAssertEqual(ids, ["qwen3-0.6b-q4_k_m", "qwen3-0.6b-q8_0", "qwen3-1.7b-q4_k_m", "qwen3.5-0.8b-q4_k_m", "lfm3-1.7b-qad-q4_0", "cue-only"])
        XCTAssertEqual(SetupStage.responder.kind, .holder)
        XCTAssertTrue(SetupCatalog.ladder(.responder).filter { !$0.isFloor }.allSatisfy { $0.role == "responder" })
        XCTAssertTrue(SetupCatalog.ladder(.decision).filter { !$0.isFloor }.allSatisfy { $0.role == "decision" }, "setup can pick the two independently")
    }

    func testThePlanUsesTheDevicesOwnReadingOfAvailableMemory() {
        var tight = device(8); tight.availableMemoryMB = 1200       // plenty of physical memory, little left for this process (600 MB headroom is kept)
        let plan = SetupCatalog.plan(for: tight)
        XCTAssertEqual(plan[.responder]!.first?.id, "qwen3-0.6b-q4_k_m", "600 MB fits in 1200 minus headroom")
        XCTAssertFalse(plan[.responder]!.contains { $0.id == "lfm3-1.7b-qad-q4_0" }, "1.5 GB does not")
        XCTAssertEqual(plan[.transcriber]!.first?.id, "parakeet-eou-120m", "the larger listener does not fit what is left")
        var roomy = device(8); roomy.availableMemoryMB = 4000
        XCTAssertEqual(SetupCatalog.plan(for: roomy)[.transcriber]!.first?.id, "parakeet-tdt-0.6b-v2")
        XCTAssertEqual(DeviceProfile.current().availableMemoryMB == nil, true, "on the Mac there is no such reading (iOS only)")
    }

    func testTooSlowOrTooLittleMemoryLeftIsAFailedCheckSetupMovesOn() async throws {
        let p = FakeProvider(); p.decode = ["qwen3-0.6b-q4_k_m": 6, "qwen3-0.6b-q8_0": 30]; p.leftMB = ["qwen3-0.6b-q8_0": 900]
        ModelProviders.current = p
        let b = LiveSetupBackend(store: .shared)
        let slow = try await b.check(SetupCatalog.ladder(.responder)[0])
        XCTAssertFalse(slow.outputOK); XCTAssertTrue(slow.note.contains("Too slow"), slow.note)
        let fast = try await b.check(SetupCatalog.ladder(.responder)[1])
        XCTAssertTrue(fast.outputOK)
        p.leftMB = ["qwen3-0.6b-q8_0": 100]
        let tight = try await b.check(SetupCatalog.ladder(.responder)[1])
        XCTAssertFalse(tight.outputOK); XCTAssertTrue(tight.note.contains("Too little memory"), tight.note)
    }

    func testTheMeasuredNumbersReadInPlainWords() {
        let c = SetupCheck(loadMs: 2300, speed: 0, unit: "", memoryMB: 610, outputOK: true, promptTokensPerSecond: 400, decodeTokensPerSecond: 40,
                           firstAudioMs: 310, availableMemoryMB: 1500, thermal: "fair")
        let n = VoiceSetup.numbers(c)
        XCTAssertTrue(n.contains("reads about 300 words a second"), n)
        XCTAssertTrue(n.contains("writes about 30 words a second"), n)
        XCTAssertTrue(n.contains("first sound in 310 ms"), n)
        XCTAssertTrue(n.contains("610 MB in use") && n.contains("1500 MB left"), n)
        XCTAssertTrue(n.contains("the phone is warm"), n)
        XCTAssertFalse(VoiceSetup.numbers(SetupCheck(loadMs: 5, speed: 0, unit: "", memoryMB: 0, outputOK: true, thermal: "nominal")).contains("phone is"))
    }

    func testResponderCandidatesAreRegisteredWithPinnedSources() {
        for id in ["qwen3-0.6b-q4_k_m", "qwen3-1.7b-q4_k_m", "qwen3.5-0.8b-q4_k_m", "lfm3-1.7b-qad-q4_0"] {
            let spec = KnownModels.pinned.first { $0.id == id }
            XCTAssertNotNil(spec, id)
            XCTAssertEqual(spec?.sha256.count, 64)
            XCTAssertTrue(spec?.url.absoluteString.hasPrefix("https://huggingface.co/") == true)
        }
        XCTAssertEqual(ResponderChoice.defaultID, "qwen3-0.6b-q4_k_m")
    }
}
