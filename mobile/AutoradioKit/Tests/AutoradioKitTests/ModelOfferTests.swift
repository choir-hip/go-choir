import XCTest
@testable import AutoradioKit

// What voice needs from the network, read from the model lists (ModelOffer),. The failure list, written first:
//  M1 the jobs are the list's own, each option with a size from the list and the host, one option per job marked recommended, the same one setup would download first; a built-in floor is never an option.
//  M2 a stage is ready when setup recorded a model that is not its built-in floor; unknown ids from the configurator count unless their stage is a built-in one; no record is nothing ready.
//  M3 the picks round-trip, only the hearing kinds are kept, and a bad file is empty.
final class ModelOfferTests: XCTestCase {
    private let phone = DeviceProfile(chip: "A", memoryGB: 8, freeDiskGB: 50, onWiFi: true, availableMemoryMB: 6000)
    func testM1JobsAreTheListsOwn() {
        let jobs = ModelOffer.jobs(profile: phone)
        XCTAssertEqual(jobs.map(\.job), ["transcriber", "bounds", "voice", "decision", "responder", "speaker", "search"])
        XCTAssertEqual(jobs.last?.options.first?.bytes, KnownModels.embeddingGemma2.bytes)
        for j in jobs {
            XCTAssertEqual(j.host, "Hugging Face"); XCTAssertFalse(j.options.isEmpty)
            XCTAssertTrue(j.options.allSatisfy { $0.bytes > 0 && !$0.sizeLabel.isEmpty && !$0.model.isEmpty })
            XCTAssertLessThanOrEqual(j.options.filter(\.recommended).count, 1)
            XCTAssertEqual(ModelOffer.group(j.job), j.job == "voice" ? "voice" : j.job == "search" ? "search" : "voice-rest")
        }
        // The decision stage downloads its own model, so it is a job on the page with its size (it was left out and fetched unshown).
        XCTAssertEqual(jobs.first { $0.job == "decision" }?.options.first?.id, "qwen3-0.6b-q8_0")
        // Every stage the setup runs that has a download is offered: nothing is fetched that is not here.
        for st in SetupStage.allCases where SetupCatalog.ladder(st).contains(where: { $0.bytes > 0 }) { XCTAssertTrue(jobs.contains { $0.job == st.rawValue }, "\(st)") }
        XCTAssertFalse(jobs.flatMap(\.options).contains { ["apple-speech", "energy", "system-voice", "cue-only", "system1-computer"].contains($0.id) }, "a built-in floor is never an offer")
        let ready = ModelOffer.jobs(profile: phone, ready: ["voice": jobs.first { $0.job == "voice" }!.options[0].id])
        XCTAssertTrue(ready.first { $0.job == "voice" }!.options[0].ready); XCTAssertFalse(ready.first { $0.job == "transcriber" }!.options.contains { $0.ready })
    }
    // L4 (owner, 2026-11-24): an iPhone 14 Pro Max (6 GB, about 5.6 as the OS reports it) is recommended Fluid Speech Mini; a phone of the 8 GB class is recommended Fluid Speech Pro (the voice-pro lane), from the built-in list, no test-only list.
    func testM1bFluidSpeechMiniIsRecommendedOnASixGigabyteIPhoneAndProOnTheEightGigabyteClass() {
        let six = ModelOffer.jobs(profile: DeviceProfile(chip: "iPhone15,3", memoryGB: 5.6, freeDiskGB: 50, onWiFi: true)).first { $0.job == "transcriber" }!
        XCTAssertEqual(six.options.first { $0.recommended }?.id, "fluid-speech-mini")
        XCTAssertTrue(six.options.contains { $0.id == "fluid-speech-pro" }, "Pro is listed on every phone, and recommended on none below its class")
        for (machine, gb) in [("iPhone16,2", 7.4), ("iPhone17,1", 7.4)] {
            let t = ModelOffer.jobs(profile: DeviceProfile(chip: machine, memoryGB: gb, freeDiskGB: 50, onWiFi: true)).first { $0.job == "transcriber" }!
            XCTAssertEqual(t.options.first { $0.recommended }?.id, "fluid-speech-pro", machine)
            XCTAssertEqual(t.options.first?.id, "fluid-speech-pro", "the best-ranked row is first")
        }
    }
    // A row added to the list is an option with no change to the code; its own device rule decides whether it is the recommendation (the voice-pro lane's row).
    func testM1cARowAddedToTheListIsOfferedAndItsRuleDecides() throws {
        var list = ModelList.builtIn()
        let base = list.models.first { $0.id == "fluid-speech-mini" }!
        var pro = base; pro.id = "fluid-speech-pro-test"; pro.label = "A bigger recogniser"; pro.rank = 99
        pro.recommendedFor = [.init(machinePrefixes: ["iPhone17"], minMemoryGB: 7)]
        list.models.append(pro)
        let big = ModelOffer.jobs(profile: DeviceProfile(chip: "iPhone17,1", memoryGB: 7.4, freeDiskGB: 50, onWiFi: true), list: list).first { $0.job == "transcriber" }!
        XCTAssertEqual(big.options.first?.id, "fluid-speech-pro-test")
        let six = ModelOffer.jobs(profile: DeviceProfile(chip: "iPhone15,3", memoryGB: 5.6, freeDiskGB: 50, onWiFi: true), list: list).first { $0.job == "transcriber" }!
        XCTAssertEqual(six.options.first { $0.recommended }?.id, "fluid-speech-mini", "its rule does not fit a 6 GB iPhone15")
    }
    // The larger answering model carries a licence limit; the page must be able to say so (review 6).
    func testM1dTheLicenceLimitTravelsWithTheOption() {
        let r = ModelOffer.jobs(profile: DeviceProfile(chip: "iPhone15,3", memoryGB: 5.6, freeDiskGB: 50, onWiFi: true)).first { $0.job == "responder" }!
        let lfm = r.options.first { $0.id.hasPrefix("lfm3") }!
        XCTAssertTrue(lfm.note.contains("US$10M"), lfm.note); XCTAssertTrue(r.options.first { $0.id.hasPrefix("qwen3-0.6b") }!.note.isEmpty)
        XCTAssertTrue(r.options.first { $0.recommended }!.id.hasPrefix("lfm3"), "LFM3 stays the recommended answering model (owner, 2026-11-24)")
    }
    // A pick makes the stage's plan the pick and the floor, nothing else: a failed pick ends on what is built in, never on a model the person was not shown (review 3). Run only the shown stages, never the others.
    final class Fetched: @unchecked Sendable { var ids: [String] = []; let l = NSLock(); func add(_ i: String) { l.lock(); ids.append(i); l.unlock() } }
    struct Recording: SetupBackend {
        let seen: Fetched, failing: Set<String>
        func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws { seen.add(c.id); if failing.contains(c.id) { throw URLError(.notConnectedToInternet) } }
        func check(_ c: SetupCandidate) async throws -> SetupCheck { SetupCheck(loadMs: 5, speed: 0.1, unit: "x real time", memoryMB: 0, outputOK: true) }
    }
    func testM1ePreferLeavesOnlyThePickAndTheFloorAndOnlyShownStagesRun() async {
        let seen = Fetched()
        let s = VoiceSetup(backend: Recording(seen: seen, failing: ["lfm3-1.7b-q5_k_m"]), profile: phone, stallSeconds: 5, patience: 0.05, maxTries: 1, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))
        let pick = ModelOffer.candidate(job: "responder", id: "lfm3-1.7b-q5_k_m")!
        s.prefer(.responder, pick)
        await s.run(only: [.responder])
        XCTAssertEqual(seen.ids, ["lfm3-1.7b-q5_k_m"], "only the shown stage ran, and a failed pick fell to the floor, not to another model")
        XCTAssertEqual(s.record.items.first { $0.stage == "responder" }?.chosen, "cue-only", "the floor, not another model")
        XCTAssertNil(ModelOffer.candidate(job: "responder", id: "not-in-the-table"), "a pick that cannot be built is not run")
    }
    // A pick this build cannot fetch is not run, and the stage does not fall back to a model that was never shown (review notes).
    func testM1fAPickThatCannotBeBuiltRunsNothingForItsStage() {
        let p = ModelOffer.plan(stages: ["voice", "responder", "transcriber"], picks: ["voice": "kokoro-82m-ane", "responder": "a-row-this-build-has-no-fetch-for"])
        XCTAssertTrue(p.run.contains(.voice)); XCTAssertFalse(p.run.contains(.responder), "the unfetchable pick's stage is left alone")
        XCTAssertTrue(p.run.contains(.transcriber), "a stage with no pick runs on its own ladder")
        XCTAssertEqual(p.chosen[.voice]?.id, "kokoro-82m-ane"); XCTAssertNil(p.chosen[.responder])
        XCTAssertTrue(ModelOffer.plan(stages: ["nonsense"], picks: [:]).run.isEmpty)
    }
    // A licence that asks for a credit (CC BY 4.0) has it shown with the option, from the list (release blocker, round 2).
    func testM1gTheCreditTravelsWithTheOption() {
        let r = ModelOffer.jobs(profile: phone, list: .builtIn()).first { $0.job == "transcriber" }!
        let fluid = r.options.first { $0.id == "fluid-speech-mini" }!
        XCTAssertTrue(fluid.credit.contains("Fluid Speech Mini by FluidVoice"), fluid.credit); XCTAssertTrue(fluid.credit.contains("CC BY 4.0"))
        XCTAssertEqual(fluid.dict["credit"] as? String, fluid.credit)
        let speaker = ModelOffer.jobs(profile: phone, list: .builtIn()).first { $0.job == "speaker" }!
        XCTAssertFalse(speaker.options[0].credit.isEmpty, "the voice-print model's attribution is shown too")
    }

    // Voice and models lists every option of a job and says which are on the phone, which streams words and which is in use (owner, 2026-11-24).
    //  V1 every option the readiness record calls ready is on the phone, not only the best-ranked one of its job.
    //  V2 a recogniser that streams words says so (the page lists it as live words); one that does not, does not.
    //  V3 what the radio is using is flagged, and the flag is in the dictionary shared code reads.
    func testV1EveryReadyOptionIsOnThePhone() throws {
        let ready: Set<String> = ["fluid-speech-mini", "parakeet-tdt-0.6b-v2"]
        let t = try XCTUnwrap(ModelOffer.jobs(profile: phone, isReady: { ready.contains($0) }).first { $0.job == "transcriber" })
        XCTAssertEqual(Set(t.options.filter(\.ready).map(\.id)), ready)
        XCTAssertTrue(ModelOffer.jobs(profile: phone).flatMap(\.options).allSatisfy { !$0.ready }, "nothing is on the phone unless the record says so")
    }
    func testV2AStreamingRecogniserSaysSo() throws {
        let t = try XCTUnwrap(ModelOffer.jobs(profile: phone).first { $0.job == "transcriber" })
        XCTAssertEqual(t.options.first { $0.id == "parakeet-eou-120m" }?.streams, true)
        XCTAssertEqual(t.options.first { $0.id == "fluid-speech-mini" }?.streams, false)
        XCTAssertEqual(try XCTUnwrap(ModelOffer.jobs(profile: phone).first { $0.job == "voice" }).options.contains { $0.streams }, false)
    }
    func testV3WhatIsInUseIsFlagged() throws {
        let t = try XCTUnwrap(ModelOffer.jobs(profile: phone, inUse: ["fluid-speech-mini"]).first { $0.job == "transcriber" })
        XCTAssertEqual(t.options.filter(\.inUse).map(\.id), ["fluid-speech-mini"])
        XCTAssertEqual(t.options.first { $0.id == "fluid-speech-mini" }?.dict["inUse"] as? Bool, true)
        XCTAssertEqual(t.options.first { $0.id == "parakeet-eou-120m" }?.dict["streams"] as? Bool, true)
    }

    //  V4 (review 2026-11-24) "In use" is the model the phone uses for THAT job: the decision model is its own choice (not the Qwen3 file the decision stage always lists), a downloaded model that is not chosen is not "in use" and can be removed.
    //  V5 a row is sized as its own model: no companion's bytes are in a recogniser's size.
    func testV4InUseIsThePhonesChoiceForEachJob() throws {
        let protected: Set<String> = ["qwen3-0.6b-q8_0", "lfm2.5-350m-q4_k_m"]          // files a stage loads: never removable
        let jobs = ModelOffer.jobs(profile: phone, inUse: protected, chosen: ["decision": "lfm2.5-350m-q4_k_m", "responder": "qwen3-0.6b-q8_0"], isReady: { _ in true }, onDisk: { _ in true })
        let d = try XCTUnwrap(jobs.first { $0.job == "decision" })
        XCTAssertEqual(d.options.filter(\.inUse).map(\.id), ["lfm2.5-350m-q4_k_m"], "only the chosen decision model is in use")
        XCTAssertEqual(d.options.first { $0.id == "qwen3-0.6b-q8_0" }?.removable, false, "protected files stay protected")
        let r = try XCTUnwrap(jobs.first { $0.job == "responder" })
        XCTAssertEqual(r.options.filter(\.inUse).map(\.id), ["qwen3-0.6b-q8_0"])
        XCTAssertEqual(r.options.first { $0.id == "lfm3-1.7b-q5_k_m" }?.inUse, false, "an option that is not the answering model chosen is not in use")
    }
    func testV5ARecogniserIsSizedAsItsOwnModel() throws {
        let t = try XCTUnwrap(ModelOffer.jobs(profile: phone).first { $0.job == "transcriber" })
        XCTAssertEqual(t.options.first { $0.id == "fluid-speech-mini" }?.bytes, KnownModels.fluidSpeechMini.totalBytes)
        XCTAssertEqual(t.options.first { $0.id == "fluid-speech-pro" }?.bytes, KnownModels.fluidSpeechPro.totalBytes)
    }
}
