package choir.autoradio

/** What the deck needs from an audio engine. Every callback names the start generation it belongs to. */
interface DeckEngine {
    var host: DeckEngineHost?
    suspend fun prepareSession(): Boolean
    suspend fun play(item: DeckItem, generation: Long): Boolean
    fun pause()
    fun resume()
    fun stop()
    fun seekTo(seconds: Double)
    fun seekBy(seconds: Double)
    fun clearEnd()
    fun restart()
    fun position(): DeckPosition
    fun release()
    fun wipeCache()
}

interface DeckEngineHost {
    /** Real rendered evidence: Media3's audio position advanced, not merely prepare/play returning. */
    fun onSounding(generation: Long, itemID: String, position: DeckPosition)
    fun onEnded(generation: Long, itemID: String)
    fun onFailed(generation: Long, itemID: String, reason: String)
    /** The service or another player paused outside a deck command. The item stays open and the session is retried. */
    fun onPausedOutsideDeck(generation: Long, itemID: String)
    /** Audio focus or an unsuitable output paused the player. A noisy route is never resumed automatically. */
    fun onSystemPause(generation: Long, itemID: String, routeLost: Boolean, transient: Boolean) {
        onPausedOutsideDeck(generation, itemID)
    }
}

/** A deterministic retry clock in tests; a main-looper Handler in the app. */
interface DeckScheduler {
    fun schedule(delayMs: Long, action: () -> Unit): () -> Unit
}

