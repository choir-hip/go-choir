import XCTest
@testable import AutoradioKit

// What voice needs from the network, read from the model lists (ModelOffer), and the picks setup keeps (StagePicksStore). The failure list, written first:
//  M1 the rows are the lists' own: the voice first, then the hearing and the rest, then search; every row has a size from its list and the host; a built-in floor is never a row.
//  M2 a stage is ready when setup recorded a model that is not its built-in floor; unknown ids from the configurator count unless their stage is a built-in one; no record is nothing ready.
//  M3 the picks round-trip, only the hearing kinds are kept, and a bad file is empty.
final class ModelOfferTests: XCTestCase {
    private let phone = DeviceProfile(chip: "A", memoryGB: 8, freeDiskGB: 50, onWiFi: true, availableMemoryMB: 6000)
    func testM1RowsAreTheListsOwn() {
        let rows = ModelOffer.rows(profile: phone)
        XCTAssertEqual(rows.first?.stage, "voice"); XCTAssertEqual(rows.last?.stage, "search"); XCTAssertEqual(rows.last?.bytes, KnownModels.embeddingGemma2.bytes)
        XCTAssertTrue(rows.allSatisfy { $0.bytes > 0 && $0.host == "Hugging Face" && !$0.sizeLabel.isEmpty })
        XCTAssertEqual(Set(rows.map(\.id)).count, rows.count)
        for r in rows where r.stage != "search" { XCTAssertEqual(r.bytes, SetupCatalog.ladder(SetupStage(rawValue: r.stage)!).first { $0.id == r.id }?.bytes) }
    }
    func testM2Ready() {
        XCTAssertEqual(ModelOffer.readyStages(record: nil), [])
        func item(_ s: String, _ chosen: String, _ kit: String) -> SetupRecord.Item { .init(stage: s, chosen: chosen, kitStageID: kit, fellBackFrom: [], reasons: [], check: nil) }
        let rec = SetupRecord(device: phone, at: 0, items: [item("voice", "kokoro-82m-ane", "kokoro"), item("transcriber", "apple-speech", "apple-speech"), item("bounds", "some-new-vad", "silero-vad"), item("responder", "cue-only", "local-ack")])
        let ready = ModelOffer.readyStages(record: rec)
        XCTAssertTrue(ready.contains("voice")); XCTAssertFalse(ready.contains("transcriber")); XCTAssertTrue(ready.contains("bounds")); XCTAssertFalse(ready.contains("responder"))
    }
    func testM3Picks() throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("picks-\(UUID().uuidString)/stage-picks.json"); let s = StagePicksStore(file: f)
        XCTAssertEqual(s.load(), [:])
        s.save([.transcriber: "parakeet-tdt", .speechBounds: "silero-vad", .voice: "kokoro"])
        XCTAssertEqual(s.load(), ["transcriber": "parakeet-tdt", "speechBounds": "silero-vad"])
        try Data("nope".utf8).write(to: f); XCTAssertEqual(s.load(), [:])
    }
}
