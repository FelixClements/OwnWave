package io.github.felixclements.ownwave.playback

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class StationQueueTest {
    private val queue = StationQueue(recentLimit = 3, refillWhenLeft = 3)

    @Test fun `refills when three or fewer items are left`() {
        assertFalse(queue.needsRefill(currentIndex = 0, itemCount = 10))
        assertFalse(queue.needsRefill(currentIndex = 6, itemCount = 10))
        assertTrue(queue.needsRefill(currentIndex = 7, itemCount = 10))
        assertTrue(queue.needsRefill(currentIndex = 9, itemCount = 10))
        assertTrue(queue.needsRefill(currentIndex = -1, itemCount = 0))
    }

    @Test fun `fresh skips upcoming and recently played, keeps server order`() {
        queue.markPlayed("a")
        val fresh = queue.fresh(listOf("a", "b", "c", "d", "c"), { it }, upcoming = setOf("b"))
        assertEquals(listOf("c", "d"), fresh)
    }

    @Test fun `only the last recentLimit plays count as recent`() {
        listOf("a", "b", "c", "d").forEach(queue::markPlayed)
        assertEquals(listOf("a", "e"), queue.fresh(listOf("a", "b", "c", "d", "e"), { it }, emptySet()))
    }

    @Test fun `a small station repeats recent tracks rather than stop`() {
        listOf("a", "b").forEach(queue::markPlayed)
        assertEquals(listOf("a"), queue.fresh(listOf("a", "b"), { it }, upcoming = setOf("b")))
    }

    @Test fun `reset forgets recent plays`() {
        queue.markPlayed("a")
        queue.reset()
        assertEquals(listOf("a"), queue.fresh(listOf("a"), { it }, emptySet()))
    }
}
