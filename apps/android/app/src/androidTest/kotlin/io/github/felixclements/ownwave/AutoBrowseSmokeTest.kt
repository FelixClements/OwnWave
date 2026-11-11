package io.github.felixclements.ownwave

import android.content.ComponentName
import android.media.MediaMetadata
import android.media.browse.MediaBrowser
import android.media.session.MediaController
import android.media.session.PlaybackState
import android.os.Bundle
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import io.github.felixclements.ownwave.artwork.ArtworkUris
import io.github.felixclements.ownwave.playback.PlaybackService
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import java.util.concurrent.CompletableFuture
import java.util.concurrent.TimeUnit

/**
 * Talks to [PlaybackService] the way Android Auto does: through the platform [MediaBrowser]
 * (`android.media.browse.MediaBrowserService`), against the real server.
 *
 * Manual only, on a device where the app is signed in (it is skipped when signed out). Do not use
 * `connectedDebugAndroidTest`: it uninstalls the app afterwards, which deletes the sign-in. Instead:
 *
 * ```
 * ./gradlew :app:installDebug :app:installDebugAndroidTest
 * adb shell am instrument -w io.github.felixclements.ownwave.test/androidx.test.runner.AndroidJUnitRunner
 * ```
 *
 * The voice test plays a station for a few seconds and pauses it before a play is recorded (30 s).
 */
@RunWith(AndroidJUnit4::class)
class AutoBrowseSmokeTest {
    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val context = instrumentation.targetContext
    private lateinit var browser: MediaBrowser

    @Before fun connect() {
        val deadline = System.currentTimeMillis() + 5_000
        while (context.container.sessionStore.current == null && System.currentTimeMillis() < deadline) Thread.sleep(100)
        assumeTrue("Sign in on the device first", context.container.sessionStore.current != null)

        val connected = CompletableFuture<Unit>()
        instrumentation.runOnMainSync {
            browser = MediaBrowser(
                context,
                ComponentName(context, PlaybackService::class.java),
                object : MediaBrowser.ConnectionCallback() {
                    override fun onConnected() { connected.complete(Unit) }
                    override fun onConnectionFailed() { connected.completeExceptionally(IllegalStateException("refused")) }
                },
                null,
            )
            browser.connect()
        }
        connected.get(10, TimeUnit.SECONDS)
    }

    @After fun disconnect() {
        if (::browser.isInitialized) instrumentation.runOnMainSync { browser.disconnect() }
    }

    private fun children(parentId: String): List<MediaBrowser.MediaItem> {
        val result = CompletableFuture<List<MediaBrowser.MediaItem>>()
        instrumentation.runOnMainSync {
            browser.subscribe(
                parentId,
                object : MediaBrowser.SubscriptionCallback() {
                    override fun onChildrenLoaded(parentId: String, children: MutableList<MediaBrowser.MediaItem>) {
                        result.complete(children)
                    }
                    override fun onError(parentId: String) {
                        result.completeExceptionally(IllegalStateException("No children for $parentId"))
                    }
                },
            )
        }
        return result.get(15, TimeUnit.SECONDS).also { instrumentation.runOnMainSync { browser.unsubscribe(parentId) } }
    }

    @Test fun rootHasFourTabsAndAutoHints() {
        val extras = checkNotNull(browser.extras) { "root has no extras" }
        assertTrue(extras.getBoolean("android.media.browse.SEARCH_SUPPORTED"))
        assertEquals(2, extras.getInt("android.media.browse.CONTENT_STYLE_PLAYABLE_HINT"))

        val tabs = children(browser.root)
        assertEquals(listOf("stations", "recent", "genres", "playlists"), tabs.map { it.mediaId })
        assertTrue(tabs.all { it.isBrowsable })
    }

    @Test fun stationsArePlayableWithReadableCovers() {
        val stations = children("stations")
        assertTrue("no stations", stations.isNotEmpty())
        assertTrue(stations.all { it.isPlayable && it.mediaId!!.startsWith("station:") })

        val icon = stations.first().description.iconUri!!
        assertEquals(ArtworkUris.AUTHORITY, icon.authority)
        val head = context.contentResolver.openInputStream(icon)!!.use { it.readNBytes(12) }
        assertTrue("cover is not an image: ${head.toList()}", head.isImage())
    }

    @Test fun genresAndPlaylistsOpen() {
        val genre = children("genres").firstOrNull() ?: return
        assertTrue(children(genre.mediaId!!).all { it.isPlayable })
        val playlist = children("playlists").firstOrNull() ?: return
        assertTrue(children(playlist.mediaId!!).all { it.isPlayable })
    }

    /** The platform browser has no search, so this one uses Media3's (same onSearch / onGetSearchResult). */
    @Test fun searchFindsStations() {
        lateinit var future: com.google.common.util.concurrent.ListenableFuture<androidx.media3.session.MediaBrowser>
        instrumentation.runOnMainSync {
            future = androidx.media3.session.MediaBrowser.Builder(
                context,
                androidx.media3.session.SessionToken(context, ComponentName(context, PlaybackService::class.java)),
            ).buildAsync()
        }
        val media3 = future.get(10, TimeUnit.SECONDS)
        try {
            lateinit var search: com.google.common.util.concurrent.ListenableFuture<*>
            instrumentation.runOnMainSync { search = media3.search(QUERY, null) }
            search.get(15, TimeUnit.SECONDS)
            lateinit var results: com.google.common.util.concurrent.ListenableFuture<
                androidx.media3.session.LibraryResult<com.google.common.collect.ImmutableList<androidx.media3.common.MediaItem>>>
            instrumentation.runOnMainSync { results = media3.getSearchResult(QUERY, 0, 50, null) }
            val items = checkNotNull(results.get(15, TimeUnit.SECONDS).value) { "search failed" }
            assertTrue("no hits for $QUERY", items.isNotEmpty())
            assertTrue(items.all { it.mediaMetadata.isPlayable == true && it.mediaId.startsWith("station:") })
        } finally {
            instrumentation.runOnMainSync { media3.release() }
        }
    }

    /** The queue must be replaced (a fresh station load), not just keep playing what was playing before. */
    @Test fun voiceSearchPlaysAStation() {
        lateinit var controller: MediaController
        lateinit var before: List<String?>
        instrumentation.runOnMainSync {
            controller = MediaController(context, browser.sessionToken)
            before = controller.queue.orEmpty().map { it.description.mediaId }
            controller.transportControls.playFromSearch(QUERY, Bundle.EMPTY)
        }
        val deadline = System.currentTimeMillis() + 20_000
        var started = false
        while (System.currentTimeMillis() < deadline && !started) {
            Thread.sleep(250)
            val queue = controller.queue.orEmpty().map { it.description.mediaId }
            started = controller.playbackState?.state == PlaybackState.STATE_PLAYING &&
                controller.metadata?.getString(MediaMetadata.METADATA_KEY_TITLE) != null &&
                queue.isNotEmpty() && queue != before
        }
        instrumentation.runOnMainSync { controller.transportControls.pause() }
        assertTrue("voice search did not load and play a station", started)
    }

    private fun ByteArray.isImage(): Boolean {
        fun at(i: Int, vararg b: Int) = b.withIndex().all { (k, v) -> this.getOrNull(i + k)?.toInt()?.and(0xFF) == v }
        return at(0, 0x89, 0x50, 0x4E, 0x47) || // PNG
            at(0, 0xFF, 0xD8, 0xFF) || // JPEG
            (at(0, 0x52, 0x49, 0x46, 0x46) && at(8, 0x57, 0x45, 0x42, 0x50)) // WebP
    }

    private companion object {
        const val QUERY = "techno"
    }
}
