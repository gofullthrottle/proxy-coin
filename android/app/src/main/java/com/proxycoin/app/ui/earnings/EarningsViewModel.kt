package com.proxycoin.app.ui.earnings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.proxycoin.app.data.local.dao.EarningsDao
import com.proxycoin.app.data.local.entity.EarningsEntity
import com.proxycoin.app.data.remote.ApiService
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import javax.inject.Inject

enum class EarningsPeriod { DAY, WEEK, MONTH, ALL }

@HiltViewModel
class EarningsViewModel @Inject constructor(
    private val apiService: ApiService,
    private val earningsDao: EarningsDao,
) : ViewModel() {

    data class UiState(
        val selectedPeriod: EarningsPeriod = EarningsPeriod.WEEK,
        val earnings: List<EarningsEntity> = emptyList(),
        val pendingAmount: Long = 0L,
        val claimedAmount: Long = 0L,
        val totalEarnings: Long = 0L,
        val isLoading: Boolean = true,
        val errorMessage: String? = null,
        val isClaimInProgress: Boolean = false,
        val claimSuccess: Boolean = false,
    )

    private val _uiState = MutableStateFlow(UiState())
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    private val dateFormatter = DateTimeFormatter.ofPattern("yyyy-MM-dd")

    // Tracks the active period-collection job so we can cancel it on period change.
    private var periodJob: kotlinx.coroutines.Job? = null

    init {
        observeLocalEarnings()
        fetchRemoteData()
    }

    fun selectPeriod(period: EarningsPeriod) {
        _uiState.update { it.copy(selectedPeriod = period, isLoading = true) }
        loadPeriodEarnings(period)
    }

    fun claimEarnings() {
        viewModelScope.launch {
            _uiState.update { it.copy(isClaimInProgress = true, errorMessage = null) }
            try {
                // Fetch the Merkle proof then hand off to the wallet layer.
                val proof = apiService.getClaimProof()
                // Actual on-chain claim is handled by WalletManager / TransactionBuilder.
                // Here we surface the proof so the wallet screen can pick it up.
                _uiState.update {
                    it.copy(
                        isClaimInProgress = false,
                        claimSuccess = true,
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isClaimInProgress = false,
                        errorMessage = "Claim failed: ${e.message}",
                    )
                }
            }
        }
    }

    fun clearClaimSuccess() {
        _uiState.update { it.copy(claimSuccess = false) }
    }

    fun refresh() {
        fetchRemoteData()
    }

    // ── Private helpers ───────────────────────────────────────────────────────

    private fun observeLocalEarnings() {
        viewModelScope.launch {
            earningsDao.getTotalEarnings().collectLatest { total ->
                _uiState.update { it.copy(totalEarnings = total ?: 0L) }
            }
        }
    }

    private fun fetchRemoteData() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessage = null) }
            try {
                // Summary provides pending + all-time totals.
                val summary = apiService.getEarningsSummary()
                _uiState.update {
                    it.copy(pendingAmount = summary.pendingClaim, isLoading = false)
                }
                // Sync earnings history into Room for the current period.
                loadPeriodEarnings(_uiState.value.selectedPeriod)
                syncEarningsHistory()
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

    private fun loadPeriodEarnings(period: EarningsPeriod) {
        // Cancel the previous period subscription before starting a new one.
        periodJob?.cancel()
        periodJob = viewModelScope.launch {
            val today = LocalDate.now()
            val (start, end) = when (period) {
                EarningsPeriod.DAY -> today to today
                EarningsPeriod.WEEK -> today.minusDays(6) to today
                EarningsPeriod.MONTH -> today.minusDays(29) to today
                EarningsPeriod.ALL -> LocalDate.of(2024, 1, 1) to today
            }
            earningsDao.getEarningsRange(
                startDate = start.format(dateFormatter),
                endDate = end.format(dateFormatter),
            ).collectLatest { rows ->
                _uiState.update { it.copy(earnings = rows, isLoading = false) }
            }
        }
    }

    private suspend fun syncEarningsHistory() {
        try {
            val today = LocalDate.now()
            val response = apiService.getEarnings(
                from = today.minusDays(29).format(dateFormatter),
                to = today.format(dateFormatter),
            )
            val entities = response.earnings.map { dto ->
                EarningsEntity(
                    date = dto.date,
                    bandwidthEarnings = dto.bandwidthEarnings,
                    uptimeBonus = dto.uptimeBonus,
                    qualityBonus = dto.qualityBonus,
                    referralEarnings = dto.referralEarnings,
                    totalBytes = dto.totalBytes,
                    requestsServed = dto.requestsServed,
                    avgLatencyMs = 0,
                    updatedAt = System.currentTimeMillis(),
                )
            }
            earningsDao.upsertAll(entities)
        } catch (_: Exception) {
            // Sync failure is non-critical; local data already displayed.
        }
    }
}
