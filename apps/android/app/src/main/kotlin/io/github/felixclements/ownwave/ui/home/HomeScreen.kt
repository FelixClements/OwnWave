package io.github.felixclements.ownwave.ui.home

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Radio
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import coil3.compose.SubcomposeAsyncImage
import io.github.felixclements.ownwave.R
import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.container

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(session: Session) {
    val app = LocalContext.current.container
    val vm: HomeViewModel = viewModel(
        key = session.server.toString() + session.userId,
        factory = viewModelFactory {
            initializer { HomeViewModel(session, app.stationRepository, app.accountRepository, app.sessionStore) }
        },
    )
    val state by vm.state.collectAsStateWithLifecycle()
    var menuOpen by remember { mutableStateOf(false) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.app_name)) },
                actions = {
                    IconButton(onClick = { menuOpen = true }) {
                        Icon(Icons.Default.MoreVert, contentDescription = stringResource(R.string.menu))
                    }
                    DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                        DropdownMenuItem(
                            text = { Text(stringResource(R.string.sign_out_user, session.username)) },
                            onClick = {
                                menuOpen = false
                                vm.signOut()
                            },
                        )
                    }
                },
            )
        },
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = state.loading,
            onRefresh = vm::refresh,
            modifier = Modifier.fillMaxSize().padding(padding),
        ) {
            when {
                state.failed -> Message(stringResource(R.string.stations_error), action = vm::refresh)
                !state.loading && state.yourStations.isEmpty() && state.genreStations.isEmpty() ->
                    Message(stringResource(R.string.stations_empty), action = null)
                else -> StationGrid(state, coverUrl = vm::coverUrl)
            }
        }
    }
}

@Composable
private fun StationGrid(state: HomeUiState, coverUrl: (String) -> String) {
    LazyVerticalGrid(
        columns = GridCells.Adaptive(minSize = 150.dp),
        contentPadding = PaddingValues(16.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        modifier = Modifier.fillMaxSize(),
    ) {
        section(R.string.section_your_stations, state.yourStations, coverUrl)
        section(R.string.section_genre_stations, state.genreStations, coverUrl)
    }
}

private fun androidx.compose.foundation.lazy.grid.LazyGridScope.section(
    title: Int,
    stations: List<Station>,
    coverUrl: (String) -> String,
) {
    if (stations.isEmpty()) return
    item(span = { GridItemSpan(maxLineSpan) }, key = "header-$title") {
        Text(
            stringResource(title),
            style = MaterialTheme.typography.titleLarge,
            modifier = Modifier.padding(top = 8.dp),
        )
    }
    items(stations, key = { it.id }) { station -> StationCard(station, coverUrl(station.id)) }
}

@Composable
private fun StationCard(station: Station, cover: String) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        val shape = RoundedCornerShape(8.dp)
        SubcomposeAsyncImage(
            model = cover,
            contentDescription = null,
            contentScale = ContentScale.Crop,
            modifier = Modifier.fillMaxWidth().aspectRatio(1f).clip(shape),
            loading = { CoverPlaceholder() },
            error = { CoverPlaceholder() },
        )
        Text(
            station.name,
            style = MaterialTheme.typography.titleSmall,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Text(
            pluralStringResource(R.plurals.song_count, station.trackCount, station.trackCount),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun CoverPlaceholder() {
    Box(
        Modifier.fillMaxSize().background(MaterialTheme.colorScheme.surfaceContainerHigh),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            Icons.Default.Radio,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.outlineVariant,
            modifier = Modifier.size(40.dp),
        )
    }
}

@Composable
private fun Message(text: String, action: (() -> Unit)?) {
    // Scrollable so pull-to-refresh still works on the message.
    LazyVerticalGrid(columns = GridCells.Fixed(1), modifier = Modifier.fillMaxSize()) {
        item {
            Column(
                Modifier.fillMaxWidth().padding(32.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                Text(text, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (action != null) Button(onClick = action) { Text(stringResource(R.string.retry)) }
            }
        }
    }
}
