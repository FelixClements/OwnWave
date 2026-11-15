package io.github.felixclements.ownwave.ui.common

import androidx.compose.foundation.layout.Box
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Block
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.github.felixclements.ownwave.api.models.QueueTrack
import io.github.felixclements.ownwave.api.models.Track
import io.github.felixclements.ownwave.ui.player.LocalPlayer
import io.github.felixclements.ownwave.ui.player.TrackUi
import io.github.felixclements.ownwave.ui.theme.Ow

/** One song in a list, whatever endpoint it came from. */
data class TrackItem(
    val id: String,
    val title: String,
    val artist: String?,
    val album: String?,
    val durationSeconds: Double? = null,
    val liked: Boolean = false,
)

fun QueueTrack.toItem() = TrackItem(id, title, artist, album, durationSeconds, liked)

fun Track.toItem() = TrackItem(id, title, artist, album, durationSeconds)

fun TrackUi.toItem(): TrackItem? = trackId?.let { TrackItem(it, title, artist, album, durationMs?.let { ms -> ms / 1000.0 }) }

/**
 * A row of the web's TrackList: number (or the equalizer while it plays), cover, title and artist,
 * then like and a menu with "Not in this station" (when [onHide] is set) and "Never play".
 */
@Composable
fun TrackListRow(
    track: TrackItem,
    index: Int?,
    modifier: Modifier = Modifier,
    showActions: Boolean = true,
    onHide: ((TrackItem) -> Unit)? = null,
    onClick: (() -> Unit)? = null,
) {
    val app = LocalAppData.current
    val player = LocalPlayer.current
    val current = player.state.current?.trackId == track.id
    TrackRow(
        title = track.title,
        subtitle = track.artist ?: "Unknown artist",
        art = app.trackCover(track.id),
        artName = track.album?.takeIf { it.isNotBlank() } ?: track.title,
        modifier = modifier,
        index = index,
        current = current,
        playing = current && player.state.isPlaying,
        onClick = onClick,
    ) {
        if (showActions) {
            val liked = player.liked(track.id, track.liked)
            OwIconButton(
                if (liked) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
                if (liked) "Unlike ${track.title}" else "Like ${track.title}",
                { player.toggleLike(track.id, track.liked) },
                active = liked,
                size = 36.dp,
                iconSize = 18.dp,
            )
            var menu by remember { mutableStateOf(false) }
            Box {
                OwIconButton(Icons.Default.MoreVert, "More for ${track.title}", { menu = true }, size = 36.dp, iconSize = 18.dp)
                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }, containerColor = Ow.Raised) {
                    if (onHide != null) {
                        DropdownMenuItem(
                            text = { Text("Not in this station") },
                            leadingIcon = { Icon(Icons.Default.VisibilityOff, null) },
                            onClick = {
                                menu = false
                                onHide(track)
                            },
                        )
                    }
                    DropdownMenuItem(
                        text = { Text("Never play") },
                        leadingIcon = { Icon(Icons.Default.Block, null) },
                        onClick = {
                            menu = false
                            player.ban(track.id)
                        },
                    )
                }
            }
        }
    }
}
