package io.github.felixclements.ownwave.ui.playlists

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.OpenInNew
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Link
import androidx.compose.material.icons.filled.LinkOff
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Radio
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Restore
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.api.models.PlaylistItem
import io.github.felixclements.ownwave.api.models.PlaylistPick
import io.github.felixclements.ownwave.api.models.SetPlaylistItemHiddenRequest
import io.github.felixclements.ownwave.api.models.SetPlaylistMatchRequest
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.data.apiBody
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.ui.LocalNavigator
import io.github.felixclements.ownwave.ui.Route
import io.github.felixclements.ownwave.ui.common.Art
import io.github.felixclements.ownwave.ui.common.Badge
import io.github.felixclements.ownwave.ui.common.ConfirmDialog
import io.github.felixclements.ownwave.ui.common.Empty
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.HeroHeader
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.Mutation
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.OwDialog
import io.github.felixclements.ownwave.ui.common.OwIconButton
import io.github.felixclements.ownwave.ui.common.OwTextField
import io.github.felixclements.ownwave.ui.common.PagePad
import io.github.felixclements.ownwave.ui.common.Spinner
import io.github.felixclements.ownwave.ui.common.TabItem
import io.github.felixclements.ownwave.ui.common.Tabs
import io.github.felixclements.ownwave.ui.common.TopSpacer
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.common.aspectSquare
import io.github.felixclements.ownwave.ui.common.rememberArtColor
import io.github.felixclements.ownwave.ui.common.rememberMutation
import io.github.felixclements.ownwave.ui.common.rememberQuery
import io.github.felixclements.ownwave.ui.rememberPageListState
import io.github.felixclements.ownwave.ui.stations.StationDraft
import io.github.felixclements.ownwave.ui.stations.StationEditor
import io.github.felixclements.ownwave.ui.theme.Ow
import java.time.Duration
import java.time.OffsetDateTime
import java.util.UUID
import kotlinx.coroutines.delay

private const val NEW_DAYS = 7L

private enum class ItemTab { Matched, Picks, Missing, Hidden }

/** apps/web/src/pages/Playlist.tsx: hero, make a station, the tracks by match state, admin fixes. */
@Composable
fun PlaylistScreen(id: String, session: Session) {
    val app = LocalAppData.current
    val nav = LocalNavigator.current
    val uri = LocalUriHandler.current
    val uuid = remember(id) { runCatching { UUID.fromString(id) }.getOrNull() }
    val all = rememberQuery("playlist:$id", enabled = uuid != null) { apiBody { apis.playlists.getPlaylist(uuid!!) } }
    var tab by rememberSaveable { mutableStateOf(ItemTab.Matched) }
    var picking by remember { mutableStateOf<PlaylistItem?>(null) }
    var creating by rememberSaveable { mutableStateOf(false) }
    var deleting by rememberSaveable { mutableStateOf(false) }
    val sync = rememberMutation()
    val remove = rememberMutation()
    val loaded = all.data?.playlist
    val color = rememberArtColor(loaded?.let { app.playlistArt(it) }, loaded?.name ?: id)

    val data = all.data
    if (data == null) {
        Column(Modifier.fillMaxSize()) {
            TopSpacer()
            if (all.loading) Spinner() else Empty("Playlist not found", body = all.error)
        }
        return
    }
    val p = data.playlist
    val isAdmin = session.isAdmin
    val items = data.items.filter {
        when (tab) {
            ItemTab.Hidden -> it.hidden
            ItemTab.Matched -> !it.hidden && it.trackId != null
            else -> !it.hidden && it.trackId == null
        }
    }

    LazyColumn(state = rememberPageListState(), modifier = Modifier.fillMaxSize()) {
        item(key = "hero") {
            HeroHeader(
                color = color,
                art = { Art(app.playlistArt(p), p.name, Modifier.fillMaxWidth().aspectSquare()) },
                kind = "Spotify playlist",
                title = p.name,
                meta = {
                    Text(
                        buildAnnotatedString {
                            withStyle(SpanStyle(fontWeight = FontWeight.SemiBold)) { append("${p.matchPct()}% in your library") }
                            append("  •  ${p.matchedCount} of ${Format.plural(p.trackCount, "track")}")
                            p.lastSyncedAt?.let { append("  •  synced ${Format.timeAgo(it)}") }
                        },
                        fontSize = 14.sp,
                        textAlign = TextAlign.Center,
                    )
                },
            )
        }
        item(key = "actions") {
            Column(Modifier.padding(horizontal = PagePad)) {
                Row(
                    Modifier.fillMaxWidth().padding(vertical = 16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    OwButton("Make a station", { creating = true }, variant = Variant.Primary, icon = Icons.Default.Radio)
                    OwButton("Spotify", { uri.openUri("https://open.spotify.com/playlist/${p.spotifyId}") }, icon = Icons.AutoMirrored.Filled.OpenInNew)
                    if (isAdmin) {
                        OwIconButton(Icons.Default.Refresh, "Sync now", { sync.run<Any?> { apiCall { apis.admin.syncPlaylist(UUID.fromString(p.id)) } } }, enabled = !sync.pending)
                        OwIconButton(Icons.Default.Delete, "Delete playlist", { deleting = true })
                    }
                }
                p.lastSyncError?.let { Text("Last sync failed: $it", fontSize = 14.sp, color = Ow.Danger, modifier = Modifier.padding(bottom = 16.dp)) }
                ErrorText(sync.error ?: remove.error)
                Tabs(
                    buildList {
                        add(TabItem(ItemTab.Matched, "In your library", p.matchedCount))
                        add(TabItem(ItemTab.Picks, "From your library", p.pickCount))
                        add(TabItem(ItemTab.Missing, "Missing", p.trackCount - p.matchedCount))
                        if (p.hiddenCount > 0 || isAdmin) add(TabItem(ItemTab.Hidden, "Hidden", p.hiddenCount))
                    },
                    tab,
                    { tab = it },
                    Modifier.padding(bottom = 16.dp),
                )
                val note = when {
                    tab == ItemTab.Missing && items.isNotEmpty() -> "Songs this playlist listed that are not in your library. Add the files and the next scan picks them up."
                    tab == ItemTab.Hidden && items.isNotEmpty() -> "Songs left out of this playlist's stations. Syncs keep them hidden."
                    else -> null
                }
                note?.let { Text(it, fontSize = 13.sp, color = Ow.Muted, modifier = Modifier.padding(bottom = 12.dp)) }
            }
        }
        if (tab == ItemTab.Picks) {
            item(key = "picks") { Picks(p.id, canRemove = isAdmin) }
        } else if (items.isEmpty()) {
            item(key = "empty") {
                when (tab) {
                    ItemTab.Matched -> Empty("No matches yet", body = "None of these songs were found in your library (yet).")
                    ItemTab.Hidden -> Empty("Nothing hidden", body = "Hide a song to keep it out of this playlist's stations.")
                    else -> Empty("Nothing missing", body = "Every song of this playlist is in your library.")
                }
            }
        } else {
            itemsIndexed(items, key = { _, it -> it.spotifyTrackId }) { i, it ->
                ItemRow(it, i + 1, p.lastSyncedAt, actions = if (isAdmin) ({ ItemActions(it, p.id) { picking = it } }) else null)
            }
        }
        item(key = "end") { Spacer(Modifier.height(32.dp)) }
    }

    picking?.let { if (isAdmin) MatchPicker(it, p.id) { picking = null } }
    if (creating) {
        StationEditor(
            onDismiss = { creating = false },
            initial = StationDraft(name = p.name, playlistIds = listOf(p.id)),
            onSaved = { nav.push(Route.Station(it.id)) },
        )
    }
    if (deleting) {
        ConfirmDialog(
            text = "Delete “${p.name}” and every track it collected?",
            confirm = "Delete",
            onConfirm = {
                deleting = false
                remove.run<Unit?>(onSuccess = { nav.replace(Route.Playlists) }) { apiCall { apis.admin.deletePlaylist(UUID.fromString(p.id)) } }
            },
            onDismiss = { deleting = false },
        )
    }
}

@Composable
private fun ItemRow(item: PlaylistItem, index: Int, lastSync: OffsetDateTime?, actions: (@Composable () -> Unit)?) {
    val app = LocalAppData.current
    val now = OffsetDateTime.now()
    val isNew = Duration.between(item.firstSeenAt, now).toDays() < NEW_DAYS
    // Every sync refreshes last_seen_at of the tracks Spotify still lists.
    val rotatedOut = lastSync != null && item.lastSeenAt.isBefore(lastSync.minusMinutes(1))
    Row(
        Modifier.fillMaxWidth().padding(horizontal = PagePad - 4.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("$index", fontSize = 14.sp, color = Ow.Muted, modifier = Modifier.width(32.dp))
        val trackId = item.trackId
        if (trackId != null) {
            Art(app.trackCover(trackId), item.title, Modifier.size(40.dp))
        } else {
            Box(
                Modifier.size(40.dp).border(1.dp, Ow.Line, RoundedCornerShape(6.dp)),
                contentAlignment = Alignment.Center,
            ) { Text("?", color = Ow.Faint) }
        }
        Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
            Text(item.title, fontWeight = FontWeight.Medium, fontSize = 14.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(item.artists.joinToString(", "), fontSize = 13.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
            val badges = buildList {
                if (isNew) add("New" to true)
                if (rotatedOut) add("Rotated out" to false)
                if (item.matchMethod == PlaylistItem.MatchMethod.FUZZY) add("Fuzzy" to false)
                if (item.manual) add((if (trackId != null) "Set by hand" else "Marked missing") to false)
            }
            if (badges.isNotEmpty()) {
                Row(Modifier.padding(top = 4.dp), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    badges.forEach { (text, accent) -> Badge(text, accent) }
                }
            }
        }
        item.durationMs?.let { Text(Format.duration(it.toDouble() / 1000), fontSize = 13.sp, color = Ow.Muted) }
        actions?.invoke()
    }
}

/** Admin fixes on a playlist track. Matches are shared by every playlist with the track; hiding is per playlist. */
@Composable
private fun ItemActions(item: PlaylistItem, playlistId: String, onPick: () -> Unit) {
    val act = rememberMutation()
    var menu by remember { mutableStateOf(false) }
    val pid = UUID.fromString(playlistId)
    fun hide(hidden: Boolean) = act.run<Unit?> { apiCall { apis.admin.setPlaylistItemHidden(pid, item.spotifyTrackId, SetPlaylistItemHiddenRequest(hidden = hidden)) } }
    if (item.hidden) {
        OwIconButton(Icons.Default.Visibility, "Show ${item.title} again", { hide(false) }, enabled = !act.pending, size = 36.dp, iconSize = 18.dp)
        return
    }
    Box {
        OwIconButton(Icons.Default.MoreVert, "Fix ${item.title}", { menu = true }, enabled = !act.pending, size = 36.dp, iconSize = 18.dp)
        DropdownMenu(expanded = menu, onDismissRequest = { menu = false }, containerColor = Ow.Raised) {
            DropdownMenuItem(
                text = { Text(if (item.trackId != null) "Change the file" else "Pick the file") },
                leadingIcon = { Icon(Icons.Default.Link, null) },
                onClick = {
                    menu = false
                    onPick()
                },
            )
            if (item.trackId != null) {
                DropdownMenuItem(
                    text = { Text("Wrong match: not in library") },
                    leadingIcon = { Icon(Icons.Default.LinkOff, null) },
                    onClick = {
                        menu = false
                        act.run<Unit?> { apiCall { apis.admin.setPlaylistMatch(pid, item.spotifyTrackId, SetPlaylistMatchRequest(trackId = null)) } }
                    },
                )
            }
            if (item.manual) {
                DropdownMenuItem(
                    text = { Text("Match automatically again") },
                    leadingIcon = { Icon(Icons.Default.Restore, null) },
                    onClick = {
                        menu = false
                        act.run<Unit?> { apiCall { apis.admin.resetPlaylistMatch(pid, item.spotifyTrackId) } }
                    },
                )
            }
            DropdownMenuItem(
                text = { Text("Hide from this playlist's stations") },
                leadingIcon = { Icon(Icons.Default.VisibilityOff, null) },
                onClick = {
                    menu = false
                    hide(true)
                },
            )
        }
    }
    act.error?.let { ErrorText(it) }
}

/** Search the library and pick the local file a playlist track plays. */
@Composable
private fun MatchPicker(item: PlaylistItem, playlistId: String, onClose: () -> Unit) {
    val app = LocalAppData.current
    var q by rememberSaveable(item.spotifyTrackId) { mutableStateOf(item.title) }
    var query by remember(item.spotifyTrackId) { mutableStateOf(item.title.trim()) }
    LaunchedEffect(q) {
        delay(250)
        query = q.trim()
    }
    val results = rememberQuery("picker:$query", enabled = query.isNotEmpty()) {
        apiBody { apis.library.listTracks(limit = 20, q = query) }.map { it.tracks }
    }
    val set: Mutation = rememberMutation()
    fun pick(trackId: String?) = set.run<Unit?>(onSuccess = { onClose() }) {
        apiCall { apis.admin.setPlaylistMatch(UUID.fromString(playlistId), item.spotifyTrackId, SetPlaylistMatchRequest(trackId = trackId)) }
    }
    OwDialog(
        "Pick the local file",
        onClose,
        footer = { OwButton("Not in my library", { pick(null) }, enabled = !set.pending) },
    ) {
        Column {
            Text(
                buildAnnotatedString {
                    append("For ")
                    withStyle(SpanStyle(fontWeight = FontWeight.SemiBold, color = Ow.Ink)) { append(item.title) }
                    append(" · ${item.artists.joinToString(", ")}. Every playlist with this song uses the file you pick, and syncs keep it.")
                },
                fontSize = 14.sp,
                color = Ow.Muted,
                modifier = Modifier.padding(bottom = 12.dp),
            )
            OwTextField(q, { q = it }, label = "Search your library")
            ErrorText(results.error ?: set.error)
            Column(Modifier.padding(top = 12.dp).heightIn(max = 320.dp).verticalScroll(rememberScrollState())) {
                if (results.loading) Spinner()
                results.data?.forEach { t ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .clip(RoundedCornerShape(6.dp))
                            .background(if (t.id == item.trackId) Ow.AccentSoft else androidx.compose.ui.graphics.Color.Transparent)
                            .clickable(enabled = !set.pending) { pick(t.id) }
                            .padding(8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Art(app.trackCover(t.id), t.title, Modifier.size(36.dp))
                        Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
                            Text(t.title, fontWeight = FontWeight.Medium, fontSize = 14.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            Text(
                                buildAnnotatedString {
                                    append(t.artist ?: "Unknown artist")
                                    t.album?.let { withStyle(SpanStyle(color = Ow.Faint)) { append(" · $it") } }
                                },
                                fontSize = 13.sp,
                                color = Ow.Muted,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                        }
                        t.durationSeconds?.let { Text(Format.duration(it), fontSize = 13.sp, color = Ow.Muted) }
                    }
                }
                if (results.data?.isEmpty() == true) {
                    Text("No songs match “$q”.", fontSize = 14.sp, color = Ow.Muted, textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth().padding(vertical = 16.dp))
                }
            }
        }
    }
}

@Composable
private fun Picks(playlistId: String, canRemove: Boolean) {
    val picks = rememberQuery("picks:$playlistId") { apiBody { apis.playlists.listPlaylistPicks(UUID.fromString(playlistId)) }.map { it.picks } }
    val remove = rememberMutation()
    when {
        picks.loading -> Spinner()
        else -> Column {
            ErrorText(picks.error ?: remove.error, Modifier.padding(horizontal = PagePad))
            val list = picks.data.orEmpty()
            if (list.isNotEmpty()) {
                Text(
                    "Songs from your library that sound like this playlist. Its stations play them alongside the matched tracks.",
                    fontSize = 13.sp,
                    color = Ow.Muted,
                    modifier = Modifier.padding(horizontal = PagePad).padding(bottom = 12.dp),
                )
                list.forEachIndexed { i, pick ->
                    PickRow(pick, i + 1, removing = remove.pending, onRemove = if (canRemove) {
                        { remove.run<Unit?> { apiCall { apis.admin.removePlaylistPick(UUID.fromString(playlistId), pick.trackId) } } }
                    } else null)
                }
            } else if (picks.error == null) {
                Empty("No picks yet", body = "Picks follow the playlist's matched tracks once those are analysed.")
            }
        }
    }
}

@Composable
private fun PickRow(pick: PlaylistPick, index: Int, removing: Boolean, onRemove: (() -> Unit)?) {
    val app = LocalAppData.current
    Row(
        Modifier.fillMaxWidth().padding(horizontal = PagePad - 4.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("$index", fontSize = 14.sp, color = Ow.Muted, modifier = Modifier.width(32.dp))
        Art(app.trackCover(pick.trackId), pick.title, Modifier.size(40.dp))
        Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
            Text(pick.title, fontWeight = FontWeight.Medium, fontSize = 14.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(pick.artist ?: "Unknown artist", fontSize = 13.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        pick.durationSeconds?.let { Text(Format.duration(it), fontSize = 13.sp, color = Ow.Muted) }
        if (onRemove != null) {
            OwIconButton(Icons.Default.Close, "Remove ${pick.title} from this playlist's picks", onRemove, enabled = !removing, size = 36.dp, iconSize = 18.dp)
        }
    }
}
