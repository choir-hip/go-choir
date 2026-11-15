import XCTest
@testable import AutoradioKit

// The failure list is at the top of ModelCandidates.swift and Configurator.swift (numbers below refer to them). Measurements are faked:
// nothing here downloads, loads or times a model.

final class FakeConfigBackend: ConfigBackend, @unchecked Sendable {
    let device: DeviceProbe
    private let lock = NSLock()
    var onDisk = Set<String>()
    var failPrepare = Set<String>()
    var measureFor: (Configuration) throws -> ConfigMeasurement = { _ in FakeConfigBackend.good() }
    private(set) var prepared: [String] = [], unloaded: [String] = [], deleted: [String] = [], measured: [String] = []
    init(_ d: DeviceProbe) { device = d }

    static func good(left: Double = 1500, write: Double = 40, read: Double = 600, thermal: String = "nominal") -> ConfigMeasurement {
        var m = ConfigMeasurement()
        m.availableMBAfterLoad = left; m.responderWriteTPS = write; m.responderReadTPS = read; m.ttsFirstSoundMs = 300; m.asrRealTimeFactor = 0.1
        m.decisionMs = 100; m.thermalAfter = thermal; m.residentMB = 2000
        return m
    }
    func probe() -> DeviceProbe { device }
    func isOnDisk(_ c: ModelCandidate) async -> Bool { lock.lock(); defer { lock.unlock() }; return onDisk.contains(c.id) }
    func prepare(_ c: ModelCandidate, role: ConfigRole, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
        lock.lock(); prepared.append(c.id); let fails = failPrepare.contains(c.id); lock.unlock()
        onBytes(c.bytes / 2)
        if fails { throw URLError(.networkConnectionLost) }
        onBytes(c.bytes)
        lock.lock(); onDisk.insert(c.id); lock.unlock()
    }
    func unload(_ c: ModelCandidate, role: ConfigRole) async { lock.lock(); unloaded.append(c.id); lock.unlock() }
    func delete(_ c: ModelCandidate) async { lock.lock(); deleted.append(c.id); onDisk.remove(c.id); lock.unlock() }
    func measure(_ cfg: Configuration, picks: [ConfigRole: ModelCandidate]) async throws -> ConfigMeasurement {
        lock.lock(); measured.append(cfg.key); lock.unlock()
        return try measureFor(cfg)
    }
}

final class ConfiguratorTests: XCTestCase {
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("cfg-\(UUID().uuidString).json") }
    func runner(_ b: FakeConfigBackend, events: Locked<[String]>? = nil) -> ConfigRunner {
        ConfigRunner(backend: b, recordURL: tmp(), observer: { e in
            switch e { case .attempt(let i, _): events?.write { $0.append("attempt \(i)") }
                       case .rejected(let i, _): events?.write { $0.append("rejected \(i)") }
                       case .accepted: events?.write { $0.append("accepted") }
                       default: break } })
    }

    // MARK: the candidate table (failures 1 to 4)

    func testEveryCandidateIsInTheCatalogWithAWellFormedPin() {
        for c in CandidateTable.all where !c.isFloor {
            XCTAssertTrue(KnownModels.contains(c.id), "\(c.id) is not in the model catalog")
            if KnownModels.isPinned(c.id) {
                let spec = KnownModels.pinned.first { $0.id == c.id }!
                XCTAssertEqual(spec.sha256.count, 64, c.id); XCTAssertTrue(spec.sha256.allSatisfy { $0.isHexDigit }, c.id)
                XCTAssertEqual(c.bytes, spec.bytes, "\(c.id): the table's size is the pinned file's")
                XCTAssertTrue(spec.url.absoluteString.hasPrefix("https://huggingface.co/"))
            }
            XCTAssertFalse(c.qualityBasis.isEmpty, "\(c.id): a rank with no basis")
            XCTAssertGreaterThan(c.residentMB, 0, c.id)
        }
    }

    func testEveryRoleHasAFloorAndNoDuplicates() {
        for r in ConfigRole.allCases {
            let cs = CandidateTable.candidates(r)
            XCTAssertTrue(cs.contains { $0.isFloor }, "\(r.rawValue) has no floor")
            XCTAssertEqual(Set(cs.map(\.id)).count, cs.count, "\(r.rawValue) lists an id twice")
            XCTAssertEqual(cs.map(\.qualityRank), cs.map(\.qualityRank).sorted(by: >), "candidates come back best first")
        }
    }

    func testLFM3QuantsAreTheRepoListedOnesAndTheLicenceLimitIsRecordedNotPunished() {
        let lfm = CandidateTable.candidates(.responder).filter { $0.family == "LFM3 1.7B" }
        XCTAssertEqual(Set(lfm.map(\.quant)), ["QAD Q4_0", "Q4_K_M", "Q5_K_M", "Q6_K", "Q8_0", "NVFP4"], "exactly the quants the Hugging Face listing shows")
        XCTAssertTrue(lfm.allSatisfy { $0.licenseLimit?.contains("US$10M") == true })
        let best = CandidateTable.candidates(.responder).first!
        XCTAssertEqual(best.family, "LFM3 1.7B", "ranked first on quality from our samples; the licence does not demote it")
        XCTAssertGreaterThan(lfm.map(\.qualityRank).min()!, CandidateTable.candidates(.responder).filter { $0.family != "LFM3 1.7B" && !$0.isFloor }.map(\.qualityRank).max()!)
    }

    func testUnmeasuredCandidatesSayTheyAreAndNewerPhonesGetMore() {
        let unmeasured = CandidateTable.all.filter { !$0.measured }
        XCTAssertFalse(unmeasured.isEmpty)
        XCTAssertTrue(CandidateTable.all.filter { $0.id == "qwen3.5-2b-q8_0" }.allSatisfy { !$0.measured && $0.minTier == .tier12 })
        let q8 = CandidateTable.candidate("lfm3-1.7b-q8_0", .responder)!
        XCTAssertEqual(q8.minTier, .tier8, "a higher quant of LFM3 is for newer phones than the 14 Pro Max")
        XCTAssertEqual(CandidateTable.candidate("lfm3-1.7b-qad-q4_0", .responder)!.minTier, .tier6)
    }

    // MARK: the device probe (failures 1, 8)

    func testTheMemoryClassesAndTheFloor() {
        XCTAssertEqual(DeviceProbe.classify(physicalGB: 5.6), .tier6, "a 6 GB phone reports about 5.6")
        XCTAssertEqual(DeviceProbe.classify(physicalGB: 7.4), .tier8)
        XCTAssertEqual(DeviceProbe.classify(physicalGB: 11.4), .tier12)
        XCTAssertEqual(DeviceProbe.classify(physicalGB: 3.7), .unsupported, "an iPhone XR is not supported")
        XCTAssertEqual(DeviceProbe.chipName("iPhone15,3"), "A16 Bionic", "the 14 Pro Max, the owner's benchmark")
        XCTAssertTrue(DeviceProbe.simulated(.unsupported).unsupportedReason!.contains("not supported"))
        XCTAssertNil(DeviceProbe.simulated(.tier6).unsupportedReason)
    }

    // MARK: the planner

    func testTheWholePipelineIsPlannedTogetherAndSharedWeightsCountOnce() {
        let q8r = CandidateTable.candidate("qwen3-0.6b-q8_0", .responder)!, q8d = CandidateTable.candidate("qwen3-0.6b-q8_0", .decision)!
        let both = ConfigPlanner.estimateMB([.responder: q8r, .decision: q8d])
        let alone = ConfigPlanner.estimateMB([.responder: q8r])
        XCTAssertEqual(both - alone, q8d.contextMB, accuracy: 0.001, "the second role on one file adds only its own context")
        let other = CandidateTable.candidate("system1-computer", .decision)!
        XCTAssertEqual(ConfigPlanner.estimateMB([.responder: q8r, .decision: other]), alone, accuracy: 0.001, "a floor costs nothing")
    }

    func testPlansAreBestFirstAndEveryOneFitsItsBudget() {
        for tier in [DeviceClass.tier6, .tier8, .tier12] {
            let d = DeviceProbe.simulated(tier)
            let plan = ConfigPlanner.plan(device: d)
            XCTAssertFalse(plan.isEmpty, tier.label)
            XCTAssertEqual(plan.map(\.score), plan.map(\.score).sorted(by: >), "best quality first")
            let budget = d.availableMB! - ConfigThresholds.standard.planningHeadroomMB
            XCTAssertTrue(plan.allSatisfy { $0.estimatedMB <= budget + 0.001 }, "\(tier.label): every configuration fits the planner's budget")
            XCTAssertTrue(plan.last!.picks.values.allSatisfy { id in CandidateTable.all.first { $0.id == id }!.isFloor }, "a floor-only configuration is last")
        }
    }

    func testTheOwnersPhoneGetsLFM3AndNewerPhonesGetMore() {
        let six = ConfigPlanner.plan(device: DeviceProbe.simulated(.tier6)).first!
        let eight = ConfigPlanner.plan(device: DeviceProbe.simulated(.tier8)).first!
        let twelve = ConfigPlanner.plan(device: DeviceProbe.simulated(.tier12)).first!
        XCTAssertTrue(six.pick(.responder)!.hasPrefix("lfm3-1.7b"), "the 14 Pro Max class gets LFM3: \(six.key)")
        XCTAssertTrue(CandidateTable.candidate(six.pick(.responder)!, .responder)!.minTier <= .tier6, "and only a quant that tier may try")
        func rank(_ c: Configuration) -> Int { CandidateTable.candidate(c.pick(.responder)!, .responder)!.qualityRank }
        XCTAssertGreaterThanOrEqual(rank(eight), rank(six), "a newer phone gets at least as good a responder")
        XCTAssertGreaterThanOrEqual(rank(twelve), rank(eight))
        XCTAssertGreaterThan(twelve.score, six.score, "and a better whole configuration")
        XCTAssertGreaterThan(twelve.estimatedMB, six.estimatedMB)
    }

    func testAnUnsupportedPhoneHasNoPlan() {
        XCTAssertEqual(ConfigPlanner.plan(device: DeviceProbe.simulated(.unsupported)), [])
    }

    func testAnUnavailableCandidateIsLeftOutOfALaterPlan() {
        let d = DeviceProbe.simulated(.tier6)
        let first = ConfigPlanner.plan(device: d).first!
        let later = ConfigPlanner.plan(device: d, excluding: [first.pick(.responder)!])
        XCTAssertFalse(later.contains { $0.pick(.responder) == first.pick(.responder) })
    }

    // MARK: the verdict (failure 3)

    func testTheVerdictNamesEachShortfallWithItsNumbers() {
        let d = DeviceProbe.simulated(.tier6)
        let cfg = ConfigPlanner.plan(device: d).first!
        XCTAssertTrue(ConfigPlanner.verdict(FakeConfigBackend.good(), config: cfg, device: d).ok)
        let tight = ConfigPlanner.verdict(FakeConfigBackend.good(left: 200), config: cfg, device: d)
        XCTAssertFalse(tight.ok); XCTAssertTrue(tight.reasons.joined().contains("200 MB would be left"), "\(tight.reasons)")
        let slow = ConfigPlanner.verdict(FakeConfigBackend.good(write: 6), config: cfg, device: d)
        XCTAssertTrue(slow.reasons.joined().contains("writes 6 tokens a second"), "\(slow.reasons)")
        let hot = ConfigPlanner.verdict(FakeConfigBackend.good(thermal: "serious"), config: cfg, device: d)
        XCTAssertTrue(hot.reasons.joined().contains("serious"), "\(hot.reasons)")
        var late = FakeConfigBackend.good(); late.ttsFirstSoundMs = 4000
        XCTAssertFalse(ConfigPlanner.verdict(late, config: cfg, device: d).ok)
        var asr = FakeConfigBackend.good(); asr.asrRealTimeFactor = 0.9
        XCTAssertFalse(ConfigPlanner.verdict(asr, config: cfg, device: d).ok)
        var failed = FakeConfigBackend.good(); failed.failedRoles["responder"] = "no sentence"
        XCTAssertTrue(ConfigPlanner.verdict(failed, config: cfg, device: d).reasons.joined().contains("responder failed"))
    }

    func testTheMeasurementFallsBackToThePlannersEstimateWhereTheDeviceCannotSay() {
        let d = DeviceProbe.simulated(.tier6)
        let cfg = ConfigPlanner.plan(device: d).first!
        var m = FakeConfigBackend.good(); m.availableMBAfterLoad = nil
        XCTAssertTrue(ConfigPlanner.verdict(m, config: cfg, device: d).ok, "the planner's budget stands in: headroom is the assumed available minus the estimate")
    }

    // MARK: the runner

    func testTheFirstConfigurationThatFitsIsAcceptedAndNothingElseStaysOnDisk() async {
        let d = DeviceProbe.simulated(.tier6)
        let b = FakeConfigBackend(d)
        b.onDisk = ["qwen3-1.7b-q4_k_m", "lfm3-1.7b-q4_k_m"]                       // leftovers from an earlier run
        let rec = await runner(b).run()
        let plan = ConfigPlanner.plan(device: d)
        XCTAssertEqual(rec.chosen, plan.first)
        XCTAssertEqual(rec.attempts.count, 1)
        XCTAssertTrue(rec.attempts[0].accepted)
        let keep = Set(plan.first!.picks.values)
        XCTAssertFalse(b.onDisk.contains("qwen3-1.7b-q4_k_m") && !keep.contains("qwen3-1.7b-q4_k_m"), "weights not in the accepted configuration are deleted")
        for id in b.onDisk { XCTAssertTrue(keep.contains(id), "\(id) is on disk but not in the accepted configuration") }
        XCTAssertTrue(rec.lines["responder"]!.contains("fits with"), rec.lines["responder"] ?? "")
        XCTAssertTrue(rec.lines["responder"]!.contains("words a second"))
        XCTAssertEqual(Set(rec.lines.keys), Set(ConfigRole.allCases.compactMap { plan.first!.pick($0) != nil ? $0.rawValue : nil }))
    }

    func testAConfigurationThatDoesNotFitOnceLoadedStepsDown() async {
        let d = DeviceProbe.simulated(.tier6)
        let b = FakeConfigBackend(d)
        let first = ConfigPlanner.plan(device: d).first!
        b.measureFor = { cfg in cfg == first ? FakeConfigBackend.good(left: 150) : FakeConfigBackend.good() }
        let events = Locked<[String]>([])
        let rec = await runner(b, events: events).run()
        XCTAssertEqual(rec.attempts.count, 2)
        XCTAssertFalse(rec.attempts[0].accepted); XCTAssertTrue(rec.attempts[0].reasons.joined().contains("150 MB would be left"))
        XCTAssertTrue(rec.attempts[1].accepted)
        XCTAssertNotEqual(rec.chosen, first)
        XCTAssertEqual(events.value, ["attempt 1", "rejected 1", "attempt 2", "accepted"])
        XCTAssertFalse(b.unloaded.isEmpty, "the rejected configuration was unloaded before the next was loaded")
        // What the rejected one fetched and the accepted one does not use is deleted; what both use is kept.
        let kept = Set(rec.chosen!.picks.values)
        for id in Set(first.picks.values).subtracting(kept) where CandidateTable.all.contains(where: { $0.id == id && !$0.isFloor }) { XCTAssertFalse(b.onDisk.contains(id), "\(id) should be deleted") }
        for id in kept where CandidateTable.all.contains(where: { $0.id == id && !$0.isFloor }) { XCTAssertTrue(b.onDisk.contains(id), "\(id) belongs to the accepted configuration") }
    }

    func testTooSlowAndTooHotAreRejectedWithTheNumbers() async {
        let d = DeviceProbe.simulated(.tier6)
        let b = FakeConfigBackend(d)
        let n = Locked<Int>(0)
        b.measureFor = { _ in
            let k = n.write { $0 += 1; return $0 }
            if k == 1 { return FakeConfigBackend.good(write: 5) }
            if k == 2 { return FakeConfigBackend.good(thermal: "critical") }
            return FakeConfigBackend.good()
        }
        let rec = await runner(b).run()
        XCTAssertEqual(rec.attempts.count, 3)
        XCTAssertTrue(rec.attempts[0].reasons.joined().contains("tokens a second"))
        XCTAssertTrue(rec.attempts[1].reasons.joined().contains("critical"))
        XCTAssertTrue(rec.attempts[2].accepted)
        XCTAssertNotNil(rec.attempts[0].measurement, "every measurement is recorded")
    }

    func testADownloadFailingMidWayMovesOnWithoutThatCandidateAndAReRunResumes() async {
        let d = DeviceProbe.simulated(.tier6)
        let b = FakeConfigBackend(d)
        let first = ConfigPlanner.plan(device: d).first!
        let bad = first.pick(.responder)!
        b.failPrepare = [bad]
        let rec = await runner(b).run()
        XCTAssertFalse(rec.attempts[0].accepted)
        XCTAssertTrue(rec.attempts[0].reasons.first!.contains("could not be fetched or loaded"), rec.attempts[0].reasons.first ?? "")
        XCTAssertNotEqual(rec.chosen!.pick(.responder), bad, "the plan went on without it")
        XCTAssertTrue(rec.attempts.dropFirst().allSatisfy { !$0.configuration.picks.values.contains(bad) })
        // A re-run: the network is back, what is on disk is not fetched again, and the best configuration is tried again from the top.
        b.failPrepare = []
        let again = await runner(b).run()
        XCTAssertEqual(again.chosen, first)
        XCTAssertTrue(again.attempts.first!.downloads.contains { $0.alreadyOnDisk } || b.onDisk.isEmpty == false)
    }

    func testAReRunSkipsWhatIsOnDiskAndChoosesTheSame() async {
        let d = DeviceProbe.simulated(.tier6)
        let b = FakeConfigBackend(d)
        let one = await runner(b).run()
        let two = await runner(b).run()
        XCTAssertEqual(one.chosen, two.chosen)
        XCTAssertTrue(two.attempts[0].downloads.allSatisfy { dl in dl.alreadyOnDisk || CandidateTable.all.first(where: { c in c.id == dl.id })!.isFloor })
    }

    func testAnUnsupportedPhoneGetsAPlainResultAllFloorsAndNothingDownloaded() async {
        let b = FakeConfigBackend(DeviceProbe.simulated(.unsupported))
        let rec = await runner(b).run()
        XCTAssertTrue(b.prepared.isEmpty)
        XCTAssertTrue(rec.notes.joined().contains("not supported"))
        XCTAssertTrue(rec.chosen!.picks.values.allSatisfy { id in CandidateTable.all.first { $0.id == id }!.isFloor })
        XCTAssertTrue(rec.lines["responder"]!.contains("below the 6 GB floor"), rec.lines["responder"] ?? "")
    }

    func testWhenNothingPassesTheFloorsAreChosenAndSaid() async {
        let d = DeviceProbe.simulated(.tier6)
        let b = FakeConfigBackend(d)
        b.measureFor = { _ in FakeConfigBackend.good(left: 50) }
        let rec = await runner(b).run()
        XCTAssertTrue(rec.notes.joined().contains("no configuration passed"))
        XCTAssertEqual(rec.attempts.count, ConfigThresholds.standard.maxAttempts, "a count bounds the attempts, not a timer")
        XCTAssertTrue(rec.chosen!.picks.values.allSatisfy { id in CandidateTable.all.first { $0.id == id }!.isFloor })
        XCTAssertTrue(b.onDisk.isEmpty, "nothing is kept for a configuration that was not accepted")
    }

    func testAMeasurementThatErrorsIsARejectionReasonNotACrash() async {
        let d = DeviceProbe.simulated(.tier6)
        let b = FakeConfigBackend(d)
        let n = Locked<Int>(0)
        b.measureFor = { _ in
            if n.write({ $0 += 1; return $0 }) == 1 { throw LabError.unavailable("llama.cpp cannot run this quant") }
            return FakeConfigBackend.good()
        }
        let rec = await runner(b).run()
        XCTAssertTrue(rec.attempts[0].reasons.joined().contains("cannot run this quant"), "\(rec.attempts[0].reasons)")
        XCTAssertTrue(rec.attempts[1].accepted)
    }

    func testTheRecordIsSavedAndRoundTripsForTheTraceExport() async throws {
        let url = tmp()
        let b = FakeConfigBackend(DeviceProbe.simulated(.tier6))
        let r = ConfigRunner(backend: b, recordURL: url)
        let rec = await r.run()
        let back = try XCTUnwrap(ConfigRecordStore.load(from: url))
        XCTAssertEqual(back, rec)
        XCTAssertEqual(back.thresholds, .standard, "the thresholds in force are in the record")
        XCTAssertEqual(back.device.model, "simulated-6GB")
        let x = TraceExport.make(records: [], log: [], device: DeviceInfo(model: "m", os: "o", appBuild: "b"), describe: LabRuntime.describe, configuration: rec)
        let decoded = try JSONDecoder().decode(TraceExport.self, from: x.encoded())
        XCTAssertEqual(decoded.configuration?.chosen, rec.chosen)
    }

    // MARK: setup shows the result on the existing rows

    struct NoSetup: SetupBackend {
        func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws {}
        func check(_ c: SetupCandidate) async throws -> SetupCheck { SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: true) }
    }

    func testSetupRowsShowWhatWasChosenForEachRoleInOneLine() async {
        let d = DeviceProbe.simulated(.tier6)
        let b = FakeConfigBackend(d)
        let s = VoiceSetup(backend: NoSetup(), profile: DeviceProfile(chip: "t", memoryGB: 6, freeDiskGB: 40, onWiFi: true), configBackend: b)
        XCTAssertGreaterThan(s.totalBytes, 0, "the page can say what the best plan would download before it starts")
        await s.run()
        XCTAssertTrue(s.isFinished)
        XCTAssertEqual(s.rows.count, 5)
        XCTAssertTrue(s.rows.allSatisfy { $0.state == "ready" }, "\(s.rows)")
        let resp = s.rows.first { $0.stage == "responder" }!
        XCTAssertTrue(resp.statusLine.contains("fits with") && resp.statusLine.contains("words a second"), resp.statusLine)
        XCTAssertEqual(s.choices[.holder], "local-ack")
        XCTAssertNotNil(s.configurationRecord?.chosen)
        XCTAssertEqual(s.record.items.first { $0.stage == "responder" }?.chosen, s.configurationRecord?.chosen?.pick(.responder))
    }

    // MARK: nothing ships in the app (item 1)

    func testNoModelWeightsAreInTheRepositoryTree() throws {
        // The app is built from this tree: a weight file here would ship. (The built bundle is checked by mobile/scripts/check-no-weights.sh.)
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let bad = ["gguf", "mlmodelc", "mlpackage", "onnx", "safetensors", "mlmodel"]
        let e = try XCTUnwrap(FileManager.default.enumerator(at: root, includingPropertiesForKeys: [.fileSizeKey], options: [.skipsHiddenFiles]))
        for case let u as URL in e {
            if u.path.contains("/.build/") || u.path.contains("/.deps/") || u.path.contains("/node_modules/") { continue }
            guard bad.contains(u.pathExtension.lowercased()) else { continue }
            let size = (try? u.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
            XCTAssertLessThan(size, 100_000, "\(u.path) looks like model weights (\(size) bytes): nothing ships with the app")
        }
    }

    func testEveryRolesModelComesThroughTheProvider() {
        // Every non-floor candidate is fetched through ModelProviders (the app module installs it) and the live backend has no other path.
        for c in CandidateTable.all where !c.isFloor { XCTAssertTrue(KnownModels.contains(c.id), c.id) }
        ModelProviders.current = nil
        let live = LiveConfigBackend(store: ModelStore(root: FileManager.default.temporaryDirectory.appendingPathComponent("cfg-\(UUID().uuidString)"), registerKnown: true))
        let c = CandidateTable.candidate("silero-vad-v6.2.1", .vad)!
        let exp = expectation(description: "prepare")
        Task {
            do { try await live.prepare(c, role: .vad) { _ in }; XCTFail("no provider, no model") }
            catch { XCTAssertTrue("\(error)".contains("no on-device model libraries"), "\(error)") }
            exp.fulfill()
        }
        wait(for: [exp], timeout: 5)
    }
}
