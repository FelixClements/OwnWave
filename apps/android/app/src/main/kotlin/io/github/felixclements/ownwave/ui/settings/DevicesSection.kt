package io.github.felixclements.ownwave.ui.settings

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Smartphone
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.R
import io.github.felixclements.ownwave.api.models.Device
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.container
import io.github.felixclements.ownwave.ui.common.ConfirmDialog
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.Spinner
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.theme.Ow
import kotlinx.coroutines.launch

private sealed interface DevicesState {
    data object Loading : DevicesState
    data object Failed : DevicesState
    data class Loaded(val devices: List<Device>) : DevicesState
}

/** The account's signed-in apps. Other devices can be signed out here; this one uses "Sign out". */
@Composable
fun DevicesSection(session: Session) {
    val app = LocalContext.current.container
    val scope = rememberCoroutineScope()
    var state by remember(session) { mutableStateOf<DevicesState>(DevicesState.Loading) }
    var reload by remember { mutableIntStateOf(0) }
    var confirm by remember { mutableStateOf<Device?>(null) }
    var revoking by remember { mutableStateOf<String?>(null) }
    var revokeFailed by remember { mutableStateOf(false) }

    LaunchedEffect(session, reload) {
        state = DevicesState.Loading
        state = app.deviceRepository.devices(session).fold(
            onSuccess = { DevicesState.Loaded(it) },
            onFailure = { DevicesState.Failed },
        )
    }

    when (val s = state) {
        DevicesState.Loading -> Spinner()
        DevicesState.Failed -> Row(verticalAlignment = Alignment.CenterVertically) {
            Text(stringResource(R.string.devices_error), fontSize = 14.sp, color = Ow.Danger, modifier = Modifier.weight(1f))
            OwButton(stringResource(R.string.retry), { reload++ }, small = true)
        }
        is DevicesState.Loaded -> Column {
            if (s.devices.isEmpty()) Text("No apps signed in.", fontSize = 14.sp, color = Ow.Muted)
            s.devices.forEachIndexed { i, device ->
                if (i > 0) HorizontalDivider(color = Ow.Line)
                DeviceRow(device, revoking == device.id) { confirm = device }
            }
            if (revokeFailed) ErrorText(stringResource(R.string.devices_revoke_error))
        }
    }

    confirm?.let { device ->
        ConfirmDialog(
            text = stringResource(R.string.devices_revoke_title, device.name) + " " + stringResource(R.string.devices_revoke_text),
            confirm = stringResource(R.string.sign_out),
            onConfirm = {
                confirm = null
                revoking = device.id
                scope.launch {
                    val result = app.deviceRepository.revoke(session, device)
                    revoking = null
                    revokeFailed = result.isFailure
                    val current = state
                    if (result.isSuccess && current is DevicesState.Loaded) {
                        state = DevicesState.Loaded(current.devices.filterNot { it.id == device.id })
                    }
                }
            },
            onDismiss = { confirm = null },
        )
    }
}

@Composable
private fun DeviceRow(device: Device, revoking: Boolean, onSignOut: () -> Unit) {
    Row(Modifier.fillMaxWidth().padding(vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        Icon(Icons.Default.Smartphone, null, tint = Ow.Muted, modifier = Modifier.size(20.dp))
        Column(Modifier.weight(1f).padding(horizontal = 16.dp)) {
            Text(
                buildAnnotatedString {
                    withStyle(SpanStyle(fontWeight = FontWeight.Medium)) { append(device.name) }
                    if (device.current) withStyle(SpanStyle(color = Ow.Accent, fontSize = 12.sp)) { append(" · this device") }
                },
                fontSize = 15.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                (device.lastUsedAt?.let { "Last used ${Format.timeAgo(it)}" } ?: "Never used") + " · signed in ${Format.timeAgo(device.createdAt)}",
                fontSize = 13.sp,
                color = Ow.Muted,
            )
        }
        if (!device.current) OwButton(stringResource(R.string.sign_out), onSignOut, variant = Variant.Danger, small = true, loading = revoking)
    }
}
