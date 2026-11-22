import Foundation

// Read-aloud bridge for mail (2026-11-19: the mail pages moved to shared code, AutoradioLab/mail.ts, which hands the text over). The one thing that stays native
// is the voice: the request is held here (the mail page shows that it was asked) and handed to the runtime's `speakText` path.

/// What the radio is asked to read aloud. The request is held here (the mail page shows that it was asked) and handed to
/// `handler`, which the runtime sets to the engine's `speakText(address:text:)`.
public final class MailReading: @unchecked Sendable {
    public static let shared = MailReading()
    private let lock = NSLock()
    private var _last: (address: String, text: String)?
    private var _handler: (@Sendable (String, String) -> Void)?
    public var last: (address: String, text: String)? { lock.lock(); defer { lock.unlock() }; return _last }
    public var handler: (@Sendable (String, String) -> Void)? {
        get { lock.lock(); defer { lock.unlock() }; return _handler }
        set { lock.lock(); _handler = newValue; lock.unlock() }
    }
    public func request(address: String, text: String) {
        lock.lock(); _last = (address, text); let h = _handler; lock.unlock()
        h?(address, text)
    }
}
