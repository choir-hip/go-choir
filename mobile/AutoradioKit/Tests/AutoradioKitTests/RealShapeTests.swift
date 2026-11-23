import XCTest
@testable import AutoradioKit

// Trimmed copies (invented content) of the shapes choir.news really answers, found by the first real-data run
// (mobile/scripts/proof-real.sh, 2026-11-11). Failure list first:
//  1. related: the real route answers `links` (with `reason` and `shared`), not `related`/`stories`/`items`.
//  2. (front list: the Swift PaperStory is gone; shared code reads it, AutoradioLab/paper-model.ts.)
//  3. texture document list: no `metadata`, no `agent_revision_pending`; `revision_count` and `current_version_number` are numbers.
//  4. edition: `{"found":false}` is a normal answer, not an error.
//  5. whoami: `computer_id` may be the empty string for an owner-wide key.

final class RealShapeTests: XCTestCase {
    func json(_ s: String) -> [String: Any] { (try? JSONSerialization.jsonObject(with: Data(s.utf8))) as? [String: Any] ?? [:] }

    func testEditionNotFoundIsNotAnError() throws {
        XCTAssertEqual(try EditionInfo.parse(Data(#"{"found":false}"#.utf8)), EditionInfo())
        XCTAssertFalse(try EditionInfo.parse(Data(#"{"found":true}"#.utf8)).found, "found without a document id cannot be opened")
        let e = try EditionInfo.parse(Data(#"{"found":true,"doc_id":"d9","revision_id":"r2","updated_at":"2026-11-11T05:00:00Z"}"#.utf8))
        XCTAssertEqual(e, EditionInfo(found: true, docID: "d9", revisionID: "r2", updatedAt: "2026-11-11T05:00:00Z"))
        XCTAssertThrowsError(try EditionInfo.parse(Data("[]".utf8)))
    }

}
