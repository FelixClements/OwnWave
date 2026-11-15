package io.github.felixclements.ownwave.ui.settings

import androidx.annotation.StringRes
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
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ExpandMore
import androidx.compose.material.icons.filled.SignalCellularAlt
import androidx.compose.material.icons.filled.Wifi
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.felixclements.ownwave.BuildConfig
import io.github.felixclements.ownwave.R
import io.github.felixclements.ownwave.api.models.ChangePasswordRequest
import io.github.felixclements.ownwave.api.models.Quality
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.container
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.playback.QualityPolicy
import io.github.felixclements.ownwave.ui.common.CardBox
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.OwTextField
import io.github.felixclements.ownwave.ui.common.PagePad
import io.github.felixclements.ownwave.ui.common.PageTitle
import io.github.felixclements.ownwave.ui.common.TopSpacer
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.common.rememberMutation
import io.github.felixclements.ownwave.ui.rememberPageListState
import io.github.felixclements.ownwave.ui.theme.Ow
import kotlinx.coroutines.launch

/** apps/web/src/pages/Settings.tsx: streaming quality, signed-in apps, account. */
@Composable
fun SettingsScreen(session: Session) {
    val app = LocalContext.current.container
    val settings by app.settingsStore.settings.collectAsStateWithLifecycle()
    val scope = rememberCoroutineScope()

    LazyColumn(state = rememberPageListState(), modifier = Modifier.fillMaxSize()) {
        item(key = "body") {
            Column(Modifier.padding(horizontal = PagePad), verticalArrangement = Arrangement.spacedBy(24.dp)) {
                TopSpacer()
                PageTitle("Settings", Modifier.padding(top = 8.dp))
                CardBox(
                    "Streaming quality",
                    hint = "Lower qualities are converted once on the server, then cached. Changes apply from the next track.",
                ) {
                    Column(verticalArrangement = Arrangement.spacedBy(20.dp)) {
                        QualitySelect(Icons.Default.Wifi, stringResource(R.string.settings_quality_wifi), QualityPolicy.wifiOptions, settings.wifi) {
                            scope.launch { app.settingsStore.setWifiQuality(it) }
                        }
                        QualitySelect(Icons.Default.SignalCellularAlt, stringResource(R.string.settings_quality_mobile), QualityPolicy.mobileOptions, settings.mobile) {
                            scope.launch { app.settingsStore.setMobileQuality(it) }
                        }
                    }
                }
                CardBox("Signed-in apps", hint = "Android and Android Auto stay signed in until you sign them out here.") {
                    DevicesSection(session)
                }
                CardBox("Account", hint = stringResource(R.string.settings_signed_in_as, session.username, session.server.host)) {
                    Column(verticalArrangement = Arrangement.spacedBy(24.dp)) {
                        Password()
                        OwButton(stringResource(R.string.sign_out), { scope.launch { app.signOut(session) } }, variant = Variant.Danger)
                    }
                }
                Text(stringResource(R.string.settings_version, BuildConfig.VERSION_NAME), fontSize = 12.sp, color = Ow.Faint)
                Spacer(Modifier.height(16.dp))
            }
        }
    }
}

/** The web's Select: a field showing the choice, opening a menu of options. */
@Composable
private fun QualitySelect(icon: ImageVector, title: String, options: List<Quality>, selected: Quality, onSelect: (Quality) -> Unit) {
    var open by remember { mutableStateOf(false) }
    Column {
        Row(Modifier.padding(bottom = 8.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Icon(icon, null, tint = Ow.Muted, modifier = Modifier.size(16.dp))
            Text(title, fontSize = 13.sp, color = Ow.Muted)
        }
        Box {
            Row(
                Modifier
                    .fillMaxWidth()
                    .height(44.dp)
                    .background(Ow.Base, RoundedCornerShape(6.dp))
                    .border(1.dp, Ow.Line, RoundedCornerShape(6.dp))
                    .clickable { open = true }
                    .padding(horizontal = 14.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(stringResource(selected.label()), fontSize = 15.sp, modifier = Modifier.weight(1f))
                Icon(Icons.Default.ExpandMore, null, tint = Ow.Muted)
            }
            DropdownMenu(expanded = open, onDismissRequest = { open = false }, containerColor = Ow.Raised) {
                options.forEach { q ->
                    DropdownMenuItem(
                        text = {
                            Column {
                                Text(stringResource(q.label()), fontWeight = if (q == selected) FontWeight.Bold else FontWeight.Normal, color = if (q == selected) Ow.Accent else Ow.Ink)
                                Text(stringResource(q.detail()), fontSize = 12.sp, color = Ow.Muted)
                            }
                        },
                        onClick = {
                            open = false
                            onSelect(q)
                        },
                    )
                }
            }
        }
        Text(stringResource(selected.detail()), fontSize = 12.sp, color = Ow.Muted, modifier = Modifier.padding(top = 6.dp))
    }
}

@Composable
private fun Password() {
    var current by remember { mutableStateOf("") }
    var next by remember { mutableStateOf("") }
    var done by remember { mutableStateOf(false) }
    val change = rememberMutation(invalidate = false)
    fun submit() {
        if (current.isEmpty() || next.length < 8) return
        done = false
        change.run<Any?>(onSuccess = {
            current = ""
            next = ""
            done = true
        }) { apiCall { apis.auth.changePassword(ChangePasswordRequest(currentPassword = current, newPassword = next)) } }
    }
    Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
        OwTextField(current, { current = it }, label = "Current password", password = true)
        OwTextField(next, { next = it }, label = "New password", password = true, hint = "At least 8 characters. Signs out every other browser and app.", onDone = ::submit)
        ErrorText(change.error)
        if (done) Text("Password changed.", fontSize = 14.sp, color = Ow.Accent)
        OwButton("Change password", ::submit, variant = Variant.Primary, loading = change.pending, enabled = current.isNotEmpty() && next.length >= 8)
    }
}

@StringRes
private fun Quality.label(): Int = when (this) {
    Quality.ORIGINAL -> R.string.quality_original
    Quality.HIGH -> R.string.quality_high
    Quality.NORMAL -> R.string.quality_normal
    Quality.LOW -> R.string.quality_low
    Quality.AAC -> R.string.quality_aac
}

@StringRes
private fun Quality.detail(): Int = when (this) {
    Quality.ORIGINAL -> R.string.quality_original_detail
    Quality.HIGH -> R.string.quality_high_detail
    Quality.NORMAL -> R.string.quality_normal_detail
    Quality.LOW -> R.string.quality_low_detail
    Quality.AAC -> R.string.quality_aac_detail
}
