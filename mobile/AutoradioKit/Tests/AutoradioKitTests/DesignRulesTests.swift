import XCTest
@testable import AutoradioKit

// The design rule (owner, 2026-11-15): "i want no pills. no pills. no outlined bubbles. pills promote bad design, bad typography." Controls read
// as controls through type, position and one affordance, not a container. The rule is written at the top of ui.contract; these tests keep it.
// Failure list, written before the code:
//  R1 a theme's radii come through as a capsule (999, or a card of 25 pt): the Kit clamps cards to 8...12, blocks to 4...6, whatever a theme asks.
//  R2 a contract draws a capsule: `th.radiusPill`, or a border-radius of 999 or more than 12, outside ui.contract (every contract is scanned,
//     including ones other lanes add later).
//  R3 a contract draws an outline around something (`border-style="solid"`, or a `border-width` with a colour) outside ui.contract. A hairline
//     above a row is a rule and is allowed.
//  R5 a circle or a half-height rounding: a box whose border-radius is half its height or more (a dot, a thumb, a round button, a progress bar
//     with round ends) outside ui.contract. A marker is a short rule; a progress bar is a flat line.
//  R6 a radio drawn as a glyph (● or ○): a single choice shows its chosen row with a check, never a circle.
//  R4 a contract reads the retired capsule helpers' look back in: a background fill and a pill radius together on one line.
final class DesignRulesTests: XCTestCase {
    static let lab = NavigationTests.lab

    private func contracts() throws -> [(name: String, lines: [String])] {
        try FileManager.default.contentsOfDirectory(atPath: Self.lab.path).filter { $0.hasSuffix(".contract") }.sorted().map {
            ($0, try String(contentsOf: Self.lab.appendingPathComponent($0), encoding: .utf8).split(separator: "\n", omittingEmptySubsequences: false).map(String.init))
        }
    }
    private func code(_ line: String) -> String {                  // a line without its comment
        guard let r = line.range(of: "//") else { return line }
        let before = line[..<r.lowerBound]
        return before.filter { $0 == "\"" }.count % 2 == 0 ? String(before) : line      // a // inside a string (a URL) is not a comment
    }

    /// The numbers written as `border-radius=14`, `border-top-left-radius=20` on a line (an expression like `(th.radiusCard * 1.7)` is not a number).
    static func radii(in l: String) -> [Int] {
        let re = try! NSRegularExpression(pattern: "border(?:-[a-z]+)*-radius=([0-9]+)(?![0-9.])")
        return re.matches(in: l, range: NSRange(l.startIndex..., in: l)).compactMap { m in Range(m.range(at: 1), in: l).flatMap { Int(l[$0]) } }
    }

    // R1
    func testThemesAreClampedToTheRadiusRule() {
        let d = UserDefaults(suiteName: "design-\(UUID().uuidString)")!
        for id in ["chrome-pastel", "depth", "lantern", "liquid-metal", "newsprint", "signal"] {
            let s = ThemeStore(userDir: nil, defaults: d, systemScheme: { "light" }); s.setTheme(id)
            let t = s.current()
            XCTAssertTrue((8...12).contains(t.radiusCard), "\(id) card \(t.radiusCard)")
            XCTAssertTrue((4...6).contains(t.radiusControl), "\(id) control \(t.radiusControl)")
            XCTAssertLessThanOrEqual(t.radiusPill, 6, "\(id) has no pill")
        }
        XCTAssertLessThanOrEqual(Theme().radiusPill, 6); XCTAssertLessThanOrEqual(Theme().radiusCard, 12)
    }

    // R2
    func testNoContractDrawsACapsule() throws {
        var seen = 0
        for f in try contracts() where f.name != "ui.contract" {
            for (n, raw) in f.lines.enumerated() {
                let l = code(raw); seen += 1
                XCTAssertFalse(l.contains("radiusPill"), "\(f.name):\(n + 1) uses th.radiusPill")
                for v in Self.radii(in: l) {
                    XCTAssertLessThanOrEqual(v, 12, "\(f.name):\(n + 1) a radius above 12 (a capsule or a bubble)")
                }
            }
        }
        XCTAssertGreaterThan(seen, 500)
    }

    // R3: outlines. A hairline above a row (`border-top-width=1` with the other sides 0) is a rule, not an outline; nothing else is allowed.
    static func isRowRule(_ l: String) -> Bool { l.contains("border-top-width=1 border-right-width=0 border-bottom-width=0") }
    func testNoContractOutlinesAnything() throws {
        for f in try contracts() where f.name != "ui.contract" {
            for (n, raw) in f.lines.enumerated() {
                let l = code(raw)
                guard l.contains("border-style=\"solid\"") || l.contains("border-width=") && l.contains("border-color=") else { continue }
                if Self.isRowRule(l) { continue }
                XCTFail("\(f.name):\(n + 1) draws an outline: \(l.trimmingCharacters(in: .whitespaces).prefix(110))")
            }
        }
    }

    // R5, R6
    func testNoCirclesOrRoundEndedBars() throws {
        for f in try contracts() where f.name != "ui.contract" {
            for (n, raw) in f.lines.enumerated() {
                let l = code(raw)
                XCTAssertFalse(l.contains("●") || l.contains("○"), "\(f.name):\(n + 1) a radio glyph")
                let radii = Self.radii(in: l)
                guard let r = radii.first, r > 0, let h = Self.number(after: " height=", in: l) else { continue }
                XCTAssertLessThan(Double(r) * 2, h, "\(f.name):\(n + 1) a circle or a round-ended bar (radius \(r), height \(Int(h)))")
            }
        }
    }
    /// One element is its own line plus the attribute-only lines that continue it (a line that starts `name=`), so a radius on one line and the size on
    /// the next are read together.
    static func elements(_ lines: [String], code: (String) -> String) -> [(line: Int, text: String)] {
        var out: [(line: Int, text: String)] = []
        let attr = try! NSRegularExpression(pattern: "^[a-z-]+=")
        for (n, raw) in lines.enumerated() {
            let t = code(raw).trimmingCharacters(in: .whitespaces)
            if t.isEmpty { continue }
            let isAttr = attr.firstMatch(in: t, range: NSRange(t.startIndex..., in: t)) != nil
            if isAttr, !out.isEmpty { out[out.count - 1].text += " " + t } else { out.append((n + 1, t)) }
        }
        return out
    }
    static func sizes(_ key: String, in t: String) -> [Double] {
        let re = try! NSRegularExpression(pattern: "(?:^| )" + key + "=([0-9.]+)(?![0-9.%])")
        return re.matches(in: t, range: NSRange(t.startIndex..., in: t)).compactMap { m in Range(m.range(at: 1), in: t).flatMap { Double(t[$0]) } }
    }
    /// R5 across lines, in every contract including ui.contract: an element whose border-radius is half its width or height, or more, is a capsule or a
    /// circle, wherever on the element the radius and the size are written.
    func testNoElementIsACapsuleAcrossLines() throws {
        var checked = 0, elements = 0
        for f in try contracts() {
            for e in Self.elements(f.lines, code: code) {
                elements += 1
                let radii = Self.radii(in: e.text).filter { $0 > 0 }
                guard let r = radii.max() else { continue }
                let dims = Self.sizes("height", in: e.text) + Self.sizes("width", in: e.text) + Self.sizes("min-height", in: e.text)
                guard let d = dims.min() else { continue }
                checked += 1
                XCTAssertLessThan(Double(r) * 2, d, "\(f.name):\(e.line) radius \(r) on a box \(Int(d)) pt across (half or more of it)")
            }
        }
        XCTAssertGreaterThan(elements, 2000, "the scan reads every element of every contract"); _ = checked
    }

    static func number(after key: String, in l: String) -> Double? {
        guard let r = l.range(of: key) else { return nil }
        let digits = l[r.upperBound...].prefix { $0.isNumber || $0 == "." }
        return Double(digits)
    }

    // R4: the capsule button face is gone from ui.contract itself.
    func testTheSharedFacesDrawNoCapsuleOrOutline() throws {
        let ui = try String(contentsOf: Self.lab.appendingPathComponent("ui.contract"), encoding: .utf8).split(separator: "\n").map { code(String($0)) }.joined(separator: "\n")
        XCTAssertFalse(ui.contains("radiusPill")); XCTAssertFalse(ui.contains("border-style")); XCTAssertFalse(ui.contains("border-width"))
        XCTAssertTrue(ui.contains("component ChoiceWord") && ui.contains("component ChipFace"), "ChipFace stays as a deprecated name")
    }
}
