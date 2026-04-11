package com.proxycoin.app.ui.dashboard

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.proxycoin.app.data.remote.ApiService
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class DashboardViewModel @Inject constructor(
    private val apiService: ApiService,
) : ViewModel() {

    data class UiState(
        val isConnected: Boolean = false,
        val todayEarnings: Double = 0.0,
        val allTimeEarnings: Double = 0.0,
        val bandwidthSharedBytes: Long = 0L,
        val uploadBytes: Long = 0L,
        val downloadBytes: Long = 0L,
        val uptimeSeconds: Long = 0L,
        val requestsServed: Int = 0,
        val avgLatencyMs: Int = 0,
        val trustScore: Float = 0f,
        val tokenPrice: Double? = null,
        val isLoading: Boolean = true,
        val errorMessage: String? = null,
    )

    private val _uiState = MutableStateFlow(UiState())
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    // Tracks uptime ticks when connected.
    private var uptimeJob: kotlinx.coroutines.Job? = null

    init {
        loadSummary()
    }

    fun toggleConnection() {
        val current = _uiState.value.isConnected
        val next = !current
        _uiState.update { it.copy(isConnected = next) }
        if (next) {
            startUptimeTicker()
        } else {
            stopUptimeTicker()
        }
    }

    fun refresh() {
        loadSummary()
    }

    private fun loadSummary() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessage = null) }
            try {
                val summary = apiService.getEarningsSummary()
                // Convert from smallest token units (1e-6 PRXY) to PRXY.
                val todayPrxy = summary.todayEarnings / 1_000_000.0
                val allTimePrxy = summary.allTimeEarnings / 1_000_000.0
                _uiState.update { state ->
                    state.copy(
                        todayEarnings = todayPrxy,
                        allTimeEarnings = allTimePrxy,
                        trustScore = summary.trustScore,
                        isLoading = false,
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        errorMessage = "Failed to load earnings: ${e.message}",
                    )
                }
            }
        }
    }

    /**
     * Accumulates bandwidth from a completed proxy request. Called by the service layer.
     */
    fun recordBandwidth(bytesIn: Long, bytesOut: Long, latencyMs: Int) {
        _uiState.update { state ->
            val newRequests = state.requestsServed + 1
            val newDownload = state.downloadBytes + bytesIn
            val newUpload = state.uploadBytes + bytesOut
            val newBandwidth = state.bandwidthSharedBytes + bytesIn + bytesOut
            // Exponential moving average for latency.
            val newLatency = if (state.requestsServed == 0) {
                latencyMs
            } else {
                ((state.avgLatencyMs * 0.8) + (latencyMs * 0.2)).toInt()
            }
            state.copy(
                requestsServed = newRequests,
                downloadBytes = newDownload,
                uploadBytes = newUpload,
                bandwidthSharedBytes = newBandwidth,
                avgLatencyMs = newLatency,
            )
        }
    }

    private fun startUptimeTicker() {
        uptimeJob?.cancel()
        uptimeJob = viewModelScope.launch {
            while (true) {
                delay(1_000L)
                _uiState.update { it.copy(uptimeSeconds = it.uptimeSeconds + 1L) }
            }
        }
    }

    private fun stopUptimeTicker() {
        uptimeJob?.cancel()
        uptimeJob = null
    }

    override fun onCleared() {
        super.onCleared()
        uptimeJob?.cancel()
    }
}
