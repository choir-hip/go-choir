// The app's native module. It exists so that the app target compiles Swift of its own: Exact links
// `modules/apple/*.xcframework` (static third-party libraries) into this module, and the model stages
// (Models_*.swift, copied in from AutoradioKit/Sources/AutoradioModels by scripts/sync-exact-package.sh) import them.
// The kit (package module ExactPkg_autoradio) finds AutoradioModelsRegistrar by name; see ExternalModels.swift there.
import Foundation
import ExactPkg_autoradio

// The one native call the signed-in seam (api.ts) makes, with the reason only native code can meet: it reads the Keychain item today's
// sign-in wrote (service news.choir.autoradio.choir), in place. Exact's own secret store keeps items under this app's id, so shared code
// cannot see that item; the owner's phone stays signed in because nothing is moved. Read only: `{op: "get", account}` for the three
// accounts CredentialReader allows, answering `{found, value}`. It logs nothing. It goes when sign-in itself is moved and the key is
// kept by Exact's store.
final class ModelsModule: ExactModule {
    override func call(_ request: [String: Any]) -> [String: Any] { CredentialReader().reply(request) }
    override func later(_ request: [String: Any], reply: ExactReply) { reply.send(call(request)) }
}
let exactModule: ExactModule.Type = ModelsModule.self
