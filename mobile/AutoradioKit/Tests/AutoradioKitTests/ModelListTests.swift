import XCTest
@testable import AutoradioKit

// The model list as data. Failure list (each line a test): L1 the built-in copy does not survive JSON; L2 an unknown job or field from a newer
// server empties the list; L3 an entry the store could not verify is offered; L4 recommendations ignore the device; L5 the bundle the store
// fetches differs from the one the build pins; L6 the built-in copy disagrees with CandidateTable; L7 the served list differs from the build's, or holds an entry the phone drops.
final class ModelListTests: XCTestCase {
    /// L7: the list the server hands out (internal/proxy/phone_model_list.json, served at /api/phone/model-list) is this build's list. A phone
    /// that fetches it must lose no entry to the validity rules, and the two must not drift: run once with AR_WRITE_MODEL_LIST=1 to rewrite the file.
    func testServedListIsTheBuiltInList() throws {
        let served = URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("../../../../internal/proxy/phone_model_list.json").standardizedFileURL
        let built = ModelList.builtIn()
        if ProcessInfo.processInfo.environment["AR_WRITE_MODEL_LIST"] == "1" { try (built.encoded() + Data("\n".utf8)).write(to: served) }
        let data = try Data(contentsOf: served)
        let raw = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let list = try ModelList.decode(data)
        XCTAssertEqual(list.models.count, (raw["models"] as? [Any])?.count, "the phone dropped a served entry as invalid")
        XCTAssertEqual(list, built, "the served list differs from the build's: regenerate with AR_WRITE_MODEL_LIST=1, or bring the tables to match")
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
    func testBuiltInHasEveryCandidate() {                                                             // L6
        let ids = Set(ModelList.builtIn().models.map(\.id))
        for c in CandidateTable.all where !c.isFloor { XCTAssertTrue(ids.contains(c.id), "\(c.id) is in the table but not the built-in list") }
    }
}
