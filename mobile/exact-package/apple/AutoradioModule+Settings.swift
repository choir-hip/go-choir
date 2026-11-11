// Lane file: settings. Owned by the settings lane (mobile/README.md, "File ownership"). Register this lane's sources and sends here:
//   handle("someSource") { args, reply in reply(json(...)) }          // the names are declared in exact-package/interface/settings.json
// Reach the shared kit objects through self: pages, signals, themes, pageSources (register a PageSource for your
// kinds), runtime, choir; call touch(["someSource"]) when a live source changes. The spine calls registerSettings() once.
import Foundation

extension AutoradioModule {
    func registerSettings() {
    }
}
