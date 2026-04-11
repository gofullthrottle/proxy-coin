package com.proxycoin.app.ui.settings

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

// One DataStore per process — name must be unique.
private val Context.settingsDataStore: DataStore<Preferences> by preferencesDataStore(name = "app_settings")

object SettingsKeys {
    // Bandwidth
    val MAX_BANDWIDTH_PERCENT = intPreferencesKey("max_bandwidth_percent")
    val WIFI_ONLY = booleanPreferencesKey("wifi_only")
    val ALLOW_CELLULAR = booleanPreferencesKey("allow_cellular")
    // Power
    val BATTERY_THRESHOLD = intPreferencesKey("battery_threshold")
    val CHARGING_ONLY = booleanPreferencesKey("charging_only")
    // Notifications
    val NOTIFY_EARNINGS = booleanPreferencesKey("notify_earnings")
    val NOTIFY_CONNECTION = booleanPreferencesKey("notify_connection")
    val NOTIFY_WEEKLY_SUMMARY = booleanPreferencesKey("notify_weekly_summary")
    // Identity (read-only display values written elsewhere)
    val DEVICE_ID = stringPreferencesKey("device_id")
    val NODE_ID = stringPreferencesKey("node_id")
    val WALLET_ADDRESS = stringPreferencesKey("wallet_address")
}

@HiltViewModel
class SettingsViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
) : ViewModel() {

    data class UiState(
        // Bandwidth
        val maxBandwidthPercent: Int = 80,
        val wifiOnly: Boolean = true,
        val allowCellular: Boolean = false,
        // Power
        val batteryThreshold: Int = 20,
        val chargingOnly: Boolean = false,
        // Notifications
        val notifyEarnings: Boolean = true,
        val notifyConnection: Boolean = true,
        val notifyWeeklySummary: Boolean = true,
        // Identity
        val deviceId: String = "",
        val nodeId: String = "",
        val walletAddress: String = "",
        val trustScore: Float = 0f,
        // Meta
        val appVersion: String = "0.1.0",
        val isSaving: Boolean = false,
    )

    private val _uiState = MutableStateFlow(UiState())
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    private val dataStore = context.settingsDataStore

    init {
        observeSettings()
    }

    // ── Bandwidth setters ─────────────────────────────────────────────────────

    fun setMaxBandwidthPercent(value: Int) {
        _uiState.update { it.copy(maxBandwidthPercent = value.coerceIn(10, 100)) }
        persist { prefs -> prefs[SettingsKeys.MAX_BANDWIDTH_PERCENT] = value.coerceIn(10, 100) }
    }

    fun setWifiOnly(enabled: Boolean) {
        _uiState.update { it.copy(wifiOnly = enabled) }
        persist { prefs -> prefs[SettingsKeys.WIFI_ONLY] = enabled }
    }

    fun setAllowCellular(enabled: Boolean) {
        _uiState.update { it.copy(allowCellular = enabled) }
        persist { prefs -> prefs[SettingsKeys.ALLOW_CELLULAR] = enabled }
    }

    // ── Power setters ─────────────────────────────────────────────────────────

    fun setBatteryThreshold(percent: Int) {
        _uiState.update { it.copy(batteryThreshold = percent.coerceIn(5, 50)) }
        persist { prefs -> prefs[SettingsKeys.BATTERY_THRESHOLD] = percent.coerceIn(5, 50) }
    }

    fun setChargingOnly(enabled: Boolean) {
        _uiState.update { it.copy(chargingOnly = enabled) }
        persist { prefs -> prefs[SettingsKeys.CHARGING_ONLY] = enabled }
    }

    // ── Notification setters ──────────────────────────────────────────────────

    fun setNotifyEarnings(enabled: Boolean) {
        _uiState.update { it.copy(notifyEarnings = enabled) }
        persist { prefs -> prefs[SettingsKeys.NOTIFY_EARNINGS] = enabled }
    }

    fun setNotifyConnection(enabled: Boolean) {
        _uiState.update { it.copy(notifyConnection = enabled) }
        persist { prefs -> prefs[SettingsKeys.NOTIFY_CONNECTION] = enabled }
    }

    fun setNotifyWeeklySummary(enabled: Boolean) {
        _uiState.update { it.copy(notifyWeeklySummary = enabled) }
        persist { prefs -> prefs[SettingsKeys.NOTIFY_WEEKLY_SUMMARY] = enabled }
    }

    // ── Internal ──────────────────────────────────────────────────────────────

    private fun observeSettings() {
        viewModelScope.launch {
            dataStore.data.map { prefs ->
                UiState(
                    maxBandwidthPercent = prefs[SettingsKeys.MAX_BANDWIDTH_PERCENT] ?: 80,
                    wifiOnly = prefs[SettingsKeys.WIFI_ONLY] ?: true,
                    allowCellular = prefs[SettingsKeys.ALLOW_CELLULAR] ?: false,
                    batteryThreshold = prefs[SettingsKeys.BATTERY_THRESHOLD] ?: 20,
                    chargingOnly = prefs[SettingsKeys.CHARGING_ONLY] ?: false,
                    notifyEarnings = prefs[SettingsKeys.NOTIFY_EARNINGS] ?: true,
                    notifyConnection = prefs[SettingsKeys.NOTIFY_CONNECTION] ?: true,
                    notifyWeeklySummary = prefs[SettingsKeys.NOTIFY_WEEKLY_SUMMARY] ?: true,
                    deviceId = prefs[SettingsKeys.DEVICE_ID] ?: "",
                    nodeId = prefs[SettingsKeys.NODE_ID] ?: "",
                    walletAddress = prefs[SettingsKeys.WALLET_ADDRESS] ?: "",
                )
            }.collectLatest { state ->
                _uiState.update { current ->
                    state.copy(
                        trustScore = current.trustScore,
                        appVersion = current.appVersion,
                        isSaving = current.isSaving,
                    )
                }
            }
        }
    }

    private fun persist(block: (MutablePreferences) -> Unit) {
        viewModelScope.launch {
            dataStore.edit { prefs -> block(prefs) }
        }
    }
}

// Type alias so the lambda signature can reference MutablePreferences without the full import.
private typealias MutablePreferences = androidx.datastore.preferences.core.MutablePreferences
