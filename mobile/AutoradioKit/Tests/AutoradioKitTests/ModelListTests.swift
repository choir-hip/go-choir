import XCTest
@testable import AutoradioKit

// The model list as data. Failure list (each line a test): L1 the built-in copy does not survive JSON; L2 an unknown job or field from a newer
// server empties the list; L3 an entry the store could not verify is offered; L4 recommendations ignore the device; L5 the bundle the store
// fetches differs from the one the build pins; L6 the built-in copy disagrees with CandidateTable.
final class ModelListTests: XCTestCase {
    func testBuiltInSurvivesJSON() throws {                                                            // L1
        let list = ModelList.builtIn()
        XCTAssertFalse(list.models.isEmpty)
        XCTAssertEqual(try ModelList.decode(list.encoded()), list)
    }
    func testNewerServerEntriesAreSkippedNotFatal() throws {                                          // L2, L3
        let good = #"{"id":"m","job":"transcriber","label":"M","repository":"a/b","license":"MIT","recommendedFor":[],"rank":5,"future":1}"#
        let unknownJob = #"{"id":"x","job":"hologram","label":"X","repository":"a/b","license":"MIT","recommendedFor":[],"rank":1}"#
        let badDigest = #"{"id":"y","job":"vad","label":"Y","repository":"a/b","revision":"\#(String(repeating: "a", count: 40))","files":[{"path":"f","bytes":3,"sha256":"ABC"}],"license":"MIT","recommendedFor":[],"rank":1}"#
        let l = try ModelList.decode(Data(#"{"version":2,"newField":true,"models":[\#(good),\#(unknownJob),\#(badDigest)]}"#.utf8))
        XCTAssertEqual(l.models.map(\.id), ["m"])
        XCTAssertThrowsError(try ModelList.decode(Data("[]".utf8)))
    }
    func testRecommendationsFollowTheDevice() {                                                       // L4
        let big = ModelListEntry(id: "big", job: "transcriber", label: "", repository: "a/b", revision: nil, files: nil, license: "", credit: nil,
                                 recommendedFor: [.init(machinePrefixes: ["iPhone16,"], minMemoryGB: 7)], rank: 90)
        let small = ModelListEntry(id: "small", job: "transcriber", label: "", repository: "a/b", revision: nil, files: nil, license: "", credit: nil, recommendedFor: [], rank: 60)
        let l = ModelList(version: 1, models: [small, big])
        XCTAssertEqual(l.entries(job: .transcriber, machine: "iPhone16,2", memoryGB: 7.4).map(\.id), ["big", "small"])
        XCTAssertEqual(l.entries(job: .transcriber, machine: "iPhone14,2", memoryGB: 5.6).map(\.id), ["small", "big"], "not recommended, still listed after")
    }
    func testBundleSpecMatchesThePinnedOne() throws {                                                 // L5
        let e = try XCTUnwrap(ModelList.builtIn().models.first { $0.id == KnownModels.fluidSpeechMini.id })
        let b = try XCTUnwrap(e.bundleSpec(folder: KnownModels.fluidSpeechMini.folder, copies: KnownModels.fluidSpeechMini.copies))
        XCTAssertEqual(b.commit, KnownModels.fluidSpeechMini.commit)
        XCTAssertEqual(b.files, KnownModels.fluidSpeechMini.files)
        XCTAssertEqual(b.totalBytes, KnownModels.fluidSpeechMini.totalBytes)
        XCTAssertEqual(e.credit, KnownModels.fluidSpeechMini.credit)
    }
    func testProIsRecommendedFromTheEightGigabyteClassAndMiniBelow() throws {                         // the owner's per-device rule
        let l = ModelList.builtIn()
        let pro = try XCTUnwrap(l.models.first { $0.id == KnownModels.fluidSpeechPro.id })
        XCTAssertEqual(pro.bundleSpec(folder: KnownModels.fluidSpeechPro.folder, copies: KnownModels.fluidSpeechPro.copies)?.files, KnownModels.fluidSpeechPro.files)
        XCTAssertEqual(pro.credit, KnownModels.fluidSpeechPro.credit)
        XCTAssertTrue(pro.isRecommended(machine: "iPhone17,1", memoryGB: 7.4))                         // an 8 GB iPhone
        XCTAssertTrue(pro.isRecommended(machine: "iPhone18,1", memoryGB: 11.4))
        XCTAssertFalse(pro.isRecommended(machine: "iPhone15,3", memoryGB: 5.6), "the owner's 14 Pro Max (6 GB) gets Mini")
        func order(_ machine: String, _ gb: Double) -> [String] { l.entries(job: .transcriber, machine: machine, memoryGB: gb).map(\.id).filter { $0.hasPrefix("fluid-speech") } }
        XCTAssertEqual(order("iPhone15,3", 5.6), ["fluid-speech-mini", "fluid-speech-pro"], "not recommended is still listed, after")
        XCTAssertEqual(order("iPhone17,1", 7.4), ["fluid-speech-pro", "fluid-speech-mini"])
    }
    func testBuiltInHasEveryCandidate() {                                                             // L6
        let ids = Set(ModelList.builtIn().models.map(\.id))
        for c in CandidateTable.all where !c.isFloor { XCTAssertTrue(ids.contains(c.id), "\(c.id) is in the table but not the built-in list") }
    }

    // DOOR E (review round 3): a licence that asks for a credit (every CC licence) cannot be offered without it. A new model with a CC licence and no credit fails here, not in review.
    func testEveryOfferedModelWithACreativeCommonsLicenceCarriesItsCredit() {
        let list = ModelList.builtIn()
        var seen = 0
        for e in list.models where e.license.uppercased().contains("CC-BY") || e.license.uppercased().contains("CC BY") {
            seen += 1
            XCTAssertFalse((e.credit ?? "").isEmpty, "\(e.id) (\(e.license)) is offered with no credit")
            XCTAssertTrue((e.credit ?? "").contains("CC BY 4.0") || (e.credit ?? "").contains("CC-BY-4.0"), "\(e.id): the credit names the licence")
            XCTAssertTrue((e.credit ?? "").contains("creativecommons.org/licenses/by/4.0"), "\(e.id): and links it")
        }
        XCTAssertGreaterThanOrEqual(seen, 3, "Fluid Speech Mini, Parakeet TDT and Pocket are CC BY 4.0")
        let phone = DeviceProfile(chip: "A", memoryGB: 8, freeDiskGB: 50, onWiFi: true)
        for j in ModelOffer.jobs(profile: phone, list: list) { for o in j.options where o.credit.isEmpty { XCTAssertFalse(list.models.first { $0.id == o.id }.map { $0.license.uppercased().contains("CC") } ?? false, "\(o.id) offered with no credit") } }
    }
    func testTheVoicePrintCreditNamesWeSpeakerPyannoteFluidInferenceTheLicenceAndTheChange() {
        let c = WeSpeakerEmbedder.attribution
        for want in ["WeSpeaker", "pyannote", "Fluid Inference", "CC BY 4.0", "creativecommons.org/licenses/by/4.0", "modified"] { XCTAssertTrue(c.contains(want), "the voice-print credit lacks \(want)") }
    }

    // Round 4: the credit/licence test covers every offered FILE, not only list rows.
    func testEveryOfferedFileWhoseLicenceAsksForSomethingCarriesIt() {
        let phone = DeviceProfile(chip: "A", memoryGB: 8, freeDiskGB: 50, onWiFi: true)
        var seen = 0
        for j in ModelOffer.jobs(profile: phone) { for o in j.options {
            seen += 1
            let lic = (KnownModels.managed.first { $0.id == o.id }?.license ?? (o.id == ModelOffer.speakerID ? "CC-BY-4.0" : "")).uppercased()
            if lic.contains("CC-BY") || lic.contains("CC BY") { XCTAssertFalse(o.credit.isEmpty, "\(o.id) is CC BY and is offered with no credit") }
            if lic.contains("PROHIBITED USE") { XCTAssertTrue(o.note.contains("Prohibited Use Policy"), "\(o.id): the use policy its licence names is shown where it is offered") }
        } }
        XCTAssertGreaterThan(seen, 10)
        let search = ModelOffer.jobs(profile: phone).first { $0.job == "search" }!.options[0]
        XCTAssertTrue(search.note.contains("Gemma") && search.note.contains("Prohibited Use Policy"), search.note)
    }
}
