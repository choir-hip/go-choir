package choir.autoradio

import java.io.File
import java.nio.file.Files
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.yield
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** The deck's named rules, with no Android player and no wall-clock waits. */
class DeckControllerTest {
    private val dirs = ArrayList<File>()
    private val scopes = ArrayList<CoroutineScope>()

    private fun temp(): File {
        val dir = Files.createTempDirectory("android-deck-test-").toFile()
        dirs += dir
        return dir
    }

    private fun deck(engine: FakeEngine = FakeEngine(), journal: DeckJournal = DeckJournal()): DeckController {
        val scope = CoroutineScope(SupervisorJob() + Dispatchers.Unconfined)
        scopes += scope
        return DeckController(journal, engine, FakeScheduler(), scope).also { engine.host = it }
    }

    private fun speech(id: String, requires: Boolean = false) = DeckItem(id, "speech", paragraphs = "One.\n\nTwo.", requiresPreviousHeard = requires)
    private fun audio(id: String) = DeckItem(id, "audio", url = "https://example.test/$id.mp3", start = 12.0)
    private fun ends(deck: DeckController) = deck.journal.lines.filter { it.type == "ended" }.map { "${it.item}:${it.end}:${it.rendered}" }
    private suspend fun settle() { repeat(8) { yield() } }

    @After
    fun clean() {
        scopes.forEach { it.cancel() }
        dirs.forEach { it.deleteRecursively() }
    }

    @Test fun playReplacesAndLateCallbacksCannotEndTheLatestStart() = runBlocking {
        val engine = FakeEngine()
        val deck = deck(engine)
        assertTrue(deck.play(speech("a")))
        engine.sound(1)
        assertTrue(deck.play(speech("b")))
        assertEquals(listOf("a:replaced:true"), ends(deck))
        engine.host?.onEnded(1, "a")
        engine.host?.onSounding(1, "a", DeckPosition(0, 9.0, 1, 0.0)); settle()
        assertEquals("b", deck.snapshot().item)
        assertEquals(1, deck.journal.lines.count { it.type == "sounding" })
    }

    @Test fun heldOrderDedupeAtTrimAndCapMatchApple() = runBlocking {
        val engine = FakeEngine()
        val deck = deck(engine)
        assertTrue(deck.play(speech("a")))
        deck.enqueue(speech("b")); deck.enqueue(speech("c")); deck.enqueue(speech("d")); deck.enqueue(speech("e"))
        assertEquals(listOf("b", "c", "d", "e"), deck.snapshot().held)
        for (n in 0..104) deck.enqueue(speech("m$n"))
        assertEquals(100, deck.snapshot().held.size)
        deck.trim(0); deck.enqueue(speech("b")); deck.enqueue(speech("c")); deck.enqueue(speech("x"), 1)
        assertEquals(listOf("b", "x"), deck.snapshot().held)
        deck.enqueue(speech("b"))
        assertEquals(listOf("x", "b"), deck.snapshot().held)
        deck.trim(1)
        assertEquals(listOf("x"), deck.snapshot().held)
        engine.sound(1); settle()
        engine.host?.onEnded(1, "a"); settle()
        assertEquals("x", deck.snapshot().item)
        assertEquals("speak:x", engine.calls.last())
    }

    @Test fun aTransitionAfterAnUnheardItemIsDroppedWithoutAJournalLine() = runBlocking {
        val engine = FakeEngine()
        val deck = deck(engine)
        assertTrue(deck.play(speech("a")))
        deck.enqueue(speech("transition", true)); deck.enqueue(speech("next"))
        deck.skip()
        assertEquals("next", deck.snapshot().item)
        assertFalse(deck.journal.lines.any { it.item == "transition" })
        engine.soundCurrent(); settle()
        deck.enqueue(speech("transition-2", true))
        engine.endCurrent(); settle()
        assertEquals("transition-2", deck.snapshot().item)
    }

    @Test fun twoSilentItemsStopAndLeaveTheRestHeld() = runBlocking {
        val engine = FakeEngine(fail = setOf("a", "b"))
        val deck = deck(engine)
        assertFalse(deck.play(speech("a")))
        deck.enqueue(speech("b")); deck.enqueue(speech("c"))
        assertEquals(DeckWhy.STOPPED.wire, deck.snapshot().why)
        assertNull(deck.current)
        assertEquals(listOf("a:failed:false", "b:failed:false"), ends(deck))
        assertEquals(listOf("c"), deck.snapshot().held)
    }

    @Test fun sessionRetryUsesOneTwoFourEightFifteenAndTheLastRepeats() = runBlocking {
        val engine = FakeEngine()
        val scheduler = FakeScheduler()
        val scope = CoroutineScope(SupervisorJob() + Dispatchers.Unconfined); scopes += scope
        var tries = 0
        val deck = DeckController(DeckJournal(), engine, scheduler, scope, sessionReady = { tries += 1; tries >= 7 })
        engine.host = deck
        assertFalse(deck.play(speech("a")))
        assertEquals(DeckWhy.HELD_SESSION.wire, deck.snapshot().why)
        assertTrue(deck.journal.lines.none { it.type == "ended" })
        for (expected in listOf(1000L, 2000L, 4000L, 8000L, 15000L, 15000L)) {
            assertEquals(expected, scheduler.delays.last())
            scheduler.runNext(); settle()
        }
        assertEquals(listOf(1000L, 2000L, 4000L, 8000L, 15000L, 15000L), scheduler.delays)
        assertEquals("a", deck.snapshot().item)
        assertEquals(DeckWhy.PLAYING.wire, deck.snapshot().why)
        assertEquals(7, tries)
        assertEquals(listOf("session-held", "session-back"), deck.journal.lines.filter { it.type == "system" }.map { it.command })
    }

    @Test fun listenerPauseAndNoisyRouteAreNeverUndoneByRegainOrTimer() = runBlocking {
        val engine = FakeEngine()
        val scheduler = FakeScheduler()
        val scope = CoroutineScope(SupervisorJob() + Dispatchers.Unconfined); scopes += scope
        val deck = DeckController(DeckJournal(), engine, scheduler, scope); engine.host = deck
        assertTrue(deck.play(speech("a")))
        deck.pause()
        assertEquals(DeckWhy.PAUSED_LISTENER.wire, deck.snapshot().why)
        deck.systemMayResume(); scheduler.runAll()
        assertEquals(DeckWhy.PAUSED_LISTENER.wire, deck.snapshot().why)
        deck.resumed()
        deck.systemPaused(routeLost = true)
        deck.systemMayResume(); scheduler.runAll()
        assertEquals(DeckWhy.PAUSED_SYSTEM.wire, deck.snapshot().why)
        assertTrue(engine.calls.none { it == "resume" && scheduler.delays.isNotEmpty() })
    }

    @Test fun remoteNextWritesItsLineForSharedCodeAndPreviousRestartsOnlyWhatHasGoneOn() = runBlocking {
        val engine = FakeEngine()
        val deck = deck(engine)
        assertTrue(deck.play(audio("au")))
        deck.enqueue(speech("b")); deck.enqueue(speech("c"))
        engine.position = DeckPosition(0, 40.0, 0, 600.0)
        deck.remotePrevious()
        assertEquals("seek:12.0", engine.calls.last())
        engine.position = DeckPosition(0, 40.0, 0, 600.0)
        deck.remoteNext()
        // A media-session next ends what plays and goes on to an item the deck already holds, in
        // its own engine; the journal line tells shared code what happened. A fetched entry is
        // started by shared code on its next pulse (a call from this native callback would be
        // outside a data answer and its reply let go).
        val remote = deck.journal.lines.filter { it.type == "remote" }
        assertEquals("next", remote.last().command)
        assertEquals(40.0, remote.last().seconds, 0.001)
        assertEquals("b", deck.snapshot().item)
        engine.position = DeckPosition(0, 1.0, 5, 0.0)
        deck.remotePrevious()
        assertFalse(engine.calls.contains("restart"))                       // audio seeks to its start
        assertTrue(deck.play(speech("d")))                                   // a spoken entry restarts from its block
        engine.position = DeckPosition(2, 1.0, 5, 0.0)
        deck.remotePrevious()
        assertTrue(engine.calls.contains("restart"))
        engine.calls.clear()
        engine.position = DeckPosition(0, 1.0, 0, 0.0)
        deck.remotePrevious()
        assertFalse(engine.calls.contains("restart"))                       // at its start: nothing to restart
    }

    @Test fun soundingIsWrittenOnceAndASilentFinishIsFailed() = runBlocking {
        val engine = FakeEngine()
        val deck = deck(engine)
        assertTrue(deck.play(speech("a")))
        engine.host?.onSounding(1, "a", DeckPosition(0, 1.0, 1, 0.0))
        engine.host?.onSounding(1, "a", DeckPosition(0, 2.0, 1, 0.0)); settle()
        assertEquals(1, deck.journal.lines.count { it.type == "sounding" })
        engine.host?.onEnded(1, "a"); settle()
        assertTrue(deck.play(speech("b")))
        engine.endCurrent(); settle()
        assertEquals(listOf("a:finished:true", "b:failed:false"), ends(deck))
    }

    @Test fun wipeClearsCurrentHeldJournalAndCacheAndLateWorkWritesNothing() = runBlocking {
        val engine = FakeEngine()
        val journal = DeckJournal(temp().resolve("deck-journal.json"))
        val deck = deck(engine, journal)
        assertTrue(deck.play(speech("a")))
        engine.sound(1); settle(); deck.enqueue(speech("b"))
        val cache = temp().resolve("cache").also { it.mkdirs() }
        cache.resolve("old.wav").writeText("private words")
        engine.cache = cache
        deck.wipe()
        assertNull(deck.current); assertTrue(deck.held.isEmpty()); assertTrue(deck.journal.lines.isEmpty())
        assertEquals(0, deck.journal.seq); assertFalse(cache.resolve("old.wav").exists())
        engine.host?.onEnded(1, "a"); engine.host?.onSounding(1, "a", DeckPosition(0, 3.0, 1, 0.0)); settle()
        assertTrue(deck.journal.lines.isEmpty())
        assertTrue(deck.play(speech("new-account")))
        assertEquals(1, deck.journal.lines.first().seq)
        assertEquals("new-account", deck.journal.lines.first().item)
    }

    @Test fun aStartSuspendedAcrossWipeCannotSoundOrWriteForTheNextAccount() = runBlocking {
        val engine = FakeEngine()
        val deck = deck(engine)
        val entered = CompletableDeferred<Unit>(); val release = CompletableDeferred<Unit>()
        engine.holdPlay = { entered.complete(Unit); release.await() }
        val playing = deck.playAsync(speech("old"))
        entered.await()
        deck.wipe()
        release.complete(Unit)
        playing.join()
        engine.host?.onSounding(1, "old", DeckPosition(0, 1.0, 1, 0.0))
        assertTrue(deck.journal.lines.isEmpty()); assertNull(deck.current)
        assertTrue(engine.calls.contains("stop"))
    }
}

private class FakeScheduler : DeckScheduler {
    private val tasks = ArrayList<Pair<Long, () -> Unit>>()
    val delays = ArrayList<Long>()
    override fun schedule(delayMs: Long, action: () -> Unit): () -> Unit {
        delays += delayMs; tasks += delayMs to action
        var live = true
        return { live = false; tasks.removeAll { it.second === action } }
    }
    fun runNext() { if (tasks.isNotEmpty()) tasks.removeAt(0).second() }
    fun runAll() { while (tasks.isNotEmpty()) runNext() }
}

private class FakeEngine(var fail: Set<String> = emptySet()) : DeckEngine {
    override var host: DeckEngineHost? = null
    val calls = ArrayList<String>()
    var position = DeckPosition(0, 0.0, 0, 0.0)
    var cache: File? = null
    var holdPlay: (suspend () -> Unit)? = null
    private var generation = 0L
    private var current = ""

    override suspend fun prepareSession(): Boolean = true
    override suspend fun play(item: DeckItem, generation: Long): Boolean {
        holdPlay?.invoke()
        this.generation = generation; current = item.id
        calls += if (item.kind == "audio") "audio:${item.id}" else "speak:${item.id}"
        return item.id !in fail
    }
    override fun pause() { calls += "pause" }
    override fun resume() { calls += "resume" }
    override fun stop() { calls += "stop"; current = "" }
    override fun seekTo(seconds: Double) { calls += "seek:$seconds"; position = position.copy(seconds = seconds) }
    override fun seekBy(seconds: Double) { calls += "seekBy:$seconds"; position = position.copy(seconds = position.seconds + seconds) }
    override fun clearEnd() { calls += "clearEnd" }
    override fun restart() { calls += "restart"; position = DeckPosition(0, 0.0, position.final, position.duration) }
    override fun position(): DeckPosition = position
    override fun release() { calls += "release" }
    override fun wipeCache() { cache?.deleteRecursively() }
    fun sound(generation: Long) { host?.onSounding(generation, current, position) }
    fun soundCurrent() = sound(generation)
    fun endCurrent() { host?.onEnded(generation, current) }
}
