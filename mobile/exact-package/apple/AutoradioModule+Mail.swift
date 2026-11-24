// Lane file: mail. What is left of Swift mail (2026-11-19: the mailbox, the messages, drafts and the phone's copy are shared code, AutoradioLab/mail.ts).
// One send stays here because only native code can reach what it touches: the app's own web view (the approval page). "Read this to me" is shared code's (radio-bridge.ts radioListen). The phone never talks to a mail provider: that is all in mail.ts.
import Foundation

extension AutoradioModule {
    func registerMail() {
        // (Sign-out of the mail copy is shared code's: mail.ts `wipeMail`, run by account.ts after the native sign-out; the native key goes in `signOut`.)
        // Open the approval page in the app's own web view like any web link (SourceLink: http and https are in-app; mailto, tel, sms and
        // facetime go to the system; anything else is refused), only on the configured Choir host (same scheme, host and port), over https.
        // Native because the in-app web view is a native view.
        handle("mailOpenReview") { [weak self] args, reply in
            if let u = URL(string: args["url"] as? String ?? ""), u.scheme == "https", RemoteHost.allows(u) {
                DispatchQueue.main.async { [weak self] in
                    self?.barWebURL = u.absoluteString
                    self?.touch(["queueView", "barFacts"], now: true)
                }
                reply(["status": "opened", "message": ""])
            } else {
                reply(["status": "refused", "message": "That link is not on your computer's address."])
            }
            _ = self
        }
    }
}
