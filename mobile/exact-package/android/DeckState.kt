package choir.autoradio

/** The scalar wire shapes in interface/deck.json. No title, text or address is ever copied into the journal. */
data class DeckItem(
    val id: String,
    val kind: String,
    val title: String = "",
    val show: String = "",
    val document: String = "",
    val paragraphs: String = "",
    val url: String = "",
    val start: Double = 0.0,
    val end: Double = 0.0,
    val block: Int = 0,
    val seconds: Double = 0.0,
    val requiresPreviousHeard: Boolean = false,
    val segment: String = "",
) {
    val paragraphList: List<String>
        get() = paragraphs.split("\n\n").map { it.trim() }.filter { it.isNotEmpty() }
    val documentID: String get() = if (document.isEmpty()) "deck:$id" else document
}

data class DeckLine(
    val seq: Int,
    val type: String,
    val item: String,
    val at: String,
    val block: Int = 0,
    val seconds: Double = 0.0,
    val final: Int = 0,
    val rendered: Boolean = false,
    val end: String = "",
    val command: String = "",
    val reason: String = "",
)

data class DeckState(
    val why: String,
    val item: String,
    val block: Int,
    val seconds: Double,
    val duration: Double,
    val held: List<String>,
    val seq: Int,
    val acked: Int,
    val journal: List<DeckLine>,
)

data class DeckPosition(val block: Int = 0, val seconds: Double = 0.0, val final: Int = 0, val duration: Double = 0.0)

enum class DeckWhy(val wire: String) {
    PLAYING("playing"),
    PAUSED_LISTENER("paused_listener"),
    PAUSED_SYSTEM("paused_system"),
    HELD_SESSION("held_session"),
    HELD_ENGINE("held_engine"),
    STOPPED("stopped"),
    EMPTY("empty"),
}

enum class DeckEnd(val wire: String) {
    FINISHED("finished"), SKIPPED("skipped"), REPLACED("replaced"), STOPPED("stopped"), FAILED("failed"),
}

object DeckRules {
    const val HELD_MAX = 100
    const val JOURNAL_MAX = 2000
    val RETRY_DELAYS_MS = listOf(1_000L, 2_000L, 4_000L, 8_000L, 15_000L)
    const val PREVIOUS_RESTART_SECONDS = 3.0
}
