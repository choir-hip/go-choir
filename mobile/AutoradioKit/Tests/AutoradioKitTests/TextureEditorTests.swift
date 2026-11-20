import XCTest
@testable import AutoradioKit

// S1: the editor's save rules (TextureEditor.swift lists the ways it can fail).
@MainActor
final class TextureEditorTests: XCTestCase {
    final class FakeAPI: TextureAPI {
        var head = TextureHead(content: "Hello world\n\nSecond", body: ["schema": "choir.texture_doc.v1", "doc": ["type": "doc", "content": [
            ["type": "paragraph", "attrs": ["id": "p1"], "content": [["type": "text", "text": "Hello world"]]],
            ["type": "paragraph", "attrs": ["id": "p2"], "content": [["type": "text", "text": "Second"]]]]]], revisionId: "r1", version: 1)
        var posts: [(content: String, body: [String: Any], parent: String)] = []
        var failure: Error?
        func head(docId: String) async throws -> TextureHead { head }
        func post(docId: String, content: String, body: [String: Any], parent: String) async throws -> (revisionId: String, version: Int) {
            if let failure { throw failure }
            posts.append((content, body, parent))
            head = TextureHead(content: content, body: body, revisionId: "r\(posts.count + 1)", version: posts.count + 1)
            return (head.revisionId, head.version)
        }
    }
    func make(_ api: FakeAPI, storage: URL? = nil) -> TextureEditor { let e = TextureEditor(api: api, storage: storage); e.idle = 0.15; e.retryAfter = 3600; return e }

    func test1_ABurstOfTypingIsOneRevisionPostedAfterThePause() async throws {
        let api = FakeAPI(), e = make(api)
        for t in ["Hello world!", "Hello world!!", "Hello world!!!\n\nSecond"] { await e.edit(address: "texture:d1", markdown: t) }
        XCTAssertEqual(e.info(address: "texture:d1").status, "dirty"); XCTAssertTrue(api.posts.isEmpty)
        try await Task.sleep(nanoseconds: 600_000_000)
        XCTAssertEqual(api.posts.count, 1, "one revision for the burst")
        XCTAssertEqual(api.posts[0].content, "Hello world!!!\n\nSecond"); XCTAssertEqual(api.posts[0].parent, "r1")
        XCTAssertEqual(e.info(address: "texture:d1").label, "Saved · version 2")
        let body = api.posts[0].body
        let nodes = TextureBodyMerge.topNodes(body)
        XCTAssertEqual(nodes.count, 2); XCTAssertEqual((nodes[1]["attrs"] as? [String: Any])?["id"] as? String, "p2", "the paragraph that did not change is the original node")
    }

    func test2_LeavingThePagePostsAtOnce() async throws {
        let api = FakeAPI(), e = make(api); e.idle = 60
        await e.edit(address: "texture:d1", markdown: "Changed\n\nSecond")
        await e.flush(address: "texture:d1")
        XCTAssertEqual(api.posts.count, 1); XCTAssertEqual(e.info(address: "texture:d1").status, "saved")
    }

    func test3_ARevisionAndAStoryHaveNoEditor() async {
        let api = FakeAPI(), e = make(api)
        await e.edit(address: "texture:d1@r0", markdown: "x"); await e.edit(address: "pub:story", markdown: "x"); await e.flushAll()
        XCTAssertTrue(api.posts.isEmpty)
        XCTAssertFalse(e.info(address: "texture:d1@r0").editable); XCTAssertFalse(e.info(address: "pub:story").editable); XCTAssertTrue(e.info(address: "texture:d1").editable)
    }

    func test4_NothingChangedPostsNothing() async {
        let api = FakeAPI(), e = make(api)
        await e.edit(address: "texture:d1", markdown: "Hello world\n\nSecond"); await e.flush(address: "texture:d1")
        XCTAssertTrue(api.posts.isEmpty)
    }

    func test5_OfflineEditsAreKeptAcrossALaunchAndPostedOnce() async throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("tex-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: f) }
        let api = FakeAPI(); api.failure = ChoirError.transport("offline")
        let e = make(api, storage: f)
        await e.edit(address: "texture:d1", markdown: "Kept offline\n\nSecond"); await e.flush(address: "texture:d1")
        XCTAssertEqual(e.info(address: "texture:d1").status, "offline"); XCTAssertEqual(e.pending["d1"], "Kept offline\n\nSecond")
        // a relaunch: a new editor reads what was kept, the route answers, it is posted once
        let api2 = FakeAPI()
        let e2 = make(api2, storage: f)
        XCTAssertEqual(e2.pending["d1"], "Kept offline\n\nSecond")
        XCTAssertEqual(e2.info(address: "texture:d1").status, "offline")
        await e2.retryPending()
        XCTAssertEqual(api2.posts.count, 1); XCTAssertEqual(api2.posts[0].content, "Kept offline\n\nSecond")
        XCTAssertTrue(e2.pending.isEmpty)
        await e2.retryPending()
        XCTAssertEqual(api2.posts.count, 1, "not twice")
    }

    func test6_TypingWhileItIsPostingSavesRightAfter() async throws {
        final class Slow: TextureAPI {
            let inner = FakeAPI(); var gate: CheckedContinuation<Void, Never>?
            func head(docId: String) async throws -> TextureHead { inner.head }
            func post(docId: String, content: String, body: [String: Any], parent: String) async throws -> (revisionId: String, version: Int) {
                if inner.posts.isEmpty { await withCheckedContinuation { gate = $0 } }
                return try await inner.post(docId: docId, content: content, body: body, parent: parent)
            }
        }
        let api = Slow(); let e = TextureEditor(api: api); e.idle = 0.05
        await e.edit(address: "texture:d1", markdown: "one\n\nSecond")
        try await Task.sleep(nanoseconds: 300_000_000)                              // posting, held at the gate
        await e.edit(address: "texture:d1", markdown: "one two\n\nSecond")
        api.gate?.resume(); api.gate = nil
        try await Task.sleep(nanoseconds: 700_000_000)
        XCTAssertEqual(api.inner.posts.map(\.content), ["one\n\nSecond", "one two\n\nSecond"])
    }
}
