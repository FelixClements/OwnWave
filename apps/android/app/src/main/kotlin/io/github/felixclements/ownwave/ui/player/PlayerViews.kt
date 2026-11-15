package io.github.felixclements.ownwave.ui.player

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Block
import androidx.compose.material.icons.filled.ExpandMore
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.SkipNext
import androidx.compose.material.icons.filled.SkipPrevious
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.ui.common.Art
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.OwIconButton
import io.github.felixclements.ownwave.ui.common.rememberArtColor
import io.github.felixclements.ownwave.ui.common.rememberStations
import io.github.felixclements.ownwave.ui.theme.Ow

/** The track's cover over HTTP when it has an id, else whatever the player has. */
@Composable
private fun TrackUi.cover(): String? = trackId?.let { LocalAppData.current.trackCover(it) } ?: artworkUri?.toString()

/** Phone mini player above the bottom nav. Tapping it opens [NowPlaying]. */
@Composable
fun MiniPlayer(modifier: Modifier = Modifier) {
    val player = LocalPlayer.current
    val state = player.state
    val track = state.current ?: return
    Box(
        modifier
            .padding(horizontal = 4.dp)
            .fillMaxWidth()
            .clip(RoundedCornerShape(6.dp))
            .background(Ow.Raised),
    ) {
        Row(Modifier.fillMaxWidth().padding(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Row(
                Modifier
                    .weight(1f)
                    .clip(RoundedCornerShape(4.dp))
                    .clickable(onClick = player.expand)
                    .semantics { contentDescription = "Open now playing" },
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Art(track.cover(), track.album ?: track.title, Modifier.size(40.dp))
                Column(Modifier.padding(start = 12.dp)) {
                    Text(track.title, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    track.artist?.let { Text(it, fontSize = 12.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis) }
                }
            }
            LikeButton(state.liked, onClick = { track.trackId?.let { player.toggleLike(it) } }, small = true)
            OwIconButton(
                if (state.isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
                if (state.isPlaying) "Pause" else "Play",
                player::toggle,
                tint = Ow.Ink,
                iconSize = 24.dp,
            )
        }
        val progress = if (state.durationMs > 0) (state.positionMs.toFloat() / state.durationMs).coerceIn(0f, 1f) else 0f
        Box(Modifier.align(Alignment.BottomStart).padding(horizontal = 8.dp).fillMaxWidth().height(2.dp).background(Color.White.copy(alpha = 0.2f))) {
            Box(Modifier.fillMaxHeight().fillMaxWidth(progress).background(Ow.Ink))
        }
    }
}

@Composable
private fun LikeButton(liked: Boolean, onClick: () -> Unit, small: Boolean = false) {
    OwIconButton(
        if (liked) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
        if (liked) "Unlike" else "Like",
        onClick,
        active = liked,
        size = if (small) 32.dp else 40.dp,
        iconSize = 18.dp,
    )
}

/** Full-screen now playing with the up-next queue, coloured from the artwork. */
@Composable
fun NowPlaying(onClose: () -> Unit) {
    val player = LocalPlayer.current
    val state = player.state
    val track = state.current ?: return
    val stations = rememberStations().data.orEmpty()
    val stationName = stations.firstOrNull { it.id == state.stationId }?.name
    val color = rememberArtColor(track.cover(), track.album ?: track.title)
    BackHandler(onBack = onClose)

    LazyColumn(
        Modifier
            .fillMaxSize()
            .background(Ow.Base)
            .background(Brush.verticalGradient(0f to color, 0.45f to color.copy(alpha = 0.4f), 0.9f to Ow.Base))
            .clickable(enabled = false) {},
    ) {
        item {
            Row(
                Modifier.statusBarsPadding().fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                OwIconButton(Icons.Default.ExpandMore, "Close now playing", onClose, tint = Ow.Ink, iconSize = 28.dp)
                Column(Modifier.weight(1f), horizontalAlignment = Alignment.CenterHorizontally) {
                    Text("PLAYING FROM STATION", fontSize = 11.sp, letterSpacing = 1.6.sp, color = Ow.Ink.copy(alpha = 0.7f))
                    Text(stationName.orEmpty(), fontSize = 14.sp, fontWeight = FontWeight.Bold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                Spacer(Modifier.width(40.dp))
            }
        }
        item {
            Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Box(Modifier.widthIn(max = 520.dp).fillMaxWidth().aspectRatio(1f).shadow(32.dp, RoundedCornerShape(8.dp))) {
                    Art(track.cover(), track.album ?: track.title, Modifier.fillMaxSize(), RoundedCornerShape(8.dp))
                }
                Row(Modifier.fillMaxWidth().padding(top = 24.dp), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(track.title, fontSize = 24.sp, fontWeight = FontWeight.Bold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(
                            listOfNotNull(track.artist, track.album).joinToString(" · "),
                            fontSize = 15.sp,
                            color = Ow.Ink.copy(alpha = 0.7f),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    val trackId = track.trackId
                    val stationId = state.stationId
                    if (trackId != null && stationId != null) {
                        OwIconButton(Icons.Default.VisibilityOff, "Not in this station: ${track.title}", { player.hide(stationId, trackId) })
                    }
                    if (trackId != null) {
                        OwIconButton(Icons.Default.Block, "Never play ${track.title}", { player.ban(trackId) })
                    }
                    LikeButton(state.liked, { trackId?.let { player.toggleLike(it) } })
                }
                SeekBar(state, player::seekTo, Modifier.padding(top = 16.dp))
                Row(
                    Modifier.padding(top = 16.dp, bottom = 40.dp),
                    horizontalArrangement = Arrangement.spacedBy(32.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    OwIconButton(Icons.Default.SkipPrevious, "Previous", player::previous, tint = Ow.Ink, size = 48.dp, iconSize = 32.dp)
                    Box(
                        Modifier
                            .size(64.dp)
                            .clip(CircleShape)
                            .background(Ow.Ink)
                            .clickable(onClick = player::toggle)
                            .semantics { contentDescription = if (state.isPlaying) "Pause" else "Play" },
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(if (state.isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow, null, tint = Color.Black, modifier = Modifier.size(32.dp))
                    }
                    OwIconButton(Icons.Default.SkipNext, "Next", player::next, tint = Ow.Ink, size = 48.dp, iconSize = 32.dp)
                }
            }
        }
        item {
            Column(
                Modifier
                    .padding(horizontal = 24.dp)
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(12.dp))
                    .background(Color.Black.copy(alpha = 0.25f))
                    .padding(16.dp),
            ) {
                Text("Up next", fontWeight = FontWeight.Bold, fontSize = 16.sp, modifier = Modifier.padding(bottom = 12.dp))
                if (state.upNext.isEmpty()) Text("More tracks load as you listen.", fontSize = 14.sp, color = Ow.Muted)
            }
        }
        itemsIndexed(state.upNext.take(30), key = { i, t -> "${t.mediaId}-$i" }) { _, u ->
            Row(
                Modifier
                    .padding(horizontal = 24.dp)
                    .fillMaxWidth()
                    .background(Color.Black.copy(alpha = 0.25f))
                    .padding(horizontal = 16.dp, vertical = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Art(u.cover(), u.album ?: u.title, Modifier.size(40.dp))
                Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
                    Text(u.title, fontSize = 14.sp, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    u.artist?.let { Text(it, fontSize = 12.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis) }
                }
                Text(Format.duration(u.durationMs?.let { it / 1000.0 }), fontSize = 12.sp, color = Ow.Muted)
            }
        }
        item { Spacer(Modifier.navigationBarsPadding().height(40.dp)) }
    }
}

/** Thin bar that fills with the accent, with times on both sides (the web's `ow-slider`). */
@Composable
private fun SeekBar(state: PlayerUiState, onSeek: (Long) -> Unit, modifier: Modifier = Modifier) {
    var dragging by remember { mutableStateOf<Float?>(null) }
    val duration = state.durationMs
    val enabled = state.seekable && duration > 0
    val fraction = dragging ?: if (duration > 0) (state.positionMs.toFloat() / duration).coerceIn(0f, 1f) else 0f
    Row(modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        val shown = dragging?.let { (it * duration).toLong() } ?: state.positionMs
        Text(Format.duration(shown / 1000.0), fontSize = 11.sp, color = Ow.Muted, modifier = Modifier.width(44.dp))
        BoxWithConstraints(
            Modifier
                .weight(1f)
                .height(24.dp)
                .semantics { contentDescription = "Seek" }
                .pointerInput(enabled, duration) {
                    if (!enabled) return@pointerInput
                    detectTapGestures { offset -> onSeek(((offset.x / size.width).coerceIn(0f, 1f) * duration).toLong()) }
                }
                .pointerInput(enabled, duration) {
                    if (!enabled) return@pointerInput
                    detectHorizontalDragGestures(
                        onDragStart = { dragging = (it.x / size.width).coerceIn(0f, 1f) },
                        onDragEnd = {
                            dragging?.let { onSeek((it * duration).toLong()) }
                            dragging = null
                        },
                        onDragCancel = { dragging = null },
                    ) { change, _ -> dragging = (change.position.x / size.width).coerceIn(0f, 1f) }
                },
            contentAlignment = Alignment.CenterStart,
        ) {
            Box(Modifier.fillMaxWidth().height(4.dp).clip(CircleShape).background(Color.White.copy(alpha = 0.25f))) {
                Box(Modifier.fillMaxHeight().fillMaxWidth(fraction).background(if (dragging != null) Ow.Accent else Ow.Ink))
            }
            Box(
                Modifier
                    .padding(start = (maxWidth - 12.dp) * fraction)
                    .size(12.dp)
                    .clip(CircleShape)
                    .background(Ow.Ink),
            )
        }
        Text(
            if (duration > 0) Format.duration(duration / 1000.0) else "–:––",
            fontSize = 11.sp,
            color = Ow.Muted,
            modifier = Modifier.width(44.dp).padding(start = 8.dp),
        )
    }
}
