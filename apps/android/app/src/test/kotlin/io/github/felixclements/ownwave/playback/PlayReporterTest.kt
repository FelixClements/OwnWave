package io.github.felixclements.ownwave.playback

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PlayReporterTest {
    private val reporter = PlayReporter()

    @Test fun `long track counts after 30 seconds`() {
        assertFalse(reporter.shouldReport("a", positionMs = 29_999, durationMs = 240_000))
        assertTrue(reporter.shouldReport("a", positionMs = 30_000, durationMs = 240_000))
    }

    @Test fun `short track counts after half its length`() {
        assertFalse(reporter.shouldReport("a", positionMs = 9_999, durationMs = 20_000))
        assertTrue(reporter.shouldReport("a", positionMs = 10_000, durationMs = 20_000))
    }

    @Test fun `unknown duration uses 30 seconds`() {
        assertFalse(reporter.shouldReport("a", positionMs = 20_000, durationMs = 0))
        assertTrue(reporter.shouldReport("a", positionMs = 30_000, durationMs = 0))
    }

    @Test fun `each item is reported once`() {
        assertTrue(reporter.shouldReport("a", 31_000, 240_000))
        assertFalse(reporter.shouldReport("a", 60_000, 240_000))
        assertTrue(reporter.shouldReport("b", 31_000, 240_000))
    }

    @Test fun `reset lets the same item count again`() {
        assertTrue(reporter.shouldReport("a", 31_000, 240_000))
        reporter.reset()
        assertTrue(reporter.shouldReport("a", 31_000, 240_000))
    }
}
