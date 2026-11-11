// Lane 4: cards, launcher, signals. Sources and sends are declared in exact-package/interface/cards.json.
//
// What lives here: the shaded story list (Preferences: "What I've told the paper"), the story stack session, the launcher's
// data, the Undo offer after a close, and the gestures as events. The drawing is Contract (cards.contract, launcher.contract).
//
// The shading reaches lane 2 (Paper layout) two ways: (1) `shadingList` {route, lift, rank} for the layout to read, and
// (2) the `choirStories` order itself: this file answers `choirStories` with the paper's order shaded by the listener's acts
// (a demo-mode-visible effect on any front page that lists `sl.rows` in order). Lane 2 should read `shadingList` for lifts and
// may keep `sl.rows` order. See "Requests to the spine" in the lane report.
import Foundation

/// The cards lane's state. A module cannot hold new stored properties from an extension, and there is one module, so the lane
/// keeps its objects in one place.
final class CardsState: @unchecked Sendable {
    let lock = NSLock()
    var prefs: Preferences!
    let stack = StackSession()
    var undo: UndoOffer?
    var note = ""
    var noteAt = 0.0
    var ready = false
    static let shared = CardsState()
}

extension AutoradioModule {
    private var cs: CardsState { CardsState.shared }

    // MARK: facts

    private func entityMap() -> [String: [String]] { Demo.isOn ? StoryTopics.demoMap() : [:] }

    private func allFacts() -> [StoryFacts] {
        let map = entityMap()
        return choir.storyRows().map { StoryTopics.facts(route: $0.route, title: $0.title, dek: $0.dek, section: $0.section, map: map) }
    }
    private func facts(forAddress address: String) -> StoryFacts? {
        guard address.hasPrefix("pub:") else { return nil }
        let route = PageAddress.rest(of: address)
        return allFacts().first { $0.route == route }
    }
    private func shaded() -> [(story: StoryFacts, lift: Double)] { cs.prefs.order(allFacts()) }

    private func storyJSON(_ s: StoryFacts, lift: Double, n: Int, dir: String) -> [String: Any] {
        ["route": s.route, "address": s.address, "title": s.title, "dek": s.dek, "section": s.section,
         "entityLine": s.entities.joined(separator: " · "), "primary": Preferences.primary(s), "lift": lift, "n": n, "dir": dir,
         "why": cs.prefs.why(for: s)]
    }

    private func now() -> Double { Date().timeIntervalSince1970 }

    private func setNote(_ text: String) {
        cs.lock.lock(); cs.note = text; cs.noteAt = now(); cs.lock.unlock()
        touch(["toldPaper"])
        DispatchQueue.main.asyncAfter(deadline: .now() + 2.7) { [weak self] in self?.touch(["toldPaper"]) }
    }

    private func touchLearning() {
        touch(["toldPaper", "cardStories", "storyStack", "choirStories", "shadingList", "launcherData", "signalList"])
    }

    // MARK: close with an Undo offer

    private func closeWithUndo(_ address: String, after delay: Double) {
        let title = pages.list().first { $0.address == address }?.title ?? PageAddress.defaultTitle(address)
        DispatchQueue.main.asyncAfter(deadline: .now() + delay) { [weak self] in
            guard let self else { return }
            self.pages.close(address)
            self.cs.lock.lock(); self.cs.undo = UndoOffer(title: title, address: address, since: self.now()); self.cs.lock.unlock()
            self.touch(["openPages", "undoOffer", "launcherData"])
            DispatchQueue.main.asyncAfter(deadline: .now() + UndoOffer.seconds + 0.1) { [weak self] in self?.touch(["undoOffer"]) }
        }
    }

    private func gesture(_ kind: String, _ f: StoryFacts, surface: String, meaning: String) {
        signals.record(SignalEvent(kind: kind, subject: f.address,
                                   detail: SignalDetail.gesture(surface: surface, page: "reader", meaning: meaning, entities: f.entities, topic: f.topic)))
    }

    /// The scored act for a story, with its signal and its acknowledgement. Returns whether an act was written.
    @discardableResult
    private func tell(_ kind: String, _ f: StoryFacts, surface: String) -> Bool {
        guard cs.prefs.record(kind: kind == "more" ? "more" : "less", f) != nil else { return false }
        gesture(kind == "more" ? SignalKind.more : SignalKind.less, f, surface: surface, meaning: kind)
        setNote(Preferences.note(kind: kind, f))
        touchLearning()
        return true
    }

    // MARK: registration

    func registerCards() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        cs.prefs = Preferences(file: support.appendingPathComponent("what-i-told-the-paper.json"))
        cs.ready = true
        pageSources.register(ToldPaperSource(prefs: cs.prefs))

        // ---- sources
        // The front list in the paper's order shaded by what the listener told it (the effect on the front page in demo mode).
        handle("choirStories") { [unowned self] _, reply in
            let rows = choir.storyRows()
            let order = shaded().map(\.story.route)
            let by = Dictionary(rows.map { ($0.route, $0) }, uniquingKeysWith: { a, _ in a })
            let sorted = order.compactMap { by[$0] }
            reply(["rows": (sorted.isEmpty ? rows : sorted).map { json($0) }])
        }
        handle("shadingList") { [unowned self] _, reply in
            reply(["rows": shaded().enumerated().map { ["route": $0.element.story.route, "lift": $0.element.lift, "rank": $0.offset] as [String: Any] }])
        }
        handle("cardStories") { [unowned self] _, reply in
            let s = shaded()
            reply(["rows": s.enumerated().map { storyJSON($0.element.story, lift: $0.element.lift, n: $0.offset, dir: "") },
                   "index": 0, "setAside": 0, "total": s.count])
        }
        handle("storyStack") { [unowned self] _, reply in
            let s = shaded()
            let rows = cs.stack.rows(shaded: s)
            reply(["rows": rows.map { storyJSON($0.story, lift: $0.lift, n: $0.n, dir: $0.dir) },
                   "index": cs.stack.index(shaded: s), "setAside": cs.stack.setAside, "total": rows.count])
        }
        handle("toldPaper") { [unowned self] _, reply in
            let g = cs.prefs.gestures
            let all = cs.prefs.all.reversed()
            cs.lock.lock(); let fresh = now() - cs.noteAt < 2.5; let note = fresh ? cs.note : ""; cs.lock.unlock()
            let df = DateFormatter(); df.locale = Locale(identifier: "en_US_POSIX"); df.dateFormat = "d MMM HH:mm"
            var d: [String: Any] = [:]
            d["count"] = cs.prefs.count
            d["rows"] = all.prefix(12).map { a -> [String: Any] in
                ["kind": a.kind, "title": a.title, "entities": a.entities.joined(separator: ", "),
                 "when": df.string(from: Date(timeIntervalSince1970: a.at)), "route": a.route]
            }
            d["lines"] = cs.prefs.lines()
            d["left"] = g.left
            d["right"] = g.right
            let labels: [String: String] = ["close": "Close", "close-less": "Close, and less of this", "less-in-stack": "Close; less of this in the stack"]
            d["leftLabel"] = labels[g.left] ?? "Close"
            d["rightLabel"] = g.right == "more" ? "More of this" : "Nothing"
            d["note"] = note
            reply(d)
        }
        handle("undoOffer") { [unowned self] _, reply in
            cs.lock.lock(); let u = cs.undo; cs.lock.unlock()
            let open = u?.isOpen(at: now()) ?? false
            reply(["open": open, "title": open ? (u?.title ?? "") : "", "address": open ? (u?.address ?? "") : ""])
        }
        handle("launcherData") { [unowned self] args, reply in
            let q = (args["q"] as? String) ?? ""
            let s = shaded()
            let stories = s.map(\.story)
            let list = pages.list()
            let places = Places.rows(topStory: stories.first?.title, extra: Demo.isOn ? Places.demoExtra() : [:])
            let recentPages = RecentPages.addresses(events: signals.recent(limit: 200), excluding: Set(list.map(\.address)))
                .map { Page(address: $0, kind: PageAddress.kind(of: $0), title: RecentPages.title(for: $0, stories: stories)) }
            let hits = LauncherSearch.hits(query: q, open: list, places: places, recent: recentPages, stories: stories)
            func hit(_ h: SearchHit) -> [String: Any] { ["address": h.address, "title": h.title, "kind": h.kind, "detail": h.detail] }
            reply(["query": q, "hasQuery": !q.trimmingCharacters(in: .whitespaces).isEmpty,
                   "places": places.map { ["slug": $0.slug, "name": $0.name, "address": $0.address, "line": $0.line] },
                   "recent": recentPages.map { ["address": $0.address, "title": $0.title, "kind": $0.kind, "detail": "Recent"] },
                   "hits": hits.map(hit), "stories": stories.count])
        }

        // ---- sends (each answers {message})
        func ack(_ reply: @escaping ([String: Any]) -> Void, _ message: String = "") { reply(["message": message]) }

        // A swipe right on a page card in the pile: more of this when it is a story; the card leans and springs back otherwise.
        handle("storyMore") { [unowned self] args, reply in
            let address = args["address"] as? String ?? "", surface = args["surface"] as? String ?? "pile"
            guard let f = facts(forAddress: address) else { return ack(reply) }
            if cs.prefs.gestures.right == "nothing" {
                gesture(SignalKind.ignored, f, surface: surface, meaning: "nothing"); touch(["signalList"]); return ack(reply)
            }
            tell("more", f, surface: surface)
            ack(reply, Preferences.note(kind: "more", f))
        }
        handle("storyLess") { [unowned self] args, reply in
            let address = args["address"] as? String ?? "", surface = args["surface"] as? String ?? "pile"
            guard let f = facts(forAddress: address) else { return ack(reply) }
            tell("less", f, surface: surface)
            ack(reply, Preferences.note(kind: "less", f))
        }
        // A card thrown left: close after the card has left the screen. With the "close and less of this" meaning, also less.
        handle("cardFling") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            if let f = facts(forAddress: address), cs.prefs.gestures.leftIsLess(surface: "pile") { tell("less", f, surface: "pile") }
            closeWithUndo(address, after: 0.26)
            ack(reply)
        }
        handle("cardClose") { [unowned self] args, reply in
            closeWithUndo(args["address"] as? String ?? "", after: 0); ack(reply)
        }
        handle("cardUndo") { [unowned self] _, reply in
            pages.reopenLastClosed()
            cs.lock.lock(); let u = cs.undo; cs.undo = nil; cs.lock.unlock()
            signals.record(SignalEvent(kind: SignalKind.undo, subject: u?.address ?? "", detail: "close"))
            touch(["openPages", "undoOffer", "signalList"]); ack(reply)
        }
        // The story stack: a card thrown left (set aside; neutral unless the meanings say less) or right (more of this).
        handle("stackDecide") { [unowned self] args, reply in
            let address = args["address"] as? String ?? "", side = args["side"] as? String ?? ""
            guard let f = facts(forAddress: address) else { return ack(reply) }
            if side == "right" {
                if cs.prefs.gestures.right == "more" { tell("more", f, surface: "stack") }
                else { gesture(SignalKind.ignored, f, surface: "stack", meaning: "nothing") }
            } else if side == "left" {
                if cs.prefs.gestures.leftIsLess(surface: "stack") { tell("less", f, surface: "stack") }
                else { gesture(SignalKind.setAside, f, surface: "stack", meaning: "close") }
            } else { return ack(reply) }
            cs.stack.decide(route: f.route, side: side)
            touchLearning()
            ack(reply, side == "right" ? Preferences.note(kind: "more", f) : "Set aside")
        }
        handle("stackUndo") { [unowned self] _, reply in
            if let d = cs.stack.undo() {
                if d.side == "right" { cs.prefs.removeLast(route: d.route, kind: "more") } else { cs.prefs.removeLast(route: d.route, kind: "less") }
                signals.record(SignalEvent(kind: SignalKind.undo, subject: "pub:" + d.route, detail: SignalDetail.gesture(surface: "stack", page: "reader", meaning: d.side, entities: [], topic: "")))
            }
            touchLearning(); ack(reply)
        }
        handle("stackReset") { [unowned self] _, reply in cs.stack.reset(); touchLearning(); ack(reply) }
        handle("toldForget") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            let n = cs.prefs.forget(route: PageAddress.rest(of: address))
            if n > 0 { signals.record(SignalEvent(kind: SignalKind.forget, subject: address, detail: "\(n)")) }
            touchLearning(); ack(reply)
        }
        handle("toldReset") { [unowned self] _, reply in
            let n = cs.prefs.reset(); cs.stack.reset()
            if n > 0 { signals.record(SignalEvent(kind: SignalKind.reset, subject: "told", detail: "\(n)")) }
            touchLearning(); ack(reply)
        }
        handle("gestureSet") { [unowned self] args, reply in
            let side = args["side"] as? String ?? "", meaning = args["meaning"] as? String ?? ""
            if cs.prefs.setGesture(side: side, meaning: meaning) {   // Settings owns the meaning: keep it in step
                let back = ["close-less": "closeLess", "less-in-stack": "lessInStack"][meaning] ?? meaning
                _ = AppSettings.shared.set(side == "left" ? "swipeLeft" : "swipeRight", back)
                touch(["settingRows"])
            }
            touch(["toldPaper"]); ack(reply)
        }
    }
}

/// "What I've told the paper" as a page the reader opens: `texture:what-i-told-the-paper`. DEMO PATH: the acts are local. REAL PATH
/// (named, not built): a private Texture beside the edition, written through the Texture desk; the reader lane's Texture source
/// would then answer this address and this source is removed.
struct ToldPaperSource: PageSource {
    let prefs: Preferences
    static let address = "texture:what-i-told-the-paper"
    func handles(_ address: String) -> Bool { address == Self.address }
    func load(_ address: String) async throws -> PageModel {
        let lines = prefs.lines()
        return PageModel(address: address, kind: "reader", title: "What I've told the paper", status: "ready",
                         lines: lines.isEmpty ? ["Nothing yet. Swipe a story right for more like it."] : lines)
    }
}
