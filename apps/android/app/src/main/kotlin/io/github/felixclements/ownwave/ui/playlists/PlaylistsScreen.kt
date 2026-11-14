package io.github.felixclements.ownwave.ui.playlists

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.QueueMusic
import androidx.compose.material.icons.filled.Add
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.api.models.AddPlaylistRequest
import io.github.felixclements.ownwave.api.models.Playlist
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.ui.LocalNavigator
import io.github.felixclements.ownwave.ui.Route
import io.github.felixclements.ownwave.ui.common.AppData
import io.github.felixclements.ownwave.ui.common.Empty
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.MediaCard
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.OwTextField
import io.github.felixclements.ownwave.ui.common.PagePad
import io.github.felixclements.ownwave.ui.common.PageTitle
import io.github.felixclements.ownwave.ui.common.Spinner
import io.github.felixclements.ownwave.ui.common.TopSpacer
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.common.cardGrid
import io.github.felixclements.ownwave.ui.common.percent
import io.github.felixclements.ownwave.ui.common.rememberMutation
import io.github.felixclements.ownwave.ui.common.rememberPlaylists
import io.github.felixclements.ownwave.ui.rememberPageListState
import io.github.felixclements.ownwave.ui.theme.Ow

fun Playlist.matchPct() = percent(matchedCount, trackCount)

fun AppData.playlistArt(p: Playlist) = if (p.hasCover) playlistCover(p.id) else null

/** apps/web/src/pages/Playlists.tsx */
@Composable
fun PlaylistsScreen(session: Session) {
    val playlists = rememberPlaylists()
    val add = rememberMutation()
    var url by rememberSaveable { mutableStateOf("") }
    var adding by rememberSaveable { mutableStateOf(false) }
    val list = playlists.data.orEmpty()

    fun submit() {
        if (url.isBlank()) return
        add.run<Playlist?>(onSuccess = {
            url = ""
            adding = false
        }) { apiCall { apis.admin.addPlaylist(AddPlaylistRequest(url = url.trim())) } }
    }

    LazyColumn(state = rememberPageListState(), modifier = Modifier.fillMaxSize()) {
        item(key = "head") {
            Column(Modifier.padding(horizontal = PagePad)) {
                TopSpacer()
                PageTitle(
                    "Playlists",
                    Modifier.padding(top = 8.dp),
                    subtitle = "Public Spotify playlists, synced daily. They only grow: songs Spotify drops stay.",
                )
                if (session.isAdmin && !adding) {
                    OwButton("Add playlist", { adding = true }, Modifier.padding(top = 16.dp), variant = Variant.Primary, icon = Icons.Default.Add)
                }
                if (adding) {
                    Column(
                        Modifier
                            .padding(top = 16.dp)
                            .fillMaxWidth()
                            .clip(RoundedCornerShape(8.dp))
                            .background(Ow.Card)
                            .padding(16.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        OwTextField(
                            url,
                            { url = it },
                            label = "Spotify playlist link",
                            placeholder = "https://open.spotify.com/playlist/…",
                            keyboardType = KeyboardType.Uri,
                            imeAction = ImeAction.Go,
                            onDone = ::submit,
                        )
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            OwButton("Add and sync", ::submit, variant = Variant.Primary, loading = add.pending, enabled = url.isNotBlank())
                            OwButton("Cancel", { adding = false }, variant = Variant.Ghost)
                        }
                    }
                }
                ErrorText(add.error, Modifier.padding(top = 8.dp))
                Spacer(Modifier.height(16.dp))
            }
        }
        when {
            playlists.loading -> item(key = "spin") { Spinner() }
            list.isEmpty() -> item(key = "empty") {
                Empty(
                    "No playlists yet",
                    icon = Icons.AutoMirrored.Filled.QueueMusic,
                    body = playlists.error ?: if (session.isAdmin) "Paste a public Spotify playlist link to start." else "An admin adds Spotify playlists here.",
                )
            }
            else -> cardGrid(list, key = { it.id }) { PlaylistCard(it) }
        }
        item(key = "end") { Spacer(Modifier.height(32.dp)) }
    }
}

@Composable
private fun PlaylistCard(p: Playlist) {
    val app = LocalAppData.current
    val nav = LocalNavigator.current
    MediaCard(
        art = app.playlistArt(p),
        name = p.name,
        subtitle = null,
        onClick = { nav.push(Route.Playlist(p.id)) },
        overlay = {
            Text(
                "${p.matchPct()}% in library",
                fontSize = 11.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier
                    .align(Alignment.BottomStart)
                    .padding(8.dp)
                    .clip(CircleShape)
                    .background(Color.Black.copy(alpha = 0.7f))
                    .padding(horizontal = 8.dp, vertical = 2.dp),
            )
        },
        footer = {
            Text(
                buildAnnotatedString {
                    append("${p.matchedCount} of ${p.trackCount} tracks")
                    when {
                        p.lastSyncError != null -> withStyle(SpanStyle(color = Ow.Danger)) { append(" · sync failed") }
                        p.lastSyncedAt != null -> append(" · synced ${Format.timeAgo(p.lastSyncedAt!!)}")
                    }
                },
                fontSize = 13.sp,
                color = Ow.Muted,
            )
        },
    )
}

