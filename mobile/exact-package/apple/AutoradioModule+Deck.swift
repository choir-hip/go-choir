// The deck's six @choir/autoradio package names (interface/deck.json). Exact generates their TypeScript functions, so shared code no longer needs an Apple-only app-module op.
// The deck itself is Deck.swift in the Kit; this adapter only reads scalar options and answers its state. Native because the audio session, engine and lock-screen controls are native.
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

    /// One package send, answered after its deck mutation. Every name answers the same DeckState shape.
    private func deckSend(_ name: String, _ args: [String: Any], _ reply: @escaping ([String: Any]) -> Void) {
        Task { @MainActor in
            switch name {
            case "deckPlay":
                if !runtime.snapshot().running { await runtime.start(virtual: virtualEngine) }
                _ = await Deck.shared.play(deckItem(args["item"] as? [String: Any] ?? [:]))
            case "deckEnqueue":
                let at = (args["at"] as? NSNumber).map { Int(truncating: $0) }
                Deck.shared.enqueue(deckItem(args["item"] as? [String: Any] ?? [:]), at: (at ?? -1) < 0 ? nil : at)
            case "deckAct": await Deck.shared.act(args["op"] as? String ?? "", value: num(args, "value"))
            case "deckAck": Deck.shared.ack(through: Int(num(args, "through")))
            case "deckWipe": Deck.shared.wipe()
            default: break
            }
            reply(json(Deck.shared.snapshot()))
        }
    }

    func registerDeck() {
        _ = onMainThread { Deck.shared.boot() } // a kill leaves no item open when the first package state is read
        onMainThread { Deck.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["deckState"]) } } }
        handle("deckState") { [unowned self] _, reply in reply(self.json(onMainThread { Deck.shared.snapshot() })) }
        handle("deckPlay") { [unowned self] args, reply in self.deckSend("deckPlay", args, reply) }
        handle("deckEnqueue") { [unowned self] args, reply in self.deckSend("deckEnqueue", args, reply) }
        handle("deckAct") { [unowned self] args, reply in self.deckSend("deckAct", args, reply) }
        handle("deckAck") { [unowned self] args, reply in self.deckSend("deckAck", args, reply) }
        handle("deckWipe") { [unowned self] args, reply in self.deckSend("deckWipe", args, reply) }
    }
}
