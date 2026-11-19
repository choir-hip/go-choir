// The deck's names (interface/deck.json): deckState (live), deckPlay, deckEnqueue, deckAct, deckAck. The deck itself is Deck.swift in the Kit; this
// file only reads the options and answers the state. Native because the audio session, the engine and the lock screen are native: shared code
// (deck.ts) commands the deck and reads its journal; nothing here decides what plays next.
import Foundation

extension AutoradioModule {
    private func num(_ a: [String: Any], _ k: String) -> Double { (a[k] as? NSNumber)?.doubleValue ?? 0 }

    private func deckItem(_ a: [String: Any]) -> DeckItem {
        var i = DeckItem(id: a["id"] as? String ?? "", kind: a["kind"] as? String == "audio" ? "audio" : "speech")
        i.title = a["title"] as? String ?? ""; i.show = a["show"] as? String ?? ""; i.document = a["document"] as? String ?? ""
        i.paragraphs = a["paragraphs"] as? String ?? ""; i.url = a["url"] as? String ?? ""
        i.start = num(a, "start"); i.end = num(a, "end"); i.block = Int(num(a, "block")); i.seconds = num(a, "seconds")
        i.requiresPreviousHeard = (a["requiresPreviousHeard"] as? Bool) ?? false
        return i
    }

    func registerDeck() {
        onMainThread { Deck.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["deckState"]) } } }
        handle("deckState") { [unowned self] _, reply in reply(self.json(onMainThread { Deck.shared.snapshot() })) }
        handle("deckPlay") { [unowned self] args, reply in
            let item = deckItem(args)
            Task { @MainActor in
                if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                _ = await Deck.shared.play(item)
                reply(self.json(Deck.shared.snapshot()))
            }
        }
        handle("deckEnqueue") { [unowned self] args, reply in
            let item = deckItem(args)
            let at = (args["at"] as? NSNumber).map { Int(truncating: $0) }
            Task { @MainActor in
                Deck.shared.enqueue(item, at: (at ?? -1) < 0 ? nil : at)
                reply(self.json(Deck.shared.snapshot()))
            }
        }
        handle("deckAct") { [unowned self] args, reply in
            let op = args["op"] as? String ?? "", v = num(args, "value")
            Task { @MainActor in await Deck.shared.act(op, value: v); reply(self.json(Deck.shared.snapshot())) }
        }
        handle("deckAck") { [unowned self] args, reply in
            let n = Int(num(args, "through"))
            Task { @MainActor in Deck.shared.ack(through: n); reply(self.json(Deck.shared.snapshot())) }
        }
    }
}
