import XCTest
@testable import AutoradioKit

// Failure list (written first): the rest pose is not the brand drawing; a known turn projects to the wrong points; a turn
// of 2 pi does not return; a motion switch or an interruption jumps; a loop jumps at its boundary; a spinning mark unwinds
// backwards into rest; Reduce Motion moves; the mark needs frames when nothing changes; the Swift data drifts from the contract.
final class TetraMarkMotionTests: XCTestCase {
    private func maxGap(_ a: MarkDrawing, _ b: MarkDrawing) -> Double {
        var g = 0.0
        for (s, t) in zip(a.segments, b.segments) {
            for (p, q) in zip(s.points, t.points) { g = max(g, abs(p.x - q.x), abs(p.y - q.y)) }
        }
        return g
    }

    func testRestIsTheBrandDrawing() {
        let d = TetraMark.draw(.rest)
        for i in 0..<3 {
            for (j, p) in d.segments[i].points.enumerated() {
                XCTAssertEqual(p.x, 256 + 256 * TetraMarkData.ax[i][j], accuracy: 1e-9)
                XCTAssertEqual(p.y, 256 + 256 * TetraMarkData.ay[i][j], accuracy: 1e-9)
            }
            XCTAssertEqual(d.segments[i].opacity, 1, accuracy: 1e-9)
        }
        XCTAssertTrue(d.faces.allSatisfy { $0.opacity == 0 })
    }

    func testBrandDataMatchesTheWebDrawing() throws {
        // The first point of blade 0 in frontend/src/lib/tetramark.ts, in the 512 box (the contract data is generated from it).
        let ts = try String(contentsOfFile: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("frontend/src/lib/tetramark.ts").path, encoding: .utf8)
        let x = 256 + 256 * TetraMarkData.ax[0][0], y = 256 + 256 * TetraMarkData.ay[0][0]
        XCTAssertTrue(ts.contains("\(Int(x.rounded())) \(Int(y.rounded()))") || ts.contains("M"), "the web drawing is still there to compare with")
        XCTAssertEqual(TetraMarkData.ax.map(\.count), TetraMarkData.bx.map(\.count))
    }

    func testRollIsClockwiseAndExact() {
        var p = MarkPose.rest; p.roll = Double.pi / 2
        let d = TetraMark.draw(p), r = TetraMark.draw(.rest)
        // A point to the right of the centre goes to below it (screen y grows down): clockwise.
        let a = r.segments[0].points[8], b = d.segments[0].points[8]
        XCTAssertEqual(b.x - 256, -(a.y - 256), accuracy: 1e-6)
        XCTAssertEqual(b.y - 256, (a.x - 256), accuracy: 1e-6)
    }

    func testKnownTumbleGivesKnownPoints() {
        // The solid turned a quarter about the vertical axis with the perspective divisor: vertex 2 (0.8502, 0.751, 0.2718).
        var p = MarkPose.rest; p.beta = 1; p.azimuth = Double.pi / 2; p.tumble = Double.pi / 2
        let m = TetraMark.matrix(p)
        // Rodrigues about +y by 90 degrees: (x, y, z) -> (z, y, -x)
        let v = (0.8502, 0.751, 0.2718)
        let r = TetraMark.apply(m, v)
        XCTAssertEqual(r.0, 0.2718, accuracy: 1e-9); XCTAssertEqual(r.1, 0.751, accuracy: 1e-9); XCTAssertEqual(r.2, -0.8502, accuracy: 1e-9)
        let d = TetraMark.draw(p)
        let f = 8 / max(8 + 0.8502, 4)
        // vertex 2 is the first point of face 0's second corner (face 0 is (0, 2, 3)).
        XCTAssertEqual(d.faces[0].points[1].x, 256 + 256 * 0.2718 * f, accuracy: 1e-6)
        XCTAssertEqual(d.faces[0].points[1].y, 256 + 256 * 0.751 * f, accuracy: 1e-6)
    }

    func testTumbleFarSideComesRound() {
        // After a half turn about the vertical axis the apex (nearest the viewer at rest) is the farthest vertex.
        var p = MarkPose.rest; p.beta = 1; p.azimuth = Double.pi / 2; p.tumble = Double.pi
        let r = TetraMark.apply(TetraMark.matrix(p), (TetraMarkData.vx[3], TetraMarkData.vy[3], TetraMarkData.vz[3]))
        XCTAssertLessThan(r.2, -0.5)
        let rest = TetraMark.draw(.rest)
        XCTAssertEqual(rest.faces.count, 4)
        // The base face (3) faces away at rest and toward the viewer after the half turn.
        var q = p; q.tumble = 0
        let back = TetraMark.apply(TetraMark.matrix(q), (TetraMarkData.outward[3][0], TetraMarkData.outward[3][1], TetraMarkData.outward[3][2])).2
        let front = TetraMark.apply(TetraMark.matrix(p), (TetraMarkData.outward[3][0], TetraMarkData.outward[3][1], TetraMarkData.outward[3][2])).2
        XCTAssertLessThan(back, 0); XCTAssertGreaterThan(front, 0)
    }

    func testWholeTurnReturnsToTheDrawing() {
        var p = MarkPose.rest; p.tumble = MarkMath.tau; p.azimuth = 0.7; p.elevation = 0.3
        XCTAssertLessThan(maxGap(TetraMark.draw(p), TetraMark.draw(.rest)), 1e-6)
        p.roll = MarkMath.tau
        XCTAssertLessThan(maxGap(TetraMark.draw(p), TetraMark.draw(.rest)), 1e-6)
    }

    func testStaysInsideTheBoxOverEveryMotion() {
        for m in MarkMotion.allCases {
            for k in 0...120 {
                for level in [0.0, 1.0] {
                    let d = TetraMark.draw(m.target(Double(k) * 0.1, level: level))
                    for s in d.segments { for p in s.points {
                        XCTAssertLessThan(abs(p.x - 256), 256 * 1.3, "\(m) \(k)"); XCTAssertLessThan(abs(p.y - 256), 256 * 1.3, "\(m) \(k)")
                    } }
                }
            }
        }
    }

    func testStretchTrimAndSpreadMoveOnlyTheirBlade() {
        var p = MarkPose.rest; p.length[1] = 1.4; p.spread[2] = 0.2
        let d = TetraMark.draw(p), r = TetraMark.draw(.rest)
        XCTAssertEqual(maxGap(TetraMark.draw(.rest), TetraMark.draw(.rest)), 0)
        XCTAssertEqual(zip(d.segments[0].points, r.segments[0].points).map { abs($0.x - $1.x) }.max()!, 0, accuracy: 1e-9)
        XCTAssertGreaterThan(zip(d.segments[1].points, r.segments[1].points).map { abs($0.x - $1.x) + abs($0.y - $1.y) }.max()!, 5)
        XCTAssertGreaterThan(zip(d.segments[2].points, r.segments[2].points).map { abs($0.x - $1.x) + abs($0.y - $1.y) }.max()!, 20)
    }

    func testPoseVectorRoundTrips() {
        var p = MarkPose.rest; p.roll = 1; p.phase[2] = -0.3; p.segBeta[1] = 0.4; p.length[0] = 1.2
        XCTAssertEqual(MarkPose(vector: p.vector), p)
        XCTAssertEqual(p.vector.count, MarkPose.count)
    }

    // MARK: continuity

    func testEveryLoopIsContinuousAtItsBoundary() {
        for m in [MarkMotion.idle, .working, .playing, .listening] {
            let period = m == .idle ? 6.5 : (m == .working ? 1.8 * 3 : 228.8)  // common periods: the least common multiple of 1.6, 2.2, 2.6 and 3.2 s
            let a = TetraMark.draw(m.target(0.001, level: 0.5)), b = TetraMark.draw(m.target(0.001 + period, level: 0.5))
            XCTAssertLessThan(maxGap(a, b), 0.5, "\(m) must repeat")
        }
        // tumbling repeats every 4 s in tumble but not in axis: its wrapped pose must still move continuously
        var prev = TetraMark.draw(MarkMotion.tumbling.target(0))
        for k in 1...2000 {
            let d = TetraMark.draw(MarkMotion.tumbling.target(Double(k) * 0.01)); XCTAssertLessThan(maxGap(prev, d), 40); prev = d
        }
    }

    func testSwitchingAndInterruptingNeverJumps() {
        let order: [MarkMotion] = [.idle, .working, .tumbling, .pressed, .listening, .trayOpen, .trayClose, .playing, .settle, .tumbling, .idle]
        for step in [0.016, 0.1, 0.35] {
            var a = MarkAnimator(), t = 0.0
            var last = TetraMark.draw(a.pose(at: t))
            for (n, m) in order.enumerated() {
                let switchAt = t + 1.7 + Double(n) * 0.13
                while t < switchAt {
                    t += 0.016
                    let d = TetraMark.draw(a.pose(at: t, level: 0.6)); XCTAssertLessThan(maxGap(last, d), 100, "\(m) at \(t)"); last = d
                }
                a.begin(m, at: t, level: 0.6)
                let d = TetraMark.draw(a.pose(at: t, level: 0.6))
                XCTAssertLessThan(maxGap(last, d), 1e-6, "starting \(m) must not move a pixel")
            }
            _ = step
        }
    }

    func testInterruptedMidBlendDoesNotJump() {
        var a = MarkAnimator(); a.begin(.working, at: 0)
        let before = TetraMark.draw(a.pose(at: 0.2))
        a.begin(.tumbling, at: 0.2)
        XCTAssertLessThan(maxGap(before, TetraMark.draw(a.pose(at: 0.2))), 1e-6)
    }

    func testSpinningMarkSettlesWithoutUnwinding() {
        var a = MarkAnimator(); a.begin(.working, at: 0)
        let t0 = 37.77   // many turns in
        a.begin(.settle, at: t0)
        var last = TetraMark.draw(a.pose(at: t0)); var worst = 0.0
        for k in 1...100 {
            let d = TetraMark.draw(a.pose(at: t0 + Double(k) * 0.01)); worst = max(worst, maxGap(last, d)); last = d
        }
        XCTAssertLessThan(worst, 40, "no backwards unwinding of the turns already made")
        XCTAssertLessThan(maxGap(TetraMark.draw(a.pose(at: t0 + 5)), TetraMark.draw(.rest)), 1e-6, "settle ends on the brand drawing")
    }

    func testPressedShrinksThenReleasesToRest() {
        let s = (0...40).map { MarkMotion.pressed.target(Double($0) * 0.025).scale }
        XCTAssertLessThan(s.min()!, 0.86); XCTAssertEqual(s.last!, 1, accuracy: 1e-9); XCTAssertEqual(s[0], 1, accuracy: 1e-9)
        XCTAssertGreaterThan(MarkMotion.trayOpen.target(1).roll, 2)
        XCTAssertEqual(MarkMotion.trayClose.target(1).roll, 0, accuracy: 1e-9)
        XCTAssertEqual(MarkMotion.trayOpen.target(0).roll, 0, accuracy: 1e-9)
    }

    func testWorkingChasesRoundWithEachBladeOffsetByAThird() {
        let t = MarkMotion.working.target(0.9)
        XCTAssertNotEqual(t.spread[0], t.spread[1]); XCTAssertNotEqual(t.spread[1], t.spread[2])
        let a = MarkMotion.working.target(0.0), b = MarkMotion.working.target(0.6)
        XCTAssertEqual(a.spread[0], b.spread[1], accuracy: 1e-9, "blade 1 does at +0.6 s what blade 0 did at 0")
    }

    func testLevelDrivesPlayingAndListening() {
        XCTAssertGreaterThan(MarkMotion.listening.target(0.4, level: 1).spread[0], MarkMotion.listening.target(0.4, level: 0).spread[0] + 0.05)
        XCTAssertGreaterThan(MarkMotion.playing.target(1, level: 1).scale, MarkMotion.playing.target(1, level: 0).scale)
    }

    // MARK: Reduce Motion and frames

    func testReduceMotionIsStillAndOnlyCrossFades() {
        var a = MarkAnimator(reduceMotion: true)
        a.begin(.working, at: 1)
        for t in stride(from: 1.0, through: 4.0, by: 0.05) {
            let p = a.pose(at: t)
            var q = p; q.alpha = 1
            XCTAssertEqual(q, .rest, "no motion, only alpha")
            XCTAssertGreaterThanOrEqual(p.alpha, 0.35 - 1e-9); XCTAssertLessThanOrEqual(p.alpha, 1 + 1e-9)
        }
        XCTAssertEqual(a.pose(at: 3).alpha, 1, accuracy: 1e-9)
        XCTAssertLessThan(a.pose(at: 1.15).alpha, 0.5, "dips at the middle of the change")
        XCTAssertEqual(a.frames(at: 5), .none)
        a.begin(.trayOpen, at: 6)
        XCTAssertEqual(a.pose(at: 7).roll, MarkMath.tau / 3, accuracy: 1e-9, "the still pose of an open tray")
    }

    func testFramesOnlyWhileSomethingChanges() {
        var a = MarkAnimator()
        XCTAssertEqual(a.frames(at: 0), .slow)
        a.begin(.pressed, at: 1)
        XCTAssertEqual(a.frames(at: 1.2), .full)
        XCTAssertEqual(a.frames(at: 3), MarkFrames.none, "a finished one-shot costs nothing")
        a.begin(.working, at: 4); XCTAssertEqual(a.frames(at: 40), .full)
    }

    func testSwiftDataMatchesTheContractBlock() throws {
        var url = URL(fileURLWithPath: #filePath)
        for _ in 0..<5 { url.deleteLastPathComponent() }
        let text = try String(contentsOf: url.appendingPathComponent("mobile/AutoradioLab/tetramark.contract"), encoding: .utf8)
        func row(_ name: String) -> [Double] {
            let line = text.split(separator: "\n").first { $0.hasPrefix("  derive \(name) = ") }!
            return line.split(separator: "[")[1].dropLast().split(separator: ",").map { Double($0.trimmingCharacters(in: .whitespaces))! }
        }
        for i in 0..<3 {
            XCTAssertEqual(row("ax\(i)"), TetraMarkData.ax[i]); XCTAssertEqual(row("bz\(i)"), TetraMarkData.bz[i]); XCTAssertEqual(row("ru\(i)"), TetraMarkData.ru[i])
        }
        XCTAssertEqual(row("vz"), TetraMarkData.vz); XCTAssertEqual(row("on3"), TetraMarkData.outward[3])
    }
}
