import XCTest
@testable import AutoradioKit

// The model list as data. Failure list (each line a test): L1 the built-in copy does not survive JSON; L2 an unknown job or field from a newer
// server empties the list; L3 an entry the store could not verify is offered; L4 recommendations ignore the device; L5 the bundle the store
// fetches differs from the one the build pins; L6 the built-in copy disagrees with CandidateTable.
// LFM3-Omni-2B as a larger responder (owner, 2026-11-24): O1 the row is missing or its pin is not a commit with a verifiable file; O2 it is offered
// on a phone that cannot hold it, or it displaces LFM3 as the first fit on the owner's class; O3 its licence and the credit the licence asks for are
// not carried; O4 the pinned download url is the moving `main` and not the commit; O5 the text path lays the chat out differently from LFM3's card.
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
