import XCTest
@testable import AutoradioKit

// Owner, 2026-11-16: "for reader and for the embedded web view, the content should get the full screen except for the prompt bar area at the
// bottom. The screen real estate at the top should not be used that way. We could have some controls just above the prompt bar, which can expand."
// The layout lives in Exact contracts, so these tests read the contract text (as DesignRulesTests does). Failure list, written before the code:
//  C1 a document (PDF pages, the text view of any document) still has a title block, tab row, action row, find field or stepper above its content.
//  C2 the PDF frame does not start at the top safe area, or does not stop at the lens (it keeps the old fixed 80 point margin, or an outer gutter).
//  C3 an embedded web page (a source over a story, a page typed into the bar) still has a header row or an action row, or stops at the old 72 points.
//  C4 a control the old rows had is lost: Read aloud, Add to queue, Contents, Find (field, hits, clear), Pages and Text, page stepping, Open in Safari, Close.
//  C5 the row does not collapse: an action leaves it open, there is no tap-outside scrim, or stepping (which must stay open) collapses it.
//  C6 closing is not one obvious step: the collapsed line lacks a Close for a document, a source page or a typed page; closing a document leaves the deck on it.
//  C7 the row is not in the lens above the "On:" line, or the bar's own lines (now playing, the field, the mic, play) changed.
//  C8 a name a component injects is not provided by the root (the contract would fail to build on a device only).
//  C9 the clearance is smaller than the lens plus the row (the page's last line or the PDF's bottom edge would sit under the glass).
final class ContentFullScreenTests: XCTestCase {
    static let lab = NavigationTests.lab
    func read(_ n: String) throws -> String { try String(contentsOf: Self.lab.appendingPathComponent(n), encoding: .utf8) }
    /// The text of one top-level component (from its `component Name` line to the next top-level line).
    func component(_ name: String, in file: String) throws -> String {
        let lines = try read(file).split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        guard let i = lines.firstIndex(where: { $0.hasPrefix("component \(name)") }) else { XCTFail("no component \(name) in \(file)"); return "" }
        var out: [String] = []
        for l in lines[i...].dropFirst() { if let f = l.first, f != " " && f != "\t" && !l.isEmpty { break }; out.append(l) }
        return out.joined(separator: "\n")
    }
    func code(_ s: String) -> String { s.split(separator: "\n", omittingEmptySubsequences: false).map { l -> String in
        guard let r = l.range(of: "//") else { return String(l) }
        return l[..<r.lowerBound].filter { $0 == "\"" }.count % 2 == 0 ? String(l[..<r.lowerBound]) : String(l)
    }.joined(separator: "\n") }
    func has(_ s: String, _ needles: [String], _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        for n in needles { XCTAssertTrue(s.contains(n), "\(what): missing `\(n)`", file: file, line: line) }
    }
    func lacks(_ s: String, _ needles: [String], _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        for n in needles { XCTAssertFalse(s.contains(n), "\(what): still has `\(n)`", file: file, line: line) }
    }

    // C1
    func testDocumentHasNothingAboveItsContent() throws {
        let page = code(try component("DocPage", in: "imported-docs.contract"))
        // the two ready branches: pages and text
        lacks(page, ["PageHeader(title=ldInfo.title", "SegmentedTrack", "DocActions", "DocFind", "doc-find", "doc-mode", "doc-position", "doc-toc",
                     "testId=\"doc-prev\"", "testId=\"doc-next\"", "pages with text", "ldToc", "ldFind"], "DocPage")
        let books = code(try read("imported-docs.contract"))
        lacks(books, ["component DocActions", "component DocFind"], "imported-docs.contract")
        // the text view starts with the chapter label, inside the scroll, and nothing else precedes the blocks
        let text = page.components(separatedBy: "PageScroll(id=\"doc\")").last ?? ""
        let firstLines = text.split(separator: "\n").prefix(2).map { $0.trimmingCharacters(in: .whitespaces) }
        XCTAssertEqual(firstLines.first, "SectionHeader(label=ldChap.title)")
    }

    // C2
    func testPdfFrameRunsFromTheStatusBarToTheLens() throws {
        let page = code(try component("DocPage", in: "imported-docs.contract"))
        has(page, ["testId=\"doc-pages\"", "padding-top=\"env(safe-area-inset-top)\"", "ctlClear: number", "${ctlClear}px", "testId=\"doc-frame\""], "PDF frame")
        lacks(page, ["padding-left=20", "padding-right=20", "+ 12px", "56px + 8px", "+ 150px"], "PDF frame")
        let frameOuter = page.components(separatedBy: "testId=\"doc-pages\"").last?.split(separator: "\n").first.map(String.init) ?? ""
        XCTAssertFalse(frameOuter.contains("gap="), "no gap row above the frame")
    }

    // C3
    func testWebPagesHaveNoHeaderAndStopAtTheLens() throws {
        for (name, file, view, id) in [("WebLayer", "reader.contract", "reader-web-view", "reader-web"), ("BarWebLayer", "queue.contract", "bar-web-view", "bar-web")] {
            let c = code(try component(name, in: file))
            has(c, ["testId=\"\(id)\"", "top=0", "padding-top=\"env(safe-area-inset-top)\"", "${ctlClear}px", "web-page page=", "testId=\"\(view)\""], name)
            lacks(c, ["Open in Safari", "Read aloud", "Add to queue", "-close", "-host", "+ 72px", "symbol:close", "<row", "\n      row "], name)
        }
    }

    // C4
    func testEveryFormerControlIsInTheRow() throws {
        let row = code(try read("content.contract"))
        has(row, ["label=\"Read aloud\"", "label=\"Add to queue\"", "label=\"Contents\"", "label=\"Find\"", "label=\"Pages\"", "label=\"Text\"",
                  "label=\"Open in Safari\"", "‹ Previous", "Next ›"], "words")
        has(row, ["testId=\"doc-read\"", "testId=\"doc-queue\"", "testId=\"doc-toc\"", "testId=\"doc-find\"", "testId=\"doc-find-clear\"", "testId=\"doc-find-none\"",
                  "testId=\"doc-mode-pages\"", "testId=\"doc-mode-text\"", "testId=\"doc-prev\"", "testId=\"doc-next\"", "testId=\"doc-position\"", "doc-chapter-${s.index}", "doc-hit-${h.block}",
                  "${pre}-read", "${pre}-add", "${pre}-safari", "${pre}-host", "bar-web-close", "reader-web-close", "testId=\"doc-close\""], "test ids")
        // each is wired to the action the old control used
        has(row, ["press=ctlRead ", "press=ctlQueueDoc ", "press=ctlPanelOf(\"contents\")", "press=ctlPanelOf(\"find\")", "press=ctlSetText(false)", "press=ctlSetText(true)",
                  "press=ctlStep(ldSec - 1)", "press=ctlStep(ldSec + 1)", "press=ctlGo(s.index)", "press=ctlJump(h.section, h.block)", "submit=ldDoFind", "press=ldClearFind",
                  "press=ctlWebRead(ctlKey, ctlTitle)", "press=ctlWebLater(ctlKey, ctlTitle)", "press=ctlSafari(ctlKey)"], "wiring")
        let m = code(try read("members-content.contract"))
        has(m, ["importedReadAloud(pg.top, ldSec, ldBlock)", "importedQueue(pg.top, ldSec, ldBlock)", "importedPlace(",
                "queueAdd(\"now\", \"page\"", "queueAdd(\"add\", \"page\"", "readerOpenExternal(url=u)"], "actions")
        // Pages and Text are shown for a PDF only; the contents list and the find hits are inside the expanded row
        let exp = code(try component("DocExpanded", in: "content.contract"))
        XCTAssertTrue(exp.contains("if pdf"))
        XCTAssertTrue(exp.contains("ctlPanel == \"contents\"") && exp.contains("ctlPanel == \"find\""))
    }

    // C5
    func testRowCollapsesAfterAnActionAndOnATapOutsideButNotOnAStep() throws {
        let m = code(try read("members-content.contract"))
        func body(_ action: String) -> String {
            guard let r = m.range(of: "  action \(action)") else { XCTFail("no action \(action)"); return "" }
            let rest = m[r.upperBound...]
            var out: [String] = []
            for l in rest.split(separator: "\n", omittingEmptySubsequences: false).dropFirst() { if l.hasPrefix("  ") && !l.hasPrefix("   ") || l.isEmpty { break }; out.append(String(l)) }
            return out.joined(separator: "\n")
        }
        for a in ["ctlRead", "ctlQueueDoc", "ctlGo(n: number)", "ctlJump(section: number, block: number)", "ctlSetText(on: bool)", "ctlCloseDoc", "ctlWebRead(u: string, h: string)",
                  "ctlWebLater(u: string, h: string)", "ctlSafari(u: string)", "ctlCloseWeb", "ctlCloseBarWeb", "ctlHide"] {
            XCTAssertTrue(body(a).contains("ctlOpen = false"), "\(a) must put the row away")
        }
        XCTAssertFalse(body("ctlStep(n: number)").contains("ctlOpen = false"), "stepping keeps the row open so a page can be turned again")
        XCTAssertTrue(body("ctlToggle").contains("ctlOpen = ctlUp == false"))
        let app = code(try read("app.contract"))
        has(app, ["testId=\"content-scrim\"", "press=ctlHide", "if ctlUp and layer == \"\"", "members \"./members-content.contract\""], "app.contract")
        // the scrim sits above the page and its layers (z 8) and below the bar (the bar is drawn after it)
        let scrim = app.range(of: "content-scrim")!.lowerBound, bar = app.range(of: "BarShell()")!.lowerBound
        XCTAssertLessThan(scrim, bar)
        XCTAssertTrue(app.contains("z-index=9"))
        // a row is for the page in front: expanded for one page it is collapsed for another
        has(m, ["derive ctlUp = ctlMode != \"\" and ctlOpen and ctlFor == ctlKey", "ctlFor = ctlKey"], "per page")
    }

    // C6
    func testClosingIsOneStepFromTheCollapsedLine() throws {
        let row = code(try component("ContentRow", in: "content.contract"))
        let collapsed = row.components(separatedBy: "testId=\"content-line\"").last ?? ""
        has(collapsed, ["testId=\"doc-close\"", "testId=\"bar-web-close\"", "testId=\"reader-web-close\"", "press=ctlCloseDoc", "press=ctlCloseWeb", "press=ctlCloseBarWeb"], "collapsed line")
        has(collapsed, ["press=ctlToggle", "ctlTitle", "ctlPos", "ctlBusy"], "collapsed line shows the title, the place and Loading")
        let m = code(try read("members-content.contract"))
        let close = m.components(separatedBy: "action ctlCloseDoc").last?.components(separatedBy: "// ---- a web page").first ?? ""
        has(close, ["order = order.filter(a => a != gone)", "pagesOpen(nxt", "pagesClose(gone)"], "closing a document")
        XCTAssertTrue(m.contains("webUrl = \"\""), "closing a source page empties its address")
        XCTAssertTrue(m.contains("queueAct(\"closeWeb\", \"\")"), "closing a typed page")
    }

    // C7
    func testRowIsInTheLensAboveTheOnLine() throws {
        let bar = code(try read("bar.contract"))
        let row = bar.range(of: "ContentRow()")!.lowerBound, indicator = bar.range(of: "testId=\"page-indicator\"")!.lowerBound
        XCTAssertLessThan(row, indicator, "the row is the first line in the lens, above On:")
        XCTAssertLessThan(bar.range(of: "testId=\"bar\" position=\"relative\"")!.lowerBound, row, "inside the lens")
        has(bar, ["if onbd == false and layer == \"\" and ctlMode != \"\"", "ctlMode: string", "testId=\"bar-now\"", "testId=\"bar-field\"", "testId=\"bar-mic\"", "testId=\"bar-play\"",
                  "On: ${aimTitle} · ${stackCount} in the deck"], "bar")
    }

    // C8: every name a component of the touched files injects is provided by the root
    func testEveryInjectedNameIsProvided() throws {
        let app = try read("app.contract").split(separator: "\n").map { $0.trimmingCharacters(in: .whitespaces) }
        guard let p = app.firstIndex(of: "provide") else { return XCTFail("no provide list") }
        var provided = Set<String>()
        for l in app[(p + 1)...] { if l.hasPrefix("//") || l.isEmpty { continue }; if l.contains(" ") || l.hasPrefix("view") { break }; provided.insert(l) }
        XCTAssertGreaterThan(provided.count, 200)
        var checked = 0
        for f in ["content.contract", "imported-docs.contract", "reader.contract", "queue.contract", "bar.contract"] {
            let lines = code(try read(f)).split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
            var inInject = false
            for l in lines {
                if l == "  inject" { inInject = true; continue }
                if inInject {
                    if l.hasPrefix("    ") && !l.hasPrefix("     ") {
                        let name = l.trimmingCharacters(in: .whitespaces).components(separatedBy: ":").first!
                        XCTAssertTrue(provided.contains(name), "\(f) injects `\(name)`, which the root does not provide"); checked += 1
                    } else { inInject = false }
                }
            }
        }
        XCTAssertGreaterThan(checked, 60)
        let m = code(try read("members-content.contract"))
        for n in ["ctlOpen", "ctlFor", "ctlPanel", "ctlMode", "ctlKey", "ctlUp", "ctlClear", "ctlTitle", "ctlBusy", "ctlPos"] { XCTAssertTrue(m.contains(" \(n) =") || m.contains("derive \(n)"), n) }
    }

    // C9: the clearance is the lens plus the row. Lens idle: 8 pad + the On: line (16) + gap 4 + controls 48 + 6 pad ~ 82 (+ safe area); row 36 + gap 4.
    func testClearanceCoversTheLensAndTheRow() throws {
        let m = code(try read("members-content.contract"))
        let re = try NSRegularExpression(pattern: "derive ctlClear = ctlMode == \"\" \\? 0 : \\(bi.state == \"idle\" \\? ([0-9]+) : ([0-9]+)\\)")
        guard let r = re.firstMatch(in: m, range: NSRange(m.startIndex..., in: m)), let a = Range(r.range(at: 1), in: m), let b = Range(r.range(at: 2), in: m) else { return XCTFail("ctlClear") }
        let idle = Int(m[a])!, busy = Int(m[b])!
        XCTAssertGreaterThanOrEqual(idle, 82 + 40, "idle lens plus the row")
        XCTAssertGreaterThanOrEqual(busy, idle + 40, "a line above the controls (now playing: 30 + the progress hairline + gaps)")
        // no other contract still uses the old fixed clearance for a full-content frame of these two views
        let reader = code(try component("WebLayer", in: "reader.contract")), books = code(try component("DocPage", in: "imported-docs.contract"))
        lacks(reader + books, ["+ 72px", "56px + 8px"], "old clearance")
    }
}
