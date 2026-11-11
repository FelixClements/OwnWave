package io.github.felixclements.ownwave

import android.os.Bundle
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
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.felixclements.ownwave.auth.SessionState
import io.github.felixclements.ownwave.ui.home.HomeScreen
import io.github.felixclements.ownwave.ui.login.LoginScreen
import io.github.felixclements.ownwave.ui.theme.OwnWaveTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
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
                        is SessionState.SignedIn -> HomeScreen(session = s.session)
                    }
                }
            }
        }
    }
}
