package io.github.felixclements.ownwave.ui.player

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Block
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.MusicNote
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.SkipNext
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import coil3.compose.SubcomposeAsyncImage
import io.github.felixclements.ownwave.R

/** One-line player above the system bar. Tapping it opens [NowPlayingSheet]. */
@Composable
fun MiniPlayer(state: PlayerUiState, onOpen: () -> Unit, onTogglePlay: () -> Unit, onNext: () -> Unit) {
    val track = state.current ?: return
    Surface(color = MaterialTheme.colorScheme.surfaceContainer, tonalElevation = 3.dp) {
        Column(Modifier.navigationBarsPadding()) {
            LinearProgressIndicator(
                progress = { if (state.durationMs > 0) (state.positionMs.toFloat() / state.durationMs).coerceIn(0f, 1f) else 0f },
                modifier = Modifier.fillMaxWidth().height(2.dp),
                drawStopIndicator = {},
            )
            Row(
                Modifier.fillMaxWidth().clickable(onClick = onOpen).padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Cover(track, Modifier.size(48.dp), corner = 6.dp)
                Column(Modifier.weight(1f)) {
                    Text(track.title, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    track.artist?.let {
                        Text(
                            it,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                PlayPauseButton(state, onTogglePlay, filled = false)
                IconButton(onClick = onNext) {
                    Icon(Icons.Default.SkipNext, contentDescription = stringResource(R.string.player_next))
                }
            }
        }
    }
}

/** Cover, title, progress, like and ban, and the next tracks in the queue. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NowPlayingSheet(
    state: PlayerUiState,
    onDismiss: () -> Unit,
    onTogglePlay: () -> Unit,
    onNext: () -> Unit,
    onSeek: (Long) -> Unit,
    onToggleLike: () -> Unit,
    onBan: () -> Unit,
) {
    val track = state.current ?: return
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)) {
        LazyColumn(Modifier.fillMaxWidth().padding(horizontal = 24.dp)) {
            item {
                Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
                    Cover(track, Modifier.widthIn(max = 360.dp).fillMaxWidth().aspectRatio(1f), corner = 12.dp)
                    Spacer(Modifier.height(20.dp))
                    Text(
                        track.title,
                        style = MaterialTheme.typography.titleLarge,
                        textAlign = TextAlign.Center,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                    track.artist?.let {
                        Text(
                            it,
                            style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            textAlign = TextAlign.Center,
                        )
                    }
                    Spacer(Modifier.height(12.dp))
                    SeekBar(state, onSeek)
                    Row(
                        Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceEvenly,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        IconButton(onClick = onBan) {
                            Icon(Icons.Default.Block, contentDescription = stringResource(R.string.player_ban))
                        }
                        PlayPauseButton(state, onTogglePlay, filled = true)
                        IconButton(onClick = onNext) {
                            Icon(Icons.Default.SkipNext, contentDescription = stringResource(R.string.player_next))
                        }
                        IconButton(onClick = onToggleLike) {
                            Icon(
                                if (state.liked) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
                                contentDescription = stringResource(if (state.liked) R.string.player_unlike else R.string.player_like),
                                tint = if (state.liked) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface,
                            )
                        }
                    }
                    Spacer(Modifier.height(16.dp))
                }
            }
            if (state.upNext.isNotEmpty()) {
                item {
                    Text(
                        stringResource(R.string.player_up_next),
                        style = MaterialTheme.typography.titleMedium,
                        modifier = Modifier.padding(vertical = 8.dp),
                    )
                }
                items(state.upNext, key = { it.mediaId }) { next ->
                    Row(
                        Modifier.fillMaxWidth().padding(vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Cover(next, Modifier.size(40.dp), corner = 4.dp)
                        Column(Modifier.weight(1f)) {
                            Text(next.title, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            next.artist?.let {
                                Text(
                                    it,
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                            }
                        }
                    }
                }
            }
            item { Spacer(Modifier.height(24.dp)) }
        }
    }
}

@Composable
private fun SeekBar(state: PlayerUiState, onSeek: (Long) -> Unit) {
    var dragging by remember { mutableStateOf<Float?>(null) }
    val duration = state.durationMs
    Column(Modifier.fillMaxWidth()) {
        Slider(
            value = dragging ?: if (duration > 0) (state.positionMs.toFloat() / duration).coerceIn(0f, 1f) else 0f,
            onValueChange = { dragging = it },
            onValueChangeFinished = {
                dragging?.let { if (duration > 0) onSeek((it * duration).toLong()) }
                dragging = null
            },
            enabled = state.seekable && duration > 0,
        )
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            val shown = dragging?.let { (it * duration).toLong() } ?: state.positionMs
            Text(formatTime(shown), style = MaterialTheme.typography.labelSmall)
            Text(if (duration > 0) formatTime(duration) else "–:––", style = MaterialTheme.typography.labelSmall)
        }
    }
}

@Composable
private fun PlayPauseButton(state: PlayerUiState, onClick: () -> Unit, filled: Boolean) {
    val icon = if (state.isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow
    val label = stringResource(if (state.isPlaying) R.string.player_pause else R.string.player_play)
    if (filled) {
        FilledIconButton(onClick = onClick, modifier = Modifier.size(64.dp)) {
            Icon(icon, contentDescription = label, modifier = Modifier.size(36.dp))
        }
    } else {
        IconButton(onClick = onClick) { Icon(icon, contentDescription = label) }
    }
}

@Composable
private fun Cover(track: TrackUi, modifier: Modifier, corner: Dp) {
    val shape = RoundedCornerShape(corner)
    SubcomposeAsyncImage(
        model = track.artworkUri?.toString(),
        contentDescription = null,
        contentScale = ContentScale.Crop,
        modifier = modifier.clip(shape),
        loading = { CoverPlaceholder() },
        error = { CoverPlaceholder() },
    )
}

@Composable
private fun CoverPlaceholder() {
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.surfaceContainerHigh), contentAlignment = Alignment.Center) {
        Icon(Icons.Default.MusicNote, contentDescription = null, tint = MaterialTheme.colorScheme.outlineVariant)
    }
}

private fun formatTime(ms: Long): String {
    val total = ms / 1000
    return "%d:%02d".format(total / 60, total % 60)
}
