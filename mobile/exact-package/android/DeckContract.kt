package choir.autoradio

/** The stable package contract. Shared TypeScript owns these names; Kotlin only implements the native audio surface. */
object DeckContract {
    const val STATE = "deckState"
    const val PLAY = "deckPlay"
    const val ENQUEUE = "deckEnqueue"
    const val ACT = "deckAct"
    const val ACK = "deckAck"
    const val WIPE = "deckWipe"
    const val TOPIC = "autoradio.deckState"
    val names = listOf(STATE, PLAY, ENQUEUE, ACT, ACK, WIPE)

    fun finite(value: Double, fallback: Double = 0.0): Double = if (value.isFinite()) value else fallback
    fun count(value: Double): Int = finite(value).toInt().coerceAtLeast(0)
}
