// The radio's native side (the run sheet, the queue and the Autoradio page are shared code: AutoradioLab/radio-runtime.ts, radio-bridge.ts). What is left here is the mailbox:
//   radioNote   a number the contract asks again on; touched when native handed shared code a verb (RadioBridge.send) or something the pump reads changed.
//   the app module's ops `radioFacts`, `radioSet` (ModelsEntry.swift) answer shared code through `radioFacts(peek:)` (AutoradioModule+Bar.swift) and `radioSet(_:)` below.
// Native because the voice turn engine and the handlers that have not moved are native, and only native code knows the audio's own facts; nothing here decides what plays.
import Foundation

extension AutoradioModule {
    func registerQueue() {
        var tick = 0
        RadioBridge.shared.changed = { [weak self] in DispatchQueue.main.async { self?.touch(["radioNote"]) } }
        handle("radioNote") { _, reply in tick += 1; reply(["tick": tick]) }
    }

    /// shared code -> native: the turn route's queue snapshot (the voice turn carries it) and the entry on air and the sheet as shared code last made them.
    public func radioSet(_ r: [String: Any]) -> [String: Any] {
        let snap = (r["snapshot"] as? String)?.data(using: .utf8)
        let onAir = (r["onAir"] as? [String: Any]).flatMap { try? JSONSerialization.data(withJSONObject: $0) }
        let sheet = (r["sheet"] as? [String: Any]).flatMap { try? JSONSerialization.data(withJSONObject: $0) }
        RadioBridge.shared.set(snapshot: snap, onAir: onAir, sheet: sheet)
        return ["ok": true]
    }
}
