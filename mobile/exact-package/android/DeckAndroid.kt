package choir.autoradio

import android.os.Handler
import android.os.Looper
import dev.exact.canvas.ExtensionContext
import java.io.File
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import org.json.JSONArray
import org.json.JSONObject

/** Process-wide Android deck. Boot closes a killed item before the first state reply; position reads do not publish change notifications. */
object DeckAndroid {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private val scheduler = HandlerScheduler(Handler(Looper.getMainLooper()))
    @Volatile private var live: Controller? = null

    private class Controller(val context: ExtensionContext) {
        lateinit var deck: DeckController
        var published = 0L
        fun publish() {
            val revision = deck.revision
            if (revision == published) return
            published = revision
            context.changed(DeckContract.TOPIC)
        }
    }

    fun controller(context: ExtensionContext): DeckController {
        live?.let { return it.deck }
        return synchronized(this) {
            (live ?: make(context).also { live = it }).deck
        }
    }

    private fun make(context: ExtensionContext): Controller {
        lateinit var deck: DeckController
        val engine = DeckMediaEngine(context,
            remote = { command, seconds -> scope.launch { if (seconds == null) deck.remoteCommand(command) else deck.remoteSeek(seconds) } },
            focusRegained = { scope.launch { deck.systemMayResume() } })
        deck = DeckController(DeckJournal(File(context.data, "deck-journal.json")), engine, scheduler, scope)
        return Controller(context).also { holder ->
            holder.deck = deck
            holder.published = deck.revision
            deck.onChange = holder::publish
        }
    }

    fun state(context: ExtensionContext): JSONObject = state(controller(context).snapshot())
    suspend fun play(context: ExtensionContext, args: JSONObject): JSONObject = controller(context).let { it.play(item(args.getJSONObject("item"))); state(it.snapshot()) }
    suspend fun enqueue(context: ExtensionContext, args: JSONObject): JSONObject = controller(context).let { it.enqueue(item(args.getJSONObject("item")), args.optInt("at", -1)); state(it.snapshot()) }
    suspend fun act(context: ExtensionContext, args: JSONObject): JSONObject = controller(context).let { it.act(args.optString("op"), args.optDouble("value", 0.0)); state(it.snapshot()) }
    suspend fun ack(context: ExtensionContext, args: JSONObject): JSONObject = controller(context).let { it.ack(args.optInt("through", 0)); state(it.snapshot()) }
    suspend fun wipe(context: ExtensionContext): JSONObject = controller(context).let { it.wipe(); state(it.snapshot()) }
    suspend fun destroy(context: ExtensionContext) {
        val holder = synchronized(this) { live.also { live = null } }
        if (holder != null && holder.context === context) holder.deck.release()
    }

    private fun item(args: JSONObject): DeckItem = DeckItem(
        args.optString("id"), if (args.optString("kind") == "audio") "audio" else "speech",
        args.optString("title"), args.optString("show"), args.optString("document"), args.optString("paragraphs"), args.optString("url"),
        args.optDouble("start", 0.0), args.optDouble("end", 0.0), args.optInt("block", 0), args.optDouble("seconds", 0.0),
        args.optBoolean("requiresPreviousHeard", false), args.optString("segment"),
    )

    private fun state(s: DeckState): JSONObject = JSONObject().put("why", s.why).put("item", s.item)
        .put("block", s.block).put("seconds", s.seconds.numberOrNull()).put("duration", s.duration.numberOrNull())
        .put("place", JSONObject().put("entry", s.place.entry).put("digest", s.place.digest).put("start", s.place.start).put("length", s.place.length)
            .put("seconds", s.place.seconds.numberOrNull()).put("total", s.place.total.numberOrNull()))
        .put("held", JSONArray(s.held)).put("seq", s.seq).put("acked", s.acked)
        .put("journal", JSONArray(s.journal.map {
            JSONObject().put("seq", it.seq).put("type", it.type).put("item", it.item).put("at", it.at)
                .put("block", it.block).put("seconds", it.seconds.numberOrNull()).put("final", it.final)
                .put("rendered", it.rendered).put("end", it.end).put("command", it.command).put("reason", it.reason)
        }))

    private fun Double.numberOrNull(): Any = if (isFinite()) this else JSONObject.NULL
}

private class HandlerScheduler(private val handler: Handler) : DeckScheduler {
    override fun schedule(delayMs: Long, action: () -> Unit): () -> Unit {
        val run = Runnable(action)
        if (delayMs <= 0) handler.post(run) else handler.postDelayed(run, delayMs)
        return { handler.removeCallbacks(run) }
    }
}
