import XCTest
@testable import AutoradioKit

// Trimmed copies (invented content) of the shapes choir.news really answers, found by the first real-data run
// (mobile/scripts/proof-real.sh, 2026-11-11). Failure list first:
//  1. related: the real route answers `links` (with `reason` and `shared`), not `related`/`stories`/`items`.
//  2. front list: stories carry `route.bare`, `sections`, `entities: [{id,label}]`, `importance` and a fractional-second `updated_at`.
//  3. texture document list: no `metadata`, no `agent_revision_pending`; `revision_count` and `current_version_number` are numbers.
//  4. edition: `{"found":false}` is a normal answer, not an error.
//  5. whoami: `computer_id` may be the empty string for an owner-wide key.

final class RealShapeTests: XCTestCase {
    func json(_ s: String) -> [String: Any] { (try? JSONSerialization.jsonObject(with: Data(s.utf8))) as? [String: Any] ?? [:] }

    func testRelatedReadsLinks() {
        let o = json("""
        {"route":"/pub/texture/a-b-pub1","revision":5,"basis":{"captures":5,"events":0,"mode":"captures+relations+entities+floor"},"partial":false,
         "links":[{"publication_id":"pub-2","title":"Harbor tolls rise","dek":"A short line.","route":{"bare":"/pub/texture/harbor-tolls-rise-pub2","permalink":"/pub/texture/harbor-tolls-rise-pub2/v4"},
                   "section":"world","words":170,"reason":"entity","shared":{"events":0,"sources":0}},
                  {"publication_id":"pub-3","title":"","route":{"bare":"/pub/texture/x"}}]}
        """)
        let r = ReaderStore.parseRelated(o)
        XCTAssertEqual(r.count, 1)
        XCTAssertEqual(r.first?.address, "pub:pub/texture/harbor-tolls-rise-pub2")
        XCTAssertEqual(r.first?.title, "Harbor tolls rise")
    }

    func testFrontListRealShape() {
        let o = json("""
        {"stories":[{"publication_id":"pub-1","title":"A tide barrier is approved","dek":"Council votes.","updated_at":"2026-11-11T06:25:41.153634413Z",
          "version_count":1,"latest_revision":4,"route":{"bare":"/pub/texture/a-tide-barrier-pub1","permalink":"/pub/texture/a-tide-barrier-pub1/v4"},
          "importance":2,"promo":true,"section":"tech_ai","sections":["business","tech_ai"],"source_count":4,"words":449,
          "entities":[{"id":"ent:c8ce3f86","label":"Harbor Board"}]},
         {"publication_id":"pub-2","title":"No entities here","dek":"","updated_at":"2026-11-11T06:17:21.475191798Z","route":{"bare":"/pub/texture/no-ents-pub2"},"section":"world","words":194}],
         "next_cursor":"","sections":[]}
        """)
        let l = PaperStory.parseList(o)
        XCTAssertEqual(l.count, 2)
        XCTAssertEqual(l[0].entities, [PaperEntity(id: "ent:c8ce3f86", label: "Harbor Board")])
        XCTAssertEqual(l[0].importance, 2)
        XCTAssertNotNil(l[0].updated, "fractional seconds with nine digits must parse")
        XCTAssertEqual(l[1].entities, [])
    }

    func testTextureListRealShape() {
        let o = json("""
        {"documents":[{"doc_id":"d1","owner_id":"o","title":"Harbor notes","computer_id":"computer-x","trajectory_id":"t","current_revision_id":"r4",
          "current_version_number":4,"created_at":"2026-11-10T23:26:43.085Z","updated_at":"2026-11-10T23:28:13.500Z","revision_count":5,"last_editor":"appagent","last_author_kind":"appagent"}],
         "next_cursor":""}
        """)
        let l = TextureList.parse(o)
        XCTAssertEqual(l.status, "ready")
        XCTAssertEqual(l.rows.count, 1)
        XCTAssertEqual(l.rows[0].address, "texture:d1")
        XCTAssertFalse(l.hasEdition)
        XCTAssertTrue(l.rows[0].meta.contains("5 versions"))
    }

    func testWhoAmIOwnerKeyHasEmptyComputer() throws {
        let w = try WhoAmI.parse(Data(#"{"handle":"writer-abcdef","key_id":"ak_1","key_label":"CLI key","scopes":["admin","owner:acts"],"computer_id":""}"#.utf8))
        XCTAssertEqual(w.handle, "writer-abcdef")
        XCTAssertEqual(w.computerID, "")
        XCTAssertEqual(ChoirAccountClient.filled(w, stored: "computer-demo-1").computerID, "computer-demo-1", "the account page names the computer the phone found")
        XCTAssertEqual(ChoirAccountClient.filled(w, stored: nil).computerID, "")
        let bound = WhoAmI(handle: "h", computerID: "computer-bound")
        XCTAssertEqual(ChoirAccountClient.filled(bound, stored: "computer-other").computerID, "computer-bound")
    }

    func testEditionNotFoundIsNotAnError() throws {
        XCTAssertEqual(try EditionInfo.parse(Data(#"{"found":false}"#.utf8)), EditionInfo())
        XCTAssertFalse(try EditionInfo.parse(Data(#"{"found":true}"#.utf8)).found, "found without a document id cannot be opened")
        let e = try EditionInfo.parse(Data(#"{"found":true,"doc_id":"d9","revision_id":"r2","updated_at":"2026-11-11T05:00:00Z"}"#.utf8))
        XCTAssertEqual(e, EditionInfo(found: true, docID: "d9", revisionID: "r2", updatedAt: "2026-11-11T05:00:00Z"))
        XCTAssertThrowsError(try EditionInfo.parse(Data("[]".utf8)))
    }

    func testTheServersEditionIsPinnedFirst() {
        let list = TextureList(status: "ready", rows: [
            TextureRow(address: "texture:a", title: "Notes", updated: "", meta: "", edition: false, revising: false),
            TextureRow(address: "texture:d9", title: "Morning paper", updated: "", meta: "", edition: false, revising: false)])
        let pinned = TextureList.applying(EditionInfo(found: true, docID: "d9"), to: list)
        XCTAssertEqual(pinned.rows.map(\.address), ["texture:d9", "texture:a"])
        XCTAssertTrue(pinned.rows[0].edition)
        XCTAssertTrue(pinned.hasEdition)
        let absent = TextureList.applying(EditionInfo(found: true, docID: "old"), to: list)
        XCTAssertEqual(absent.rows.first?.address, "texture:old", "an edition older than the page of documents is still shown")
        XCTAssertEqual(TextureList.applying(EditionInfo(), to: list), list)
        XCTAssertEqual(TextureList.applying(nil, to: list), list)
    }

    func testDekTidiesSentenceSpacing() {
        // The real dek comes from a sentence tokeniser: "Abu Dhabi . The Kremlin ... earlier: ."
        XCTAssertEqual(PaperStory.tidy("talks opened in Abu Dhabi . The Kremlin had said earlier: ."), "talks opened in Abu Dhabi. The Kremlin had said earlier:")
        XCTAssertEqual(PaperStory.tidy("A plain line, kept."), "A plain line, kept.")
        XCTAssertEqual(PaperStory.tidy("cut off\u{2026}"), "cut off\u{2026}")
    }
}
