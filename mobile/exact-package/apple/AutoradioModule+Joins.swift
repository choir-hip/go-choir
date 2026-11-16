// Lane file: autoradio-joins. What the clip rows do ("Hear it" on a story's page, "Clips" on a show's page). Sources and sends are declared in
// exact-package/interface/joins.json; the logic and its tests are in Kit/Choir/DJQueueAdapter.swift (HearIt, ClipActions) and PodcastClips.swift.
//   showClips(address)   the clips found for a show (a podcast show page's address), quote first; the show's own fetch runs in the background
//   clipAct(op, id)      op play | queue | open, id: the clip (play applies the 3 second length rule through DJClipRunner.preflight)
import Foundation

extension AutoradioModule {
    func registerJoins() {
        handle("showClips") { [unowned self] args, reply in
            guard let feed = PodcastService.parseShow(args["address"] as? String ?? "") else { return reply(self.json(ClipRowsView())) }
            reply(self.json(ClipRowsView(rows: HearIt.rows(feed: feed))))
            // Asked for in the background (at most every 15 minutes per show); silent when the route is missing or has nothing.
            Task { [weak self] in
                if await PodcastClips.ensure(client: ChoirEnvironment.shared.client(), feed: feed) { self?.touch(["showClips"]) }
            }
        }
        handle("clipAct") { [unowned self] args, reply in
            let op = args["op"] as? String ?? "", id = args["id"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                if op == "play", !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                let note = await ClipActions.run(op: op, clipID: id)
                self.touch(["queueView", "radioView", "barState"], now: true)
                reply(self.json(ClipAckView(message: note)))
            }
        }
    }
}
