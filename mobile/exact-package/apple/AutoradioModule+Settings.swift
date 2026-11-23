// Settings, native half. The pages are shared code: the Computer page (AutoradioLab/computer.ts), Activity (activity.ts), About (settings.ts) and the themes (themes.ts).
// What only native code can say, in ONE source:
//   settingsFacts   the clock (a data source may not read it), the app's version (the bundle: only native code reads it), the radio engine's phase and its kept turns (the voice
//                   runtime holds them). Facts only: the rows, their words and the page are shared code.
import Foundation

extension AutoradioModule {
    func registerSettings() {

        handle("settingsFacts") { [unowned self] _, reply in      // (self: the runtime)
            let info = Bundle.main.infoDictionary ?? [:]
            var radio: [String: Any] = ["known": false, "running": false, "phase": ""]
            if let s = runtime?.snapshot() { radio = ["known": true, "running": s.running, "phase": s.phase] }
            let turns: [[String: Any]] = (runtime?.turns() ?? []).map { ["id": "\($0.id)", "outcome": $0.outcome, "transcript": $0.transcript, "configName": $0.configName] }
            reply(["now": Date().timeIntervalSince1970 * 1000, "appVersion": (info["CFBundleShortVersionString"] as? String) ?? "", "radio": radio, "turns": turns])
        }

    }
}
