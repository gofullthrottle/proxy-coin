package com.proxycoin.app.service

import android.content.Context
import android.util.Log
import com.google.android.play.core.integrity.IntegrityManagerFactory
import com.google.android.play.core.integrity.IntegrityTokenRequest
import com.proxycoin.app.data.remote.ApiService
import com.proxycoin.app.data.remote.dto.AttestationRequest
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.tasks.await
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Manages Play Integrity API attestation flows.
 *
 * Play Integrity provides a verdict signed by Google that certifies:
 * - The device passes basic integrity checks.
 * - The app is a genuine Play Store build (not sideloaded or modified).
 * - The device passes strong integrity (Pixel-class hardware attestation).
 *
 * Attestation tokens are short-lived and should not be cached. Each call to
 * [requestIntegrityToken] issues a new network round-trip to Google.
 */
@Singleton
class PlayIntegrityManager @Inject constructor(
    @ApplicationContext private val context: Context,
    private val apiService: ApiService,
) {

    companion object {
        private const val TAG = "PlayIntegrityManager"
    }

    private val integrityManager = IntegrityManagerFactory.create(context)

    /**
     * Requests a Play Integrity token bound to [nonce].
     *
     * [nonce] must be a URL-safe base64-encoded string, at least 16 bytes of
     * cryptographically random data. The backend should generate and supply this
     * nonce to prevent replay attacks.
     *
     * Returns the encoded token on success, or null if the request fails (e.g.,
     * Play Store unavailable, emulator, device integrity failure).
     */
    suspend fun requestIntegrityToken(nonce: String): String? {
        return try {
            val request = IntegrityTokenRequest.builder()
                .setNonce(nonce)
                .build()
            val response = integrityManager.requestIntegrityToken(request).await()
            val token = response.token()
            Log.d(TAG, "Play Integrity token obtained (length=${token.length})")
            token
        } catch (e: Exception) {
            Log.e(TAG, "Failed to obtain Play Integrity token: ${e.message}", e)
            null
        }
    }

    /**
     * Obtains an integrity token for [nonce] and submits it to the backend
     * for server-side decryption and verdict evaluation.
     *
     * Returns true if the attestation was accepted by the backend, false otherwise.
     * The backend is responsible for decoding the token using the Play Integrity API
     * and enforcing the required verdict levels (e.g., MEETS_DEVICE_INTEGRITY).
     */
    suspend fun attestDevice(nonce: String): Boolean {
        val token = requestIntegrityToken(nonce)
        if (token == null) {
            Log.w(TAG, "attestDevice: no token obtained — attestation skipped")
            return false
        }
        return try {
            apiService.submitAttestation(AttestationRequest(token))
            Log.i(TAG, "Device attestation accepted by backend")
            true
        } catch (e: Exception) {
            Log.e(TAG, "Backend rejected attestation: ${e.message}", e)
            false
        }
    }
}
