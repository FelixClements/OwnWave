package io.github.felixclements.ownwave.ui.stations

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.ExpandLess
import androidx.compose.material.icons.filled.ExpandMore
import androidx.compose.material.icons.filled.Radio
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.api.models.StationRequest
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.data.apiBody
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.ui.LocalNavigator
import io.github.felixclements.ownwave.ui.Route
import io.github.felixclements.ownwave.ui.common.Art
import io.github.felixclements.ownwave.ui.common.Colors
import io.github.felixclements.ownwave.ui.common.ConfirmDialog
import io.github.felixclements.ownwave.ui.common.Empty
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.HeroHeader
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.LocalTopInset
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.OwIconButton
import io.github.felixclements.ownwave.ui.common.PagePad
import io.github.felixclements.ownwave.ui.common.PageTitle
import io.github.felixclements.ownwave.ui.common.PlayButton
import io.github.felixclements.ownwave.ui.common.Spinner
import io.github.felixclements.ownwave.ui.common.TabItem
import io.github.felixclements.ownwave.ui.common.Tabs
import io.github.felixclements.ownwave.ui.common.TopSpacer
import io.github.felixclements.ownwave.ui.common.TrackItem
import io.github.felixclements.ownwave.ui.common.TrackListRow
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.common.aspectSquare
import io.github.felixclements.ownwave.ui.common.cardGrid
import io.github.felixclements.ownwave.ui.common.rememberArtColor
import io.github.felixclements.ownwave.ui.common.rememberMutation
import io.github.felixclements.ownwave.ui.common.rememberPlaylists
import io.github.felixclements.ownwave.ui.common.rememberQuery
import io.github.felixclements.ownwave.ui.common.rememberStationGenreLabel
import io.github.felixclements.ownwave.ui.common.rememberStations
import io.github.felixclements.ownwave.ui.common.toColor
import io.github.felixclements.ownwave.ui.common.toItem
import io.github.felixclements.ownwave.ui.player.LocalPlayer
import io.github.felixclements.ownwave.ui.rememberPageListState
import io.github.felixclements.ownwave.ui.theme.Ow
import java.util.UUID

/** The station page lists this many of the queue, not the whole pool. */
private const val PREVIEW_TRACKS = 100

private enum class Filter { All, Genre, Shared, Mine }

private fun Filter.matches(s: Station) = when (this) {
    Filter.All -> true
    Filter.Genre -> s.kind == Station.Kind.GENRE
    Filter.Shared -> s.shared && s.kind != Station.Kind.GENRE
    Filter.Mine -> !s.shared
}

/** apps/web/src/pages/Stations.tsx */
@Composable
fun StationsScreen() {
    val stations = rememberStations()
    var filter by rememberSaveable { mutableStateOf(Filter.All) }
    var creating by rememberSaveable { mutableStateOf(false) }
    val all = stations.data.orEmpty()
    val list = all.filter(filter::matches)
    LazyColumn(state = rememberPageListState(), modifier = Modifier.fillMaxSize()) {
        item(key = "top") {
            Column(Modifier.padding(horizontal = PagePad)) {
                TopSpacer()
                PageTitle("Stations", Modifier.padding(top = 8.dp)) {
                    OwButton("New station", { creating = true }, variant = Variant.Primary, icon = Icons.Default.Add)
                }
                Tabs(
                    listOf(
                        TabItem(Filter.All, "All", all.size),
                        TabItem(Filter.Genre, "Genres", all.count(Filter.Genre::matches)),
                        TabItem(Filter.Shared, "Shared", all.count(Filter.Shared::matches)),
                        TabItem(Filter.Mine, "Yours", all.count(Filter.Mine::matches)),
                    ),
                    filter,
                    { filter = it },
                    Modifier.padding(vertical = 16.dp),
                )
            }
        }
        when {
            stations.loading -> item(key = "spin") { Spinner() }
            stations.data == null -> item(key = "err") { ErrorText(stations.error, Modifier.padding(PagePad)) }
            list.isEmpty() -> item(key = "empty") {
                Empty(
                    "Nothing here yet",
                    icon = Icons.Default.Radio,
                    body = "A station plays the songs of one or more playlists that are in your library, optionally only some genres. Each main genre gets its own station once enough of your playlist songs are in it.",
                )
            }
            else -> cardGrid(list, key = { it.id }) { StationCard(it) }
        }
        item(key = "end") { Spacer(Modifier.height(32.dp)) }
    }
    if (creating) StationEditor(onDismiss = { creating = false })
}

/** apps/web/src/pages/Genre.tsx: the stations of one genre, from the browse tree. */
@Composable
fun GenreScreen(id: String) {
    val node = rememberQuery("browse:genre:$id") { apiBody { apis.stations.browse("genre:$id") } }
    val stations = rememberStations().data.orEmpty()
    LazyColumn(state = rememberPageListState(), modifier = Modifier.fillMaxSize()) {
        val n = node.data
        when {
            node.loading -> item(key = "spin") {
                TopSpacer()
                Spinner()
            }
            n == null -> item(key = "missing") {
                TopSpacer()
                Empty("Genre not found", body = node.error)
            }
            else -> {
                val ids = n.items.mapNotNull { it.stationId }.toSet()
                val list = stations.filter { it.id in ids }
                item(key = "head") {
                    val c = Colors.nameColor(n.title).toColor()
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .background(Brush.verticalGradient(listOf(c, Color.Transparent)))
                            .padding(top = LocalTopInset.current + 48.dp, bottom = 32.dp, start = PagePad, end = PagePad),
                    ) {
                        Text("Genre", fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        Text(n.title, fontSize = 48.sp, lineHeight = 52.sp, fontWeight = FontWeight.Black, letterSpacing = (-1.2).sp)
                    }
                }
                if (list.isEmpty()) {
                    item(key = "empty") { Empty("No ${n.title} stations") }
                } else {
                    cardGrid(list, key = { it.id }) { StationCard(it) }
                }
            }
        }
        item(key = "end") { Spacer(Modifier.height(32.dp)) }
    }
}

/** apps/web/src/pages/Station.tsx: hero, play and edit, the queue (or what plays next), hidden tracks. */
@Composable
fun StationScreen(id: String, session: Session) {
    val app = LocalAppData.current
    val nav = LocalNavigator.current
    val player = LocalPlayer.current
    val uuid = remember(id) { runCatching { UUID.fromString(id) }.getOrNull() }
    val station = rememberQuery("station:$id", enabled = uuid != null) { apiBody { apis.stations.getStation(uuid!!) } }
    val active = player.isActive(id)
    val preview = rememberQuery("queue:$id", enabled = uuid != null && !active) { apiBody { apis.stations.getQueue(uuid!!) }.map { it.queue } }
    val hidden = rememberQuery("hidden:$id", enabled = uuid != null) { apiBody { apis.stations.listHiddenTracks(uuid!!) }.map { it.tracks } }
    val playlists = rememberPlaylists().data.orEmpty()
    val genreLabel = rememberStationGenreLabel()
    val color = rememberArtColor(app.stationCover(id), station.data?.name ?: id)
    val save = rememberMutation()
    val del = rememberMutation()
    val hide = rememberMutation()
    val restore = rememberMutation()
    var editing by rememberSaveable { mutableStateOf(false) }
    var deleting by rememberSaveable { mutableStateOf(false) }
    var showHidden by rememberSaveable { mutableStateOf(false) }

    val s = station.data
    if (s == null) {
        Column(Modifier.fillMaxSize()) {
            TopSpacer()
            if (station.loading) Spinner() else Empty("Station not found", body = "It may have been deleted or hidden.")
        }
        return
    }
    val genreStation = s.kind == Station.Kind.GENRE
    val canChange = !s.shared || session.isAdmin
    val names = if (s.allPlaylists) emptyList() else s.playlistIds.mapNotNull { pid -> playlists.find { it.id == pid } }
    val tracks: List<TrackItem> = if (active) {
        listOfNotNull(player.state.current?.toItem()) + player.state.upNext.mapNotNull { it.toItem() }
    } else {
        preview.data.orEmpty().map { it.toItem() }
    }
    val shown = minOf(tracks.size, PREVIEW_TRACKS)
    val hiddenList = hidden.data.orEmpty()

    LazyColumn(state = rememberPageListState(), modifier = Modifier.fillMaxSize()) {
        item(key = "hero") {
            HeroHeader(
                color = color,
                art = { Art(app.stationCover(s.id), s.name, Modifier.fillMaxWidth().aspectSquare()) },
                kind = if (genreStation) "Genre station" else if (s.shared) "Station" else "Your station",
                title = s.name,
                meta = {
                    val text = buildAnnotatedString {
                        withStyle(SpanStyle(fontWeight = FontWeight.SemiBold)) { append(genreLabel(s)) }
                        append("  •  ")
                        append(Format.plural(s.trackCount, "track"))
                        if (s.allPlaylists || genreStation) {
                            append("  •  ")
                            append(if (genreStation) "from your whole library" else "from every playlist")
                        }
                    }
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(text, fontSize = 14.sp, textAlign = TextAlign.Center)
                        if (names.isNotEmpty()) {
                            Row(Modifier.padding(top = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                                Text("from ", fontSize = 14.sp)
                                names.forEachIndexed { i, p ->
                                    if (i > 0) Text(", ", fontSize = 14.sp)
                                    Text(
                                        p.name,
                                        fontSize = 14.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier.clickable { nav.push(Route.Playlist(p.id)) },
                                    )
                                }
                            }
                        }
                    }
                },
                actions = {
                    PlayButton(
                        playing = active && player.state.isPlaying,
                        label = if (active && player.state.isPlaying) "Pause" else "Play ${s.name}",
                        onClick = { player.playOrToggle(s.id, s.name) },
                    )
                    if (!active) OwIconButton(Icons.Default.Refresh, "Shuffle the preview", preview.refresh)
                    if (canChange && !genreStation) {
                        OwButton(
                            if (s.enhance) "Enhanced" else "Enhance",
                            { save.run<Station?> { apiCall { apis.stations.updateStation(UUID.fromString(s.id), StationRequest(enhance = !s.enhance)) } } },
                            variant = if (s.enhance) Variant.Primary else Variant.Ghost,
                            small = true,
                            loading = save.pending,
                            icon = Icons.Default.AutoAwesome,
                        )
                    }
                    if (canChange) {
                        OwIconButton(Icons.Default.Edit, if (genreStation) "Rename station" else "Edit station", { editing = true })
                        // Genre stations are hidden in Admin, not deleted.
                        if (!genreStation) OwIconButton(Icons.Default.Delete, "Delete station", { deleting = true })
                    }
                },
            )
        }
        item(key = "head") {
            Column(Modifier.padding(horizontal = PagePad)) {
                ErrorText(del.error ?: save.error ?: hide.error)
                Text(
                    buildAnnotatedString {
                        append(if (active) "Now playing and up next" else "Coming up when you press play (shuffled each time)")
                        if (shown in 1 until s.trackCount) {
                            withStyle(SpanStyle(fontWeight = FontWeight.Normal)) { append(" · showing $shown of ${Format.plural(s.trackCount, "track")}") }
                        }
                    },
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = Ow.Muted,
                    modifier = Modifier.padding(vertical = 8.dp),
                )
            }
        }
        if (preview.loading && !active) {
            item(key = "spin") { Spinner() }
        } else if (tracks.isEmpty()) {
            item(key = "empty") {
                Empty(
                    "No tracks yet",
                    body = if (genreStation) {
                        "No song in your library has this main genre right now. Rescan, or check the genres in Admin."
                    } else {
                        "None of this station's playlist tracks are in your library with these genres. Rescan, or widen the genres."
                    },
                )
            }
        } else {
            itemsIndexed(tracks.take(PREVIEW_TRACKS), key = { i, t -> "${t.id}-$i" }) { i, t ->
                TrackListRow(t, i + 1, onHide = { tr ->
                    if (active) player.hide(s.id, tr.id) else hide.run<Unit?> { apiCall { apis.stations.hideTrack(UUID.fromString(s.id), io.github.felixclements.ownwave.api.models.HideTrackRequest(trackId = tr.id)) } }
                })
            }
        }
        if (hiddenList.isNotEmpty()) {
            item(key = "hidden-h") {
                Row(
                    Modifier
                        .padding(horizontal = PagePad)
                        .padding(top = 32.dp)
                        .clip(RoundedCornerShape(4.dp))
                        .clickable { showHidden = !showHidden }
                        .padding(vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(if (showHidden) Icons.Default.ExpandLess else Icons.Default.ExpandMore, null, tint = Ow.Muted)
                    Text("Hidden on this station (${hiddenList.size})", fontSize = 14.sp, fontWeight = FontWeight.SemiBold, color = Ow.Muted)
                }
            }
            if (showHidden) {
                itemsIndexed(hiddenList, key = { _, t -> "h-${t.trackId}" }) { _, t ->
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = PagePad, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Text(
                            buildAnnotatedString {
                                withStyle(SpanStyle(fontWeight = FontWeight.Medium)) { append(t.title) }
                                withStyle(SpanStyle(color = Ow.Muted)) { append(" · ${t.artist ?: "Unknown artist"}") }
                            },
                            fontSize = 14.sp,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f),
                        )
                        OwButton(
                            "Bring back",
                            { restore.run<Unit?>(tag = t.trackId) { apiCall { apis.stations.unhideTrack(UUID.fromString(s.id), t.trackId) } } },
                            small = true,
                            enabled = !restore.pending,
                            loading = restore.pending && restore.tag == t.trackId,
                        )
                    }
                }
                item(key = "hidden-err") { ErrorText(restore.error, Modifier.padding(horizontal = PagePad)) }
            }
        }
        item(key = "end") { Spacer(Modifier.height(32.dp)) }
    }
    if (editing) StationEditor(onDismiss = { editing = false }, station = s)
    if (deleting) {
        ConfirmDialog(
            text = "Delete “${s.name}”${if (s.shared) " for everyone" else ""}?",
            confirm = "Delete",
            onConfirm = {
                deleting = false
                del.run<Unit?>(onSuccess = { nav.replace(Route.Stations) }) { apiCall { apis.stations.deleteStation(UUID.fromString(s.id)) } }
            },
            onDismiss = { deleting = false },
        )
    }
}
