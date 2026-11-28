package choir.autoradio

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/** The Apple deck's named rules over a replaceable engine. Shared TypeScript still chooses items and posts journal verdicts. */
class DeckController(
    val journal: DeckJournal,
    private val engine: DeckEngine,
    private val scheduler: DeckScheduler,
    private val scope: CoroutineScope,
    private val sessionReady: (suspend () -> Boolean)? = null,
    private val retryDelays: List<Long> = DeckRules.RETRY_DELAYS_MS,
) : DeckEngineHost {
    private val mutex = Mutex()
    var current: DeckItem? = null
        private set
    var held: List<DeckItem> = emptyList()
        private set
    var why: DeckWhy = DeckWhy.EMPTY
        private set

    @Volatile private var revisionValue = 0L
    val revision: Long get() = revisionValue
    var onChange: (() -> Unit)? = null
    private var generation = 0L
    private var sounded = false
    private var silentRun = 0
    private var lastHeard = false
    private var attempts = 0
    private var retryCancel: (() -> Unit)? = null
    private var systemCause = ""
    private var systemMayResume = false

    init {
        engine.host = this
        val before = journal.seq
        journal.closeOpenAtBoot()
        if (journal.seq != before) changed()
    }

    fun snapshot(): DeckState {
        val item = current
        val p = if (item == null) DeckPosition() else engine.position()
        return DeckState(why.wire, item?.id ?: "", p.block, if (item == null) 0.0 else p.seconds, p.duration,
            held.map { it.id }, journal.seq, journal.acked, journal.lines,
            place = if (item == null || item.kind != "speech") DeckPlace() else engine.place().takeIf { it.entry == item.id } ?: DeckPlace())
    }

    fun playAsync(item: DeckItem): Job = scope.launch { play(item) }

    suspend fun play(item: DeckItem): Boolean {
        val mine = mutex.withLock {
            if (current != null) endLocked(DeckEnd.REPLACED)
            silentRun = 0
            startLocked(item)
        }
        return begin(item, mine)
    }

    suspend fun enqueue(item: DeckItem, at: Int = -1) {
        val start = mutex.withLock {
            val next = held.filter { it.id != item.id }.toMutableList()
            if (at in 0 until next.size) while (next.size > at) next.removeAt(next.size - 1)
            if (next.size < DeckRules.HELD_MAX) next += item
            held = next
            val playOn = current == null && why != DeckWhy.STOPPED
            changed()
            playOn
        }
        if (start) playOn()
    }

    suspend fun trim(keeping: Int) {
        mutex.withLock {
            val n = keeping.coerceAtLeast(0)
            if (held.size > n) { held = held.take(n); changed() }
        }
    }

    suspend fun act(op: String, value: Double = 0.0) {
        when (op) {
            "pause" -> pause()
            "resume" -> resumed()
            "stop" -> stop()
            "skip" -> skip()
            "seekTo" -> if (current?.kind == "audio") engine.seekTo(DeckContract.finite(value))
            "seekBy" -> if (current?.kind == "audio") engine.seekBy(DeckContract.finite(value))
            "trim" -> trim(DeckContract.count(value))
            "clearEnd" -> if (current?.kind == "audio") engine.clearEnd()
        }
    }

    suspend fun skip() {
        val had = mutex.withLock {
            if (current == null) false else { endLocked(DeckEnd.SKIPPED); true }
        }
        if (had) engine.stop()
        playOn()
    }

    private suspend fun stop() {
        mutex.withLock {
            if (current != null) endLocked(DeckEnd.STOPPED)
            held = emptyList(); why = DeckWhy.STOPPED; changed()
        }
        engine.stop()
    }

    suspend fun pause() {
        mutex.withLock {
            if (current == null) return@withLock
            engine.pause()
            why = DeckWhy.PAUSED_LISTENER; systemCause = ""; systemMayResume = false
            noteLocked("remote", command = "pause"); changed()
        }
    }

    suspend fun resumed() {
        var held = false; var heldAt = 0L
        val item = mutex.withLock {
            val item = current
            if (item == null) { why = DeckWhy.EMPTY; changed(); null }
            else {
                held = why == DeckWhy.HELD_SESSION || why == DeckWhy.HELD_ENGINE; heldAt = generation
                why = DeckWhy.PLAYING; systemCause = ""; systemMayResume = false
                noteLocked("remote", command = "play"); changed(); item
            }
        }
        if (item == null) { playOn(); return }
        // A session or engine that was never started (HELD_*) must be started from the item, not
        // resumed on an empty player; a live session is resumed.
        if (held) { begin(item, heldAt); return }
        // The service can have let the player go without an item: prepare again, and start if the
        // player is empty (the engine's `play` is the only thing that installs media items).
        val ready = try { engine.prepareSession() } catch (_: Exception) { false }
        if (ready) engine.resume() else begin(item, generation)
    }

    suspend fun systemPaused(routeLost: Boolean = false, transient: Boolean = false) {
        mutex.withLock {
            if (current == null) return@withLock
            engine.pause()
            why = DeckWhy.PAUSED_SYSTEM; systemCause = if (routeLost) "route" else "interruption"; systemMayResume = !routeLost && transient
            noteLocked("system", command = if (routeLost) "route-lost" else "interruption-began"); changed()
        }
    }

    suspend fun systemMayResume() {
        var resumeEngine = false
        val retry = mutex.withLock {
            if (why != DeckWhy.PAUSED_SYSTEM && why != DeckWhy.HELD_SESSION && why != DeckWhy.HELD_ENGINE) return@withLock null
            if (why == DeckWhy.PAUSED_SYSTEM && (!systemMayResume || systemCause != "interruption")) return@withLock null
            noteLocked("system", command = "interruption-ended")
            if (why == DeckWhy.PAUSED_SYSTEM) {
                why = DeckWhy.PLAYING; systemCause = ""; systemMayResume = false; changed(); resumeEngine = true
                return@withLock null
            }
            current?.let { it to generation }
        }
        if (resumeEngine) engine.resume() else retry?.let { (item, mine) -> begin(item, mine) }
    }

    fun remoteCommand(command: String) {
        when (command) {
            "next" -> playAsyncRemote { remoteNext() }
            "previous" -> playAsyncRemote { remotePrevious() }
            "toggle" -> playAsyncRemote { if (why == DeckWhy.PLAYING) pause() else resumed() }
            "play" -> playAsyncRemote { resumed() }
            "pause" -> playAsyncRemote { pause() }
        }
    }

    private fun playAsyncRemote(block: suspend () -> Unit) { scope.launch { block() } }

    suspend fun remoteNext() {
        // The media-session command arrives outside any data answer. The deck ends what plays and
        // goes on to an item it already holds (its own engine, not a shared call); shared code reads
        // the journal line on its next pulse and starts a fetched entry itself (a call made from
        // this native callback would be outside an answer, and its reply would be let go).
        val had = mutex.withLock {
            if (current == null) { noteLocked("remote", command = "next"); changed(); return@withLock false }
            noteLocked("remote", command = "next")
            endLocked(DeckEnd.SKIPPED); changed(); true
        }
        if (had) { engine.stop(); playOn() }
    }

    suspend fun remotePrevious() {
        mutex.withLock {
            val item = current ?: return@withLock
            noteLocked("remote", command = "previous"); changed()
            val p = engine.position()
            when {
                item.kind == "audio" -> engine.seekTo(item.start)
                p.block > 0 || p.seconds >= DeckRules.PREVIOUS_RESTART_SECONDS -> engine.restart()
            }
            changed()
        }
    }

    suspend fun remoteSeek(seconds: Double) {
        mutex.withLock {
            if (current?.kind != "audio") return@withLock
            noteLocked("remote", command = "seek")
            engine.seekTo(DeckContract.finite(seconds)); changed()
        }
    }

    suspend fun ack(through: Int) {
        mutex.withLock {
            val before = journal.acked
            journal.ack(through)
            if (journal.acked != before) changed()
        }
    }

    suspend fun wipe() {
        mutex.withLock {
            retryCancel?.invoke(); retryCancel = null; generation += 1
            current = null; held = emptyList(); why = DeckWhy.EMPTY; sounded = false; silentRun = 0; lastHeard = false; attempts = 0
            systemCause = ""; systemMayResume = false
            journal.wipe(); changed()
        }
        engine.stop(); engine.wipeCache()
    }

    suspend fun release() {
        mutex.withLock { retryCancel?.invoke(); retryCancel = null; generation += 1; current = null; held = emptyList(); why = DeckWhy.EMPTY; changed() }
        engine.stop(); engine.release()
    }

    override fun onSounding(generation: Long, itemID: String, position: DeckPosition) {
        scope.launch {
            mutex.withLock {
                val item = current ?: return@withLock
                if (generation != this@DeckController.generation || item.id != itemID || sounded || why != DeckWhy.PLAYING) return@withLock
                sounded = true
                journal.append("sounding", item.id, position.block, position.seconds, position.final, rendered = true)
                changed()
            }
        }
    }

    override fun onEnded(generation: Long, itemID: String) {
        scope.launch { finishFromEngine(generation, itemID, DeckEnd.FINISHED, played = sounded) }
    }

    override fun onFailed(generation: Long, itemID: String, reason: String) {
        scope.launch { finishFromEngine(generation, itemID, DeckEnd.FAILED, reason = reason) }
    }

    override fun onPausedOutsideDeck(generation: Long, itemID: String) {
        scope.launch { holdFromEngine(generation, itemID) }
    }

    override fun onSystemPause(generation: Long, itemID: String, routeLost: Boolean, transient: Boolean) {
        scope.launch {
            mutex.withLock {
                val item = current ?: return@withLock
                if (generation != this@DeckController.generation || item.id != itemID) return@withLock
                // A route lost while already paused for an interruption is still a lasting cause:
                // the deck must not resume out loud when the interruption ends.
                if (why != DeckWhy.PLAYING && !(routeLost && why == DeckWhy.PAUSED_SYSTEM)) return@withLock
                why = DeckWhy.PAUSED_SYSTEM; systemCause = if (routeLost) "route" else "interruption"; systemMayResume = !routeLost && transient
                noteLocked("system", command = if (routeLost) "route-lost" else "interruption-began")
                changed()
            }
        }
    }

    private suspend fun finishFromEngine(generation: Long, itemID: String, end: DeckEnd, played: Boolean = false, reason: String = "") {
        val finish = mutex.withLock {
            val item = current ?: return@withLock false
            if (generation != this@DeckController.generation || item.id != itemID) return@withLock false
            endLocked(end, played = played, reason = reason)
            true
        }
        if (finish) { engine.stop(); playOn() }
    }

    private suspend fun holdFromEngine(generation: Long, itemID: String) {
        val item = mutex.withLock {
            val item = current ?: return@withLock null
            if (generation != this@DeckController.generation || item.id != itemID) return@withLock null
            holdLocked(item)
            item
        }
        if (item != null) scheduleRetry(item, generation)
    }

    private suspend fun begin(item: DeckItem, mine: Long): Boolean {
        // No caller-supplied gate (the Android engine): the engine's own audio session and media
        // service are prepared before the first item starts, so playback survives the screen.
        val ready = try { sessionReady?.invoke() ?: engine.prepareSession() } catch (_: Exception) { false }
        if (!ready) {
            val retry = mutex.withLock {
                val current = current ?: return@withLock null
                if (mine != generation || current.id != item.id) return@withLock null
                holdLocked(item)
                item
            }
            if (retry != null) scheduleRetry(retry, mine)
            return false
        }
        val live = mutex.withLock {
            if (mine != generation || current?.id != item.id) return@withLock false
            if (why == DeckWhy.HELD_SESSION) noteLocked("system", command = "session-back")
            attempts = 0; why = DeckWhy.PLAYING; changed()
            true
        }
        if (!live) return false
        val ok = try { engine.play(item, mine) } catch (_: Exception) { false }
        if (!ok) finishFromEngine(mine, item.id, DeckEnd.FAILED, reason = "engine-start-failed")
        return ok
    }

    private fun holdLocked(item: DeckItem) {
        if (why != DeckWhy.HELD_SESSION) noteLocked("system", command = "session-held")
        why = DeckWhy.HELD_SESSION; attempts += 1; changed()
    }

    private fun scheduleRetry(item: DeckItem, mine: Long) {
        retryCancel?.invoke()
        val delay = retryDelays[minOf(attempts - 1, retryDelays.lastIndex)]
        retryCancel = scheduler.schedule(delay) {
            retryCancel = null
            scope.launch {
                val retry = mutex.withLock {
                    if (mine != generation || current?.id != item.id || why != DeckWhy.HELD_SESSION) null else item
                }
                if (retry != null) begin(retry, mine)
            }
        }
    }

    private suspend fun playOn() {
        val next = mutex.withLock {
            if (current != null) return@withLock null
            if (silentRun >= 2) { why = DeckWhy.STOPPED; changed(); return@withLock null }
            while (held.isNotEmpty()) {
                val item = held.first(); held = held.drop(1)
                if (item.requiresPreviousHeard && !lastHeard) { changed(); continue }
                changed()
                return@withLock item
            }
            if (why != DeckWhy.STOPPED) noteLocked("system", command = "held-dry")
            why = DeckWhy.EMPTY; changed()
            null
        }
        if (next != null) {
            val mine = mutex.withLock { startLocked(next) }
            begin(next, mine)
        }
    }

    private fun startLocked(item: DeckItem): Long {
        generation += 1
        current = item; sounded = false; why = DeckWhy.PLAYING; attempts = 0; systemCause = ""; systemMayResume = false
        journal.append("started", item.id, item.block, item.seconds)
        changed()
        return generation
    }

    private fun endLocked(end: DeckEnd, played: Boolean = false, reason: String = "") {
        val item = current ?: return
        val p = engine.position()
        val rendered = sounded || played
        val actual = if (end == DeckEnd.FINISHED && !rendered) DeckEnd.FAILED else end
        val fixedReason = if (actual == DeckEnd.FAILED && reason.isEmpty()) "ended with no sound rendered" else reason
        journal.append("ended", item.id, p.block, p.seconds, p.final, rendered, actual.wire, reason = fixedReason)
        current = null; sounded = false; attempts = 0; generation += 1
        retryCancel?.invoke(); retryCancel = null
        lastHeard = actual == DeckEnd.FINISHED && rendered
        silentRun = if (rendered) 0 else silentRun + 1
        if (why != DeckWhy.STOPPED) why = DeckWhy.EMPTY
        changed()
    }

    private fun noteLocked(type: String, command: String) {
        val p = if (current == null) DeckPosition() else engine.position()
        journal.append(type, current?.id ?: "", p.block, p.seconds, command = command)
    }

    private fun changed() { revisionValue += 1; onChange?.invoke() }
}
