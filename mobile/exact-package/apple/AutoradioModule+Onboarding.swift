// Lane file: onboarding. Owned by the onboarding lane (mobile/README.md, "File ownership"). Register this lane's sources and sends here:
//   handle("someSource") { args, reply in reply(json(...)) }          // the names are declared in exact-package/interface/onboarding.json
// Reach the shared kit objects through self: pages, signals, themes, pageSources (register a PageSource for your
// kinds), runtime, choir; call touch(["someSource"]) when a live source changes. The spine calls registerOnboarding() once.
import Foundation

extension AutoradioModule {
    func registerOnboarding() {
    }
}
