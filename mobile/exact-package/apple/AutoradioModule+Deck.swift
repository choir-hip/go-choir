// The deck's names (interface/deck.json): deckState (live), deckPlay, deckEnqueue, deckAct, deckAck. The deck itself is Deck.swift in the Kit; this
// file only reads the options and answers the state. Native because the audio session, the engine and the lock screen are native: shared code
// (deck.ts) commands the deck and reads its journal; nothing here decides what plays next.
import Foundation

extension AutoradioModule {
    private func num(_ a: [String: Any], _ k: String) -> Double { (a[k] as? NSNumber)?.doubleValue ?? 0 }

    private func deckItem(_ a: [String: Any]) -> DeckItem {
        var i = DeckItem(id: a["id"] as? String ?? "", kind: a["kind"] as? String == "audio" ? "audio" : "speech")
        i.title = a["title"] as? String ?? ""; i.show = a["show"] as? String ?? ""; i.document = a["document"] as? String ?? ""
        i.paragraphs = a["paragraphs"] as? String ?? ""; i.url = playableAudioAddress(a["url"] as? String ?? "")
        i.start = num(a, "start"); i.end = num(a, "end"); i.block = Int(num(a, "block")); i.seconds = num(a, "seconds")
        i.requiresPreviousHeard = (a["requiresPreviousHeard"] as? Bool) ?? false; i.segment = a["segment"] as? String ?? ""
        return i
    }

    /// The sound of a take or a Files recording is a file the phone holds: shared code names it `app:/cache/files/<folder>/<name>` (the address its own pages read), which is not an address AVFoundation can open.
    /// This turns exactly that shape (two plain parts, the file there) into the file's URL; any other address (https) goes through unchanged. Native because only native code knows where the folder is on disk.
    func playableAudioAddress(_ address: String) -> String {
        let prefix = "app:/cache/files/"
        guard address.hasPrefix(prefix) else { return address }
        let parts = address.dropFirst(prefix.count).split(separator: "/", omittingEmptySubsequences: false).map(String.init)
        guard parts.count == 2, !parts.contains(where: { $0.isEmpty || $0 == "." || $0 == ".." }) else { return address }
        let url = filesCacheDisk.appendingPathComponent(parts[0], isDirectory: true).appendingPathComponent(parts[1])
        return FileManager.default.fileExists(atPath: url.path) ? url.absoluteString : address
    }

    /// The deck's five names for shared code, through the app module's one `deck` op (ModelsEntry.swift): `{op: "deck", name, args}` answers the deck's state. Native because only native code
    /// plays an item and keeps the journal; the names are item-shaped (docs/reference/exact/deck.md), nothing here decides what plays next.
    @MainActor public func deckCall(name: String, args: [String: Any]) async -> [String: Any] {
        switch name {
        case "deckPlay":
            if !runtime.snapshot().running { await runtime.start(virtual: virtualEngine) }
            _ = await Deck.shared.play(deckItem(args))
        case "deckEnqueue":
            let at = (args["at"] as? NSNumber).map { Int(truncating: $0) }
            Deck.shared.enqueue(deckItem(args), at: (at ?? -1) < 0 ? nil : at)
        case "deckAct": await Deck.shared.act(args["op"] as? String ?? "", value: num(args, "value"))
        case "deckAck": Deck.shared.ack(through: Int(num(args, "through")))
        default: break
        }
        return json(Deck.shared.snapshot())
    }

    func registerDeck() {
        onMainThread { Deck.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["deckState"]) } } }
        handle("deckState") { [unowned self] _, reply in reply(self.json(onMainThread { Deck.shared.snapshot() })) }
        for name in ["deckPlay", "deckEnqueue", "deckAct", "deckAck"] {
            handle(name) { [unowned self] args, reply in Task { @MainActor in reply(await self.deckCall(name: name, args: args)) } }
        }
    }
}
