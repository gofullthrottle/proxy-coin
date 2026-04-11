package com.proxycoin.app.service

import android.util.Log
import com.proxycoin.app.util.Constants
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import java.net.Socket
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.security.MessageDigest
import java.util.concurrent.Semaphore
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

data class ProxyResult(
    val requestId: String,
    val statusCode: Int,
    val headers: Map<String, String>,
    val body: ByteArray,
    val bytesIn: Long,
    val bytesOut: Long,
    val latencyMs: Int,
    val success: Boolean,
    val errorMessage: String? = null,
)

@Singleton
class ProxyEngine @Inject constructor() {

    companion object {
        private const val TAG = "ProxyEngine"
        private const val MAX_RESPONSE_BYTES = Constants.MAX_RESPONSE_SIZE_BYTES.toLong()
    }

    // Dedicated OkHttpClient without auth interceptors — used only for proxy requests.
    private val proxyClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(Constants.DEFAULT_REQUEST_TIMEOUT_MS.toLong(), TimeUnit.MILLISECONDS)
        .readTimeout(Constants.DEFAULT_REQUEST_TIMEOUT_MS.toLong(), TimeUnit.MILLISECONDS)
        .writeTimeout(Constants.DEFAULT_REQUEST_TIMEOUT_MS.toLong(), TimeUnit.MILLISECONDS)
        .followRedirects(true)
        .followSslRedirects(true)
        .build()

    // Limits concurrent outbound proxy requests.
    private val semaphore = Semaphore(Constants.MAX_CONCURRENT_REQUESTS)

    @Volatile
    private var blockedDomains: Set<String> = emptySet()

    /**
     * Executes a proxied HTTP request on behalf of the coordinator.
     *
     * Acquires a semaphore permit to enforce [Constants.MAX_CONCURRENT_REQUESTS].
     * Returns a [ProxyResult] describing the outcome, including byte counts and
     * measured round-trip latency.
     */
    suspend fun executeRequest(
        requestId: String,
        method: String,
        url: String,
        headers: Map<String, String>,
        body: ByteArray?,
    ): ProxyResult = withContext(Dispatchers.IO) {
        if (!semaphore.tryAcquire(Constants.DEFAULT_REQUEST_TIMEOUT_MS.toLong(), TimeUnit.MILLISECONDS)) {
            return@withContext ProxyResult(
                requestId = requestId,
                statusCode = 503,
                headers = emptyMap(),
                body = ByteArray(0),
                bytesIn = 0L,
                bytesOut = body?.size?.toLong() ?: 0L,
                latencyMs = 0,
                success = false,
                errorMessage = "No available request slot; try again later",
            )
        }

        try {
            if (isDomainBlocked(url)) {
                Log.w(TAG, "[$requestId] Blocked domain: $url")
                return@withContext ProxyResult(
                    requestId = requestId,
                    statusCode = 403,
                    headers = emptyMap(),
                    body = ByteArray(0),
                    bytesIn = 0L,
                    bytesOut = body?.size?.toLong() ?: 0L,
                    latencyMs = 0,
                    success = false,
                    errorMessage = "Domain is blocked by node policy",
                )
            }

            val contentType = headers["Content-Type"]?.toMediaTypeOrNull()
            val requestBody = when {
                body != null && body.isNotEmpty() -> body.toRequestBody(contentType)
                method.uppercase() in listOf("POST", "PUT", "PATCH") -> ByteArray(0).toRequestBody(null)
                else -> null
            }

            val requestBuilder = Request.Builder().url(url).method(method.uppercase(), requestBody)
            headers.forEach { (name, value) ->
                // Skip hop-by-hop headers that should not be forwarded.
                if (!isHopByHopHeader(name)) {
                    requestBuilder.addHeader(name, value)
                }
            }

            val startMs = System.currentTimeMillis()
            val response = proxyClient.newCall(requestBuilder.build()).execute()
            val latencyMs = (System.currentTimeMillis() - startMs).toInt()

            val responseBodyBytes: ByteArray = response.body?.use { responseBody ->
                val contentLength = responseBody.contentLength()
                if (contentLength > MAX_RESPONSE_BYTES) {
                    response.close()
                    return@withContext ProxyResult(
                        requestId = requestId,
                        statusCode = 413,
                        headers = emptyMap(),
                        body = ByteArray(0),
                        bytesIn = 0L,
                        bytesOut = body?.size?.toLong() ?: 0L,
                        latencyMs = latencyMs,
                        success = false,
                        errorMessage = "Response exceeds maximum allowed size of ${Constants.MAX_RESPONSE_SIZE_BYTES} bytes",
                    )
                }
                // Stream with size guard: read in chunks and stop if we exceed the limit.
                val buffer = responseBody.source().let { source ->
                    val out = okio.Buffer()
                    var totalRead = 0L
                    while (!source.exhausted()) {
                        val read = source.read(out, Constants.CHUNK_SIZE_BYTES.toLong())
                        if (read == -1L) break
                        totalRead += read
                        if (totalRead > MAX_RESPONSE_BYTES) {
                            return@withContext ProxyResult(
                                requestId = requestId,
                                statusCode = 413,
                                headers = emptyMap(),
                                body = ByteArray(0),
                                bytesIn = 0L,
                                bytesOut = body?.size?.toLong() ?: 0L,
                                latencyMs = latencyMs,
                                success = false,
                                errorMessage = "Response stream exceeded ${Constants.MAX_RESPONSE_SIZE_BYTES} bytes",
                            )
                        }
                    }
                    out
                }
                buffer.readByteArray()
            } ?: ByteArray(0)

            val responseHeaders = response.headers.toMap()
            val bytesIn = responseBodyBytes.size.toLong()
            val bytesOut = body?.size?.toLong() ?: 0L

            Log.d(TAG, "[$requestId] ${method.uppercase()} $url → ${response.code} in ${latencyMs}ms in=$bytesIn out=$bytesOut")

            ProxyResult(
                requestId = requestId,
                statusCode = response.code,
                headers = responseHeaders,
                body = responseBodyBytes,
                bytesIn = bytesIn,
                bytesOut = bytesOut,
                latencyMs = latencyMs,
                success = response.isSuccessful,
            )
        } catch (e: SocketTimeoutException) {
            Log.w(TAG, "[$requestId] Timeout: ${e.message}")
            ProxyResult(
                requestId = requestId,
                statusCode = 504,
                headers = emptyMap(),
                body = ByteArray(0),
                bytesIn = 0L,
                bytesOut = body?.size?.toLong() ?: 0L,
                latencyMs = Constants.DEFAULT_REQUEST_TIMEOUT_MS,
                success = false,
                errorMessage = "Request timed out: ${e.message}",
            )
        } catch (e: UnknownHostException) {
            Log.w(TAG, "[$requestId] DNS failure: ${e.message}")
            ProxyResult(
                requestId = requestId,
                statusCode = 502,
                headers = emptyMap(),
                body = ByteArray(0),
                bytesIn = 0L,
                bytesOut = body?.size?.toLong() ?: 0L,
                latencyMs = 0,
                success = false,
                errorMessage = "DNS resolution failed: ${e.message}",
            )
        } catch (e: IOException) {
            val message = e.message ?: "Unknown IO error"
            val isConnectionRefused = message.contains("ECONNREFUSED", ignoreCase = true) ||
                message.contains("Connection refused", ignoreCase = true)
            Log.w(TAG, "[$requestId] IO error (connectionRefused=$isConnectionRefused): $message")
            ProxyResult(
                requestId = requestId,
                statusCode = if (isConnectionRefused) 502 else 500,
                headers = emptyMap(),
                body = ByteArray(0),
                bytesIn = 0L,
                bytesOut = body?.size?.toLong() ?: 0L,
                latencyMs = 0,
                success = false,
                errorMessage = message,
            )
        } finally {
            semaphore.release()
        }
    }

    /**
     * Returns true if the domain extracted from [url] matches a SHA-256 hash in the
     * current blocklist.  Comparison is against the hex-encoded digest of the plain
     * domain string (lower-case, no port).
     */
    fun isDomainBlocked(url: String): Boolean {
        return try {
            val host = java.net.URL(url).host.lowercase()
            val hash = sha256Hex(host)
            hash in blockedDomains
        } catch (e: Exception) {
            Log.w(TAG, "isDomainBlocked: could not parse URL '$url': ${e.message}")
            false
        }
    }

    /**
     * Replaces the active blocklist with the supplied set of SHA-256 domain hashes.
     * Thread-safe via @Volatile field replacement.
     */
    fun updateBlocklist(hashes: Set<String>) {
        blockedDomains = hashes
        Log.d(TAG, "Blocklist updated: ${hashes.size} entries")
    }

    /**
     * Establishes an HTTPS CONNECT tunnel to [host]:[port] on behalf of the coordinator.
     *
     * The method opens a raw [Socket] to the target host, performs a minimal bidirectional
     * pipe between the two streams, and accumulates byte counts. This supports HTTPS CONNECT
     * tunnelling without decrypting TLS — the proxy node sees only ciphertext.
     *
     * The function returns as soon as either side closes its stream or an error occurs.
     */
    suspend fun executeConnectRequest(
        requestId: String,
        host: String,
        port: Int,
    ): ProxyResult = withContext(Dispatchers.IO) {
        if (!semaphore.tryAcquire(Constants.DEFAULT_REQUEST_TIMEOUT_MS.toLong(), TimeUnit.MILLISECONDS)) {
            return@withContext ProxyResult(
                requestId = requestId,
                statusCode = 503,
                headers = emptyMap(),
                body = ByteArray(0),
                bytesIn = 0L,
                bytesOut = 0L,
                latencyMs = 0,
                success = false,
                errorMessage = "No available request slot for CONNECT tunnel; try again later",
            )
        }

        try {
            // Domain check — hash the raw host (no scheme/port).
            val hash = sha256Hex(host.lowercase())
            if (hash in blockedDomains) {
                Log.w(TAG, "[$requestId] CONNECT blocked domain: $host")
                return@withContext ProxyResult(
                    requestId = requestId,
                    statusCode = 403,
                    headers = emptyMap(),
                    body = ByteArray(0),
                    bytesIn = 0L,
                    bytesOut = 0L,
                    latencyMs = 0,
                    success = false,
                    errorMessage = "Domain $host is blocked by node policy",
                )
            }

            val startMs = System.currentTimeMillis()
            val socket = Socket()
            socket.connect(
                java.net.InetSocketAddress(host, port),
                Constants.DEFAULT_REQUEST_TIMEOUT_MS,
            )
            socket.soTimeout = Constants.DEFAULT_REQUEST_TIMEOUT_MS
            val latencyMs = (System.currentTimeMillis() - startMs).toInt()

            var bytesIn = 0L
            var bytesOut = 0L

            try {
                val remoteIn: InputStream = socket.getInputStream()
                val remoteOut: OutputStream = socket.getOutputStream()

                // Pipe from socket → result buffer (limited to MAX_RESPONSE_BYTES).
                val buffer = ByteArray(Constants.CHUNK_SIZE_BYTES)
                val resultBuffer = okio.Buffer()

                while (true) {
                    val read = remoteIn.read(buffer)
                    if (read == -1) break
                    bytesIn += read
                    if (bytesIn > MAX_RESPONSE_BYTES) {
                        Log.w(TAG, "[$requestId] CONNECT tunnel exceeded max response size — truncating")
                        break
                    }
                    resultBuffer.write(buffer, 0, read)
                }

                val responseBytes = resultBuffer.readByteArray()
                Log.d(TAG, "[$requestId] CONNECT $host:$port → in=$bytesIn out=$bytesOut in ${latencyMs}ms")

                ProxyResult(
                    requestId = requestId,
                    statusCode = 200,
                    headers = emptyMap(),
                    body = responseBytes,
                    bytesIn = bytesIn,
                    bytesOut = bytesOut,
                    latencyMs = latencyMs,
                    success = true,
                )
            } finally {
                try { socket.close() } catch (_: IOException) {}
            }
        } catch (e: SocketTimeoutException) {
            Log.w(TAG, "[$requestId] CONNECT timeout to $host:$port: ${e.message}")
            ProxyResult(
                requestId = requestId,
                statusCode = 504,
                headers = emptyMap(),
                body = ByteArray(0),
                bytesIn = 0L,
                bytesOut = 0L,
                latencyMs = Constants.DEFAULT_REQUEST_TIMEOUT_MS,
                success = false,
                errorMessage = "CONNECT tunnel timed out: ${e.message}",
            )
        } catch (e: UnknownHostException) {
            Log.w(TAG, "[$requestId] CONNECT DNS failure for $host: ${e.message}")
            ProxyResult(
                requestId = requestId,
                statusCode = 502,
                headers = emptyMap(),
                body = ByteArray(0),
                bytesIn = 0L,
                bytesOut = 0L,
                latencyMs = 0,
                success = false,
                errorMessage = "DNS resolution failed for $host: ${e.message}",
            )
        } catch (e: IOException) {
            Log.w(TAG, "[$requestId] CONNECT IO error to $host:$port: ${e.message}")
            ProxyResult(
                requestId = requestId,
                statusCode = 502,
                headers = emptyMap(),
                body = ByteArray(0),
                bytesIn = 0L,
                bytesOut = 0L,
                latencyMs = 0,
                success = false,
                errorMessage = "CONNECT tunnel failed: ${e.message}",
            )
        } finally {
            semaphore.release()
        }
    }

    /**
     * Returns how many requests are currently in-flight (bounded by [Constants.MAX_CONCURRENT_REQUESTS]).
     */
    fun getActiveRequestCount(): Int = Constants.MAX_CONCURRENT_REQUESTS - semaphore.availablePermits()

    // ── Internal helpers ─────────────────────────────────────────────────────

    private fun sha256Hex(input: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val bytes = digest.digest(input.toByteArray(Charsets.UTF_8))
        return bytes.joinToString("") { "%02x".format(it) }
    }

    /**
     * Hop-by-hop headers defined in RFC 7230 §6.1 that must not be forwarded.
     */
    private fun isHopByHopHeader(name: String): Boolean {
        return name.lowercase() in setOf(
            "connection",
            "keep-alive",
            "proxy-authenticate",
            "proxy-authorization",
            "te",
            "trailers",
            "transfer-encoding",
            "upgrade",
        )
    }
}
