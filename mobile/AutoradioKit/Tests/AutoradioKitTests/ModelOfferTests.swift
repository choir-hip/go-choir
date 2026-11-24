import XCTest
@testable import AutoradioKit

// What voice needs from the network, read from the model lists (ModelOffer), and the picks setup keeps (StagePicksStore). The failure list, written first:
//  M1 the jobs are the list's own, each option with a size from the list and the host, one option per job marked recommended, the same one setup would download first; a built-in floor is never an option.
//  M2 a stage is ready when setup recorded a model that is not its built-in floor; unknown ids from the configurator count unless their stage is a built-in one; no record is nothing ready.
//  M3 the picks round-trip, only the hearing kinds are kept, and a bad file is empty.
final class ModelOfferTests: XCTestCase {
    private let phone = DeviceProfile(chip: "A", memoryGB: 8, freeDiskGB: 50, onWiFi: true, availableMemoryMB: 6000)
    func testM1JobsAreTheListsOwn() {
        let jobs = ModelOffer.jobs(profile: phone)
        XCTAssertEqual(jobs.map(\.job), ModelList.jobs.filter { j in !ModelList.builtIn.options(j).isEmpty })
        XCTAssertEqual(jobs.last?.job, "search"); XCTAssertEqual(jobs.last?.options.first?.bytes, KnownModels.embeddingGemma2.bytes)
        for j in jobs {
            XCTAssertEqual(j.host, "Hugging Face"); XCTAssertFalse(j.options.isEmpty)
            XCTAssertTrue(j.options.allSatisfy { $0.bytes > 0 && !$0.sizeLabel.isEmpty && !$0.model.isEmpty })
            XCTAssertLessThanOrEqual(j.options.filter(\.recommended).count, 1)
            XCTAssertEqual(ModelOffer.group(j.job), j.job == "voice" ? "voice" : j.job == "search" ? "search" : "voice-rest")
        }
        XCTAssertFalse(jobs.contains { $0.job == "decision" })
        let tr = jobs.first { $0.job == "transcriber" }!
        XCTAssertEqual(tr.options.first { $0.recommended }?.id, SetupCatalog.plan(for: phone)[.transcriber]?.first?.id, "the recommendation is what setup would download first")
        let ready = ModelOffer.jobs(profile: phone, ready: ["voice": jobs.first { $0.job == "voice" }!.options[0].id])
        XCTAssertTrue(ready.first { $0.job == "voice" }!.options[0].ready); XCTAssertFalse(ready.first { $0.job == "transcriber" }!.options.contains { $0.ready })
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
