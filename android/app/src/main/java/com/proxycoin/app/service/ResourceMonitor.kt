package com.proxycoin.app.service

import android.app.ActivityManager
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.os.BatteryManager
import android.os.PowerManager
import android.util.Log
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.proxycoin.app.util.Constants
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import javax.inject.Inject
import javax.inject.Singleton

// Extension property for DataStore — declared at file level (one name per process allowed).
private val Context.resourceDataStore: DataStore<Preferences> by preferencesDataStore(name = "resource_settings")

data class ResourceState(
    val batteryLevel: Float = 1.0f,        // 0.0 – 1.0
    val isCharging: Boolean = false,
    val networkType: NetworkType = NetworkType.NONE,
    val isMetered: Boolean = false,
    val availableMemoryMb: Long = 0,
    val shouldPause: Boolean = false,
    val pauseReason: String? = null,
)

enum class NetworkType {
    NONE, WIFI, CELLULAR, ETHERNET, OTHER
}

/** DataStore preference keys and their default values. */
object ResourcePrefs {
    val WIFI_ONLY = booleanPreferencesKey("wifi_only")
    val BATTERY_THRESHOLD = intPreferencesKey("battery_threshold")
    val CHARGING_ONLY = booleanPreferencesKey("charging_only")

    const val DEFAULT_WIFI_ONLY = true
    const val DEFAULT_BATTERY_THRESHOLD = Constants.DEFAULT_BATTERY_THRESHOLD
    const val DEFAULT_CHARGING_ONLY = false
}

// Kept for callers that still use the simpler status shape.
data class ResourceStatus(
    val batteryPercent: Int,
    val isCharging: Boolean,
    val isWifi: Boolean,
    val isCellular: Boolean,
    val cpuThrottled: Boolean,
    val shouldProxy: Boolean,
)

@Singleton
class ResourceMonitor @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    companion object {
        private const val TAG = "ResourceMonitor"
    }

    private val _state = MutableStateFlow(ResourceState())
    val state: StateFlow<ResourceState> = _state.asStateFlow()

    private val connectivityManager =
        context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
    private val activityManager =
        context.getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager
    private val powerManager =
        context.getSystemService(Context.POWER_SERVICE) as PowerManager

    private var monitorJob: Job? = null

    // ── Battery broadcast receiver ────────────────────────────────────────────

    private val batteryReceiver = object : BroadcastReceiver() {
        override fun onReceive(ctx: Context, intent: Intent) {
            if (intent.action != Intent.ACTION_BATTERY_CHANGED) return
            val level = intent.getIntExtra(BatteryManager.EXTRA_LEVEL, -1)
            val scale = intent.getIntExtra(BatteryManager.EXTRA_SCALE, 100)
            val batteryLevel = if (level >= 0 && scale > 0) level.toFloat() / scale.toFloat() else 1.0f
            val status = intent.getIntExtra(BatteryManager.EXTRA_STATUS, -1)
            val isCharging = status == BatteryManager.BATTERY_STATUS_CHARGING ||
                status == BatteryManager.BATTERY_STATUS_FULL
            Log.v(TAG, "Battery: ${(batteryLevel * 100).toInt()}% charging=$isCharging")
            updateState { it.copy(batteryLevel = batteryLevel, isCharging = isCharging) }
        }
    }

    // ── Connectivity callback ─────────────────────────────────────────────────

    private val networkCallback = object : ConnectivityManager.NetworkCallback() {
        override fun onAvailable(network: Network) {
            refreshNetworkState()
        }

        override fun onLost(network: Network) {
            updateState { it.copy(networkType = NetworkType.NONE, isMetered = false) }
        }

        override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
            val type = when {
                caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> NetworkType.WIFI
                caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> NetworkType.ETHERNET
                caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> NetworkType.CELLULAR
                else -> NetworkType.OTHER
            }
            val metered = !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED)
            Log.v(TAG, "Network: type=$type metered=$metered")
            updateState { it.copy(networkType = type, isMetered = metered) }
        }
    }

    // ── Lifecycle ─────────────────────────────────────────────────────────────

    /**
     * Registers system listeners and starts the periodic resource-check coroutine.
     * Idempotent — safe to call multiple times.
     */
    fun start(scope: CoroutineScope) {
        if (monitorJob?.isActive == true) return

        // Battery intent is sticky — delivers the current level immediately.
        context.registerReceiver(batteryReceiver, IntentFilter(Intent.ACTION_BATTERY_CHANGED))

        val networkRequest = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .build()
        try {
            connectivityManager.registerNetworkCallback(networkRequest, networkCallback)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to register network callback: ${e.message}")
        }

        refreshNetworkState()

        monitorJob = scope.launch {
            while (true) {
                delay(Constants.RESOURCE_MONITOR_INTERVAL_MS)
                val memInfo = ActivityManager.MemoryInfo()
                activityManager.getMemoryInfo(memInfo)
                val availableMb = memInfo.availMem / (1024L * 1024L)
                updateState { it.copy(availableMemoryMb = availableMb) }
                recomputePauseState()
                Log.v(TAG, "Periodic check: state=${_state.value}")
            }
        }

        Log.d(TAG, "ResourceMonitor started")
    }

    /** Unregisters all system listeners and cancels the periodic coroutine. */
    fun stop() {
        monitorJob?.cancel()
        monitorJob = null
        try { context.unregisterReceiver(batteryReceiver) } catch (_: IllegalArgumentException) {}
        try { connectivityManager.unregisterNetworkCallback(networkCallback) } catch (_: Exception) {}
        Log.d(TAG, "ResourceMonitor stopped")
    }

    // ── Pause evaluation ──────────────────────────────────────────────────────

    /**
     * Returns true if the proxy service should currently be paused.
     *
     * Pause conditions:
     * 1. Charging-only mode enabled and device is not charging.
     * 2. Battery below user-configured threshold.
     * 3. WiFi-only mode enabled and current network is not WiFi.
     * 4. Device is in power-save mode.
     */
    suspend fun shouldPause(): Boolean = computePause(_state.value, readPrefs()).first

    /** Human-readable reason for the current pause, or null if not paused. */
    suspend fun pauseReason(): String? = computePause(_state.value, readPrefs()).second

    /** Compatibility helper used by callers expecting the legacy ResourceStatus shape. */
    fun currentStatus(): ResourceStatus {
        val s = _state.value
        return ResourceStatus(
            batteryPercent = (s.batteryLevel * 100).toInt(),
            isCharging = s.isCharging,
            isWifi = s.networkType == NetworkType.WIFI,
            isCellular = s.networkType == NetworkType.CELLULAR,
            cpuThrottled = powerManager.isPowerSaveMode,
            shouldProxy = !s.shouldPause,
        )
    }

    // ── Internal helpers ──────────────────────────────────────────────────────

    private fun refreshNetworkState() {
        val activeNetwork = connectivityManager.activeNetwork
        val caps = activeNetwork?.let { connectivityManager.getNetworkCapabilities(it) }
        val type = when {
            caps == null -> NetworkType.NONE
            caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> NetworkType.WIFI
            caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> NetworkType.ETHERNET
            caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> NetworkType.CELLULAR
            else -> NetworkType.OTHER
        }
        val metered = caps != null && !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED)
        updateState { it.copy(networkType = type, isMetered = metered) }
    }

    private fun updateState(transform: (ResourceState) -> ResourceState) {
        _state.value = transform(_state.value)
    }

    private suspend fun recomputePauseState() {
        val (pause, reason) = computePause(_state.value, readPrefs())
        updateState { it.copy(shouldPause = pause, pauseReason = reason) }
    }

    private fun computePause(state: ResourceState, prefs: UserPrefs): Pair<Boolean, String?> {
        if (prefs.chargingOnly && !state.isCharging) {
            return Pair(true, "Charging-only mode: device is not charging")
        }
        val batteryPercent = (state.batteryLevel * 100).toInt()
        if (batteryPercent < prefs.batteryThreshold) {
            return Pair(true, "Battery at $batteryPercent% (threshold: ${prefs.batteryThreshold}%)")
        }
        if (prefs.wifiOnly && state.networkType != NetworkType.WIFI) {
            return Pair(true, "WiFi-only mode: current network is ${state.networkType.name}")
        }
        if (powerManager.isPowerSaveMode) {
            return Pair(true, "Device is in power-save mode")
        }
        return Pair(false, null)
    }

    private suspend fun readPrefs(): UserPrefs {
        return context.resourceDataStore.data.map { prefs ->
            UserPrefs(
                wifiOnly = prefs[ResourcePrefs.WIFI_ONLY] ?: ResourcePrefs.DEFAULT_WIFI_ONLY,
                batteryThreshold = prefs[ResourcePrefs.BATTERY_THRESHOLD] ?: ResourcePrefs.DEFAULT_BATTERY_THRESHOLD,
                chargingOnly = prefs[ResourcePrefs.CHARGING_ONLY] ?: ResourcePrefs.DEFAULT_CHARGING_ONLY,
            )
        }.first()
    }

    private data class UserPrefs(
        val wifiOnly: Boolean,
        val batteryThreshold: Int,
        val chargingOnly: Boolean,
    )
}
