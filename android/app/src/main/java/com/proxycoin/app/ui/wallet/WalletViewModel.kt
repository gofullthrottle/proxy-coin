package com.proxycoin.app.ui.wallet

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.proxycoin.app.crypto.TransactionBuilder
import com.proxycoin.app.crypto.WalletManager
import com.proxycoin.app.data.local.dao.TransactionDao
import com.proxycoin.app.data.local.entity.TransactionEntity
import com.proxycoin.app.data.remote.ApiService
import com.proxycoin.app.util.Constants
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.math.BigDecimal
import java.math.BigInteger
import javax.inject.Inject

@HiltViewModel
class WalletViewModel @Inject constructor(
    private val walletManager: WalletManager,
    private val apiService: ApiService,
    private val transactionBuilder: TransactionBuilder,
    private val transactionDao: TransactionDao,
) : ViewModel() {

    data class UiState(
        val walletAddress: String = "",
        val balance: String = "0",
        val pendingRewards: String = "0",
        val transactions: List<TransactionItem> = emptyList(),
        val isLoading: Boolean = true,
        val isClaiming: Boolean = false,
        val isSending: Boolean = false,
        val error: String? = null,
        val claimSuccess: Boolean = false,
        val sendSuccess: Boolean = false,
    )

    data class TransactionItem(
        val txHash: String,
        val type: String,
        val amount: String,
        val status: String,
        val timestamp: Long,
    )

    private val _uiState = MutableStateFlow(UiState())
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    // PRXY ERC-20 contract address on Base L2 mainnet.
    // In production this should come from NodeConfigResponse.
    private val prxyTokenAddress = "0x0000000000000000000000000000000000000000" // placeholder
    private val distributorAddress = "0x0000000000000000000000000000000000000001" // placeholder

    init {
        loadWallet()
        observeTransactions()
    }

    // ── Public API ────────────────────────────────────────────────────────────

    fun loadWallet() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val address = walletManager.getAddress() ?: run {
                    _uiState.update { it.copy(isLoading = false, walletAddress = "") }
                    return@launch
                }
                val balance = try {
                    walletManager.getBalance(prxyTokenAddress)
                } catch (_: Exception) {
                    BigDecimal.ZERO
                }
                val pendingResponse = try {
                    apiService.getPendingEarnings()
                } catch (_: Exception) {
                    null
                }
                val pendingPrxy = pendingResponse
                    ?.let { BigDecimal(it.pendingAmount).divide(BigDecimal.TEN.pow(6)) }
                    ?: BigDecimal.ZERO

                _uiState.update {
                    it.copy(
                        isLoading = false,
                        walletAddress = address,
                        balance = formatPrxy(balance),
                        pendingRewards = formatPrxy(pendingPrxy),
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        error = "Failed to load wallet: ${e.message}",
                    )
                }
            }
        }
    }

    fun refreshBalance() {
        loadWallet()
    }

    fun claimRewards() {
        viewModelScope.launch {
            _uiState.update { it.copy(isClaiming = true, error = null) }
            try {
                val proofResponse = apiService.getClaimProof()
                val cumulativeAmount = BigInteger(proofResponse.cumulativeAmount)
                val txHash = transactionBuilder.buildClaimTransaction(
                    distributorAddress = distributorAddress,
                    cumulativeAmount = cumulativeAmount,
                    merkleProof = proofResponse.proof,
                )
                // Poll for confirmation in background; UI shows pending immediately via Room flow.
                viewModelScope.launch {
                    transactionBuilder.waitForConfirmation(txHash)
                }
                _uiState.update {
                    it.copy(
                        isClaiming = false,
                        claimSuccess = true,
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isClaiming = false,
                        error = "Claim failed: ${e.message}",
                    )
                }
            }
        }
    }

    fun sendTokens(toAddress: String, amountString: String) {
        viewModelScope.launch {
            _uiState.update { it.copy(isSending = true, error = null) }
            try {
                val amount = BigDecimal(amountString.trim())
                require(amount > BigDecimal.ZERO) { "Amount must be greater than zero" }
                require(toAddress.startsWith("0x") && toAddress.length == 42) {
                    "Invalid Ethereum address"
                }
                val txHash = transactionBuilder.buildTransferTransaction(
                    tokenAddress = prxyTokenAddress,
                    to = toAddress,
                    amount = amount,
                )
                viewModelScope.launch {
                    transactionBuilder.waitForConfirmation(txHash)
                }
                _uiState.update {
                    it.copy(
                        isSending = false,
                        sendSuccess = true,
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isSending = false,
                        error = "Send failed: ${e.message}",
                    )
                }
            }
        }
    }

    fun clearClaimSuccess() {
        _uiState.update { it.copy(claimSuccess = false) }
    }

    fun clearSendSuccess() {
        _uiState.update { it.copy(sendSuccess = false) }
    }

    fun clearError() {
        _uiState.update { it.copy(error = null) }
    }

    fun canClaim(): Boolean {
        val pending = _uiState.value.pendingRewards.toBigDecimalOrNull() ?: BigDecimal.ZERO
        // MIN_CLAIM_AMOUNT is in micro-PRXY (1e-6); convert pending (in PRXY) for comparison.
        val minPrxy = BigDecimal(Constants.MIN_CLAIM_AMOUNT).divide(BigDecimal.TEN.pow(6))
        return pending >= minPrxy
    }

    // ── Private helpers ───────────────────────────────────────────────────────

    private fun observeTransactions() {
        viewModelScope.launch {
            transactionDao.getAllTransactions().collectLatest { entities ->
                _uiState.update {
                    it.copy(transactions = entities.map(::toTransactionItem))
                }
            }
        }
    }

    private fun toTransactionItem(entity: TransactionEntity): TransactionItem {
        return TransactionItem(
            txHash = entity.txHash,
            type = entity.type.replaceFirstChar { it.uppercaseChar() },
            amount = "${entity.amount} PRXY",
            status = entity.status.replaceFirstChar { it.uppercaseChar() },
            timestamp = entity.timestamp,
        )
    }

    private fun formatPrxy(value: BigDecimal): String {
        return "%.4f".format(value)
    }
}
