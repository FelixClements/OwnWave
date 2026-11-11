package io.github.felixclements.ownwave.playback

import kotlin.math.min

/**
 * Decides when a play counts: after 30 s, or half the track if it is shorter (the web player's rule).
 * Each item is reported once, until another item has been reported.
 */
class PlayReporter(private val countsAfterMs: Long = 30_000) {
    private var reported: String? = null

    /** True exactly once per [itemKey], when [positionMs] has passed the threshold. */
    fun shouldReport(itemKey: String, positionMs: Long, durationMs: Long): Boolean {
        if (itemKey == reported) return false
        val threshold = if (durationMs > 0) min(countsAfterMs, durationMs / 2) else countsAfterMs
        if (positionMs < threshold) return false
        reported = itemKey
        return true
    }

    /** Lets the same item count again, e.g. when a station is started over. */
    fun reset() {
        reported = null
    }
}
