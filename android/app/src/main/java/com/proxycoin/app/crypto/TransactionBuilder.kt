package com.proxycoin.app.crypto

import android.util.Log
import com.proxycoin.app.data.local.dao.TransactionDao
import com.proxycoin.app.data.local.entity.TransactionEntity
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import org.web3j.abi.FunctionEncoder
import org.web3j.abi.TypeReference
import org.web3j.abi.datatypes.Address
import org.web3j.abi.datatypes.Bool
import org.web3j.abi.datatypes.DynamicArray
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.generated.Bytes32
import org.web3j.abi.datatypes.generated.Uint256
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.math.BigDecimal
import java.math.BigInteger
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Builds, signs, and submits Ethereum transactions for the PRXY token.
 *
 * All methods require [web3j] and [credentials] to be non-null (set after
 * wallet load). Transactions are persisted to [TransactionDao] as "pending"
 * immediately upon broadcast and updated to "confirmed" or "failed" once
 * the receipt is polled.
 *
 * Chain ID: Base L2 mainnet = 8453, testnet (Base Sepolia) = 84532.
 */
@Singleton
class TransactionBuilder @Inject constructor(
    private val transactionDao: TransactionDao,
) {

    companion object {
        private const val TAG = "TransactionBuilder"

        // Default polling: wait up to ~2 minutes for a receipt (Base L2 ~2s blocks).
        private const val RECEIPT_POLL_INTERVAL_MS = 2_000L
        private const val RECEIPT_MAX_POLLS = 60

        // Base L2 mainnet chain ID.
        private const val CHAIN_ID = 8453L
    }

    // Set by DI or by the service layer after wallet initialisation.
    var web3j: Web3j? = null
    var credentials: Credentials? = null

    // ── Public API ────────────────────────────────────────────────────────────

    /**
     * Builds, signs, and sends an ERC-20 `transfer(to, amount)` transaction.
     *
     * [tokenAddress] — the PRXY ERC-20 contract address.
     * [to]          — recipient address.
     * [amount]      — amount in PRXY units (18 decimal BigDecimal, e.g. "1.5" = 1.5 PRXY).
     *
     * Returns the transaction hash on successful broadcast, or throws on error.
     * Persists a [TransactionEntity] with status "pending" immediately, then
     * polls for confirmation in the background.
     */
    suspend fun buildTransferTransaction(
        tokenAddress: String,
        to: String,
        amount: BigDecimal,
    ): String = withContext(Dispatchers.IO) {
        val w3 = requireNotNull(web3j) { "Web3j not initialised; call WalletManager.initialize first" }
        val creds = requireNotNull(credentials) { "No wallet loaded; import a private key first" }

        val amountWei = amount.multiply(BigDecimal.TEN.pow(18)).toBigInteger()

        val function = Function(
            "transfer",
            listOf(Address(to), Uint256(amountWei)),
            listOf(TypeReference.create(Bool::class.java)),
        )
        val encodedFunction = FunctionEncoder.encode(function)

        val txHash = signAndSend(w3, creds, tokenAddress, BigInteger.ZERO, encodedFunction)

        persistTransaction(
            txHash = txHash,
            type = "send",
            amount = amount.toPlainString(),
            toAddress = to,
            fromAddress = creds.address,
        )

        Log.i(TAG, "Transfer submitted: $txHash")
        txHash
    }

    /**
     * Builds, signs, and sends a claim transaction against a MerkleDistributor contract.
     *
     * The distributor exposes: `claim(uint256 cumulativeAmount, bytes32[] proof)`
     *
     * [distributorAddress]  — address of the MerkleDistributor contract.
     * [cumulativeAmount]    — cumulative earned amount (in smallest token units, uint256).
     * [merkleProof]         — list of 32-byte proof elements as hex strings.
     *
     * Returns the transaction hash. Persists a "pending" entry in Room.
     */
    suspend fun buildClaimTransaction(
        distributorAddress: String,
        cumulativeAmount: BigInteger,
        merkleProof: List<String>,
    ): String = withContext(Dispatchers.IO) {
        val w3 = requireNotNull(web3j) { "Web3j not initialised" }
        val creds = requireNotNull(credentials) { "No wallet loaded" }

        val proofBytes = merkleProof.map { hex ->
            val raw = Numeric.hexStringToByteArray(hex)
            val padded = ByteArray(32)
            System.arraycopy(raw, 0, padded, 32 - raw.size, raw.size)
            Bytes32(padded)
        }

        val function = Function(
            "claim",
            listOf(
                Uint256(cumulativeAmount),
                DynamicArray(Bytes32::class.java, proofBytes),
            ),
            listOf(TypeReference.create(Bool::class.java)),
        )
        val encodedFunction = FunctionEncoder.encode(function)

        val txHash = signAndSend(w3, creds, distributorAddress, BigInteger.ZERO, encodedFunction)

        val claimAmountPrxy = BigDecimal(cumulativeAmount)
            .divide(BigDecimal.TEN.pow(18))
            .toPlainString()

        persistTransaction(
            txHash = txHash,
            type = "claim",
            amount = claimAmountPrxy,
            toAddress = creds.address,
            fromAddress = distributorAddress,
        )

        Log.i(TAG, "Claim submitted: $txHash")
        txHash
    }

    /**
     * Submits a pre-signed raw transaction hex string and waits for confirmation.
     * Returns the transaction hash.
     */
    suspend fun sendTransaction(signedHex: String): String = withContext(Dispatchers.IO) {
        val w3 = requireNotNull(web3j) { "Web3j not initialised" }
        val response = w3.ethSendRawTransaction(signedHex).send()

        if (response.hasError()) {
            throw RuntimeException("sendRawTransaction error: ${response.error.message}")
        }

        val txHash = response.transactionHash
        Log.i(TAG, "Raw transaction sent: $txHash")
        txHash
    }

    /**
     * Polls the chain for a transaction receipt until confirmation or timeout.
     * Updates the [TransactionEntity] status in Room upon resolution.
     */
    suspend fun waitForConfirmation(txHash: String): Boolean = withContext(Dispatchers.IO) {
        val w3 = web3j ?: return@withContext false

        repeat(RECEIPT_MAX_POLLS) { attempt ->
            delay(RECEIPT_POLL_INTERVAL_MS)
            try {
                val receipt = w3.ethGetTransactionReceipt(txHash).send()
                val r = receipt.transactionReceipt.orElse(null)
                if (r != null) {
                    val success = r.isStatusOK
                    val blockNumber = r.blockNumber.toLong()
                    transactionDao.updateStatus(
                        txHash = txHash,
                        status = if (success) "confirmed" else "failed",
                        blockNumber = blockNumber,
                    )
                    Log.i(TAG, "$txHash confirmed at block $blockNumber (success=$success)")
                    return@withContext success
                }
            } catch (e: Exception) {
                Log.w(TAG, "Poll $attempt for $txHash failed: ${e.message}")
            }
        }

        Log.w(TAG, "$txHash not confirmed after $RECEIPT_MAX_POLLS polls — marking failed")
        transactionDao.updateStatus(txHash = txHash, status = "failed", blockNumber = null)
        false
    }

    // ── Internal helpers ──────────────────────────────────────────────────────

    /**
     * Signs and broadcasts a transaction. Returns the transaction hash.
     */
    private suspend fun signAndSend(
        w3: Web3j,
        creds: Credentials,
        to: String,
        value: BigInteger,
        data: String,
    ): String {
        val nonce = w3.ethGetTransactionCount(
            creds.address,
            DefaultBlockParameterName.PENDING,
        ).send().transactionCount

        // Estimate gas for this specific call.
        val gasEstimate = try {
            val callTx = Transaction.createFunctionCallTransaction(
                creds.address, nonce, DefaultGasProvider.GAS_PRICE,
                DefaultGasProvider.GAS_LIMIT, to, value, data,
            )
            val estimate = w3.ethEstimateGas(callTx).send()
            if (estimate.hasError()) {
                Log.w(TAG, "Gas estimation failed (${estimate.error.message}); using default")
                DefaultGasProvider.GAS_LIMIT
            } else {
                // Add 20% buffer.
                estimate.amountUsed.multiply(BigInteger.valueOf(12)).divide(BigInteger.TEN)
            }
        } catch (e: Exception) {
            Log.w(TAG, "Gas estimation exception: ${e.message}; using default")
            DefaultGasProvider.GAS_LIMIT
        }

        // Use current base fee + tip for EIP-1559.
        val block = w3.ethGetBlockByNumber(DefaultBlockParameterName.LATEST, false).send().block
        val baseFee = block?.baseFeePerGas ?: BigInteger.ZERO
        val maxPriorityFee = BigInteger.valueOf(1_000_000_000L) // 1 Gwei tip
        val maxFeePerGas = baseFee.multiply(BigInteger.TWO).add(maxPriorityFee)

        val rawTx = RawTransaction.createTransaction(
            CHAIN_ID,
            nonce,
            gasEstimate,
            to,
            value,
            data,
            maxPriorityFee,
            maxFeePerGas,
        )

        val signedTx = TransactionEncoder.signMessage(rawTx, CHAIN_ID, creds.ecKeyPair)
        val hexTx = Numeric.toHexString(signedTx)

        val response = w3.ethSendRawTransaction(hexTx).send()
        if (response.hasError()) {
            throw RuntimeException("Transaction broadcast failed: ${response.error.message}")
        }

        return response.transactionHash
    }

    private suspend fun persistTransaction(
        txHash: String,
        type: String,
        amount: String,
        toAddress: String?,
        fromAddress: String?,
    ) {
        transactionDao.upsert(
            TransactionEntity(
                txHash = txHash,
                type = type,
                amount = amount,
                toAddress = toAddress,
                fromAddress = fromAddress,
                status = "pending",
                blockNumber = null,
                timestamp = System.currentTimeMillis(),
            ),
        )
    }
}
