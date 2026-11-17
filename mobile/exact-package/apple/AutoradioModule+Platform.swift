// The platform seam of the module: every UIKit or AppKit call the lanes need goes through here, so the rest of the
// module reads the same on iOS and on the Mac (Exact builds the same package for both; the Mac host is AppKit).
// iOS bodies are the code that used to sit in the lane files; the Mac bodies are the AppKit equivalents.
import AVFoundation
import ExactPackage
import Foundation
#if canImport(UIKit)
import UIKit
#elseif canImport(AppKit)
import AppKit
#endif

enum Platform {
    /// Hand a URL to the system (the browser, Mail, the phone app).
    static func open(_ url: URL) {
        DispatchQueue.main.async {
            #if os(iOS)
            UIApplication.shared.open(url)
            #else
            NSWorkspace.shared.open(url)
            #endif
        }
    }

    /// The system's appearance: "dark" or "light" (main thread; other threads hop to it).
    static func systemScheme() -> String {
        let read: () -> String = {
            #if os(iOS)
            let style = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first?.windows.first?.traitCollection.userInterfaceStyle
            return style == .dark ? "dark" : "light"
            #else
            return NSApplication.shared.effectiveAppearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? "dark" : "light"
            #endif
        }
        return Thread.isMainThread ? read() : DispatchQueue.main.sync(execute: read)
    }

    /// The system's Reduce Motion setting, and the notice that it changed.
    static func reduceMotion() -> Bool {
        let read: () -> Bool = {
            #if os(iOS)
            UIAccessibility.isReduceMotionEnabled
            #else
            NSWorkspace.shared.accessibilityDisplayShouldReduceMotion
            #endif
        }
        return Thread.isMainThread ? read() : DispatchQueue.main.sync(execute: read)
    }
    #if os(iOS)
    static let reduceMotionChanged = UIAccessibility.reduceMotionStatusDidChangeNotification
    static let becameActive = UIApplication.willEnterForegroundNotification
    static let willResign = UIApplication.willResignActiveNotification
    #else
    static let reduceMotionChanged = NSWorkspace.accessibilityDisplayOptionsDidChangeNotification
    static let becameActive = NSApplication.willBecomeActiveNotification
    static let willResign = NSApplication.willResignActiveNotification
    #endif
    /// Notices after which unsaved work must be on disk: the app resigns, and on the Mac it quits (a quit gives no resign).
    static var flushNotes: [Notification.Name] {
        #if os(iOS)
        [willResign]
        #else
        [willResign, NSApplication.willTerminateNotification]
        #endif
    }

    /// Where the sound goes, for the setup screen ("Playing on ...").
    static func outputName() -> String {
        #if os(iOS)
        return AVAudioSession.sharedInstance().currentRoute.outputs.first?.portName ?? "this phone"
        #else
        return "this Mac"
        #endif
    }
}

#if os(macOS)
/// The Mac has no audio session: AVAudioEngine opens the default input and output itself (Exact: "the Mac has no
/// session"; its ExactAudioSession claim is nil there). So this owner holds no claim and always reports ready; the
/// states are tracked only so a trace line says what the radio asked for. Echo cancellation and the single voice live
/// in the engine (voice processing I/O), not here.
final class MacSessionOwner: AudioSessionOwner, @unchecked Sendable {
    private let note: (String) -> Void
    private let lock = NSLock()
    private var held: AudioState?
    init(context: ExactPackageContext, note: @escaping (String) -> Void) { self.note = note }
    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        lock.lock(); let changed = held != state; held = state; lock.unlock()
        if changed { note("claimed \(state.rawValue) (\(reason)): \(describe())") }
    }
    func release() { lock.lock(); held = nil; lock.unlock(); note("released") }
    func describe() -> String { "ready (macOS: no audio session)" }
    func observe(_ handler: @escaping @Sendable (String, String) -> Void) {}
}
typealias PlatformSessionOwner = MacSessionOwner
#else
typealias PlatformSessionOwner = ExactSessionOwner
#endif
