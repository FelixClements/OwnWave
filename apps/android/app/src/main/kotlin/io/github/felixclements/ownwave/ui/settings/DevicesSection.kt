package io.github.felixclements.ownwave.ui.settings

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.github.felixclements.ownwave.R
import io.github.felixclements.ownwave.api.models.Device
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.container
import kotlinx.coroutines.launch
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

private sealed interface DevicesState {
    data object Loading : DevicesState
    data object Failed : DevicesState
    data class Loaded(val devices: List<Device>) : DevicesState
}

/** The account's signed-in devices. Other devices can be signed out here; this one uses "Sign out". */
@Composable
fun DevicesSection(session: Session) {
    val app = LocalContext.current.container
    val scope = rememberCoroutineScope()
    var state by remember(session) { mutableStateOf<DevicesState>(DevicesState.Loading) }
    var reload by remember { mutableStateOf(0) }
    var confirm by remember { mutableStateOf<Device?>(null) }
    var revokeFailed by remember { mutableStateOf(false) }

    LaunchedEffect(session, reload) {
        state = DevicesState.Loading
        state = app.deviceRepository.devices(session).fold(
            onSuccess = { DevicesState.Loaded(it) },
            onFailure = { DevicesState.Failed },
        )
    }

    Column(Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
        Text(
            stringResource(R.string.devices_title),
            style = MaterialTheme.typography.titleMedium,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
        )
        when (val s = state) {
            DevicesState.Loading -> CircularProgressIndicator(Modifier.padding(16.dp).size(24.dp), strokeWidth = 2.dp)
            DevicesState.Failed -> Row(Modifier.padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(stringResource(R.string.devices_error), style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
                TextButton(onClick = { reload++ }) { Text(stringResource(R.string.retry)) }
            }
            is DevicesState.Loaded -> {
                s.devices.forEach { device -> DeviceRow(device, onSignOut = { confirm = device }) }
                if (revokeFailed) {
                    Text(
                        stringResource(R.string.devices_revoke_error),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                    )
                }
            }
        }
    }

    confirm?.let { device ->
        AlertDialog(
            onDismissRequest = { confirm = null },
            title = { Text(stringResource(R.string.devices_revoke_title, device.name)) },
            text = { Text(stringResource(R.string.devices_revoke_text)) },
            confirmButton = {
                TextButton(onClick = {
                    confirm = null
                    scope.launch {
                        val result = app.deviceRepository.revoke(session, device)
                        revokeFailed = result.isFailure
                        val current = state
                        if (result.isSuccess && current is DevicesState.Loaded) {
                            state = DevicesState.Loaded(current.devices.filterNot { it.id == device.id })
                        }
                    }
                }) { Text(stringResource(R.string.sign_out)) }
            },
            dismissButton = { TextButton(onClick = { confirm = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

@Composable
private fun DeviceRow(device: Device, onSignOut: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(start = 16.dp, end = 8.dp, top = 4.dp, bottom = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(device.name, style = MaterialTheme.typography.bodyLarge)
            Text(
                if (device.current) {
                    stringResource(R.string.devices_this_device)
                } else {
                    stringResource(R.string.devices_last_used, formatDate(device.lastUsedAt ?: device.createdAt))
                },
                style = MaterialTheme.typography.bodySmall,
                color = if (device.current) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (!device.current) {
            TextButton(onClick = onSignOut) { Text(stringResource(R.string.sign_out)) }
        }
    }
}

private val dateFormat: DateTimeFormatter = DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM)

private fun formatDate(time: OffsetDateTime): String = time.atZoneSameInstant(ZoneId.systemDefault()).format(dateFormat)
