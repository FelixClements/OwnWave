package io.github.felixclements.ownwave.ui.home

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.auth.AccountRepository
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.auth.SessionStore
import io.github.felixclements.ownwave.data.StationRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class HomeUiState(
    val loading: Boolean = true,
    val failed: Boolean = false,
    val yourStations: List<Station> = emptyList(),
    val genreStations: List<Station> = emptyList(),
)

class HomeViewModel(
    private val session: Session,
    private val stations: StationRepository,
    private val account: AccountRepository,
    private val sessionStore: SessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(HomeUiState())
    val state: StateFlow<HomeUiState> = _state.asStateFlow()

    init {
        refresh()
    }

    fun refresh() {
        _state.update { it.copy(loading = true, failed = false) }
        viewModelScope.launch {
            stations.stations(session)
                .onSuccess { list ->
                    val (genre, other) = list.partition { it.kind == Station.Kind.GENRE }
                    _state.value = HomeUiState(loading = false, yourStations = other, genreStations = genre)
                }
                .onFailure { _state.update { it.copy(loading = false, failed = true) } }
        }
    }

    fun coverUrl(stationId: String): String = stations.coverUrl(session, stationId)

    fun signOut() {
        viewModelScope.launch {
            account.logout(session)
            sessionStore.clear()
        }
    }
}
