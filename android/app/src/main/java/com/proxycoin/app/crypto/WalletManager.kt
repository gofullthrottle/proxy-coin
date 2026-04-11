package com.proxycoin.app.crypto

import android.content.Context
import android.util.Log
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.web3j.crypto.Bip32ECKeyPair
import org.web3j.crypto.Credentials
import org.web3j.crypto.Hash
import org.web3j.crypto.Keys
import org.web3j.crypto.MnemonicUtils
import org.web3j.crypto.Sign
import org.web3j.crypto.ECKeyPair
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.http.HttpService
import org.web3j.utils.Numeric
import java.math.BigDecimal
import java.math.BigInteger
import java.security.SecureRandom
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Manages the local BIP-44 Ethereum wallet used for PRXY token rewards.
 *
 * Private key security:
 * - Generated or imported keys are immediately encrypted via [KeystoreHelper] and
 *   stored in encrypted SharedPreferences.
 * - The decrypted [Credentials] object is held in memory only while the wallet is
 *   loaded. It is cleared when [lock] is called.
 * - The wallet derives m/44'/60'/0'/0/0 (standard MetaMask path).
 *
 * This class is safe to inject as a singleton. Callers should call [initialize]
 * once the RPC endpoint URL is known (typically from [NodeConfigResponse]).
 */
@Singleton
class WalletManager @Inject constructor(
    @ApplicationContext private val context: Context,
    private val keystoreHelper: KeystoreHelper,
) {

    companion object {
        private const val TAG = "WalletManager"

        // BIP-44 derivation path: m/44'/60'/0'/0/0
        private val DERIVATION_PATH = intArrayOf(
            0x8000002C.toInt(), // purpose 44'
            0x8000003C.toInt(), // coin type 60' (ETH)
            0x80000000.toInt(), // account 0'
            0,                  // change 0
            0,                  // address index 0
        )

        // ERC-20 balanceOf(address) function selector (first 4 bytes of keccak256).
        private const val BALANCE_OF_SELECTOR = "0x70a08231"
        private const val FUNCTION_CALL_DATA_LENGTH = 68 // 4 selector + 32 address padded
    }

    @Volatile
    private var credentials: Credentials? = null

    @Volatile
    private var web3j: Web3j? = null

    // ── Lifecycle ─────────────────────────────────────────────────────────────

    /**
     * Initialises the Web3j client against [rpcUrl].
     * Safe to call multiple times; a new client is built each time.
     */
    fun initialize(rpcUrl: String) {
        web3j?.shutdown()
        web3j = Web3j.build(HttpService(rpcUrl))
        Log.d(TAG, "Web3j initialized: $rpcUrl")

        // Auto-load credentials if a key is already stored.
        if (keystoreHelper.hasStoredKey()) {
            loadCredentials()
        }
    }

    /**
     * Clears the in-memory [Credentials]. The encrypted key remains on disk.
     */
    fun lock() {
        credentials = null
        Log.d(TAG, "Wallet locked")
    }

    // ── Wallet creation / import ──────────────────────────────────────────────

    /**
     * Generates a new BIP-39 mnemonic (128-bit entropy → 12 words).
     * Does NOT automatically import it; caller must pass the result to [importFromMnemonic].
     */
    fun generateMnemonic(): String {
        val entropy = ByteArray(16)
        SecureRandom().nextBytes(entropy)
        return MnemonicUtils.generateMnemonic(entropy)
    }

    /**
     * Derives an Ethereum wallet from [mnemonic] using the standard BIP-44 path,
     * encrypts and stores the private key, and returns the wallet address.
     *
     * Throws [IllegalArgumentException] if the mnemonic is invalid.
     */
    fun importFromMnemonic(mnemonic: String): String {
        require(MnemonicUtils.validateMnemonic(mnemonic)) {
            "Invalid BIP-39 mnemonic: checksum failed or word not in wordlist"
        }

        val seed = MnemonicUtils.generateSeed(mnemonic, null)
        val masterKey = Bip32ECKeyPair.generateKeyPair(seed)
        val derivedKey = Bip32ECKeyPair.deriveKeyPair(masterKey, DERIVATION_PATH)

        val privateKeyHex = Numeric.toHexStringWithPrefix(derivedKey.privateKey)
        return storeAndLoad(privateKeyHex)
    }

    /**
     * Imports a wallet from a raw hex private key (with or without 0x prefix),
     * encrypts and stores it, and returns the wallet address.
     */
    fun importFromPrivateKey(privateKey: String): String {
        val normalised = privateKey.removePrefix("0x").trim()
        require(normalised.length == 64) {
            "Private key must be 32 bytes (64 hex characters); got ${normalised.length}"
        }
        require(normalised.all { it.isDigit() || it in 'a'..'f' || it in 'A'..'F' }) {
            "Private key contains non-hex characters"
        }

        val privateKeyHex = "0x$normalised"
        return storeAndLoad(privateKeyHex)
    }

    // ── Queries ───────────────────────────────────────────────────────────────

    /** Returns the checksummed Ethereum address for the loaded wallet, or null if none. */
    fun getAddress(): String? = credentials?.address

    /**
     * Calls balanceOf([address]) on ERC-20 contract at [tokenAddress] and
     * converts the raw uint256 result to a [BigDecimal] in token units (18 decimals).
     *
     * Requires [initialize] to have been called.
     *
     * Throws if the RPC call fails or the response cannot be decoded.
     */
    suspend fun getBalance(tokenAddress: String): BigDecimal {
        val w3 = requireNotNull(web3j) { "Call initialize() before getBalance()" }
        val owner = requireNotNull(credentials?.address) { "No wallet loaded; call importFromMnemonic or importFromPrivateKey first" }

        // Encode balanceOf(address) call data:
        // selector (4 bytes) + address padded to 32 bytes.
        val paddedAddress = owner.removePrefix("0x").padStart(64, '0')
        val callData = BALANCE_OF_SELECTOR + paddedAddress

        val result = withContext(Dispatchers.IO) {
            w3.ethCall(
                Transaction.createEthCallTransaction(
                    owner,
                    tokenAddress,
                    callData,
                ),
                DefaultBlockParameterName.LATEST,
            ).send()
        }

        if (result.hasError()) {
            throw RuntimeException("balanceOf RPC error: ${result.error.message}")
        }

        val raw = result.value?.removePrefix("0x") ?: "0"
        val rawBigInt = if (raw.isBlank()) BigInteger.ZERO else BigInteger(raw, 16)
        // PRXY uses 18 decimals.
        return BigDecimal(rawBigInt).divide(BigDecimal.TEN.pow(18))
    }

    /**
     * Signs [message] with the loaded private key using the Ethereum personal_sign prefix
     * (EIP-191 version 0x45). Returns the 65-byte hex-encoded signature.
     *
     * Throws [IllegalStateException] if no wallet is loaded.
     */
    fun signMessage(message: String): String {
        val creds = requireNotNull(credentials) {
            "No wallet loaded; call importFromMnemonic or importFromPrivateKey first"
        }

        val messageBytes = message.toByteArray(Charsets.UTF_8)
        val prefix = "\u0019Ethereum Signed Message:\n${messageBytes.size}".toByteArray(Charsets.UTF_8)
        val prefixed = ByteArray(prefix.size + messageBytes.size)
        System.arraycopy(prefix, 0, prefixed, 0, prefix.size)
        System.arraycopy(messageBytes, 0, prefixed, prefix.size, messageBytes.size)

        val hash = Hash.sha3(prefixed)
        val signatureData = Sign.signMessage(hash, creds.ecKeyPair, false)

        val r = Numeric.toHexStringNoPrefixZeroPadded(BigInteger(1, signatureData.r), 64)
        val s = Numeric.toHexStringNoPrefixZeroPadded(BigInteger(1, signatureData.s), 64)
        val v = Integer.toHexString(signatureData.v[0].toInt())

        return "0x$r$s$v"
    }

    /** Returns true if a wallet (encrypted private key) is stored on disk. */
    fun hasWallet(): Boolean = keystoreHelper.hasStoredKey()

    // ── Internal helpers ──────────────────────────────────────────────────────

    private fun storeAndLoad(privateKeyHex: String): String {
        keystoreHelper.storePrivateKey(privateKeyHex)
        val creds = Credentials.create(privateKeyHex)
        credentials = creds
        val address = Keys.toChecksumAddress(creds.address)
        Log.i(TAG, "Wallet loaded: $address")
        return address
    }

    private fun loadCredentials() {
        val privateKey = keystoreHelper.retrievePrivateKey() ?: return
        credentials = Credentials.create(privateKey)
        Log.d(TAG, "Credentials auto-loaded from stored key: ${credentials?.address}")
    }
}
