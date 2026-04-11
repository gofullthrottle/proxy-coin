package com.proxycoin.app.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.runBlocking

private val Context.settingsDataStore by preferencesDataStore(name = "app_settings")

/**
 * Starts the proxy node service after device boot.
 *
 * Reads the user preference [PREF_AUTO_START] to decide whether to launch
 * [ProxyForegroundService]. When the preference has never been set the
 * service starts by default — matching first-launch behaviour where users
 * have opted into sharing bandwidth.
 *
 * Requires RECEIVE_BOOT_COMPLETED permission in AndroidManifest.xml.
 */
class BootReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "BootReceiver"

        /**
         * DataStore key for the auto-start preference.
         * True (default) means the node will restart after a reboot.
         */
        val PREF_AUTO_START = booleanPreferencesKey("auto_start_on_boot")
    }

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return

        Log.i(TAG, "Boot completed — checking auto-start preference")

        val shouldAutoStart = runBlocking {
            try {
                context.settingsDataStore.data
                    .map { prefs -> prefs[PREF_AUTO_START] ?: true }
                    .first()
            } catch (e: Exception) {
                Log.e(TAG, "Failed to read auto-start preference, defaulting to true: ${e.message}")
                true
            }
        }

        if (shouldAutoStart) {
            Log.i(TAG, "Auto-start enabled — starting ProxyForegroundService")
            ProxyForegroundService.start(context)
        } else {
            Log.i(TAG, "Auto-start disabled — not starting service after boot")
        }
    }
}
