package io.github.felixclements.ownwave

import android.app.SearchManager
import android.content.ComponentName
import android.content.Intent
import android.os.Bundle
import android.provider.MediaStore
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.media3.common.MediaItem
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import com.google.common.util.concurrent.ListenableFuture
import io.github.felixclements.ownwave.auth.SessionState
import io.github.felixclements.ownwave.playback.PlaybackService
import io.github.felixclements.ownwave.ui.SignedInApp
import io.github.felixclements.ownwave.ui.login.LoginScreen
import io.github.felixclements.ownwave.ui.theme.OwnWaveTheme

class MainActivity : ComponentActivity() {
    /** A controller for a voice request, kept until the activity ends so the play command is not dropped. */
    private var voiceController: ListenableFuture<MediaController>? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        if (savedInstanceState == null) handleVoiceSearch(intent)
        val sessionStore = container.sessionStore
        setContent {
            OwnWaveTheme {
                Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
                    val state by sessionStore.state.collectAsStateWithLifecycle()
                    when (val s = state) {
                        SessionState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                            CircularProgressIndicator()
                        }
                        SessionState.SignedOut -> LoginScreen()
                        is SessionState.SignedIn -> SignedInApp(session = s.session)
                    }
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleVoiceSearch(intent)
    }

    override fun onDestroy() {
        voiceController?.let(MediaController::releaseFuture)
        super.onDestroy()
    }

    /**
     * "Play techno on OwnWave" from the Assistant on the phone. The service picks the station
     * (see PlaybackService.onSetMediaItems); an empty query resumes the last one.
     */
    private fun handleVoiceSearch(intent: Intent?) {
        if (intent?.action != MediaStore.INTENT_ACTION_MEDIA_PLAY_FROM_SEARCH) return
        val query = intent.getStringExtra(SearchManager.QUERY).orEmpty()
        voiceController?.let(MediaController::releaseFuture)
        val future = MediaController.Builder(this, SessionToken(this, ComponentName(this, PlaybackService::class.java)))
            .buildAsync()
        voiceController = future
        future.addListener(
            {
                val controller = runCatching { future.get() }.getOrNull() ?: return@addListener
                controller.setMediaItem(
                    MediaItem.Builder()
                        .setRequestMetadata(MediaItem.RequestMetadata.Builder().setSearchQuery(query).build())
                        .build(),
                )
                controller.prepare()
                controller.play()
            },
            ContextCompat.getMainExecutor(this),
        )
    }
}
