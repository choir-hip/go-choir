// Lane 4: cards, launcher, signals. Sources and sends are declared in exact-package/interface/cards.json.
//
// What lives here: the shaded story list (Preferences: "What I've told the paper"), the story stack session, the launcher's
// data, the Undo offer after a close, and the gestures as events. The drawing is Contract (cards.contract, launcher.contract).
//
// The shading reaches lane 2 (Paper layout) two ways: (1) `shadingList` {route, lift, rank} for the layout to read, and
// (2) the `choirStories` order itself: this file answers `choirStories` with the paper's order shaded by the listener's acts
// (an effect on any front page that lists `sl.rows` in order). Lane 2 should read `shadingList` for lifts and
// may keep `sl.rows` order. See "Requests to the spine" in the lane report.
import Foundation

/// The cards lane's state. A module cannot hold new stored properties from an extension, and there is one module, so the lane
/// keeps its objects in one place.
final class CardsState: @unchecked Sendable {
    let lock = NSLock()
    var prefs: Preferences!
    var ready = false
    static let shared = CardsState()
}

extension AutoradioModule {
    private var cs: CardsState { CardsState.shared }

    // MARK: facts

    private func entityMap() -> [String: [String]] { [:] }

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

    private func touchLearning() {
        touch(["choirStories", "launcherData", "signalList"])
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
        // The tap also goes to the computer as the listener's own steering line, with the entry on air and the stories it cites (design 6).
        Task { @MainActor in
            if let t = SteerTap.on(entry: RunSheetPlayer.shared.store.onAir(), direction: kind == "more" ? "more" : "less", tappedStory: f.address) {
                SteerQueue.shared.enqueue(t); await SteerQueue.shared.flush()
            }
        }
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
        // The front list in the paper's order shaded by what the listener told it (an effect on the front page).
        handle("choirStories") { [unowned self] _, reply in
            let rows = choir.storyRows()
            let order = shaded().map(\.story.route)
            let by = Dictionary(rows.map { ($0.route, $0) }, uniquingKeysWith: { a, _ in a })
            let sorted = order.compactMap { by[$0] }
            reply(["rows": (sorted.isEmpty ? rows : sorted).map { json($0) }])
        }
        handle("launcherData") { [unowned self] args, reply in
            let q = (args["q"] as? String) ?? ""
            let s = shaded()
            let stories = s.map(\.story)
            let list = pages.list()
            let places = Places.rows(topStory: stories.first?.title, extra: [:])
            let recentPages = RecentPages.addresses(events: signals.recent(limit: 200), excluding: Set(list.map(\.address)))
                .map { Page(address: $0, kind: PageAddress.kind(of: $0), title: RecentPages.title(for: $0, stories: stories)) }
            let local = LauncherSearch.hits(query: q, open: list, places: places, recent: recentPages, stories: stories)
            // The whole paper is searched by shared code (retrieval-run.ts `launcherSearch`, the resource `psr`): it takes these rows and adds the stories after them.
            let hits = local
            func hit(_ h: SearchHit) -> [String: Any] { ["address": h.address, "title": h.title, "kind": h.kind, "detail": h.detail] }
            reply(["query": q, "hasQuery": !q.trimmingCharacters(in: .whitespaces).isEmpty,
                   "places": places.map { ["slug": $0.slug, "name": $0.name, "address": $0.address, "line": $0.line] },
                   "recent": recentPages.map { ["address": $0.address, "title": $0.title, "kind": $0.kind, "detail": "Recent"] },
                   "hits": hits.map(hit), "stories": stories.count, "searchNote": ""])
        }

        // ---- sends (each answers {message})
        func ack(_ reply: @escaping ([String: Any]) -> Void, _ message: String = "") { reply(["message": message]) }

        // A swipe right on a page card in the pile: more of this when it is a story; the card leans and springs back otherwise.
        handle("storyMore") { [unowned self] args, reply in
            let address = args["address"] as? String ?? "", surface = args["surface"] as? String ?? "pile"
            guard let f = facts(forAddress: address) else { return ack(reply) }
            tell("more", f, surface: surface)
            ack(reply, Preferences.note(kind: "more", f))
        }
    }
}

/// "What I've told the paper" as a page the reader opens: `texture:what-i-told-the-paper`. LOCAL PATH: the acts are local. REAL PATH
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
