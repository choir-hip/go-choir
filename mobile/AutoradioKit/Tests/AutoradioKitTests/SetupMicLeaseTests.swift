import XCTest
@testable import AutoradioKit

// SetupMicLease (Voice/SetupMicLease.swift): the failure list L1 to L5 is at its head.
final class SetupMicLeaseTests: XCTestCase {
    final class Rec: @unchecked Sendable {
        let lock = NSLock(); var log: [String] = []; var granted = true; var gate: CheckedContinuation<Void, Never>?; var hold = false
        func add(_ s: String) { lock.lock(); log.append(s); lock.unlock() }
        var all: [String] { lock.lock(); defer { lock.unlock() }; return log }
    }
    func make(_ r: Rec) -> SetupMicLease {
        SetupMicLease(.init(
            ask: { r.add("ask"); if r.hold { await withCheckedContinuation { c in r.lock.lock(); r.gate = c; r.lock.unlock() } }; return r.granted },
            openEngine: { r.add("engine") },
            setTap: { t in r.add(t == nil ? "tap:nil" : "tap:set") },
            micOn: { on in r.add(on ? "mic:on" : "mic:off") },
            restore: { r.add("restore") }))
    }
    func waitFor(_ r: Rec, _ s: String) async { for _ in 0..<200 { if r.all.contains(s) { return }; try? await Task.sleep(nanoseconds: 5_000_000) } }
    func answer(_ r: Rec) { r.lock.lock(); let g = r.gate; r.gate = nil; r.lock.unlock(); g?.resume() }

    func testL1BetweenTakesTheMicrophoneIsOffAndTheTapStays() async {
        let r = Rec(); let l = make(r)
        let a = await l.begin(wanted: { true }, onFrame: { _ in }); XCTAssertEqual(a, .open)
        await l.endTake()
        let b = await l.begin(wanted: { true }, onFrame: { _ in }); XCTAssertEqual(b, .open)
        await l.endTake(); await l.release()
        XCTAssertEqual(r.all, ["ask", "engine", "tap:set", "mic:on", "mic:off", "ask", "engine", "tap:set", "mic:on", "mic:off", "tap:nil", "restore"])
        XCTAssertFalse(r.all.dropLast(2).contains("tap:nil"), "the tap was never cleared between takes: nothing said between sentences reaches the turn engine")
    }
    func testL2AnAnswerAfterThePageIsGoneOpensNothing() async {
        let r = Rec(); r.hold = true; let l = make(r)
        let t = Task { await l.begin(wanted: { true }, onFrame: { _ in }) }
        await waitFor(r, "ask")
        await l.release()
        answer(r)
        let out = await t.value
        XCTAssertEqual(out, .stale)
        XCTAssertFalse(r.all.contains("engine")); XCTAssertFalse(r.all.contains("mic:on")); XCTAssertFalse(r.all.contains("tap:set"))
        XCTAssertEqual(r.all.last, "restore")
    }
    func testL2ATakeGivenUpWhileWaitingOpensNothingAndGivesTheLeaseBack() async {
        let r = Rec(); r.hold = true; let l = make(r)
        final class Flag: @unchecked Sendable { var v = true }; let want = Flag()
        let t = Task { await l.begin(wanted: { want.v }, onFrame: { _ in }) }
        await waitFor(r, "ask"); want.v = false; answer(r)
        let out = await t.value; XCTAssertEqual(out, .stale)
        XCTAssertFalse(r.all.contains("mic:on")); let held = await l.isHeld; XCTAssertFalse(held)
        XCTAssertEqual(Array(r.all.suffix(2)), ["tap:nil", "restore"])
    }
    func testL3ARefusalClosesEverything() async {
        let r = Rec(); r.granted = false; let l = make(r)
        let out = await l.begin(wanted: { true }, onFrame: { _ in }); XCTAssertEqual(out, .refused)
        XCTAssertEqual(r.all, ["ask", "tap:nil", "restore"]); let held = await l.isHeld; XCTAssertFalse(held)
    }
    func testL4ReleaseIsHarmlessAndAlwaysRestores() async {
        let r = Rec(); let l = make(r)
        await l.release(); await l.release()
        XCTAssertEqual(r.all, ["tap:nil", "restore", "tap:nil", "restore"])
    }
    func testL5ASecondBeginWhileTheFirstWaitsMakesTheFirstDoNothing() async {
        let r = Rec(); r.hold = true; let l = make(r)
        let first = Task { await l.begin(wanted: { true }, onFrame: { _ in }) }
        await waitFor(r, "ask")
        r.hold = false
        let second = await l.begin(wanted: { true }, onFrame: { _ in }); XCTAssertEqual(second, .open)
        answer(r)
        let f = await first.value; XCTAssertEqual(f, .stale)
        XCTAssertEqual(r.all.filter { $0 == "mic:on" }.count, 1)
        let held = await l.isHeld; XCTAssertTrue(held, "the newer begin keeps the lease")
    }
}
