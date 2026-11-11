package io.github.felixclements.ownwave.ui.settings

import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.felixclements.ownwave.BuildConfig
import io.github.felixclements.ownwave.R
import io.github.felixclements.ownwave.api.models.Quality
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.container
import io.github.felixclements.ownwave.playback.QualityPolicy
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(session: Session, onBack: () -> Unit) {
    val app = LocalContext.current.container
    val settings by app.settingsStore.settings.collectAsStateWithLifecycle()
    val scope = rememberCoroutineScope()

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.settings)) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Default.ArrowBack, contentDescription = stringResource(R.string.back))
                    }
                },
            )
        },
    ) { padding ->
        Column(Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState())) {
            QualityGroup(R.string.settings_quality_wifi, QualityPolicy.wifiOptions, settings.wifi) {
                scope.launch { app.settingsStore.setWifiQuality(it) }
            }
            HorizontalDivider()
            QualityGroup(R.string.settings_quality_mobile, QualityPolicy.mobileOptions, settings.mobile) {
                scope.launch { app.settingsStore.setMobileQuality(it) }
            }
            HorizontalDivider()
            DevicesSection(session)
            HorizontalDivider()
            Column(Modifier.padding(16.dp)) {
                Text(
                    stringResource(R.string.settings_signed_in_as, session.username, session.server.host),
                    style = MaterialTheme.typography.bodyMedium,
                )
                OutlinedButton(onClick = { scope.launch { app.signOut(session) } }, modifier = Modifier.padding(top = 12.dp)) {
                    Text(stringResource(R.string.sign_out))
                }
                Text(
                    stringResource(R.string.settings_version, BuildConfig.VERSION_NAME),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 24.dp),
                )
            }
        }
    }
}

@Composable
private fun QualityGroup(@StringRes title: Int, options: List<Quality>, selected: Quality, onSelect: (Quality) -> Unit) {
    Column(Modifier.fillMaxWidth().padding(vertical = 8.dp).selectableGroup()) {
        Text(
            stringResource(title),
            style = MaterialTheme.typography.titleMedium,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
        )
        options.forEach { quality ->
            Row(
                Modifier
                    .fillMaxWidth()
                    .selectable(selected = quality == selected, onClick = { onSelect(quality) }, role = Role.RadioButton)
                    .padding(horizontal = 16.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                RadioButton(selected = quality == selected, onClick = null)
                Column(Modifier.padding(start = 16.dp)) {
                    Text(stringResource(quality.label()), style = MaterialTheme.typography.bodyLarge)
                    Text(
                        stringResource(quality.detail()),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }
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
