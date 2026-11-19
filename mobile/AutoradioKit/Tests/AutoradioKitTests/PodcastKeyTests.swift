import XCTest
@testable import AutoradioKit

/// The names native code still uses for the podcast app must agree with the shared code's (AutoradioLab/podcast-model.ts; podcast.test.ts pins the same strings).
final class PodcastKeyTests: XCTestCase {
    func testTheFeedKeyAndTheEpisodeKeyAgreeWithTheSharedCode() {
        XCTAssertEqual(PodcastKey.feed("HTTPS://Feeds.Example.com/show.xml/"), "https://feeds.example.com/show.xml")
        XCTAssertEqual(PodcastKey.episode(feed: "https://f.example.com/a", id: "g/1"), "pod:https%3A%2F%2Ff.example.com%2Fa/g%2F1")
        XCTAssertEqual(PodcastKey.parseEpisode(PodcastKey.episode(feed: "https://f.example.com/a", id: "g/1"))?.id, "g/1")
        XCTAssertNil(PodcastKey.parseEpisode("not-a-key"))
    }
    func testTheEpisodePageAddressCarriesTheClipsAudioAndStart() {
        XCTAssertEqual(PodcastKey.episodeAddress(feed: "https://f.example.com/a", id: "g", enclosure: "https://c.example.com/x.mp3", at: 12.5),
                       "podcast:episode/https%3A%2F%2Ff.example.com%2Fa/g?audio=https%3A%2F%2Fc.example.com%2Fx.mp3&at=12.5")
        XCTAssertEqual(PodcastKey.episodeAddress(feed: "https://f.example.com/a", id: "g", enclosure: "", at: 0), "podcast:episode/https%3A%2F%2Ff.example.com%2Fa/g")
    }
}
