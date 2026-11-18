import Foundation

// The tetramark's motion model: pure arithmetic, no UI, no clock of its own (every function takes a time).
//
// WHAT THE MARK IS. Three hand-drawn blades (frontend/src/lib/tetramark.ts) that read as a triskelion and as a flat
// projection of a tetrahedron. Each blade is a closed polygon of the brand's own points, so the freehand character
// survives every transform here: nothing is replaced by a straight line, every point of every blade is moved.
//
// THE POSE is 24 numbers (`MarkPose.vector`), the whole state of the mark at one instant:
//   whole: roll (in-plane turn, radians, positive is clockwise on screen), tumble (a turn of the solid about an axis, radians),
//          azimuth and elevation (the tumble axis: (cosE cosA, cosE sinA, sinE); elevation pi/2 is the viewing axis, which
//          makes a tumble an in-plane roll), beta (0 the flat brand drawing, 1 the real tetrahedron in perspective, a
//          point-for-point blend), scale, alpha, and a nudge (dx, dy, in radii of the brand box)
//   per blade i: phase (a roll of that blade alone about the mark's centre), spread (how far it moves out from the centre,
//          in radii; negative moves in), length (1 as drawn; stretches the blade along its own tangent), scale, beta.
// The rest pose is every number at its neutral value and projects to EXACTLY the brand drawing.
//
// PROJECTION. Each blade point is the blend of its flat (x, y, 0) and its tetrahedron (x, y, z) place, stretched, rolled
// and spread by the blade's own numbers, then the whole is turned by Rz(roll) * Rodrigues(axis, tumble) and
// perspective-projected (eye 8 radii back, the divisor clamped at 4 so nothing blows up). When the tumble leaves a whole
// turn, the blades are blended toward the real solid (`solidWeight`), so a half-turn shows a tetrahedron whose hidden
// vertex has come round, and a whole turn lands on the flat drawing again.
//
// MOTION. A named motion is a closed-form target pose of the seconds since it began (and an input level). Starting a
// motion never jumps: the animator captures the pose on screen (angles wrapped to the nearest turn, which is the same
// picture) and blends from it to the target with an eased or overshooting curve. Reduce Motion: the pose is at rest, and a
// pose change only cross-fades.
//
// Every number here is mirrored by mobile/AutoradioLab/tetramark.contract (the contract draws; Tests pin this file).

public struct MarkPose: Equatable {
    public static let count = 24
    public var roll = 0.0, tumble = 0.0, azimuth = 0.0, elevation = 0.0
    public var beta = 0.0, scale = 1.0, alpha = 1.0, dx = 0.0, dy = 0.0
    public var phase = [0.0, 0.0, 0.0], spread = [0.0, 0.0, 0.0], length = [1.0, 1.0, 1.0]
    public var segScale = [1.0, 1.0, 1.0], segBeta = [0.0, 0.0, 0.0]

    public init() {}
    public static let rest = MarkPose()

    public var vector: [Double] {
        var v = [roll, tumble, azimuth, elevation, beta, scale, alpha, dx, dy]
        for i in 0..<3 { v += [phase[i], spread[i], length[i], segScale[i], segBeta[i]] }
        return v
    }

    public init(vector v: [Double]) {
        precondition(v.count == MarkPose.count, "a mark pose is 24 numbers")
        roll = v[0]; tumble = v[1]; azimuth = v[2]; elevation = v[3]
        beta = v[4]; scale = v[5]; alpha = v[6]; dx = v[7]; dy = v[8]
        for i in 0..<3 {
            let b = 9 + 5 * i
            phase[i] = v[b]; spread[i] = v[b + 1]; length[i] = v[b + 2]; segScale[i] = v[b + 3]; segBeta[i] = v[b + 4]
        }
    }

    /// k = 0 is `a`, k = 1 is `b`; every number is blended, so the result is continuous in k.
    public static func lerp(_ a: MarkPose, _ b: MarkPose, _ k: Double) -> MarkPose {
        MarkPose(vector: zip(a.vector, b.vector).map { $0 + ($1 - $0) * k })
    }

    /// The same picture with every angle brought to (-pi, pi]: a whole number of turns is the identity for roll, tumble and
    /// azimuth, so this never moves a pixel, and a blend from it never unwinds a spinning mark backwards.
    public func wrapped() -> MarkPose {
        var p = self
        p.roll = MarkMath.wrap(roll); p.tumble = MarkMath.wrap(tumble); p.azimuth = MarkMath.wrap(azimuth)
        p.phase = phase.map(MarkMath.wrap)
        return p
    }
}

public enum MarkMath {
    public static let tau = 2 * Double.pi
    /// Any angle to (-pi, pi].
    public static func wrap(_ a: Double) -> Double {
        let w = a - tau * floor((a + Double.pi) / tau)
        return w == -Double.pi ? Double.pi : w
    }
    public static func clamp01(_ x: Double) -> Double { min(1, max(0, x)) }
    public static func smooth(_ x: Double) -> Double { let u = clamp01(x); return u * u * (3 - 2 * u) }
    /// 0 at x = 0 and every whole number, 1 at the halves: a loop's breath, so every loop begins and ends at rest.
    public static func bump(_ x: Double) -> Double { 0.5 - 0.5 * cos(tau * x) }
    /// Ease out with overshoot (a spring's shape without its exponential): 0 at 0, 1 at 1, about 1.09 near 0.7.
    public static func back(_ x: Double) -> Double {
        let u = clamp01(x) - 1, c1 = 1.70158 * 0.6
        return 1 + (c1 + 1) * u * u * u + c1 * u * u
    }
    public static func lerp(_ a: Double, _ b: Double, _ k: Double) -> Double { a + (b - a) * k }
}

// MARK: the drawing

public struct MarkPoint: Equatable { public var x: Double, y: Double }

public struct MarkSegmentDrawing {
    /// The blade's polygon in the brand's 512 box.
    public var points: [MarkPoint]
    public var opacity: Double
    /// Larger is nearer the viewer; draw the smallest first.
    public var depth: Double
}

public struct MarkFaceDrawing {
    public var points: [MarkPoint]
    public var opacity: Double
}

public struct MarkDrawing {
    public var segments: [MarkSegmentDrawing]
    /// The four faces of the tetrahedron, the translucent solid behind the blades (opacity 0 unless all three blades are at the solid).
    public var faces: [MarkFaceDrawing]
}

public enum TetraMark {
    public static let eye = 8.0
    public static let minDivisor = 4.0
    public static let box = 256.0

    /// How far blade i is toward the real solid: its own beta, the whole's beta, and the tumble's own pull (out-of-plane
    /// turns need the solid; a turn about the viewing axis never does). Zero at a whole number of turns.
    public static func solidWeight(_ p: MarkPose, _ i: Int) -> Double {
        let pull = MarkMath.smooth(abs(MarkMath.wrap(p.tumble)) * cos(p.elevation) / 0.5)
        return MarkMath.clamp01(max(p.beta, p.segBeta[i], pull))
    }

    /// The 3x3 turn of the whole, row major: Rz(roll) * Rodrigues(axis, tumble).
    public static func matrix(_ p: MarkPose) -> [Double] {
        let nx = cos(p.elevation) * cos(p.azimuth), ny = cos(p.elevation) * sin(p.azimuth), nz = sin(p.elevation)
        let c = cos(p.tumble), s = sin(p.tumble), t = 1 - c
        let r0 = [t * nx * nx + c, t * nx * ny - s * nz, t * nx * nz + s * ny]
        let r1 = [t * nx * ny + s * nz, t * ny * ny + c, t * ny * nz - s * nx]
        let r2 = [t * nx * nz - s * ny, t * ny * nz + s * nx, t * nz * nz + c]
        let cr = cos(p.roll), sr = sin(p.roll)
        let m0 = (0..<3).map { cr * r0[$0] - sr * r1[$0] }
        let m1 = (0..<3).map { sr * r0[$0] + cr * r1[$0] }
        return m0 + m1 + r2
    }

    static func apply(_ m: [Double], _ v: (Double, Double, Double)) -> (Double, Double, Double) {
        (m[0] * v.0 + m[1] * v.1 + m[2] * v.2, m[3] * v.0 + m[4] * v.1 + m[5] * v.2, m[6] * v.0 + m[7] * v.1 + m[8] * v.2)
    }

    static func project(_ m: [Double], _ v: (Double, Double, Double), _ p: MarkPose) -> MarkPoint {
        let r = apply(m, v)
        let f = eye / max(eye - r.2, minDivisor)
        return MarkPoint(x: box + box * (p.scale * r.0 * f + p.dx), y: box + box * (p.scale * r.1 * f + p.dy))
    }

    /// The pose on the page: three blades and the solid behind them.
    public static func draw(_ p: MarkPose) -> MarkDrawing {
        let m = matrix(p)
        var segs: [MarkSegmentDrawing] = []
        var wmin = 1.0, spreadMax = 0.0
        for i in 0..<3 {
            let w = solidWeight(p, i)
            wmin = min(wmin, w); spreadMax = max(spreadMax, abs(p.spread[i]))
            let ax = TetraMarkData.ax[i], ay = TetraMarkData.ay[i]
            let bx = TetraMarkData.bx[i], by = TetraMarkData.by[i], bz = TetraMarkData.bz[i]
            let ux = TetraMarkData.ru[i][0], uy = TetraMarkData.ru[i][1]
            let cx = ax.reduce(0, +) / Double(ax.count), cy = ay.reduce(0, +) / Double(ay.count)
            let ct = cos(p.phase[i]), st = sin(p.phase[i])
            var pts: [MarkPoint] = []
            for j in 0..<ax.count {
                let qx = MarkMath.lerp(ax[j], bx[j], w), qy = MarkMath.lerp(ay[j], by[j], w), qz = w * bz[j]
                let k = ((qx - cx) * -uy + (qy - cy) * ux) * (p.length[i] - 1)   // stretch along the tangent
                let x1 = qx + k * -uy, y1 = qy + k * ux
                let s = p.segScale[i], rho = p.spread[i]
                let x2 = s * (x1 * ct - y1 * st) + rho * ux, y2 = s * (x1 * st + y1 * ct) + rho * uy, z2 = s * qz
                pts.append(project(m, (x2, y2, z2), p))
            }
            let cz = apply(m, (cx, cy, w * bzMean(i))).2
            segs.append(MarkSegmentDrawing(points: pts, opacity: p.alpha * bladeOpacity(m, i, w), depth: cz))
        }
        let fb = wmin * wmin * wmin * max(0, 1 - 5 * spreadMax)
        var faces: [MarkFaceDrawing] = []
        for f in 0..<4 {
            let idx = TetraMarkData.faces[f]
            let pts = idx.map { project(m, (TetraMarkData.vx[$0], TetraMarkData.vy[$0], TetraMarkData.vz[$0]), p) }
            let n = apply(m, (TetraMarkData.outward[f][0], TetraMarkData.outward[f][1], TetraMarkData.outward[f][2]))
            let lit = litness(n)
            faces.append(MarkFaceDrawing(points: pts, opacity: p.alpha * fb * (n.2 >= 0 ? 0.2 + 0.3 * lit : 0.1)))
        }
        return MarkDrawing(segments: segs, faces: faces)
    }

    static func bzMean(_ i: Int) -> Double { TetraMarkData.bz[i].reduce(0, +) / Double(TetraMarkData.bz[i].count) }

    /// 1 is bright, 0 dark: the turned normal against a light at the upper left (the old mark's).
    static func litness(_ n: (Double, Double, Double)) -> Double {
        0.5 + 0.5 * (n.2 >= 0 ? 1 : -1) * (n.0 * -0.42 + n.1 * -0.52 + n.2 * 0.74)
    }

    static func bladeOpacity(_ m: [Double], _ i: Int, _ w: Double) -> Double {
        let o = TetraMarkData.outward[i]
        let n = apply(m, (o[0], o[1], o[2]))
        return 1 - w * 0.45 * (1 - litness(n))
    }
}

// MARK: the named motions

public enum MarkMotion: String, CaseIterable {
    case idle, pressed, trayOpen, trayClose, working, playing, listening, tumbling, settle

    /// Seconds the animator takes to blend from the pose on screen to this motion's target.
    public var blendIn: Double {
        switch self {
        case .idle: return 0.6
        case .pressed: return 0.15
        case .trayOpen, .trayClose: return 0.2
        case .working: return 0.5
        case .playing: return 0.7
        case .listening: return 0.6
        case .tumbling: return 0.6
        case .settle: return 0.9
        }
    }
    /// The tray springs past its target; the rest ease (a settle from a spin must not add speed).
    var overshoots: Bool { self == .trayOpen || self == .trayClose }

    /// The motion's pose `ee` seconds after it began. `level` is 0...1 (the voice's loudness, or the mic's); 0 means a
    /// synthesized breath for playing and listening. A loop is periodic in ee, a one-shot ends at its resting target.
    public func target(_ ee: Double, level: Double = 0) -> MarkPose {
        let tau = MarkMath.tau
        var p = MarkPose.rest
        switch self {
        case .idle:
            let u = MarkMath.bump(ee / 6.5)
            p.scale = 1 + 0.025 * u; p.beta = 0.05 * u
            p.spread = [0.012 * u, 0.012 * u, 0.012 * u]
        case .pressed:
            let down = ee < 0.10 ? MarkMath.smooth(ee / 0.10) : 1 - MarkMath.back((ee - 0.10) / 0.32)
            p.scale = 1 - 0.16 * down
            p.spread = [-0.05 * down, -0.05 * down, -0.05 * down]
        case .trayOpen, .trayClose:
            let k = MarkMath.back(ee / 0.55)
            p.roll = (tau / 3) * (self == .trayOpen ? k : 1 - k)
            let bloom = 0.06 * sin(Double.pi * MarkMath.clamp01(ee / 0.55))
            p.spread = [bloom, bloom, bloom]
        case .working:
            let u = ee / 1.8
            p.roll = tau * ee / 5.4
            for i in 0..<3 {
                let q = u - Double(i) / 3
                p.phase[i] = 0.35 * sin(tau * q)
                p.spread[i] = 0.10 * MarkMath.bump(q)
                p.segScale[i] = 1 - 0.12 * MarkMath.bump(q)
                p.length[i] = 1 + 0.08 * MarkMath.bump(q)
            }
        case .playing:
            let amp = max(level, 0.5 * (0.5 + 0.5 * sin(tau * ee / 2.6)))
            p.scale = 1 + 0.05 * amp; p.beta = 0.12 * amp
            p.azimuth = Double.pi / 2; p.tumble = 0.3 * amp * sin(tau * ee / 3.2)
            for i in 0..<3 {
                p.spread[i] = 0.02 + 0.05 * amp
                p.phase[i] = 0.08 * amp * sin(tau * ee / 1.6 + 2.1 * Double(i))
            }
        case .listening:
            let amp = max(level, 0.4 * (0.5 + 0.5 * sin(tau * ee / 2.6)))
            p.scale = 1 + 0.05 * amp
            p.azimuth = Double.pi / 2; p.tumble = 0.35 * amp * sin(tau * ee / 2.2)
            for i in 0..<3 {
                let w = sin(tau * ee / 1.6 + 2.1 * Double(i))
                p.segBeta[i] = 0.2 * amp
                p.spread[i] = (0.04 + 0.14 * amp) * (0.6 + 0.4 * w)
                p.phase[i] = 0.1 * amp * w
            }
        case .tumbling:
            p.tumble = tau * ee / 4.0; p.azimuth = tau * ee / 11; p.elevation = 0.5
            p.beta = 1; p.scale = 0.86
        case .settle:
            break
        }
        return p
    }

    /// The pose shown when Reduce Motion is on: still, one picture per motion.
    public var stillPose: MarkPose {
        var p = MarkPose.rest
        if self == .trayOpen { p.roll = MarkMath.tau / 3 }
        return p
    }

    /// Whether the motion changes with time once its blend is over (the mark must be given frames), and how often.
    public var frames: MarkFrames {
        switch self {
        case .idle: return .slow
        case .working, .playing, .listening, .tumbling: return .full
        case .pressed, .trayOpen, .trayClose, .settle: return .oneShot
        }
    }
}

public enum MarkFrames: Equatable {
    /// No frames needed once a one-shot is over (see `MarkAnimator.frames(at:)`).
    case oneShot
    /// About five a second is enough for a slow breath.
    case slow
    case full
    /// No frames at all.
    case none
}

// MARK: the animator

/// One mark's current motion. A value type: the caller keeps it, calls `begin` when the state changes and `pose(at:)` each frame.
public struct MarkAnimator {
    public private(set) var motion: MarkMotion = .idle
    public private(set) var start = 0.0
    public private(set) var from = MarkPose.rest
    public private(set) var previous: MarkMotion = .idle
    public var reduceMotion: Bool
    /// The input level the pose last used (so a motion captured mid-frame blends from what was on screen).
    private var lastLevel = 0.0

    /// Seconds a Reduce Motion cross-fade takes.
    public static let fadeSeconds = 0.30
    /// Seconds a one-shot is live for (after it the mark holds its end pose and needs no frames).
    public static let oneShotSeconds = 0.9

    public init(reduceMotion: Bool = false) { self.reduceMotion = reduceMotion }

    /// Start a motion at time `t` (seconds on any steady clock). Idempotent when the motion is already running (except one-shots, which restart).
    public mutating func begin(_ m: MarkMotion, at t: Double, level: Double = 0) {
        if m == motion, m.frames != .oneShot { return }
        from = onScreen(at: t, level: level).wrapped()
        previous = motion
        motion = m; start = t; lastLevel = level
    }

    func onScreen(at t: Double, level: Double) -> MarkPose {
        let ee = max(0, t - start)
        let tg = motion.target(ee, level: level)
        let u = MarkMath.clamp01(ee / motion.blendIn)
        let r = motion.overshoots ? MarkMath.back(u) : MarkMath.smooth(u)
        return MarkPose.lerp(from, tg, r)
    }

    /// The pose to draw at time `t`. With Reduce Motion on this is a still pose and a cross-fade of `alpha` when the motion changed.
    public func pose(at t: Double, level: Double = 0) -> MarkPose {
        if !reduceMotion { return onScreen(at: t, level: level) }
        let k = MarkMath.clamp01((t - start) / MarkAnimator.fadeSeconds)
        var p = (k < 0.5 ? previous : motion).stillPose
        p.alpha = 0.35 + 0.65 * abs(1 - 2 * k)
        return p
    }

    /// What the host should do about frames at time `t`: none under Reduce Motion once the fade is over, none for a one-shot that has
    /// ended, a slow tick for the idle breath, a frame task otherwise. A mark that is off screen or an app in the background needs none
    /// whatever this says: the host stops the task, and the next `pose(at:)` is a closed form of the clock, so nothing is lost.
    public func frames(at t: Double) -> MarkFrames {
        if reduceMotion { return t - start < MarkAnimator.fadeSeconds ? .full : .none }
        if motion.frames == .oneShot { return t - start < MarkAnimator.oneShotSeconds ? .full : .none }
        return motion.frames
    }
}
