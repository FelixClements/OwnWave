package io.github.felixclements.ownwave.ui.home

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Radio
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
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
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.api.models.DismissStationSuggestionRequest
import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.api.models.StationRequest
import io.github.felixclements.ownwave.api.models.StationSuggestion
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.data.apiBody
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.ui.LocalNavigator
import io.github.felixclements.ownwave.ui.Route
import io.github.felixclements.ownwave.ui.common.Art
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.Empty
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.LocalTopInset
import io.github.felixclements.ownwave.ui.common.MediaCard
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.OwIconButton
import io.github.felixclements.ownwave.ui.common.PagePad
import io.github.felixclements.ownwave.ui.common.PlayButton
import io.github.felixclements.ownwave.ui.common.SectionHeader
import io.github.felixclements.ownwave.ui.common.Spinner
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.common.cardGrid
import io.github.felixclements.ownwave.ui.common.rememberMutation
import io.github.felixclements.ownwave.ui.common.rememberQuery
import io.github.felixclements.ownwave.ui.common.rememberStations
import io.github.felixclements.ownwave.ui.player.LocalPlayer
import io.github.felixclements.ownwave.ui.rememberPageListState
import io.github.felixclements.ownwave.ui.stations.StationCard
import io.github.felixclements.ownwave.ui.stations.StationEditor
import io.github.felixclements.ownwave.ui.theme.Ow

/** apps/web/src/pages/Home.tsx: greeting, quick picks, recent, suggestions (admins), your stations. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(session: Session) {
    val app = LocalAppData.current
    val nav = LocalNavigator.current
    val stations = rememberStations()
    val recent = rememberQuery("browse:recent") { apiBody { apis.stations.browse("recent") } }
    val suggestions = rememberQuery("suggestions", enabled = session.isAdmin) { apiBody { apis.admin.listStationSuggestions() } }
    var creating by rememberSaveable { mutableStateOf(false) }
    val listState = rememberPageListState()

    val list = stations.data.orEmpty()
    val byId = list.associateBy { it.id }
    val recentStations = recent.data?.items.orEmpty().mapNotNull { byId[it.stationId] }.distinctBy { it.id }
    val quick = (recentStations + list.filter { it !in recentStations }).take(8)
    val sugg = suggestions.data?.suggestions.orEmpty()

    PullToRefreshBox(
        isRefreshing = stations.fetching && stations.data != null,
        onRefresh = app::invalidate,
        modifier = Modifier.fillMaxSize(),
    ) {
        LazyColumn(state = listState, modifier = Modifier.fillMaxSize()) {
            item(key = "top") {
                Column(
                    Modifier
                        .fillMaxWidth()
                        .background(Brush.verticalGradient(listOf(Ow.HomeWash.copy(alpha = 0.8f), Ow.HomeWash.copy(alpha = 0.3f))))
                        .padding(top = LocalTopInset.current + 16.dp, start = PagePad, end = PagePad),
                ) {
                    Text(Format.greeting(), style = MaterialTheme.typography.headlineLarge, modifier = Modifier.padding(bottom = 24.dp))
                    when {
                        stations.loading -> Spinner()
                        stations.data == null -> {
                            ErrorText(stations.error)
                            OwButton("Retry", stations.refresh, Modifier.padding(vertical = 16.dp))
                        }
                        list.isEmpty() -> Empty(
                            "No stations yet",
                            icon = Icons.Default.Radio,
                            body = if (session.isAdmin) {
                                "Add Spotify playlists on the Playlists page. Each main genre gets its own station once enough of their songs are in your library; you can also create stations or accept a suggestion below."
                            } else {
                                "Create a station from the synced playlists. Genre stations appear by themselves once enough playlist songs are in the library."
                            },
                            action = { OwButton("Create a station", { creating = true }, variant = Variant.Primary) },
                        )
                        else -> Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            quick.forEach { QuickTile(it) }
                        }
                    }
                }
            }
            item(key = "fade") {
                Spacer(
                    Modifier
                        .fillMaxWidth()
                        .height(40.dp)
                        .background(Brush.verticalGradient(listOf(Ow.HomeWash.copy(alpha = 0.3f), Color.Transparent))),
                )
            }
            if (recentStations.isNotEmpty()) {
                item(key = "recent-h") { SectionHeader("Jump back in", Modifier.padding(horizontal = PagePad)) }
                cardGrid(recentStations.take(6), key = { "r-${it.id}" }) { StationCard(it) }
                item(key = "recent-s") { Spacer(Modifier.height(40.dp)) }
            }
            if (sugg.isNotEmpty()) {
                item(key = "sugg-h") {
                    SectionHeader("Suggested stations", Modifier.padding(horizontal = PagePad)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.AutoAwesome, null, tint = Ow.Muted, modifier = Modifier.size(16.dp))
                            Text(" One per playlist", fontSize = 13.sp, color = Ow.Muted)
                        }
                    }
                }
                cardGrid(sugg.take(12), key = { "s-${it.playlistIds.joinToString(",")}" }) { SuggestionCard(it) }
                item(key = "sugg-s") { Spacer(Modifier.height(40.dp)) }
            }
            if (list.isNotEmpty()) {
                item(key = "yours-h") {
                    SectionHeader("Your stations", Modifier.padding(horizontal = PagePad)) {
                        Text(
                            "Show all",
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Bold,
                            color = Ow.Muted,
                            modifier = Modifier.clip(RoundedCornerShape(4.dp)).clickable { nav.push(Route.Stations) }.padding(4.dp),
                        )
                    }
                }
                cardGrid(list.take(12), key = { "y-${it.id}" }) { StationCard(it) }
                item(key = "end") { Spacer(Modifier.height(40.dp)) }
            }
        }
    }
    if (creating) StationEditor(onDismiss = { creating = false })
}

/** Wide tile for the quick-pick list at the top, like a shortcut. */
@Composable
private fun QuickTile(station: Station) {
    val nav = LocalNavigator.current
    val player = LocalPlayer.current
    val app = LocalAppData.current
    val active = player.isActive(station.id)
    Row(
        Modifier
            .fillMaxWidth()
            .height(64.dp)
            .clip(RoundedCornerShape(6.dp))
            .background(Color.White.copy(alpha = 0.07f))
            .clickable { nav.push(Route.Station(station.id)) },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Art(app.stationCover(station.id), station.name, Modifier.size(64.dp), RectangleShape)
        Text(
            station.name,
            fontWeight = FontWeight.Bold,
            fontSize = 15.sp,
            color = if (active) Ow.Accent else Ow.Ink,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f).padding(horizontal = 12.dp),
        )
        if (active) {
            PlayButton(
                playing = player.isPlaying(station.id),
                label = "Play ${station.name}",
                onClick = { player.playOrToggle(station.id, station.name) },
                size = 40.dp,
                modifier = Modifier.padding(end = 12.dp),
            )
        }
    }
}

@Composable
private fun SuggestionCard(s: StationSuggestion) {
    val app = LocalAppData.current
    val nav = LocalNavigator.current
    val save = rememberMutation()
    val dismiss = rememberMutation()
    MediaCard(
        art = s.playlistIds.firstOrNull()?.let(app::playlistCover),
        name = s.name,
        subtitle = "Playlist · ${Format.plural(s.trackCount, "track")}",
        onClick = {},
        overlay = {
            OwIconButton(
                Icons.Default.Close,
                "Dismiss ${s.name}",
                { dismiss.run { apiCall { apis.admin.dismissStationSuggestion(DismissStationSuggestionRequest(playlistIds = s.playlistIds)) } } },
                modifier = Modifier.align(Alignment.TopEnd).padding(4.dp),
                size = 32.dp,
                iconSize = 16.dp,
                tint = Ow.Ink,
                background = Color.Black.copy(alpha = 0.6f),
            )
        },
        footer = {
            Box {
                OwButton(
                    "Create station",
                    {
                        save.run<Station?>(onSuccess = { st -> st?.let { nav.push(Route.Station(it.id)) } }) {
                            apiCall { apis.stations.createStation(StationRequest(name = s.name, playlistIds = s.playlistIds, genres = emptyList())) }
                        }
                    },
                    variant = Variant.Primary,
                    small = true,
                    loading = save.pending,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            ErrorText(save.error ?: dismiss.error)
        },
    )
}
