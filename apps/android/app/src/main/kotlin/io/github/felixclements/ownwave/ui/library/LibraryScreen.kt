package io.github.felixclements.ownwave.ui.library

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Block
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.LibraryMusic
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.api.apis.PlaybackApi.FeedbackListFeedback
import io.github.felixclements.ownwave.api.models.FeedbackRequest
import io.github.felixclements.ownwave.api.models.Track
import io.github.felixclements.ownwave.data.apiBody
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.ui.common.Art
import io.github.felixclements.ownwave.ui.common.Empty
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.PagePad
import io.github.felixclements.ownwave.ui.common.PageTitle
import io.github.felixclements.ownwave.ui.common.SearchField
import io.github.felixclements.ownwave.ui.common.Spinner
import io.github.felixclements.ownwave.ui.common.TabItem
import io.github.felixclements.ownwave.ui.common.Tabs
import io.github.felixclements.ownwave.ui.common.TopSpacer
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.common.rememberMutation
import io.github.felixclements.ownwave.ui.common.rememberQuery
import io.github.felixclements.ownwave.ui.rememberPageListState
import io.github.felixclements.ownwave.ui.theme.Ow
import java.util.UUID
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private enum class Tab { Tracks, Liked, History, Banned }

private const val PAGE = 100

/** apps/web/src/pages/Library.tsx: all tracks (paged, filterable), liked, recently played, banned. */
@Composable
fun LibraryScreen() {
    var tab by rememberSaveable { mutableStateOf(Tab.Tracks) }
    val listState = rememberPageListState()
    val tracks = rememberTrackPages()
    val loadMore by rememberUpdatedState(tracks.loadMore)

    // Load the next page near the end of the list, like the web's sentinel.
    LaunchedEffect(listState, tab) {
        if (tab != Tab.Tracks) return@LaunchedEffect
        snapshotFlow { listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index to listState.layoutInfo.totalItemsCount }
            .collect { (last, total) -> if (last != null && last >= total - 10) loadMore() }
    }

    LazyColumn(state = listState, modifier = Modifier.fillMaxSize()) {
        item(key = "head") {
            Column(Modifier.padding(horizontal = PagePad)) {
                TopSpacer()
                PageTitle("Library", Modifier.padding(top = 8.dp))
                Tabs(
                    listOf(
                        TabItem(Tab.Tracks, "All tracks"),
                        TabItem(Tab.Liked, "Liked"),
                        TabItem(Tab.History, "Recently played"),
                        TabItem(Tab.Banned, "Banned"),
                    ),
                    tab,
                    { tab = it },
                    Modifier.padding(vertical = 16.dp),
                )
            }
        }
        when (tab) {
            Tab.Tracks -> allTracks(tracks)
            Tab.Liked -> item(key = "liked") { FeedbackTracks(like = true) }
            Tab.Banned -> item(key = "banned") { FeedbackTracks(like = false) }
            Tab.History -> item(key = "history") { HistoryTab() }
        }
        item(key = "end") { Spacer(Modifier.height(32.dp)) }
    }
}

/** Pages of /api/tracks for a filter, appended as the list scrolls. */
private class TrackPages(
    val filter: String,
    val setFilter: (String) -> Unit,
    val query: String,
    val rows: List<Track>,
    val loading: Boolean,
    val loadingMore: Boolean,
    val error: String?,
    val loadMore: () -> Unit,
)

@Composable
private fun rememberTrackPages(): TrackPages {
    val app = LocalAppData.current
    val scope = rememberCoroutineScope()
    var filter by rememberSaveable { mutableStateOf("") }
    var query by rememberSaveable { mutableStateOf("") }
    val rows = remember(query) { mutableStateListOf<Track>() }
    var done by remember(query) { mutableStateOf(false) }
    var busy by remember(query) { mutableStateOf(false) }
    var error by remember(query) { mutableStateOf<String?>(null) }
    LaunchedEffect(filter) {
        delay(250)
        query = filter.trim()
    }
    fun load() {
        if (busy || done) return
        busy = true
        val q = query
        scope.launch {
            apiBody { app.apis.library.listTracks(limit = PAGE, offset = rows.size, q = q.ifEmpty { null }) }
                .onSuccess {
                    if (q == query) {
                        rows.addAll(it.tracks)
                        done = it.tracks.size < PAGE
                    }
                }
                .onFailure { error = it.message }
            busy = false
        }
    }
    LaunchedEffect(query) { load() }
    return TrackPages(filter, { filter = it }, query, rows, busy && rows.isEmpty(), busy && rows.isNotEmpty(), error, ::load)
}

private fun LazyListScope.allTracks(p: TrackPages) {
    item(key = "filter") {
        SearchField(
            p.filter,
            p.setFilter,
            "Filter by title, artist or album",
            Icons.Default.Search,
            Modifier.padding(horizontal = PagePad).padding(bottom = 16.dp),
            height = 40.dp,
            shape = RoundedCornerShape(6.dp),
        )
    }
    when {
        p.loading -> item(key = "spin") { Spinner() }
        p.rows.isEmpty() -> item(key = "empty") {
            Column {
                ErrorText(p.error, Modifier.padding(horizontal = PagePad))
                Empty(
                    if (p.query.isNotEmpty()) "No tracks match “${p.query}”" else "Your library is empty",
                    icon = Icons.Default.LibraryMusic,
                    body = if (p.query.isEmpty()) "An admin scans the music folder from the Admin page." else null,
                )
            }
        }
        else -> {
            itemsIndexed(p.rows, key = { i, t -> "t-${t.id}-$i" }) { _, t ->
                LibraryRow(t.id, t.title, t.artist, t.album) { Text(Format.duration(t.durationSeconds), fontSize = 13.sp, color = Ow.Muted) }
            }
            if (p.loadingMore) item(key = "more") { Spinner() }
        }
    }
}

@Composable
private fun LibraryRow(id: String, title: String, artist: String?, album: String?, right: @Composable () -> Unit) {
    val app = LocalAppData.current
    Row(
        Modifier.fillMaxWidth().padding(horizontal = PagePad - 4.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Art(app.trackCover(id), album?.takeIf { it.isNotBlank() } ?: title, Modifier.size(40.dp))
        Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
            Text(title, fontWeight = FontWeight.Medium, fontSize = 14.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(artist ?: "Unknown artist", fontSize = 13.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        right()
    }
}

@Composable
private fun FeedbackTracks(like: Boolean) {
    val kind = if (like) FeedbackListFeedback.LIKE else FeedbackListFeedback.BAN
    val list = rememberQuery("feedback:${kind.value}") { apiBody { apis.playback.listFeedback(kind) }.map { it.tracks } }
    val remove = rememberMutation()
    val rows = list.data.orEmpty()
    when {
        list.loading -> Spinner()
        rows.isEmpty() && list.data != null -> if (like) {
            Empty("No liked songs yet", icon = Icons.Default.FavoriteBorder, body = "Tap the heart while a song plays.")
        } else {
            Empty("Nothing banned", icon = Icons.Default.Block, body = "Songs you ban never play again on any station.")
        }
        rows.isEmpty() -> ErrorText(list.error, Modifier.padding(horizontal = PagePad))
        else -> Column {
            ErrorText(remove.error, Modifier.padding(horizontal = PagePad))
            rows.forEach { t ->
                LibraryRow(t.id, t.title, t.artist, t.album) {
                    OwButton(
                        if (like) "Unlike" else "Allow again",
                        {
                            remove.run<Unit?>(tag = t.id) {
                                val body = FeedbackRequest(if (like) FeedbackRequest.Feedback.LIKE else FeedbackRequest.Feedback.BAN)
                                apiCall { apis.feedback.deleteFeedback(UUID.fromString(t.id), body) }
                            }
                        },
                        variant = Variant.Ghost,
                        small = true,
                        loading = remove.pending && remove.tag == t.id,
                    )
                }
            }
        }
    }
}

@Composable
private fun HistoryTab() {
    val h = rememberQuery("history") { apiBody { apis.playback.listHistory() }.map { it.history.orEmpty() } }
    val rows = h.data.orEmpty()
    when {
        h.loading -> Spinner()
        rows.isEmpty() && h.data != null -> Empty("Nothing played yet", icon = Icons.Default.History)
        rows.isEmpty() -> ErrorText(h.error, Modifier.padding(horizontal = PagePad))
        else -> Column {
            rows.forEach { e ->
                LibraryRow(e.trackId, e.title, e.artist, e.album) { Text(Format.timeAgo(e.playedAt), fontSize = 13.sp, color = Ow.Muted) }
            }
        }
    }
}
