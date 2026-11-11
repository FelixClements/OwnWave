package io.github.felixclements.ownwave.playback

import io.github.felixclements.ownwave.api.models.BrowseItem
import io.github.felixclements.ownwave.api.models.SearchHit
import io.github.felixclements.ownwave.artwork.ArtworkUris
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class BrowseTreeTest {
    private val techno = "11111111-1111-1111-1111-111111111111"
    private val trance = "22222222-2222-2222-2222-222222222222"

    private fun station(id: String, title: String) = BrowseItem(
        id = "station:$id", title = title, browsable = false, playable = true,
        subtitle = "Techno", stationId = id, trackCount = 10, artworkUrl = "/api/stations/$id/cover",
    )

    private fun hit(id: String?, playable: Boolean = true) = SearchHit(
        id = "station:$id", title = "Hit $id", browsable = false, playable = playable,
        reason = SearchHit.Reason.entries.first(), stationId = id,
    )

    @Test fun `stations are playable with content artwork, folders are browsable`() {
        val entries = BrowseTree.entries(
            listOf(
                station(techno, "Techno"),
                BrowseItem(id = "genre:techno", title = "Techno", browsable = true, playable = false, subtitle = "2 stations"),
            ),
        )
        assertEquals(
            BrowseEntry("station:$techno", "Techno", "Techno", browsable = false, playable = true, ArtworkUris.station(techno)),
            entries[0],
        )
        assertEquals(BrowseEntry("genre:techno", "Techno", "2 stations", browsable = true, playable = false, null), entries[1])
    }

    @Test fun `items that cannot be browsed or played are dropped`() {
        val entries = BrowseTree.entries(
            listOf(
                BrowseItem(id = "station:x", title = "No station id", browsable = false, playable = true),
                BrowseItem(id = "../admin", title = "Bad node", browsable = true, playable = false),
            ),
        )
        assertTrue(entries.isEmpty())
    }

    @Test fun `only known node ids are fetched`() = runTest {
        val tree = BrowseTree()
        var fetches = 0
        val fetch: suspend (String) -> Result<List<BrowseItem>> = { fetches++; Result.success(emptyList()) }
        for (node in listOf("root", "stations", "recent", "genres", "playlists", "genre:techno", "playlist:$techno")) {
            assertTrue(node, tree.children(node, fetch).isSuccess)
        }
        for (node in listOf("station:$techno", "../admin", "playlist:not-a-uuid", "genre:", "")) {
            assertTrue(node, tree.children(node, fetch).isFailure)
        }
        assertEquals(7, fetches)
    }

    @Test fun `children are cached until the ttl, failures are not`() = runTest {
        var now = 0L
        val tree = BrowseTree(ttlMs = 1_000, now = { now })
        var fetches = 0
        var fail = true
        val fetch: suspend (String) -> Result<List<BrowseItem>> = {
            fetches++
            if (fail) Result.failure(IOException("down")) else Result.success(listOf(station(techno, "Techno")))
        }
        assertTrue(tree.children("stations", fetch).isFailure)
        fail = false
        assertEquals(1, tree.children("stations", fetch).getOrThrow().size)
        now = 999
        tree.children("stations", fetch)
        assertEquals(2, fetches)
        now = 1_000
        tree.children("stations", fetch)
        assertEquals(3, fetches)
        tree.clear()
        tree.children("stations", fetch)
        assertEquals(4, fetches)
    }

    @Test fun `paging never overflows`() {
        val items = (1..5).toList()
        assertEquals(listOf(1, 2), BrowseTree.page(items, 0, 2))
        assertEquals(listOf(5), BrowseTree.page(items, 2, 2))
        assertEquals(emptyList<Int>(), BrowseTree.page(items, 3, 2))
        assertEquals(items, BrowseTree.page(items, 0, Int.MAX_VALUE))
        assertEquals(emptyList<Int>(), BrowseTree.page(items, 1, Int.MAX_VALUE))
        assertEquals(emptyList<Int>(), BrowseTree.page(items, -1, 2))
        assertEquals(emptyList<Int>(), BrowseTree.page(items, 0, 0))
    }

    @Test fun `voice plays the best station hit`() {
        assertEquals(trance, BrowseTree.firstStation(listOf(hit(null), hit(techno, playable = false), hit(trance), hit(techno))))
        assertNull(BrowseTree.firstStation(emptyList()))
    }

    @Test fun `search results list each station once`() {
        val entries = BrowseTree.searchEntries(listOf(hit(techno, playable = false), hit(trance), hit(techno), hit(trance)))
        assertEquals(listOf("station:$trance", "station:$techno"), entries.map { it.mediaId })
        assertFalse(entries.any { it.browsable })
    }
}
