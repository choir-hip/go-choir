// Lane file: paper. Owned by the paper lane (mobile/README.md, "File ownership").
// `paperFront` answers the paper's front for one view: Front page, My edition or a category (`view`). The stories are fixtures in demo mode, the
// public front list otherwise (a category asks the route's `?section=` list); the layout is the Kit's, for the width the page reports.
// My edition reads the reader's saved picks (`edition-topics.json`, written by the topic screens) every time it answers, so it is current when
// the reader comes back from editing: the contract passes `focus` (paper, article or away) so the answer is asked again on that return, and
// `tick` (a pull to refresh) so the lists are asked again from the server. Names are declared in interface/paper.json.
// The list the answer carries is also left in `PaperCurrentList.shared` for the player's queue.
import Foundation

extension AutoradioModule {
    func registerPaper() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        let source = PaperSource(cacheFile: support.appendingPathComponent("paper-front.json"))
        let topicsFile = support.appendingPathComponent("edition-topics.json")
        let engine = LayoutEngine()
        handle("paperFront") { [weak self] args, reply in
            let asked = args["view"] as? String ?? ""
            let view = asked.isEmpty ? PaperViews.front : asked
            let edition = args["edition"] as? String ?? ""
            let w = (args["width"] as? Double) ?? Double(args["width"] as? Int ?? 0)
            let width = w >= 200 ? w : 393
            let tick = Int((args["tick"] as? Double) ?? Double(args["tick"] as? Int ?? 0))
            Task {
                let fresh = source.isReload(tick)
                let r: PaperSource.Result
                var inCategory: [PaperStory]?
                if Demo.isOn { r = source.demo(edition: edition) } else {
                    let client = ChoirEnvironment.shared.client()
                    r = await source.live(client: client, fresh: fresh)
                    // A category is asked for its own list (fuller than the front's newest hundred); mine and the front need only the front list.
                    if PaperViews.isCategory(view) {
                        let sections = PaperViews.sections(live: r.sections, stories: r.stories)
                        let id = PaperViews.resolve(view, in: sections)?.id ?? view
                        let c = await source.live(client: client, section: id, fresh: fresh)
                        if c.status == "ready" { inCategory = c.stories }
                    }
                }
                let picks = view == PaperViews.mine ? (EditionSetupSession.load(topicsFile)?.topics ?? []) : []
                // A demo edition is dated by its own newest story; a live one by the clock.
                let front = PaperComposer.front(stories: r.stories, view: view, picks: picks, categoryStories: inCategory, live: r.sections, width: width,
                                                now: Demo.isOn ? nil : Date(), edition: edition, engine: engine, status: r.status, message: r.message)
                PaperCurrentList.shared.set(view: front.viewId, routes: front.routes, titles: front.titles)
                DispatchQueue.main.async { reply(self?.json(front) ?? [:]) }
            }
        }
    }
}
