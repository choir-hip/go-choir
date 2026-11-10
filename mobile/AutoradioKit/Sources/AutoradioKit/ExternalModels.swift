import Foundation

/// In the Exact app the model stages live in the app's own module (modules/apple), which links the third-party
/// libraries; this package module cannot import it. The app module defines an Objective-C class named
/// `AutoradioModelsRegistrar` with `+register:`; the kit looks it up by name and hands it the registry. On the Mac the
/// same class lives in the AutoradioModels target and the command registers it explicitly. Registering twice is harmless.
func registerExternalModels(_ r: StageRegistry) {
    guard let cls = NSClassFromString("AutoradioModelsRegistrar") else { return }
    _ = (cls as AnyObject).perform(NSSelectorFromString("register:"), with: r)
}
