package io.github.felixclements.ownwave.auth

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/**
 * Keeps the signed-in session in DataStore, with the token encrypted by [TokenCipher].
 * [state] is in memory too, so the OkHttp interceptor can read it without suspending.
 */
class SessionStore(
    private val dataStore: DataStore<Preferences>,
    private val cipher: TokenCipher,
    scope: CoroutineScope,
) {
    private val _state = MutableStateFlow<SessionState>(SessionState.Loading)
    val state: StateFlow<SessionState> = _state.asStateFlow()

    /** The current session, or null while loading or signed out. */
    val current: Session? get() = (_state.value as? SessionState.SignedIn)?.session

    init {
        scope.launch { _state.value = load()?.let(SessionState::SignedIn) ?: SessionState.SignedOut }
    }

    suspend fun save(session: Session) {
        dataStore.edit {
            it[SERVER] = session.server.toString()
            it[TOKEN] = cipher.encrypt(session.token)
            it[USER_ID] = session.userId
            it[USERNAME] = session.username
            it[IS_ADMIN] = session.isAdmin
        }
        _state.value = SessionState.SignedIn(session)
    }

    suspend fun clear() {
        _state.value = SessionState.SignedOut
        dataStore.edit { it.clear() }
    }

    private suspend fun load(): Session? {
        val prefs = dataStore.data.first()
        val server = prefs[SERVER]?.toHttpUrlOrNull() ?: return null
        val token = prefs[TOKEN]?.let(cipher::decrypt) ?: return null
        return Session(
            server = server,
            token = token,
            userId = prefs[USER_ID] ?: return null,
            username = prefs[USERNAME] ?: return null,
            isAdmin = prefs[IS_ADMIN] ?: false,
        )
    }

    private companion object {
        val SERVER = stringPreferencesKey("server")
        val TOKEN = stringPreferencesKey("token")
        val USER_ID = stringPreferencesKey("user_id")
        val USERNAME = stringPreferencesKey("username")
        val IS_ADMIN = booleanPreferencesKey("is_admin")
    }
}
