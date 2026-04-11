package com.proxycoin.app.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.net.wifi.WifiManager
import android.os.IBinder
import android.os.PowerManager
import android.util.Log
import androidx.core.app.NotificationCompat
import com.proxycoin.app.MainActivity
import com.proxycoin.app.util.Constants
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import javax.inject.Inject

data class ServiceStats(
    val isConnected: Boolean = false,
    val uptimeSeconds: Long = 0,
    val requestsServed: Int = 0,
    val bytesProxied: Long = 0,
    val todayEarnings: Double = 0.0,
)

@AndroidEntryPoint
class ProxyForegroundService : Service() {

    @Inject lateinit var webSocketClient: WebSocketClient
    @Inject lateinit var proxyEngine: ProxyEngine
    @Inject lateinit var resourceMonitor: ResourceMonitor

    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private var wakeLock: PowerManager.WakeLock? = null
    private var wifiLock: WifiManager.WifiLock? = null
    private var startTime: Long = 0

    private val notificationManager: NotificationManager by lazy {
        getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
    }

    private val _stats = MutableStateFlow(ServiceStats())
    val stats: StateFlow<ServiceStats> = _stats.asStateFlow()

    companion object {
        private const val TAG = "ProxyService"
        const val ACTION_START = "com.proxycoin.app.START_PROXY"
        const val ACTION_STOP = "com.proxycoin.app.STOP_PROXY"

        private val _isRunning = MutableStateFlow(false)
        val isRunning: StateFlow<Boolean> = _isRunning.asStateFlow()

        private val _serviceStats = MutableStateFlow(ServiceStats())
        val serviceStats: StateFlow<ServiceStats> = _serviceStats.asStateFlow()

        fun start(context: Context) {
            val intent = Intent(context, ProxyForegroundService::class.java).apply {
                action = ACTION_START
            }
            context.startForegroundService(intent)
        }

        fun stop(context: Context) {
            val intent = Intent(context, ProxyForegroundService::class.java).apply {
                action = ACTION_STOP
            }
            context.startService(intent)
        }
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopSelf()
                return START_NOT_STICKY
            }
            else -> {
                startForegroundWithNotification()
                acquireLocks()
                startProxy()
            }
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        Log.i(TAG, "Service destroying")
        _isRunning.value = false
        webSocketClient.disconnect()
        serviceScope.cancel()
        releaseLocks()
        super.onDestroy()
    }

    // ── Private implementation ────────────────────────────────────────────────

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            Constants.NOTIFICATION_CHANNEL_ID,
            "Proxy Coin Service",
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = "Keeps the Proxy Coin node running in the background"
            setShowBadge(false)
        }
        notificationManager.createNotificationChannel(channel)
    }

    private fun startForegroundWithNotification() {
        val notification = buildNotification(_stats.value)
        startForeground(Constants.NOTIFICATION_ID, notification)
        _isRunning.value = true
        Log.i(TAG, "Service started in foreground")
    }

    private fun buildNotification(stats: ServiceStats): Notification {
        val pendingIntent = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java).apply {
                flags = Intent.FLAG_ACTIVITY_SINGLE_TOP
            },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )

        val stopIntent = PendingIntent.getService(
            this,
            1,
            Intent(this, ProxyForegroundService::class.java).apply {
                action = ACTION_STOP
            },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )

        val statusText = if (stats.isConnected) {
            "Connected • ${formatBytes(stats.bytesProxied)} proxied • ${stats.requestsServed} req"
        } else {
            "Connecting…"
        }
        val uptimeText = formatUptime(stats.uptimeSeconds)

        return NotificationCompat.Builder(this, Constants.NOTIFICATION_CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setContentTitle("Proxy Coin Node")
            .setContentText(statusText)
            .setSubText("Uptime: $uptimeText")
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .addAction(
                android.R.drawable.ic_delete,
                "Stop",
                stopIntent,
            )
            .build()
    }

    private fun acquireLocks() {
        val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
        wakeLock = powerManager.newWakeLock(
            PowerManager.PARTIAL_WAKE_LOCK,
            "ProxyCoin:ProxyService",
        ).also { lock ->
            if (!lock.isHeld) {
                lock.acquire()
                Log.d(TAG, "WakeLock acquired")
            }
        }

        val wifiManager = applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
        wifiLock = wifiManager.createWifiLock(
            WifiManager.WIFI_MODE_FULL_HIGH_PERF,
            "ProxyCoin:ProxyService",
        ).also { lock ->
            if (!lock.isHeld) {
                lock.acquire()
                Log.d(TAG, "WifiLock acquired")
            }
        }
    }

    private fun releaseLocks() {
        wakeLock?.let {
            if (it.isHeld) {
                it.release()
                Log.d(TAG, "WakeLock released")
            }
        }
        wakeLock = null

        wifiLock?.let {
            if (it.isHeld) {
                it.release()
                Log.d(TAG, "WifiLock released")
            }
        }
        wifiLock = null
    }

    private fun startProxy() {
        startTime = System.currentTimeMillis()

        // Wire incoming proxy requests from WebSocket into the engine, then stream the
        // response back over the same WebSocket connection.
        webSocketClient.onProxyRequest = { requestId, method, url, headers, body ->
            serviceScope.launch(Dispatchers.IO) {
                try {
                    val result = proxyEngine.executeRequest(requestId, method, url, headers, body)
                    webSocketClient.sendProxyResponse(
                        requestId = requestId,
                        statusCode = result.statusCode,
                        headers = result.headers,
                        body = result.body.takeIf { it.isNotEmpty() },
                    )
                    // Update aggregate stats after each completed request
                    _stats.value = _stats.value.copy(
                        requestsServed = _stats.value.requestsServed + 1,
                        bytesProxied = _stats.value.bytesProxied + result.bytesIn + result.bytesOut,
                    )
                    _serviceStats.value = _stats.value
                } catch (e: Exception) {
                    Log.e(TAG, "Error handling proxy request $requestId: ${e.message}", e)
                }
            }
        }

        // Handle config updates from the coordinator
        webSocketClient.onConfigUpdate = { config ->
            Log.d(TAG, "Config update received: $config")
            @Suppress("UNCHECKED_CAST")
            val blocklist = (config["blocked_domains"] as? List<String>)?.toSet() ?: emptySet()
            if (blocklist.isNotEmpty()) {
                proxyEngine.updateBlocklist(blocklist)
            }
        }

        // Handle earnings updates
        webSocketClient.onEarningsUpdate = { earnings ->
            Log.d(TAG, "Earnings update received: $earnings")
            val todayEarnings = (earnings["today_prxy"] as? Number)?.toDouble() ?: 0.0
            _stats.value = _stats.value.copy(todayEarnings = todayEarnings)
            _serviceStats.value = _stats.value
        }

        // Coroutine 1: Initiate WebSocket connection, then track connection state changes
        webSocketClient.connect(serviceScope)
        serviceScope.launch {
            webSocketClient.connectionState.collect { state ->
                val isConnected = state == ConnectionState.CONNECTED
                _stats.value = _stats.value.copy(isConnected = isConnected)
                _serviceStats.value = _stats.value
            }
        }

        // Coroutine 2: Resource monitoring — pause/resume proxying based on device state
        serviceScope.launch {
            resourceMonitor.observeResources().collect { status ->
                if (!status.shouldProxy) {
                    Log.i(
                        TAG,
                        "Resource threshold exceeded (battery=${status.batteryPercent}%, " +
                            "wifi=${status.isWifi}, cellular=${status.isCellular}) — disconnecting"
                    )
                    if (webSocketClient.connectionState.value == ConnectionState.CONNECTED) {
                        webSocketClient.disconnect()
                    }
                } else {
                    if (webSocketClient.connectionState.value == ConnectionState.DISCONNECTED) {
                        Log.i(TAG, "Resources OK — reconnecting WebSocket")
                        webSocketClient.connect(serviceScope)
                    }
                }
            }
        }

        // Coroutine 3: Heartbeat sender
        serviceScope.launch {
            while (isActive) {
                delay(Constants.HEARTBEAT_INTERVAL_MS)
                if (webSocketClient.connectionState.value == ConnectionState.CONNECTED) {
                    webSocketClient.sendHeartbeat(_stats.value)
                }
            }
        }

        // Coroutine 4: Uptime tracker and notification updater
        serviceScope.launch {
            while (isActive) {
                delay(Constants.NOTIFICATION_UPDATE_INTERVAL_MS)
                val uptimeSecs = (System.currentTimeMillis() - startTime) / 1000L
                _stats.value = _stats.value.copy(uptimeSeconds = uptimeSecs)
                _serviceStats.value = _stats.value
                updateNotification(_stats.value)
            }
        }

        Log.i(TAG, "All proxy coroutines launched")
    }

    private fun updateNotification(stats: ServiceStats) {
        val notification = buildNotification(stats)
        notificationManager.notify(Constants.NOTIFICATION_ID, notification)
    }

    // ── Formatting helpers ────────────────────────────────────────────────────

    private fun formatBytes(bytes: Long): String {
        return when {
            bytes < 1024 -> "${bytes}B"
            bytes < 1024 * 1024 -> "${"%.1f".format(bytes / 1024.0)}KB"
            bytes < 1024 * 1024 * 1024 -> "${"%.1f".format(bytes / (1024.0 * 1024.0))}MB"
            else -> "${"%.2f".format(bytes / (1024.0 * 1024.0 * 1024.0))}GB"
        }
    }

    private fun formatUptime(seconds: Long): String {
        val h = seconds / 3600
        val m = (seconds % 3600) / 60
        val s = seconds % 60
        return when {
            h > 0 -> "${h}h ${m}m"
            m > 0 -> "${m}m ${s}s"
            else -> "${s}s"
        }
    }
}
