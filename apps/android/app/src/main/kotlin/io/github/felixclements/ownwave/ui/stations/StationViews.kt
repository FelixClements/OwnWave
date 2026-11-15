package io.github.felixclements.ownwave.ui.stations

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CheckboxDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.api.models.StationRequest
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.ui.LocalNavigator
import io.github.felixclements.ownwave.ui.Route
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.MediaCard
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.OwChip
import io.github.felixclements.ownwave.ui.common.OwDialog
import io.github.felixclements.ownwave.ui.common.OwTextField
import io.github.felixclements.ownwave.ui.common.PlayButton
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.common.rememberMutation
import io.github.felixclements.ownwave.ui.common.rememberPlaylists
import io.github.felixclements.ownwave.ui.common.rememberQuery
import io.github.felixclements.ownwave.ui.common.rememberStationGenreLabel
import io.github.felixclements.ownwave.data.apiBody
import io.github.felixclements.ownwave.ui.player.LocalPlayer
import io.github.felixclements.ownwave.ui.theme.Ow
import java.util.UUID

/** Square station card; its play button shows while the station plays. */
@Composable
fun StationCard(station: Station, subtitle: String? = null) {
    val app = LocalAppData.current
    val nav = LocalNavigator.current
    val player = LocalPlayer.current
    val label = rememberStationGenreLabel()
    val active = player.isActive(station.id)
    val playing = player.isPlaying(station.id)
    MediaCard(
        art = app.stationCover(station.id),
        name = station.name,
        subtitle = subtitle ?: "${label(station)} · ${Format.plural(station.trackCount, "track")}",
        onClick = { nav.push(Route.Station(station.id)) },
        highlight = active,
        overlay = {
            if (playing) {
                PlayButton(
                    playing = true,
                    label = "Pause ${station.name}",
                    onClick = player::toggle,
                    size = 44.dp,
                    modifier = Modifier.align(Alignment.BottomEnd).padding(8.dp),
                )
            }
        },
    )
}

/** Initial values for a new station. */
data class StationDraft(val name: String = "", val playlistIds: List<String> = emptyList())

/**
 * Create or edit a station (apps/web/src/components/StationEditor.tsx): a name, the playlists it
 * draws from (picked ones, or every playlist including later ones), optionally the genres it keeps,
 * and Enhance. A genre station keeps itself up to date, so only its name changes here.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun StationEditor(onDismiss: () -> Unit, station: Station? = null, initial: StationDraft? = null, onSaved: (Station) -> Unit = {}) {
    val playlists = rememberPlaylists().data.orEmpty()
    val save = rememberMutation()
    val genreStation = station?.kind == Station.Kind.GENRE
    var name by remember { mutableStateOf(station?.name ?: initial?.name.orEmpty()) }
    var allPlaylists by remember { mutableStateOf(station?.allPlaylists ?: false) }
    var playlistIds by remember { mutableStateOf(if (station?.allPlaylists == true) emptyList() else station?.playlistIds ?: initial?.playlistIds.orEmpty()) }
    var genres by remember { mutableStateOf(station?.genres.orEmpty()) }
    var enhance by remember { mutableStateOf(station?.enhance ?: false) }
    val hasPool = allPlaylists || playlistIds.isNotEmpty()
    val poolKey = if (allPlaylists) "all" else playlistIds.sorted().joinToString(",")
    val pool = rememberQuery("pool:$poolKey", enabled = !genreStation && hasPool) {
        apiBody {
            if (allPlaylists) apis.stations.getPoolGenres(allPlaylists = true) else apis.stations.getPoolGenres(playlistIds = playlistIds.joinToString(","))
        }
    }
    val poolGenres = pool.data?.genres.orEmpty()
    val poolTracks = pool.data?.tracks ?: 0
    // A track in two picked genres counts twice here: an upper bound.
    val plays = if (genres.isNotEmpty()) minOf(poolTracks, poolGenres.filter { it.id in genres }.sumOf { it.tracks }) else poolTracks
    val canSave = name.isNotBlank() && (genreStation || hasPool)
    fun toggle(list: List<String>, id: String) = if (id in list) list - id else list + id

    fun submit() {
        val body = when {
            genreStation -> StationRequest(name = name.trim())
            allPlaylists -> StationRequest(name = name.trim(), allPlaylists = true, playlistIds = emptyList(), genres = genres, enhance = enhance)
            else -> StationRequest(name = name.trim(), allPlaylists = false, playlistIds = playlistIds, genres = genres, enhance = enhance)
        }
        save.run<Station?>(onSuccess = { s ->
            s?.let(onSaved)
            onDismiss()
        }) {
            apiCall { if (station != null) apis.stations.updateStation(UUID.fromString(station.id), body) else apis.stations.createStation(body) }
        }
    }

    OwDialog(
        title = if (station != null) "Edit station" else "New station",
        onDismiss = onDismiss,
        footer = {
            OwButton("Cancel", onDismiss, variant = Variant.Ghost)
            OwButton(if (station != null) "Save" else "Create station", ::submit, variant = Variant.Primary, enabled = canSave, loading = save.pending)
        },
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(24.dp)) {
            OwTextField(name, { name = it.take(100) }, label = "Name", placeholder = "Late night techno")
            if (genreStation) {
                Text(
                    "A genre station plays every song in your library with this main genre and keeps itself up to date, so only its name changes. Admins can hide it or give it back its genre's name on the Admin page.",
                    fontSize = 14.sp,
                    color = Ow.Muted,
                )
            } else {
                Column {
                    Text("Playlists", fontSize = 13.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(bottom = 8.dp))
                    if (playlists.isEmpty()) {
                        Text("No playlists yet. An admin adds Spotify playlists on the Playlists page.", fontSize = 14.sp, color = Ow.Muted)
                    } else {
                        CheckRow(allPlaylists, "All playlists", "Including ones added later") { allPlaylists = !allPlaylists }
                        if (!allPlaylists) {
                            playlists.forEach { p ->
                                CheckRow(p.id in playlistIds, p.name, "${p.matchedCount} of ${p.trackCount} in your library") {
                                    playlistIds = toggle(playlistIds, p.id)
                                }
                            }
                        }
                    }
                }
                if (hasPool) {
                    Column {
                        Row(Modifier.padding(bottom = 8.dp)) {
                            Text("Genres", fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                            Text(" · none picked plays every genre", fontSize = 13.sp, color = Ow.Muted)
                        }
                        if (pool.data != null && poolGenres.isEmpty()) {
                            Text("No genre-scored tracks in these playlists yet.", fontSize = 14.sp, color = Ow.Muted)
                        } else {
                            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                poolGenres.forEach { g ->
                                    OwChip(g.id in genres, { genres = toggle(genres, g.id) }) {
                                        Text(g.name, fontSize = 13.sp, fontWeight = FontWeight.Medium)
                                        Text("${g.tracks}", fontSize = 13.sp, color = LocalContentColor.current.copy(alpha = 0.6f))
                                    }
                                }
                            }
                        }
                        Text(
                            "Plays ${if (genres.isNotEmpty()) "up to " else ""}${Format.plural(plays, "track")} of ${Format.plural(poolTracks, "track")}" +
                                (if (enhance) ", plus similar songs from your library" else "") + ".",
                            fontSize = 13.sp,
                            color = Ow.Muted,
                            modifier = Modifier.padding(top = 12.dp),
                        )
                    }
                    CheckRow(enhance, "Enhance", "Mix in songs from your library that sound like these playlists", icon = true) { enhance = !enhance }
                }
            }
            ErrorText(save.error)
        }
    }
}

@Composable
private fun CheckRow(checked: Boolean, title: String, subtitle: String, icon: Boolean = false, onToggle: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(6.dp)).clickable(onClick = onToggle).padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Checkbox(
            checked = checked,
            onCheckedChange = { onToggle() },
            colors = CheckboxDefaults.colors(checkedColor = Ow.Accent, checkmarkColor = androidx.compose.ui.graphics.Color.Black, uncheckedColor = Ow.Muted),
        )
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (icon) Icon(Icons.Default.AutoAwesome, null, modifier = Modifier.size(16.dp).padding(end = 2.dp))
                Text(title, fontWeight = FontWeight.Medium, fontSize = 15.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            Text(subtitle, fontSize = 12.sp, color = Ow.Muted)
        }
    }
    Spacer(Modifier.height(0.dp))
}
