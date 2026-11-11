package io.github.felixclements.ownwave.ui.login

import android.os.Build
import androidx.annotation.StringRes
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import io.github.felixclements.ownwave.R
import io.github.felixclements.ownwave.auth.AccountRepository
import io.github.felixclements.ownwave.auth.LoginResult
import io.github.felixclements.ownwave.auth.SessionStore
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class LoginUiState(
    val server: String = "",
    val username: String = "",
    val password: String = "",
    val busy: Boolean = false,
    @StringRes val error: Int? = null,
    val errorDetail: String? = null,
) {
    val canSubmit: Boolean get() = !busy && server.isNotBlank() && username.isNotBlank() && password.isNotEmpty()
}

class LoginViewModel(
    private val account: AccountRepository,
    private val sessionStore: SessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(LoginUiState())
    val state: StateFlow<LoginUiState> = _state.asStateFlow()

    fun onServer(value: String) = _state.update { it.copy(server = value, error = null) }
    fun onUsername(value: String) = _state.update { it.copy(username = value, error = null) }
    fun onPassword(value: String) = _state.update { it.copy(password = value, error = null) }

    fun submit() {
        val s = _state.value
        if (!s.canSubmit) return
        _state.update { it.copy(busy = true, error = null, errorDetail = null) }
        viewModelScope.launch {
            val result = account.login(s.server, s.username, s.password, deviceName = Build.MODEL ?: "Android")
            if (result is LoginResult.Success) {
                sessionStore.save(result.session)
                _state.update { it.copy(busy = false, password = "") }
                return@launch
            }
            _state.update {
                it.copy(
                    busy = false,
                    error = result.message(),
                    errorDetail = (result as? LoginResult.Network)?.message
                        ?: (result as? LoginResult.ServerError)?.code?.let { code -> "HTTP $code" },
                )
            }
        }
    }

    @StringRes
    private fun LoginResult.message(): Int = when (this) {
        LoginResult.InvalidUrl -> R.string.login_error_invalid_url
        LoginResult.InsecureUrl -> R.string.login_error_insecure_url
        LoginResult.BadCredentials -> R.string.login_error_credentials
        LoginResult.RateLimited -> R.string.login_error_rate_limited
        LoginResult.NotOwnWave -> R.string.login_error_not_ownwave
        is LoginResult.ServerError -> R.string.login_error_server
        is LoginResult.Network -> R.string.login_error_network
        is LoginResult.Success -> error("not an error")
    }
}
