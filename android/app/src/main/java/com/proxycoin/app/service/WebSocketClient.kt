package com.proxycoin.app.service

import android.os.Build
import android.util.Log
import com.proxycoin.app.BuildConfig
import com.proxycoin.app.util.Constants
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.math.min
import kotlin.random.Random

enum class ConnectionState {
    DISCONNECTED, CONNECTING, CONNECTED, RECONNECTING
}

@Singleton
class WebSocketClient @Inject constructor(
    private val okHttpClient: OkHttpClient,
) {
    companion object {
        private const val TAG = "WebSocketClient"
        private const val RECONNECT_MAX_ATTEMPTS = 10
        private const val NORMAL_CLOSURE_CODE = 1000
        private const val NORMAL_CLOSURE_REASON = "Client disconnect"
    }

    private var webSocket: WebSocket? = null
    private var scope: CoroutineScope? = null
    private var reconnectAttempts = 0
    private var nodeId: String? = null
    private var sessionToken: String? = null
    private var deviceId: String? = null
    private var walletAddress: String? = null
    private val isReconnecting = AtomicBoolean(false)

    // Dedicated OkHttpClient for WebSocket (no read timeout — keepalive/pings instead)
    private val wsClient: OkHttpClient by lazy {
        okHttpClient.newBuilder()
            .readTimeout(0, TimeUnit.MILLISECONDS)
            .pingInterval(25, TimeUnit.SECONDS)
            .build()
    }

    private val _connectionState = MutableStateFlow(ConnectionState.DISCONNECTED)
    val connectionState: StateFlow<ConnectionState> = _connectionState

    var onProxyRequest: ((requestId: String, method: String, url: String, headers: Map<String, String>, body: ByteArray?) -> Unit)? = null
    var onConfigUpdate: ((config: Map<String, Any?>) -> Unit)? = null
    var onEarningsUpdate: ((earnings: Map<String, Any?>) -> Unit)? = null

    fun connect(scope: CoroutineScope) {
        this.scope = scope
        if (_connectionState.value == ConnectionState.CONNECTED ||
            _connectionState.value == ConnectionState.CONNECTING
        ) {
            Log.d(TAG, "Already connected or connecting, skipping")
            return
        }
        openWebSocket()
    }

    fun disconnect() {
        isReconnecting.set(false)
        val ws = webSocket
        webSocket = null
        ws?.close(NORMAL_CLOSURE_CODE, NORMAL_CLOSURE_REASON)
        _connectionState.value = ConnectionState.DISCONNECTED
        Log.i(TAG, "WebSocket disconnected by client")
    }

    fun sendRegister(deviceId: String, walletAddress: String) {
        this.deviceId = deviceId
        this.walletAddress = walletAddress
        val payload = JSONObject().apply {
            put("device_id", deviceId)
            put("wallet_address", walletAddress)
            put("app_version", BuildConfig.VERSION_NAME)
            put("os_version", Build.VERSION.RELEASE)
            put("device_model", Build.MODEL)
            put("platform", "android")
        }
        sendMessage("REGISTER", payload)
    }

    fun sendHeartbeat(stats: ServiceStats) {
        val payload = JSONObject().apply {
            put("node_id", nodeId ?: "")
            put("requests_served", stats.requestsServed)
            put("bytes_proxied", stats.bytesProxied)
            put("uptime_seconds", stats.uptimeSeconds)
            put("is_connected", stats.isConnected)
        }
        sendMessage("HEARTBEAT", payload)
    }

    fun sendProxyResponse(
        requestId: String,
        statusCode: Int,
        headers: Map<String, String>,
        body: ByteArray?,
    ) {
        // Send response start
        val startPayload = JSONObject().apply {
            put("request_id", requestId)
            put("status_code", statusCode)
            val headersObj = JSONObject()
            headers.forEach { (k, v) -> headersObj.put(k, v) }
            put("headers", headersObj)
        }
        sendMessage("PROXY_RESPONSE_START", startPayload)

        // Send body in chunks
        if (body != null && body.isNotEmpty()) {
            var offset = 0
            val chunkSize = Constants.CHUNK_SIZE_BYTES
            while (offset < body.size) {
                val end = min(offset + chunkSize, body.size)
                val chunk = body.copyOfRange(offset, end)
                val chunkPayload = JSONObject().apply {
                    put("request_id", requestId)
                    put("data", android.util.Base64.encodeToString(chunk, android.util.Base64.NO_WRAP))
                    put("offset", offset)
                }
                sendMessage("PROXY_RESPONSE_CHUNK", chunkPayload)
                offset = end
            }
        }

        // Send response end
        val endPayload = JSONObject().apply {
            put("request_id", requestId)
            put("total_bytes", body?.size ?: 0)
        }
        sendMessage("PROXY_RESPONSE_END", endPayload)
    }

    private fun sendMessage(type: String, payload: JSONObject) {
        val ws = webSocket
        if (ws == null) {
            Log.w(TAG, "Cannot send message type=$type: WebSocket is null")
            return
        }
        val envelope = JSONObject().apply {
            put("type", type)
            put("payload", payload)
        }
        val sent = ws.send(envelope.toString())
        if (!sent) {
            Log.w(TAG, "Failed to send message type=$type — buffer may be full")
        }
    }

    private fun openWebSocket() {
        _connectionState.value = ConnectionState.CONNECTING
        Log.i(TAG, "Opening WebSocket connection to ${BuildConfig.WS_URL}")
        val request = Request.Builder()
            .url(BuildConfig.WS_URL)
            .build()
        webSocket = wsClient.newWebSocket(request, createListener())
    }

    private fun createListener(): WebSocketListener = object : WebSocketListener() {

        override fun onOpen(webSocket: WebSocket, response: Response) {
            Log.i(TAG, "WebSocket opened")
            _connectionState.value = ConnectionState.CONNECTED
            isReconnecting.set(false)
            resetReconnect()

            // Re-register after (re)connect if we have credentials
            val did = deviceId
            val wa = walletAddress
            if (did != null && wa != null) {
                sendRegister(did, wa)
            }
        }

        override fun onMessage(webSocket: WebSocket, text: String) {
            try {
                val envelope = JSONObject(text)
                val type = envelope.getString("type")
                val payload = envelope.optJSONObject("payload") ?: JSONObject()
                dispatchMessage(type, payload)
            } catch (e: Exception) {
                Log.e(TAG, "Failed to parse WebSocket message: ${e.message}", e)
            }
        }

        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
            Log.i(TAG, "WebSocket closing: code=$code reason=$reason")
            webSocket.close(NORMAL_CLOSURE_CODE, null)
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            Log.i(TAG, "WebSocket closed: code=$code reason=$reason")
            this@WebSocketClient.webSocket = null
            if (code != NORMAL_CLOSURE_CODE) {
                _connectionState.value = ConnectionState.RECONNECTING
                scheduleReconnect()
            } else {
                _connectionState.value = ConnectionState.DISCONNECTED
            }
        }

        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
            Log.e(TAG, "WebSocket failure: ${t.message}", t)
            this@WebSocketClient.webSocket = null
            _connectionState.value = ConnectionState.RECONNECTING
            scheduleReconnect()
        }
    }

    private fun dispatchMessage(type: String, payload: JSONObject) {
        when (type) {
            "REGISTERED" -> {
                nodeId = payload.optString("node_id").takeIf { it.isNotEmpty() }
                sessionToken = payload.optString("session_token").takeIf { it.isNotEmpty() }
                Log.i(TAG, "Registered as node=$nodeId")
            }
            "PROXY_REQUEST" -> {
                val requestId = payload.getString("request_id")
                val method = payload.getString("method")
                val url = payload.getString("url")
                val headersObj = payload.optJSONObject("headers") ?: JSONObject()
                val headers = buildMap<String, String> {
                    headersObj.keys().forEach { key ->
                        put(key, headersObj.getString(key))
                    }
                }
                val bodyB64 = payload.optString("body").takeIf { it.isNotEmpty() }
                val body = bodyB64?.let { android.util.Base64.decode(it, android.util.Base64.NO_WRAP) }
                Log.d(TAG, "PROXY_REQUEST id=$requestId method=$method url=$url")
                onProxyRequest?.invoke(requestId, method, url, headers, body)
            }
            "CONFIG_UPDATE" -> {
                val config = jsonObjectToMap(payload)
                Log.d(TAG, "CONFIG_UPDATE: $config")
                onConfigUpdate?.invoke(config)
            }
            "EARNINGS_UPDATE" -> {
                val earnings = jsonObjectToMap(payload)
                Log.d(TAG, "EARNINGS_UPDATE: $earnings")
                onEarningsUpdate?.invoke(earnings)
            }
            "PING" -> {
                sendMessage("PONG", JSONObject())
            }
            else -> {
                Log.d(TAG, "Unhandled message type: $type")
            }
        }
    }

    private fun scheduleReconnect() {
        val currentScope = scope ?: run {
            Log.w(TAG, "No scope available for reconnect")
            return
        }
        if (!isReconnecting.compareAndSet(false, true)) {
            Log.d(TAG, "Reconnect already scheduled")
            return
        }
        currentScope.launch {
            val attempt = reconnectAttempts
            if (attempt >= RECONNECT_MAX_ATTEMPTS) {
                Log.e(TAG, "Max reconnect attempts ($RECONNECT_MAX_ATTEMPTS) reached, giving up")
                _connectionState.value = ConnectionState.DISCONNECTED
                isReconnecting.set(false)
                return@launch
            }

            // Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s (capped)
            val baseDelay = min(
                Constants.RECONNECT_INITIAL_DELAY_MS * (1L shl attempt),
                Constants.RECONNECT_MAX_DELAY_MS
            )
            // Add jitter: ±20% of base delay
            val jitter = (baseDelay * 0.2 * (Random.nextDouble() * 2 - 1)).toLong()
            val delayMs = (baseDelay + jitter).coerceAtLeast(Constants.RECONNECT_INITIAL_DELAY_MS)

            Log.i(TAG, "Reconnect attempt ${attempt + 1} in ${delayMs}ms")
            delay(delayMs)

            reconnectAttempts++
            isReconnecting.set(false)

            if (_connectionState.value != ConnectionState.DISCONNECTED) {
                openWebSocket()
            }
        }
    }

    private fun resetReconnect() {
        reconnectAttempts = 0
        Log.d(TAG, "Reconnect counter reset")
    }

    private fun jsonObjectToMap(obj: JSONObject): Map<String, Any?> {
        val map = mutableMapOf<String, Any?>()
        obj.keys().forEach { key ->
            map[key] = when (val value = obj.get(key)) {
                JSONObject.NULL -> null
                is JSONObject -> jsonObjectToMap(value)
                else -> value
            }
        }
        return map
    }
}
