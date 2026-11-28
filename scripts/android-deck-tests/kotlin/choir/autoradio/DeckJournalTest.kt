package choir.autoradio

import java.io.File
import java.nio.file.Files
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class DeckJournalTest {
    private val dirs = ArrayList<File>()
    private fun dir(): File = Files.createTempDirectory("deck-journal-test-").toFile().also { dirs += it }
    private fun file(): File = dir().resolve("deck-journal.json")

    @After fun clean() { dirs.forEach { it.deleteRecursively() } }

    @Test fun numbersNeverRepeatAcrossAcknowledgementAndRelaunch() {
        val f = file()
        val j = DeckJournal(f)
        j.append("started", item = "a"); j.append("ended", item = "a", end = "skipped")
        j.ack(2)
        assertTrue(j.lines.isEmpty()); assertEquals(2, j.seq); assertEquals(2, j.acked)
        val next = DeckJournal(f)
        assertEquals(3, next.append("started", item = "b").seq)
        next.ack(1); assertEquals(2, next.acked)
        assertEquals(listOf(3), DeckJournal(f).lines.map { it.seq })
    }

    @Test fun theNumbersFileSurvivesALostJournal() {
        val f = file(); val j = DeckJournal(f)
        for (n in 1..5) j.append("started", item = "i$n")
        assertTrue(f.delete())
        val next = DeckJournal(f)
        assertEquals(5, next.seq); assertEquals(6, next.append("started", item = "x").seq)
    }

    @Test fun bootClosesAnOpenItemOnceWithItsLastSoundingPlace() {
        val f = file(); val j = DeckJournal(f)
        j.append("started", item = "a")
        j.append("sounding", item = "a", block = 2, seconds = 8.0, final = 4, rendered = true)
        val ended = DeckJournal(f).closeOpenAtBoot()
        assertEquals("failed", ended?.end); assertEquals("interrupted by a restart", ended?.reason)
        assertEquals(2, ended?.block); assertEquals(8.0, ended?.seconds ?: 0.0, 0.001)
        assertTrue(ended?.rendered == true)
        assertNull(DeckJournal(f).closeOpenAtBoot())
    }

    @Test fun unreadableIsNotOverwrittenAndHeldLinesAreMergedAfterItReads() {
        val f = file(); val j = DeckJournal(f)
        j.append("started", item = "a"); j.append("ended", item = "a", end = "skipped")
        val good = f.readBytes()
        assertTrue(f.delete()); assertTrue(f.mkdir()) // a directory exists and cannot be read as the journal
        val held = DeckJournal(f)
        assertFalse(held.isLoaded)
        held.append("started", item = "b")
        assertTrue(f.isDirectory)
        assertTrue(f.delete()); f.writeBytes(good)
        val line = held.append("ended", item = "b", end = "skipped")
        assertTrue(held.isLoaded)
        assertEquals(listOf("started:a", "ended:a", "started:b", "ended:b"), held.lines.map { "${it.type}:${it.item}" })
        assertEquals(listOf(1, 2, 3, 4), held.lines.map { it.seq }); assertEquals(4, line.seq)
    }

    @Test fun anUnreadableLaunchStillClosesTheKilledItemWhenTheFileReads() {
        val f = file(); val j = DeckJournal(f); j.append("started", item = "a")
        val good = f.readBytes(); assertTrue(f.delete()); assertTrue(f.mkdir())
        val next = DeckJournal(f); assertNull(next.closeOpenAtBoot()); assertFalse(next.isLoaded)
        assertTrue(f.delete()); f.writeBytes(good)
        next.append("remote", item = "", command = "play")
        assertEquals(listOf("a:failed:interrupted by a restart"), next.lines.filter { it.type == "ended" }.map { "${it.item}:${it.end}:${it.reason}" })
    }

    @Test fun corruptIsMovedAsideAndTheNumbersCarryOn() {
        val f = file(); val j = DeckJournal(f)
        for (n in 1..3) j.append("started", item = "i$n")
        f.writeText("not json")
        val next = DeckJournal(f)
        assertTrue(next.isLoaded); assertEquals("not json", f.resolveSibling("deck-journal.bad").readText())
        assertEquals(4, next.lines.first().seq)
        assertEquals("journal-corrupt", next.lines.first().command)
        assertEquals(5, next.append("started", item = "x").seq)
    }

    @Test fun aSetAsideJournalGoesAtSignOut() {
        val f = file(); val bad = f.resolveSibling("deck-journal.bad")
        f.writeText("previous account deck entries")
        val j = DeckJournal(f)   // the real corrupt-journal path: setAside moves the file to .bad
        assertEquals("previous account deck entries", bad.readText())
        j.wipe()
        assertFalse("the set-aside journal holds the old account's entries and goes with the wipe", bad.exists())
    }

    @Test fun failedSaveIsCountedAndTheNextEventWritesEverything() {
        val f = File(dir(), "missing/deck-journal.json")
        val j = DeckJournal(f); val told = ArrayList<String>(); j.log = { told += it }
        j.append("started", item = "a")
        assertEquals(1, j.saveFailures); assertTrue(told.single().contains("save failed"))
        assertTrue(f.parentFile.mkdirs())
        j.append("ended", item = "a", end = "skipped")
        assertEquals(2, DeckJournal(f).lines.size)
    }

    @Test fun capKeepsTheNewestTwoThousandAndMarksTheDroppedRange() {
        val j = DeckJournal()
        for (n in 0..2004) j.append("sounding", item = "i$n")
        assertTrue(j.lines.size <= 2000)
        assertEquals(1, j.lines.count { it.command == "dropped" })
        assertTrue(j.dropped > 0); assertTrue(j.lines.first { it.command == "dropped" }.reason.startsWith("dropped "))
        assertEquals(j.lines.size, j.lines.map { it.seq }.toSet().size)
    }

    @Test fun acknowledgedLinesArePrunedButAckNeverGoesBack() {
        val f = file(); val j = DeckJournal(f)
        for (n in 1..3) j.append("started", item = "i$n")
        j.ack(2); j.ack(1)
        assertEquals(2, j.acked); assertEquals(listOf(3), j.lines.map { it.seq })
    }

    @Test fun journalHoldsIdsPlacesAndFixedReasonsNeverTitlesTextOrAddresses() {
        val f = file(); val j = DeckJournal(f)
        j.append("started", item = "entry-1", seconds = 2.0)
        j.append("ended", item = "entry-1", end = "failed", reason = "engine-error")
        val text = f.readText()
        assertFalse(text.contains("https://")); assertFalse(text.contains("paragraph")); assertFalse(text.contains("title"))
        assertTrue(text.contains("entry-1")); assertTrue(text.contains("engine-error"))
    }
}
