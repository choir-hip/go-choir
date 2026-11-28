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
    val place: DeckPlace = DeckPlace(),
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

/**
 * Where the voice is in the entry's text, as the deck reports it (interface/deck.json `DeckPlace`; shared code makes the sentence from it: AutoradioLab/radio-follow.ts).
 * `entry` is the id the deck was given for the item it speaks, "" for no place; `digest` is [DeckDigest.of] the item's `paragraphs` as received; `start` and `length` are the
 * unit being spoken, in UTF-16 code units of that string; `seconds` and `total` are media seconds in the unit and its length (0 when not known yet).
 */
data class DeckPlace(
    val entry: String = "",
    val digest: String = "",
    val start: Int = 0,
    val length: Int = 0,
    val seconds: Double = 0.0,
    val total: Double = 0.0,
)

/** One unit of speech: the paragraph (block) it is of, its words, and its range in the item's `paragraphs` string (UTF-16 code units, which is what a Kotlin String indexes). */
data class SpeechPart(val block: Int, val text: String, val start: Int, val length: Int)

/** The digest of an entry's text, and the units it is cut into with their ranges. Voice: the cut is the engine's one-utterance bound, so only native knows where each unit begins. */
object DeckDigest {
    /** The length in UTF-16 code units, a colon, and the 32-bit FNV-1a of those code units in eight hex digits: the same string as radio-follow.ts `digestOf` and the iPhone's `DeckDigest.of`. */
    fun of(text: String): String {
        var h = 0x811c9dc5.toInt()
        for (c in text) h = (h xor c.code) * 0x01000193
        return "${text.length}:" + h.toUInt().toString(16).padStart(8, '0')
    }
}

object SpeechRanges {
    /**
     * The speech units of `paragraphs` (paragraphs split on a blank line, each trimmed, empty ones dropped: [DeckItem.paragraphList]) from paragraph `fromBlock` on, each paragraph cut into parts of at most `max`
     * characters at a space or a stop. Every part carries its range in `paragraphs`, made as it is cut, not found again.
     */
    fun parts(paragraphs: String, fromBlock: Int, max: Int): List<SpeechPart> {
        val out = ArrayList<SpeechPart>()
        var block = 0
        var at = 0
        for (raw in paragraphs.split("\n\n")) {
            val base = at
            at += raw.length + 2
            val first = raw.indexOfFirst { !it.isWhitespace() }
            if (first < 0) continue
            val text = raw.substring(first, raw.indexOfLast { !it.isWhitespace() } + 1)
            val here = block++
            if (here < fromBlock.coerceAtLeast(0)) continue
            split(text, max).forEach { (s, t) -> out += SpeechPart(here, t, base + first + s, t.length) }
        }
        return out
    }

    /** `text` cut into parts no longer than `max`, as (offset in `text`, part). The cut is at the last space, else the last stop, else the bound; the part is trimmed and the rest loses its leading space. */
    fun split(text: String, max: Int): List<Pair<Int, String>> {
        if (text.length <= max) return listOf(0 to text)
        val out = ArrayList<Pair<Int, String>>()
        var from = 0
        while (text.length - from > max) {
            val rest = text.substring(from)
            var cut = rest.lastIndexOf(' ', max)
            if (cut < max / 2) cut = rest.lastIndexOf('.', max)
            if (cut < max / 2) cut = max
            val part = rest.substring(0, cut)
            val lead = part.indexOfFirst { !it.isWhitespace() }.coerceAtLeast(0)
            out += (from + lead) to part.trim()
            val next = rest.substring(cut)
            from += cut + (next.length - next.trimStart().length)
        }
        if (from < text.length) out += from to text.substring(from)
        return out
    }
}

/**
 * The place for the chunk now sounding. `index` is the player's current media item index, bounded to the chunks made so far (the list grows while synthesis runs); an empty list is no place.
 * `positionMs` is the player's position in that chunk; `durationMs` is its duration, which Media3 reports as a large negative number (C.TIME_UNSET) until it is known: that is 0 here.
 */
fun placeOf(entry: String, digest: String, parts: List<SpeechPart>, index: Int, positionMs: Long, durationMs: Long): DeckPlace {
    if (entry.isEmpty() || digest.isEmpty() || parts.isEmpty()) return DeckPlace()
    val p = parts[index.coerceIn(0, parts.lastIndex)]
    return DeckPlace(entry, digest, p.start, p.length, positionMs.coerceAtLeast(0) / 1000.0, if (durationMs > 0) durationMs / 1000.0 else 0.0)
}
