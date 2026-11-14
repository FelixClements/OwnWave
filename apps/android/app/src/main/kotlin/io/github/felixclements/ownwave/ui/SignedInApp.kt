package io.github.felixclements.ownwave.ui

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.automirrored.outlined.QueueMusic
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Shield
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.LibraryMusic
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Shield
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.MutableState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.container
import io.github.felixclements.ownwave.ui.admin.AdminScreen
import io.github.felixclements.ownwave.ui.common.AppData
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.LocalTopInset
import io.github.felixclements.ownwave.ui.home.HomeScreen
import io.github.felixclements.ownwave.ui.library.LibraryScreen
import io.github.felixclements.ownwave.ui.player.LocalPlayer
import io.github.felixclements.ownwave.ui.player.MiniPlayer
import io.github.felixclements.ownwave.ui.player.NowPlaying
import io.github.felixclements.ownwave.ui.player.PlayerHandle
import io.github.felixclements.ownwave.ui.player.PlayerViewModel
import io.github.felixclements.ownwave.ui.playlists.PlaylistScreen
import io.github.felixclements.ownwave.ui.playlists.PlaylistsScreen
import io.github.felixclements.ownwave.ui.search.SearchScreen
import io.github.felixclements.ownwave.ui.settings.SettingsScreen
import io.github.felixclements.ownwave.ui.stations.GenreScreen
import io.github.felixclements.ownwave.ui.stations.StationScreen
import io.github.felixclements.ownwave.ui.stations.StationsScreen
import io.github.felixclements.ownwave.ui.theme.Ow
import kotlinx.coroutines.launch

/** Whether the page has scrolled under the top bar, which then darkens (the web's TopBar). */
private val LocalScrolled = compositionLocalOf<MutableState<Boolean>> { mutableStateOf(false) }

/** A list state for a page, reporting to the top bar when it scrolls. */
@Composable
fun rememberPageListState(): LazyListState {
    val state = rememberLazyListState()
    val scrolled = LocalScrolled.current
    val past by remember { derivedStateOf { state.firstVisibleItemIndex > 0 || state.firstVisibleItemScrollOffset > 40 } }
    LaunchedEffect(past) { scrolled.value = past }
    return state
}

/**
 * The signed-in app, laid out like the web on a phone: the page under a top bar with the logo and
 * the account menu, then the mini player and the bottom nav. Now playing opens over everything.
 */
@Composable
fun SignedInApp(session: Session) {
    val context = LocalContext.current
    val app = context.container
    val appData = remember(session) { AppData(session, app.apiFactory.forServer(session.server)) }
    val vm: PlayerViewModel = viewModel()
    val playerState = vm.state.collectAsStateWithLifecycle()
    var nowPlaying by rememberSaveable { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val player = remember(vm, appData) { PlayerHandle(vm, playerState, appData, scope) { nowPlaying = true } }
    val nav = rememberNavigator()
    val scrolled = remember { mutableStateOf(false) }
    val statusBar = WindowInsets.statusBars.asPaddingValues().calculateTopPadding()

    BackHandler(enabled = nav.canGoBack && !nowPlaying) { nav.back() }

    CompositionLocalProvider(
        LocalAppData provides appData,
        LocalPlayer provides player,
        LocalNavigator provides nav,
        LocalScrolled provides scrolled,
        LocalTopInset provides statusBar + 64.dp,
    ) {
        Column(Modifier.fillMaxSize().background(Ow.Base)) {
            Box(Modifier.weight(1f).fillMaxWidth().background(Ow.Panel)) {
                val route = nav.current
                // A new page starts unscrolled.
                LaunchedEffect(route) { scrolled.value = false }
                Page(route, session)
                TopBar(session, scrolled.value)
            }
            Spacer(Modifier.height(8.dp))
            MiniPlayer()
            BottomNav(session.isAdmin)
        }
        AnimatedVisibility(
            visible = nowPlaying && playerState.value.current != null,
            enter = slideInVertically { it },
            exit = slideOutVertically { it },
        ) {
            NowPlaying(onClose = { nowPlaying = false })
        }
    }
}

@Composable
private fun Page(route: Route, session: Session) {
    when (route) {
        Route.Home -> HomeScreen(session)
        Route.Search -> SearchScreen()
        Route.Library -> LibraryScreen()
        Route.Playlists -> PlaylistsScreen(session)
        Route.Admin -> if (session.isAdmin) AdminScreen(session) else HomeScreen(session)
        Route.Stations -> StationsScreen()
        Route.Settings -> SettingsScreen(session)
        is Route.Station -> StationScreen(route.id, session)
        is Route.Playlist -> PlaylistScreen(route.id, session)
        is Route.Genre -> GenreScreen(route.id)
    }
}

@Composable
private fun TopBar(session: Session, scrolled: Boolean) {
    val bg by animateColorAsState(if (scrolled) Ow.Panel.copy(alpha = 0.95f) else Color.Transparent, label = "topbar")
    Row(
        Modifier
            .fillMaxWidth()
            .background(bg)
            .statusBarsPadding()
            .height(64.dp)
            .padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Logo(Modifier.weight(1f))
        UserMenu(session)
    }
}

/** The OwnWave wave mark and name (apps/web/src/components/Shell.tsx `Logo`). */
@Composable
fun Logo(modifier: Modifier = Modifier, markSize: Dp = 29.dp, textSize: Int = 18) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Canvas(Modifier.size(markSize)) {
            val k = size.width / 64f
            val path = Path().apply {
                // M6 34c5 0 6-14 11-14s6 24 11 24 6-32 11-32 6 24 11 24 7-8 8-8 (the web's SVG, in 64x64 units).
                moveTo(6 * k, 34 * k)
                cubicTo(11 * k, 34 * k, 12 * k, 20 * k, 17 * k, 20 * k)
                cubicTo(22 * k, 20 * k, 23 * k, 44 * k, 28 * k, 44 * k)
                cubicTo(33 * k, 44 * k, 34 * k, 12 * k, 39 * k, 12 * k)
                cubicTo(44 * k, 12 * k, 45 * k, 36 * k, 50 * k, 36 * k)
                cubicTo(55 * k, 36 * k, 57 * k, 28 * k, 58 * k, 28 * k)
            }
            drawPath(
                path,
                brush = Brush.linearGradient(listOf(Ow.Accent, Ow.Sky), start = Offset.Zero, end = Offset(size.width, size.height)),
                style = Stroke(width = 6 * k, cap = StrokeCap.Round, join = StrokeJoin.Round),
            )
        }
        Text("OwnWave", fontSize = textSize.sp, fontWeight = FontWeight.Black, letterSpacing = (-0.4).sp)
    }
}

@Composable
private fun UserMenu(session: Session) {
    val nav = LocalNavigator.current
    val app = LocalContext.current.container
    val scope = rememberCoroutineScope()
    var open by remember { mutableStateOf(false) }
    Box {
        Box(
            Modifier
                .size(40.dp)
                .border(4.dp, Color.Black.copy(alpha = 0.4f), CircleShape)
                .padding(4.dp)
                .clip(CircleShape)
                .background(Ow.Accent.copy(alpha = 0.9f))
                .clickable(role = Role.Button) { open = true }
                .semantics { contentDescription = "Account: ${session.username}" },
            contentAlignment = Alignment.Center,
        ) {
            Text(session.username.take(1).uppercase(), color = Color.Black, fontWeight = FontWeight.Bold, fontSize = 14.sp)
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }, containerColor = Ow.Raised) {
            Text(
                "Signed in as ${session.username}",
                fontSize = 12.sp,
                color = Ow.Muted,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            )
            MenuItem("Settings", Icons.Default.Settings) {
                open = false
                nav.push(Route.Settings)
            }
            if (session.isAdmin) {
                MenuItem("Admin", Icons.Default.Shield) {
                    open = false
                    nav.tab(Route.Admin)
                }
            }
            HorizontalDivider(color = Ow.Line)
            MenuItem("Log out", Icons.AutoMirrored.Filled.Logout) {
                open = false
                scope.launch { app.signOut(session) }
            }
        }
    }
}

@Composable
private fun MenuItem(text: String, icon: ImageVector, onClick: () -> Unit) {
    DropdownMenuItem(
        text = { Text(text, fontSize = 14.sp) },
        leadingIcon = { Icon(icon, contentDescription = null, modifier = Modifier.size(16.dp)) },
        onClick = onClick,
    )
}

private data class NavItem(val route: Route, val label: String, val icon: ImageVector)

@Composable
private fun BottomNav(isAdmin: Boolean) {
    val nav = LocalNavigator.current
    val items = buildList {
        add(NavItem(Route.Home, "Home", Icons.Outlined.Home))
        add(NavItem(Route.Search, "Search", Icons.Outlined.Search))
        add(NavItem(Route.Library, "Library", Icons.Outlined.LibraryMusic))
        add(NavItem(Route.Playlists, "Playlists", Icons.AutoMirrored.Outlined.QueueMusic))
        if (isAdmin) add(NavItem(Route.Admin, "Admin", Icons.Outlined.Shield))
    }
    Row(Modifier.fillMaxWidth().navigationBarsPadding().height(56.dp)) {
        items.forEach { item ->
            val on = nav.root == item.route
            val color = if (on) Ow.Ink else Ow.Muted
            Column(
                Modifier
                    .weight(1f)
                    .fillMaxSize()
                    .clickable(role = Role.Tab) { nav.tab(item.route) },
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center,
            ) {
                Icon(item.icon, contentDescription = null, tint = color, modifier = Modifier.size(22.dp))
                Spacer(Modifier.height(2.dp))
                Text(item.label, fontSize = 10.sp, color = color)
            }
        }
    }
}
