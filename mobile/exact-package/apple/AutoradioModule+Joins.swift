// What a clip row does is shared code (AutoradioLab/clips.ts: the rows, the choice of clip, "Open episode"). What stays native is the one thing only
// native code can do: "clips play from fingerprinted audio" (owner rule). The shared code answers WHICH clip (its id); this file asks the clip store,
// which holds the fingerprint records, and the locator (ClipLocator, in the run sheet player) finds where the clip is in the file the publisher serves.
// A clip with no usable fingerprint, or one the locator found a different file for, plays as its episode from the start, as before.
//   clipPlay(op, id, feed)   op play | queue; feed: the show the clip belongs to ("" when the record had none)
import Foundation

extension AutoradioModule {
    func registerJoins() {
        handle("clipPlay") { [unowned self] args, reply in
            let op = args["op"] as? String ?? "", id = args["id"] as? String ?? "", feed = args["feed"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                // The clip store is filled by the radio's one bulk request; a show's own clips (the shared page listed them) are asked for here when it lacks the id.
                if ClipStore.shared.clip(id) == nil, !feed.isEmpty { _ = await PodcastClips.ensure(client: ChoirEnvironment.shared.client(), feed: feed, force: true) }
                if ClipStore.shared.clip(id) == nil { ClipStore.shared.merge(await PodcastClips.fetch(client: ChoirEnvironment.shared.client()).clips) }   // a clip with no feed: the bulk set, merged, never replacing
                guard ClipStore.shared.clip(id) != nil else { reply(self.json(PodDone(status: "error", message: "That clip is not available any more."))); return }
                if op == "play", !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                if op == "queue" { HearIt.enqueue(id) } else { await HearIt.play(id) }
                self.touch(["queueView", "radioView", "barFacts", "podcastNow"], now: true)
                reply(self.json(PodDone()))
            }
        }
    }
}
