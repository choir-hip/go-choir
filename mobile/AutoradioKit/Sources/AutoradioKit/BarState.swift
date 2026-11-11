import Foundation

// The bar's state machine as data (plan section 1). The spine derives the state from what the radio already
// reports; lane 1 owns this file and fills in the rest (heard, holding, revised, typing, error timings).
//   idle | playing | listening | heard | holding | revised | typing | error

public struct BarInfo: Codable, Sendable, Equatable {
    public var state = "idle"
    /// What the centre shows for every state but idle (idle shows the aim, which the contract builds from the page).
    public var text = ""
    public var playing = false
    public var talking = false
    public var signedIn = false
    /// 0...1 along the current story; the hairline rule on the bar's lower edge.
    public var progress = 0.0
    public init() {}
}

public enum BarState {
    public static func info(player p: ChoirPlayerView, snapshot s: LabSnapshot) -> BarInfo {
        var b = BarInfo()
        b.signedIn = p.signedIn; b.playing = p.playing; b.talking = p.talking
        if s.blockCount > 0 { b.progress = Double(s.blockIndex + 1) / Double(s.blockCount) }
        if !p.message.isEmpty, p.message != "Pick a story below." , p.message.hasPrefix("Could") || p.message.hasPrefix("Your") || p.message.hasPrefix("Something") {
            b.state = "error"; b.text = p.message; return b
        }
        switch s.phase {
        case "playing": break
        case "candidate", "addressed", "capturing": b.state = "listening"; b.text = p.heard.isEmpty ? "Listening..." : p.heard; return b
        case "ended", "acknowledging", "holding": b.state = "holding"; b.text = p.heard.isEmpty ? "On it..." : p.heard; return b
        case "revised": b.state = "revised"; b.text = "Revised"; return b
        default: break
        }
        if p.playing { b.state = "playing"; b.text = p.title }
        return b
    }
}
