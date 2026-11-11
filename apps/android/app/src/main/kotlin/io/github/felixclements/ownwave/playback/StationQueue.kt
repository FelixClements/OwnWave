package io.github.felixclements.ownwave.playback

/**
 * Keeps a station playing without end: decides when to fetch the station queue again and which of
 * the fetched tracks to append. Pure Kotlin, so it is unit tested without a player.
 */
class StationQueue(
    private val recentLimit: Int = 50,
    private val refillWhenLeft: Int = 3,
) {
    private val recent = ArrayDeque<String>()

    fun reset() = recent.clear()

    /** Remembers a track that started playing, so a refill does not bring it back soon. */
    fun markPlayed(trackId: String) {
        recent.remove(trackId)
        recent.addLast(trackId)
        while (recent.size > recentLimit) recent.removeFirst()
    }

    /** True when the player is [refillWhenLeft] items or fewer from the end of its list. */
    fun needsRefill(currentIndex: Int, itemCount: Int): Boolean =
        itemCount == 0 || itemCount - currentIndex <= refillWhenLeft

    /**
     * The tracks in [fetched] to append, in the server's order: not already [upcoming] and not among
     * the recently played. A small station may have nothing left after that; then recently played
     * tracks are allowed again, so the station still never stops.
     */
    fun <T> fresh(fetched: List<T>, id: (T) -> String, upcoming: Set<String>): List<T> {
        val notQueued = fetched.distinctBy(id).filter { id(it) !in upcoming }
        val unplayed = notQueued.filter { id(it) !in recent }
        return unplayed.ifEmpty { notQueued }
    }
}
