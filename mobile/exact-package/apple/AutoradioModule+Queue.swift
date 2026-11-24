// The radio's native side (the run sheet, the queue and the Autoradio page are shared code: AutoradioLab/radio-runtime.ts, radio-bridge.ts). What is left here is the mailbox:
//   radioNote   a number the contract asks again on; touched when native handed shared code a verb (RadioBridge.send) or something the pump reads changed.
//   the app module's ops `radioFacts`, `radioSet` (ModelsEntry.swift) answer shared code through `radioFacts(peek:)` (AutoradioModule+Bar.swift) and `radioSet(_:)` below.
// Native because the voice turn engine and the handlers that have not moved are native, and only native code knows the audio's own facts; nothing here decides what plays.
import Foundation

extension AutoradioModule {
    func registerQueue() {
        var tick = 0
        RadioBridge.shared.changed = { [weak self] in DispatchQueue.main.async { self?.touch(["radioNote"], now: true) } }    // at once: a verb from the voice turn is applied on the next pulse, not a second later
        handle("radioNote") { _, reply in tick += 1; reply(["tick": tick]) }
    }

    /// shared code -> native: the turn route's queue snapshot (the voice turn carries it), the facts of the entries on air and ahead (the voice's retrieval) and the story on air (a take's provenance), as shared code last made them (voice-facts.ts).
    public func radioSet(_ r: [String: Any]) -> [String: Any] {
        if r["signedOut"] as? Bool == true { RadioBridge.shared.markSignedOut(); return ["ok": true] }
        let snap = (r["snapshot"] as? String)?.data(using: .utf8)
        let facts = (r["facts"] as? [[String: Any]]).flatMap { try? JSONSerialization.data(withJSONObject: $0) }
        let route = r["respondsRoute"] as? String ?? ""
        let responds = route.isEmpty ? nil : TakeResponds(route: route, title: r["respondsTitle"] as? String ?? "")
        let onAir = (r["facts"] as? [[String: Any]])?.contains { $0["state"] as? String == "on_air" } ?? false
        return ["ok": RadioBridge.shared.set(gen: (r["gen"] as? NSNumber)?.intValue ?? 0, snapshot: snap, facts: facts, responds: responds, onAir: onAir)]
    }
}
