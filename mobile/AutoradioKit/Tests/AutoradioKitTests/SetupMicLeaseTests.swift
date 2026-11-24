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
    // Round 3: the prompt is not raised for a take nobody wants any more.
    func testL6NothingIsAskedWhenTheTakeIsAlreadyUnwanted() async {
        let r = Rec(); let l = make(r)
        let out = await l.begin(wanted: { false }, onFrame: { _ in }); XCTAssertEqual(out, .stale)
        XCTAssertFalse(r.all.contains("ask")); XCTAssertEqual(r.all.last, "restore")
    }
    // Round 3: an older begin that was overtaken must not close the newer take's microphone on its way out.
    func testL7AnOvertakenBeginNeverTurnsTheNewerMicrophoneOff() async {
        let r = Rec(); r.hold = false; let l = make(r)
        final class Gate: @unchecked Sendable { let lock = NSLock(); var open = false; var v: Bool { get { lock.lock(); defer { lock.unlock() }; return open } set { lock.lock(); open = newValue; lock.unlock() } } }
        let tapGate = Gate()
        let slow = SetupMicLease(.init(ask: { true }, openEngine: {}, setTap: { t in r.add(t == nil ? "tap:nil" : "tap:set"); if t != nil { while !tapGate.v { try? await Task.sleep(nanoseconds: 2_000_000) } } },
                                       micOn: { on in r.add(on ? "mic:on" : "mic:off") }, restore: { r.add("restore") }))
        let first = Task { await slow.begin(wanted: { true }, onFrame: { _ in }) }
        await waitFor(r, "tap:set")
        await slow.release()                                   // the page left while the tap was being set
        tapGate.v = true
        let f = await first.value; XCTAssertEqual(f, .stale)
        XCTAssertFalse(r.all.contains("mic:on"), "an overtaken begin never turns the microphone on")
        _ = l
    }
    // Round 3: the app going to the background lets go.
    func testL8TheAppGoingToTheBackgroundReleasesTheLiveLease() async {
        let r = Rec(); let l = make(r); SetupMicLease.live = l
        _ = await l.begin(wanted: { true }, onFrame: { _ in })
        await SetupMicLease.releaseLive()
        XCTAssertEqual(Array(r.all.suffix(2)), ["tap:nil", "restore"]); let held = await l.isHeld; XCTAssertFalse(held)
        SetupMicLease.live = nil
    }
}
