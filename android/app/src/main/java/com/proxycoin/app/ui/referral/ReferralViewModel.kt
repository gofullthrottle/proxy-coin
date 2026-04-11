package com.proxycoin.app.ui.referral

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.proxycoin.app.data.remote.ApiService
import com.proxycoin.app.data.remote.dto.ReferralDto
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class ReferralViewModel @Inject constructor(
    private val apiService: ApiService,
) : ViewModel() {

    data class ReferralEntry(
        val nodeId: String,
        val earnings: Long,
        val joinedAt: Long,
    )

    data class UiState(
        val referralCode: String = "",
        val totalReferrals: Int = 0,
        val totalEarnings: Long = 0L,
        val referrals: List<ReferralEntry> = emptyList(),
        val isLoading: Boolean = true,
        val error: String? = null,
    )

    private val _uiState = MutableStateFlow(UiState())
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    init {
        loadReferralData()
    }

    fun refresh() {
        loadReferralData()
    }

    fun clearError() {
        _uiState.update { it.copy(error = null) }
    }

    private fun loadReferralData() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val codeResponse = apiService.getReferralCode()
                val statsResponse = apiService.getReferralStats()

                _uiState.update {
                    it.copy(
                        isLoading = false,
                        referralCode = codeResponse.code,
                        totalReferrals = statsResponse.totalReferrals,
                        totalEarnings = statsResponse.totalEarnings,
                        referrals = statsResponse.referrals.map { dto ->
                            dto.toEntry()
                        },
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        error = "Failed to load referral data: ${e.message}",
                    )
                }
            }
        }
    }

    private fun ReferralDto.toEntry() = ReferralEntry(
        nodeId = nodeId,
        earnings = earnings,
        joinedAt = joinedAt,
    )
}
