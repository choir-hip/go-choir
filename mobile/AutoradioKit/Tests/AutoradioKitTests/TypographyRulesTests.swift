import XCTest
@testable import AutoradioKit

// The typographic system (owner, 2026-11-16: "We need to have more typography and use typography to signify what's the difference between static
// text and a button and a headline. Typography is too same same, sans serif."). The roles are in mobile/docs/typography.md; the contracts name a role's
// face, never a face: th.fontHead, th.fontText, th.fontLabel, th.fontAction, th.fontMono. These tests keep it.
// Failure list, written before the code:
//  T1 a contract names a face outside the role set (th.fontRead, th.fontUi, a theme card's t.font*, a literal "ui-serif").
//  T2 a `text` has no role face, on itself or on an ancestor (it would draw in the root face).
//  T3 a `text` is sized in a bare number or a theme number (px: Dynamic Type never reaches it) instead of a system text style or a rem.
//  T4 an Action that is not semibold, or that is uppercase, tracked or italic; a static Label that is not regular (weight 400) unless it is the caps label.
//  T5 a word inside a button that is not an Action (a literal short word such as Play, Queue, Cancel) or an Action that sits outside any control.
//  T6 uppercase or tracking outside the caps Label (a masthead may be tracked); italic outside the serif roles.
//  T7 a figure (elapsed, remaining, speed, a position) set without tabular figures.
//  T8 the Theme record lacks a role token, or a theme's own faces leak into the roles.
final class TypographyRulesTests: XCTestCase {
    static let lab = NavigationTests.lab

    struct El { var file: String; var line: Int; var indent: Int; var text: String }

    static func code(_ line: String) -> String {
        guard let r = line.range(of: "//") else { return line }
        let before = line[..<r.lowerBound]
        return before.filter { $0 == "\"" }.count % 2 == 0 ? String(before) : line
    }
    /// Every contract's elements: a line plus the attribute-only lines that continue it.
    static func allElements() throws -> [[El]] {
        let attr = try! NSRegularExpression(pattern: "^[a-z-]+=")
        var out: [[El]] = []
        for f in try FileManager.default.contentsOfDirectory(atPath: lab.path).filter({ $0.hasSuffix(".contract") }).sorted() {
            let lines = try String(contentsOf: lab.appendingPathComponent(f), encoding: .utf8).split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
            var els: [El] = []
            for (n, raw) in lines.enumerated() {
                let c = code(raw); let t = c.trimmingCharacters(in: .whitespaces)
                if t.isEmpty { continue }
                let indent = c.prefix { $0 == " " }.count
                if attr.firstMatch(in: t, range: NSRange(t.startIndex..., in: t)) != nil, !els.isEmpty { els[els.count - 1].text += " " + t }
                else { els.append(El(file: f, line: n + 1, indent: indent, text: t)) }
            }
            out.append(els)
        }
        return out
    }
    /// The value of `key=` in an element: a string, a template, a balanced expression or a bare word.
    static func attr(_ key: String, in s: String) -> String? {
        guard let m = try! NSRegularExpression(pattern: "(?<![A-Za-z-])" + NSRegularExpression.escapedPattern(for: key) + "=").firstMatch(in: s, range: NSRange(s.startIndex..., in: s)),
              let r = Range(m.range, in: s) else { return nil }
        let chars = Array(s[r.upperBound...])
        var i = 0, depth = 0; var quote: Character? = nil
        if let q = chars.first, q == "\"" || q == "`" { i = 1; while i < chars.count, chars[i] != q { i += 1 }; return String(chars[0..<min(i + 1, chars.count)]) }
        while i < chars.count {
            let c = chars[i]
            if let q = quote { if c == q { quote = nil } }
            else if c == "\"" || c == "`" { quote = c }
            else if c == "(" || c == "[" { depth += 1 }
            else if c == ")" || c == "]" { depth -= 1 }
            else if c == " " && depth == 0 { break }
            i += 1
        }
        return String(chars[0..<i])
    }
    static let roleFaces: Set<String> = ["th.fontHead", "th.fontText", "th.fontLabel", "th.fontAction", "th.fontMono"]
    static func faces(in s: String) -> [String] {
        guard let v = attr("font-family", in: s) else { return [] }
        let re = try! NSRegularExpression(pattern: "th\\.font[A-Za-z]+|t\\.font[A-Za-z]+|\"[^\"]*\"")
        return re.matches(in: v, range: NSRange(v.startIndex..., in: v)).compactMap { Range($0.range, in: v).map { String(v[$0]) } }.filter { !$0.hasPrefix("\"") || ($0.count > 3 && !$0.contains(" ") && $0.contains("-")) }
    }
    static func ancestors(_ els: [El], _ k: Int) -> [El] {
        var cur = els[k].indent; var out: [El] = []
        var j = k - 1
        while j >= 0 { if els[j].indent < cur { cur = els[j].indent; out.append(els[j]) }; j -= 1 }
        return out
    }
    static func isText(_ e: El) -> Bool { e.text.hasPrefix("text ") }
    static func literal(_ e: El) -> String? {
        let re = try! NSRegularExpression(pattern: "^text\\s+\"([^\"]*)\"")
        guard let m = re.firstMatch(in: e.text, range: NSRange(e.text.startIndex..., in: e.text)), let r = Range(m.range(at: 1), in: e.text) else { return nil }
        return String(e.text[r])
    }
    /// A caps label: uppercase through text-transform, or a string that is already upper case.
    static func isCaps(_ e: El) -> Bool {
        if attr("text-transform", in: e.text) == "\"uppercase\"" { return true }
        let re = try! NSRegularExpression(pattern: "\"[A-Z][A-Z .·&']{3,}\"")
        return re.firstMatch(in: e.text, range: NSRange(e.text.startIndex..., in: e.text)) != nil
    }

    // T1
    func testEveryFaceIsARoleFace() throws {
        var seen = 0
        for els in try Self.allElements() { for e in els where e.text.contains("font-family=") {
            seen += 1
            let fs = Self.faces(in: e.text)
            XCTAssertFalse(fs.isEmpty, "\(e.file):\(e.line) font-family names no role face")
            for f in fs { XCTAssertTrue(Self.roleFaces.contains(f), "\(e.file):\(e.line) face \(f) is not a role face (th.fontHead, fontText, fontLabel, fontAction, fontMono)") }
        } }
        XCTAssertGreaterThan(seen, 300)
    }

    // T2
    func testEveryTextHasARoleFace() throws {
        var texts = 0
        for els in try Self.allElements() { for (k, e) in els.enumerated() where Self.isText(e) {
            texts += 1
            if e.text.contains("font-family=") { continue }
            let up = Self.ancestors(els, k).contains { $0.text.contains("font-family=") }
            XCTAssertTrue(up, "\(e.file):\(e.line) a text with no role face on it or above it: \(e.text.prefix(80))")
        } }
        XCTAssertGreaterThan(texts, 400)
    }

    // T3
    func testTextIsSizedByStyleOrRemNeverBarePoints() throws {
        for els in try Self.allElements() { for e in els where Self.isText(e) {
            guard let v = Self.attr("font-size", in: e.text) else { continue }
            let ok = v.hasPrefix("\"-exact-") || v.hasPrefix("dyn(") || v.hasPrefix("remv(") || v.hasPrefix("(") && v.contains("-exact-")
            XCTAssertTrue(ok, "\(e.file):\(e.line) font-size=\(v) is a fixed size (use a text style or dyn())")
        } }
    }

    // T4, T6
    func testActionLabelAndSpecialTreatments() throws {
        var actions = 0
        for els in try Self.allElements() { for e in els where Self.isText(e) {
            let fs = Self.faces(in: e.text); let w = Self.attr("font-weight", in: e.text)
            let caps = Self.isCaps(e); let tracked = e.text.contains("letter-spacing=")
            let italic = e.text.contains("font-style=")
            let mixed = fs.count > 1      // a ternary of two roles: the breadcrumb and the status line are the only ones (below)
            if mixed { XCTAssertTrue(["files.contract", "books.contract", "settings.contract"].contains(e.file), "\(e.file):\(e.line) a text that is two roles"); continue }
            guard let f = fs.first else { continue }
            switch f {
            case "th.fontAction":
                actions += 1
                XCTAssertTrue(w == "600", "\(e.file):\(e.line) an Action is semibold (600), not \(w ?? "regular")")
                XCTAssertFalse(caps || tracked || italic, "\(e.file):\(e.line) an Action is never uppercase, tracked or italic")
            case "th.fontLabel":
                if caps { XCTAssertEqual(w, "600", "\(e.file):\(e.line) the caps label is 600"); XCTAssertTrue(tracked, "\(e.file):\(e.line) the caps label is tracked") }
                else { XCTAssertTrue(w == nil || w == "400", "\(e.file):\(e.line) a static Label is regular, not \(w ?? "")"); XCTAssertFalse(tracked, "\(e.file):\(e.line) only the caps label is tracked") }
                XCTAssertFalse(italic, "\(e.file):\(e.line) italic is a serif treatment")
            case "th.fontHead":
                XCTAssertTrue(Self.attr("text-transform", in: e.text) == nil, "\(e.file):\(e.line) a headline is never transformed to capitals (the masthead is written in capitals)")
            case "th.fontText":
                XCTAssertFalse(caps || tracked, "\(e.file):\(e.line) reading text is never uppercase or tracked")
            default: break
            }
        } }
        XCTAssertGreaterThan(actions, 40)
    }

    // T5: a word in a button is an Action; an Action is in a control or on a control's face.
    static let faceComponents: Set<String> = ["Btn", "ChoiceWord", "SegmentFace", "ChipFace", "ChoiceChip", "Chip", "LineRow", "ListRow", "Runs", "ActionWord", "SettingsBtn", "Choice", "BarGlyph", "PodAction", "ActionButton"]
    func testActionsAreControlsAndControlsAreActions() throws {
        for els in try Self.allElements() { for (k, e) in els.enumerated() where Self.isText(e) {
            let anc = Self.ancestors(els, k)
            let inButton = anc.contains { $0.text.hasPrefix("button") }
            let isAction = Self.faces(in: e.text) == ["th.fontAction"]
            if isAction {
                let pressable = e.text.contains("press=")
                let comp = anc.first { $0.text.hasPrefix("component ") }.map { String($0.text.dropFirst(10)).split(separator: " ").first.map(String.init) ?? "" } ?? ""
                let face = Self.facesAllowed(e.file, comp)
                XCTAssertTrue(inButton || pressable || face, "\(e.file):\(e.line) an Action outside a control (component \(comp)): static text is never an Action")
            } else if inButton, let lit = Self.literal(e), lit.count <= 28, lit.contains(where: { $0.isLetter }), !Self.isCaps(e), !e.text.contains("role=\"heading\"") {
                let f = Self.faces(in: e.text).first ?? ""
                if f != "th.fontLabel" && f != "th.fontHead" { continue }
                XCTAssertTrue(Self.titleWords.contains(lit) || f == "th.fontLabel" && !Self.verbs.contains(lit), "\(e.file):\(e.line) the word \"\(lit)\" is in a button but is not an Action")
            }
        } }
    }
    /// Components that are the face of a control the caller wraps in a button (their words are Actions).
    static func facesAllowed(_ file: String, _ comp: String) -> Bool { faceComponents.contains(comp) }
    /// Words inside a button that name a thing rather than do something.
    static let titleWords: Set<String> = []
    static let verbs: Set<String> = ["Play", "Queue", "Cancel", "Close", "Sign in", "Not now", "Try again", "Delete", "Share", "Reload", "Skip for now", "Edit my edition", "Open episode", "Sections", "Source"]

    // T9 (owner, 2026-11-16): type tells a button from a headline. Pure navigation (a word with no content behind it) is an Action; a tappable title of an
    // object stays a Headline, and its button carries an action affordance: a chevron, or an Action word beside it. The serif alone never signals a tap.
    func testATappableHeadlineCarriesAnActionAffordance() throws {
        var headlineButtons = 0
        for els in try Self.allElements() { for (k, b) in els.enumerated() where b.text.hasPrefix("button ") {
            var inner: [El] = []; var j = k + 1
            while j < els.count, els[j].indent > b.indent { inner.append(els[j]); j += 1 }
            let heads = inner.filter { Self.isText($0) && Self.faces(in: $0.text).contains("th.fontHead") && !$0.text.contains("role=\"heading\"") }
            if heads.isEmpty { continue }
            headlineButtons += 1
            let affordance = inner.contains { $0.text.contains("forward-chevron") || $0.text.contains("chevron=true") || $0.text.hasPrefix("text \"›\"") || $0.text.hasPrefix("SelectMark(")
                || (Self.isText($0) && Self.faces(in: $0.text) == ["th.fontAction"]) }
            // Where the serif is the whole content and a chevron would be noise, named with the reason: the paper's story headlines (the front page is the
            // list; Play and Queue are Actions beside them), a block of a book (reading text; a tap places the voice), a file tile (its icon and card),
            // a topic row and the two demo toggles (their line under the title says "Tap to ...").
            let exempt = ["paper-story-", "doc-block-", "files-row-", "ob-topic-", "settings-dev-details"].contains { b.text.contains("testId=`\($0)") || b.text.contains("testId=\"\($0)") }
            let objectCard = inner.contains { $0.text.hasPrefix("Card(") || $0.text.hasPrefix("CardFill(") || $0.text.hasPrefix("PaperCard(") }
            XCTAssertTrue(affordance || objectCard || exempt, "\(b.file):\(b.line) a button whose title is a Headline has no chevron or Action word: the serif alone does not say it is tappable")
        } }
        XCTAssertGreaterThan(headlineButtons, 3)
    }
    func testKeyPagesAreGlyphsWithLabelsAndThePlaceholderIsALabel() throws {
        var chip = 0, labels = 0, field = false
        for els in try Self.allElements() { for (k, e) in els.enumerated() {
            if e.file == "launcher.contract", Self.ancestors(els, k).contains(where: { $0.text.hasPrefix("component KeyGlyph") }) {
                if Self.isText(e) {
                    labels += 1
                    XCTAssertTrue(e.text.hasPrefix("text name "), "the caption is the page's own name: \(e.text)")
                    XCTAssertTrue(e.text.contains("aria-hidden=true"), "the caption is hidden from VoiceOver: the button's label already says the name")
                    XCTAssertFalse(e.text.contains("line-clamp"), "a tray name is never truncated")
                }
                if e.text.hasPrefix("button") { chip += 1; XCTAssertTrue(e.text.contains("aria-label=`Open ${name}`"), "each glyph names its page for accessibility") }
            }
            if e.text.contains("testId=\"bar-field\"") {
                field = true
                let fam = Self.attr("font-family", in: e.text) ?? ""
                XCTAssertTrue(fam.contains("th.fontLabel") && fam.contains("th.fontText"), "the ask field shows its placeholder as a Label and typed words as Reading text: \(fam)")
            }
            XCTAssertFalse(e.text.contains("content-close-word"), "the content row has the X and no Close word")
        } }
        XCTAssertEqual(chip, 1); XCTAssertEqual(labels, 1, "each tray glyph has one text label under it"); XCTAssertTrue(field)
    }

    // T7
    func testFiguresAreTabular() throws {
        let ids = ["queue-elapsed", "queue-remaining", "radio-speed", "radio-elapsed", "radio-remaining", "radio-onair-sentence-number", "doc-position"]
        var found = 0
        for els in try Self.allElements() { for e in els where Self.isText(e) {
            for id in ids where e.text.contains("\"\(id)\"") {
                found += 1
                XCTAssertTrue(e.text.contains("font-variant-numeric=\"tabular-nums\""), "\(e.file):\(e.line) \(id) is a figure and takes tabular figures")
            }
        } }
        XCTAssertGreaterThanOrEqual(found, 4)       // the audio, e-book and PDF figures went with the Files players and viewers
    }

    // T8
    func testTheThemeCarriesTheRoleFaces() {
        let t = Theme()
        XCTAssertEqual(t.fontHead, "ui-serif"); XCTAssertEqual(t.fontText, "ui-serif")
        XCTAssertEqual(t.fontLabel, "system-ui"); XCTAssertEqual(t.fontAction, "system-ui")
        XCTAssertEqual(t.fontMono, "ui-monospace")
        // a theme that asks for a sans body or a serif interface does not move the roles: the roles are the system.
        let d = UserDefaults(suiteName: "type-\(UUID().uuidString)")!
        for id in ["chrome-pastel", "depth", "lantern", "liquid-metal", "newsprint", "signal"] {
            let s = ThemeStore(userDir: nil, defaults: d, systemScheme: { "light" }); s.setTheme(id)
            let c = s.current()
            XCTAssertEqual(c.fontHead, "ui-serif", id); XCTAssertEqual(c.fontText, "ui-serif", id)
            XCTAssertTrue(["system-ui", "ui-rounded"].contains(c.fontLabel), "\(id) label \(c.fontLabel)")
            XCTAssertEqual(c.fontLabel, c.fontAction, id)
        }
    }

    // the role table is in the doc and names every token
    func testTheDocNamesEveryRole() throws {
        let doc = Self.lab.deletingLastPathComponent().appendingPathComponent("docs/typography.md")
        let s = try String(contentsOf: doc, encoding: .utf8)
        for t in Self.roleFaces.sorted() { XCTAssertTrue(s.contains(t), "typography.md names \(t)") }
        for r in ["Headline", "Reading text", "Spoken line", "Label", "Action", "Figure"] { XCTAssertTrue(s.contains(r), "typography.md has the role \(r)") }
    }
}
