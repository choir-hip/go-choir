// What a clip row does is shared code (AutoradioLab/clips.ts: the rows, the choice of clip, "Open episode", and the one request for the set of clips). What
// stays native is the one thing only native code can do: "clips play from fingerprinted audio" (owner rule). The shared code hands over the clip's own
// record as the server sent it (its audio and its fingerprint); this file keeps it for the player, and the locator (ClipLocator, in the run sheet player)
// finds where the clip is in the file the publisher serves. A clip with no usable fingerprint, or one the locator found a different file for, plays as its
// episode from the start. Native code never asks the server for a clip.
//   clipPlay(op, id, record)   op play | queue; record: the clip's row as JSON
import Foundation

extension AutoradioModule {
    func registerJoins() {
        handle("clipPlay") { [unowned self] args, reply in
            let op = args["op"] as? String ?? "", id = args["id"] as? String ?? "", record = args["record"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                if let clip = PodcastClip.parse(record: record), clip.clipID == id { ClipStore.shared.put(clip) }
                guard ClipStore.shared.clip(id) != nil else { reply(self.json(PodDone(status: "error", message: "That clip is not available any more."))); return }
                if op == "play", !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                if op == "queue" { HearIt.enqueue(id) } else { await HearIt.play(id) }
                self.touch(["queueView", "radioView", "barFacts", "podcastNow"], now: true)
                reply(self.json(PodDone()))
            }
        }
    }
}
