package io.github.felixclements.ownwave.playback

import io.github.felixclements.ownwave.api.models.BrowseItem
import io.github.felixclements.ownwave.api.models.SearchHit
import io.github.felixclements.ownwave.artwork.ArtworkUris

/** One child in the Android Auto browse tree, before it becomes a Media3 MediaItem. */
data class BrowseEntry(
    val mediaId: String,
    val title: String,
    val subtitle: String?,
    val browsable: Boolean,
    val playable: Boolean,
    val artworkUri: String?,
)

/**
 * Maps `/api/browse` nodes one to one onto the Media3 library tree.
 *
 * Node ids are the server's (`root`, `stations`, `recent`, `genres`, `playlists`, `genre:<id>`,
 * `playlist:<id>`); playable items are `station:<id>`, which the player already understands. Results
 * are cached for [ttlMs] because Auto asks for the same node again on every screen change, and
 * unknown node ids are refused locally so they never reach the server as 404s. Pure Kotlin.
 */
class BrowseTree(
    private val ttlMs: Long = DEFAULT_TTL_MS,
    private val now: () -> Long = System::currentTimeMillis,
) {
    private val cache = HashMap<String, Pair<Long, List<BrowseEntry>>>()

    /** The children of [node], from the cache or [fetch]. Failures are not cached. */
    suspend fun children(
        node: String,
        fetch: suspend (String) -> Result<List<BrowseItem>>,
    ): Result<List<BrowseEntry>> {
        if (!isNode(node)) return Result.failure(IllegalArgumentException("Unknown node $node"))
        cache[node]?.let { (at, entries) -> if (now() - at < ttlMs) return Result.success(entries) }
        return fetch(node).map(::entries).onSuccess { cache[node] = now() to it }
    }

    fun clear() = cache.clear()

    companion object {
        const val DEFAULT_TTL_MS = 60_000L

        private val NODE = Regex("root|stations|recent|genres|playlists|genre:[^\\s/]{1,100}|playlist:[0-9a-fA-F-]{36}")

        fun isNode(id: String) = NODE.matches(id)

        /** Items Auto can show: folders, and stations (anything playable must name its station). */
        fun entries(items: List<BrowseItem>): List<BrowseEntry> = items.mapNotNull { item ->
            val station = item.stationId?.let(MediaIds::station)
            when {
                item.playable && station != null -> entry(item, station, playable = true)
                item.browsable && isNode(item.id) -> entry(item, item.id, playable = false)
                else -> null
            }
        }

        private fun entry(item: BrowseItem, mediaId: String, playable: Boolean) = BrowseEntry(
            mediaId = mediaId,
            title = item.title,
            subtitle = item.subtitle,
            browsable = !playable,
            playable = playable,
            artworkUri = ArtworkUris.fromServerPath(item.artworkUrl),
        )

        /** Page [page] of [pageSize] items; Auto may ask for `Int.MAX_VALUE` items, so this cannot overflow. */
        fun <T> page(items: List<T>, page: Int, pageSize: Int): List<T> {
            if (page < 0 || pageSize <= 0) return emptyList()
            val from = page.toLong() * pageSize
            if (from >= items.size) return emptyList()
            return items.subList(from.toInt(), minOf(items.size.toLong(), from + pageSize).toInt())
        }

        /** The station to play for a voice request: the best hit that is a station. */
        fun firstStation(hits: List<SearchHit>): String? =
            hits.firstOrNull { it.playable && it.stationId != null }?.stationId

        /** Search hits as playable entries, best first, each station once (it can match by name and by genre). */
        fun searchEntries(hits: List<SearchHit>): List<BrowseEntry> = hits
            .filter { it.playable && it.stationId != null }
            .distinctBy { it.stationId }
            .map { hit ->
                BrowseEntry(
                    mediaId = MediaIds.station(hit.stationId!!),
                    title = hit.title,
                    subtitle = hit.subtitle,
                    browsable = false,
                    playable = true,
                    artworkUri = ArtworkUris.fromServerPath(hit.artworkUrl),
                )
            }
    }
}
