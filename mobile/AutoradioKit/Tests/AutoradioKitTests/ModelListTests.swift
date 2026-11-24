import XCTest
@testable import AutoradioKit

// The model list as data. Failure list (each line a test): L1 the built-in copy does not survive JSON; L2 an unknown job or field from a newer
// server empties the list; L3 an entry the store could not verify is offered; L4 recommendations ignore the device; L5 the bundle the store
// fetches differs from the one the build pins; L6 the built-in copy disagrees with CandidateTable; L8 a model under an attribution licence has no credit on its row.
// LFM3-Omni-2B as a larger responder (owner, 2026-11-24): O1 the row is missing or its pin is not a commit with a verifiable file; O2 it is offered
// on a phone that cannot hold it, or it displaces LFM3 as the first fit on the owner's class; O3 its licence and the credit the licence asks for are
// not carried; O4 the pinned download url is the moving `main` and not the commit; O5 the text path lays the chat out differently from LFM3's card.
final class ModelListTests: XCTestCase {
    /// L8: a model under an attribution licence carries its credit as data on its row (CC BY 4.0 section 3(a)); the page that shows credits reads this field.
    func testEveryAttributionLicenceCarriesItsCredit() {
        for c in CandidateTable.all where c.license.uppercased().contains("CC-BY") {
            XCTAssertFalse((c.credit ?? "").isEmpty, "\(c.id) is \(c.license) and has no credit on its row")
        }
        for e in ModelList.builtIn().models where e.license.uppercased().contains("CC-BY") || e.license.uppercased().contains("CC BY") {
            let credit = e.credit ?? ""
            XCTAssertTrue(credit.contains("CC BY 4.0") && credit.contains("creativecommons.org/licenses/by/4.0"), "\(e.id): the credit names the licence and its link")
        }
        XCTAssertTrue(ModelList.builtIn().models.first { $0.id == "parakeet-tdt-0.6b-v2" }?.credit?.contains("NVIDIA") == true)
        XCTAssertTrue(ModelList.builtIn().models.first { $0.id == "pocket-tts" }?.credit?.contains("Kyutai") == true)
    }
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

    // VOICES (owner, 2026-11-24: more voices in Voice and models). The rows are generated from the packs' metadata (scripts/gen-voice-rows.ts); the list carries them with their credits.
    //  W1 the built-in list carries every generated voice row, English only, with the credit of its model, and survives JSON.
    //  W2 a voice row with no credit is not offered (dropped on decode), and a row with no name, voice or model likewise.
    //  W3 every voice row names a model the catalog knows, and its stage is one the voice runtime makes.
    //  W4 the catalog's cards are the list's rows, with the default (Heart) first and the system voice last.
    func testTheListCarriesTheVoiceRowsWithTheirCredits() throws {                                   // W1
        let l = ModelList.builtIn()
        XCTAssertEqual(l.voices.map(\.id), VoiceRows.all.map(\.id))
        XCTAssertEqual(l.voices.filter { $0.modelID == "kokoro-82m-ane" }.count, 28)
        XCTAssertEqual(l.voices.filter { $0.modelID == "phonon-31m" }.count, 25)
        for v in l.voices { XCTAssertFalse(v.credit.isEmpty, "\(v.id) is offered with no credit"); XCTAssertTrue(v.credit.contains("Apache-2.0"), v.id) }
        XCTAssertEqual(try ModelList.decode(l.encoded()), l)
    }
    func testAVoiceRowWithNoCreditIsNotOffered() throws {                                            // W2
        func row(_ id: String, credit: String, name: String = "N", model: String = "kokoro-82m-ane") -> String {
            #"{"id":"\#(id)","stageID":"kokoro","voice":"af_x","modelID":"\#(model)","group":"G","name":"\#(name)","blurb":"b","sample":"s","license":"Apache-2.0","credit":"\#(credit)","bytes":0}"#
        }
        let json = #"{"version":1,"models":[],"voices":[\#(row("kokoro:ok", credit: "Credit")),\#(row("kokoro:nocredit", credit: "")),\#(row("kokoro:noname", credit: "C", name: "")),\#(row("kokoro:nomodel", credit: "C", model: ""))]}"#
        XCTAssertEqual(try ModelList.decode(Data(json.utf8)).voices.map(\.id), ["kokoro:ok"])
        XCTAssertEqual(try ModelList.decode(Data(#"{"version":1,"models":[]}"#.utf8)).voices.count, 0, "an older list with no voices still decodes")
    }
    func testEveryVoiceRowNamesAModelAndAStageTheRuntimeMakes() {                                   // W3
        for v in ModelList.builtIn().voices {
            XCTAssertTrue(KnownModels.contains(v.modelID), "\(v.id) names a model the catalog lacks")
            XCTAssertTrue(VoiceCatalog.neuralStageIDs.contains(v.stageID), "\(v.id): stage \(v.stageID)")
        }
    }
    func testTheCatalogsCardsAreTheListsRows() {                                                     // W4
        let ids = VoiceCatalog.options.map(\.id)
        XCTAssertEqual(Array(ids.prefix(VoiceRows.all.count)), VoiceRows.all.map(\.id))
        XCTAssertEqual(VoiceCatalog.defaultOption.id, "kokoro:af_heart")
        XCTAssertEqual(VoiceCatalog.options.last?.id, "system")
        XCTAssertTrue(ids.contains("pocket-tts:alba"))
        for r in VoiceRows.all { XCTAssertEqual(VoiceCatalog.option(id: r.id)?.setting, StageSetting(id: r.stageID, params: ["voice": .string(r.voice)]), "\(r.id): the voice id reaches the stage as its \"voice\" parameter") }
        XCTAssertEqual(Set(ids).count, ids.count)
    // MARK: LFM3-Omni-2B as a larger responder

    private let omniID = "lfm3-omni-2b-q8_0"
    func testOmniRowIsPinnedToACommitAndAVerifiableFile() throws {                                      // O1
        let e = try XCTUnwrap(ModelList.builtIn().models.first { $0.id == omniID }, "the Omni row is missing")
        XCTAssertEqual(e.job, "responder")
        XCTAssertEqual(e.repository, "LiquidAI/LFM3-Omni-2B-GGUF")
        XCTAssertEqual(e.revision?.count, 40)
        let f = try XCTUnwrap(e.files?.first)
        XCTAssertEqual(e.files?.count, 1, "text in, text out: the mmproj (vision, audio, speech out) is not fetched")
        XCTAssertEqual(f.path, "LFM3-Omni-2B-Q8_0.gguf")
        XCTAssertEqual(f.bytes, 1_846_240_768)
        XCTAssertEqual(f.sha256, "59118f661eaacb93d204723d76355b53347f0940b508c11412f6290fd49158d4")
        XCTAssertNotNil(e.bundleSpec(folder: "x"))
    }
    func testOmniIsOfferedOnlyWhereItFitsAndLFM3StaysTheFirstFit() throws {                            // O2
        let l = ModelList.builtIn()
        let six = l.entries(job: .responder, machine: "iPhone15,3", memoryGB: 5.6)                       // the owner's phone class
        XCTAssertTrue(try XCTUnwrap(six.first).id.hasPrefix("lfm3-1.7b"), "LFM3 is the first row on a 6 GB phone")
        let omni = try XCTUnwrap(l.models.first { $0.id == omniID })
        XCTAssertFalse(omni.isRecommended(machine: "iPhone15,3", memoryGB: 5.6), "not offered as a fit on 6 GB")
        XCTAssertTrue(omni.isRecommended(machine: "iPhone17,3", memoryGB: 7.4), "offered on an 8 GB phone")
        XCTAssertTrue(omni.isRecommended(machine: "iPhone18,2", memoryGB: 11.4))
        let eight = l.entries(job: .responder, machine: "iPhone17,3", memoryGB: 7.4)
        XCTAssertTrue(try XCTUnwrap(eight.first).id.hasPrefix("lfm3-1.7b"), "not measured better, so offered and not first")
        XCTAssertTrue(eight.contains { $0.id == omniID })
    }
    func testOmniCarriesTheLicenceAndTheCredit() throws {                                              // O3
        let e = try XCTUnwrap(ModelList.builtIn().models.first { $0.id == omniID })
        XCTAssertTrue(e.license.contains("LFM Open License v1.0"))
        XCTAssertTrue(e.license.contains("US$10M"))
        XCTAssertTrue((e.credit ?? "").contains("Liquid AI"))
    }
    func testOmniDownloadIsPinnedToTheCommitNotMain() throws {                                         // O4
        let spec = try XCTUnwrap(KnownModels.pinned.first { $0.id == omniID })
        let rev = try XCTUnwrap(ModelList.builtIn().models.first { $0.id == omniID }?.revision)
        XCTAssertEqual(spec.url.absoluteString, "https://huggingface.co/LiquidAI/LFM3-Omni-2B-GGUF/resolve/\(rev)/LFM3-Omni-2B-Q8_0.gguf")
    }
    func testOmniIsOnlyForNewerPhonesAndSaysItIsNotYetRun() throws {                                   // O2 (table side)
        let omni = try XCTUnwrap(CandidateTable.candidate(omniID, .responder))
        XCTAssertEqual(omni.minTier, .tier8)
        XCTAssertEqual(omni.family, "LFM3 Omni 2B")
        XCTAssertFalse(omni.qualityBasis.isEmpty)
    }
}
