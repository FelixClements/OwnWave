package io.github.felixclements.ownwave.ui

import androidx.activity.compose.BackHandler
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.ui.home.HomeScreen
import io.github.felixclements.ownwave.ui.player.MiniPlayer
import io.github.felixclements.ownwave.ui.player.NowPlayingSheet
import io.github.felixclements.ownwave.ui.player.PlayerViewModel
import io.github.felixclements.ownwave.ui.settings.SettingsScreen

/** Home or Settings, with the mini player and the now playing sheet on top. */
@Composable
fun SignedInApp(session: Session) {
    val player: PlayerViewModel = viewModel()
    val playerState by player.state.collectAsStateWithLifecycle()
    var showSettings by rememberSaveable { mutableStateOf(false) }
    var showNowPlaying by rememberSaveable { mutableStateOf(false) }

    if (showSettings) {
        BackHandler { showSettings = false }
        SettingsScreen(session, onBack = { showSettings = false })
    } else {
        HomeScreen(
            session = session,
            onPlayStation = player::play,
            onOpenSettings = { showSettings = true },
            bottomBar = {
                MiniPlayer(
                    playerState,
                    onOpen = { showNowPlaying = true },
                    onTogglePlay = player::togglePlay,
                    onNext = player::next,
                )
            },
        )
    }

    if (showNowPlaying && playerState.current != null) {
        NowPlayingSheet(
            playerState,
            onDismiss = { showNowPlaying = false },
            onTogglePlay = player::togglePlay,
            onNext = player::next,
            onSeek = player::seekTo,
            onToggleLike = player::toggleLike,
            onBan = player::ban,
        )
    }
}
