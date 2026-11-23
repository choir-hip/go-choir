import XCTest
@testable import AutoradioKit

/// Review nit (phone train, finding D): `EmbedDevice.ensure` had no single-flight guard, so two concurrent first calls both ran the loader (two model loads on a phone).
/// Failure list: 1. two concurrent ensures load once and both answer true; 2. a failed load answers false to both and a later ensure tries again; 3. once loaded, ensure does not load again.
final class EmbedDeviceTests: XCTestCase {
    struct Fake: Embedder, QueryEmbedding {
        var engine: String { "fake" }
        var dimension: Int { 2 }
        func embed(_ prompts: [String]) async throws -> [[Float]] { prompts.map { _ in [1, 0] } }
        func embedQuery(_ text: String) -> [Float]? { [1, 0] }
    }
    var saved: (@Sendable () async throws -> any Embedder & QueryEmbedding)!
    override func setUp() { saved = EmbedDevice.loader }
    override func tearDown() { EmbedDevice.loader = saved }

    func testTwoConcurrentEnsuresLoadOnce() async {
        let loads = Locked(0)
        EmbedDevice.loader = { loads.write { $0 += 1 }; try await Task.sleep(nanoseconds: 150_000_000); return Fake() }
        let d = EmbedDevice()
        async let a = d.ensure(download: false), b = d.ensure(download: false)
        let (x, y) = await (a, b)
        XCTAssertTrue(x); XCTAssertTrue(y)
        XCTAssertEqual(loads.value, 1, "one load for two concurrent first calls")
        let z = await d.ensure(download: false)
        XCTAssertTrue(z); XCTAssertEqual(loads.value, 1, "loaded: not loaded again")
    }

    func testAFailedLoadAnswersFalseAndALaterEnsureTriesAgain() async {
        let loads = Locked(0)
        EmbedDevice.loader = { loads.write { $0 += 1 }; throw EmbedderError.unavailable("no") }
        let d = EmbedDevice()
        async let a = d.ensure(download: false), b = d.ensure(download: false)
        let (x, y) = await (a, b)
        XCTAssertFalse(x); XCTAssertFalse(y)
        XCTAssertEqual(loads.value, 1)
        let z = await d.ensure(download: false)
        XCTAssertFalse(z); XCTAssertEqual(loads.value, 2, "a failure is not remembered")
    }
}
