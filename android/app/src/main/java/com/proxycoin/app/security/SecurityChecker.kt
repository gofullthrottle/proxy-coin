package com.proxycoin.app.security

import android.content.Context
import android.content.pm.PackageManager
import android.hardware.Sensor
import android.hardware.SensorManager
import android.os.Build
import android.telephony.TelephonyManager
import android.util.Log
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import java.security.MessageDigest
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Device security checks for the ProxyCoin node.
 *
 * These checks defend against fraud — rooted devices, emulators, and tampered APKs
 * can fake bandwidth-sharing metrics. They are layered defenses; none individually
 * is foolproof, but together they raise the cost of fraud significantly.
 *
 * Important: Security checks are never run on the main thread. Call [performSecurityCheck]
 * from a background coroutine or WorkManager task.
 */
@Singleton
class SecurityChecker @Inject constructor(
    @ApplicationContext private val context: Context,
) {

    companion object {
        private const val TAG = "SecurityChecker"

        // Expected SHA-256 fingerprint of the ProxyCoin release signing certificate.
        // Set to null to skip APK signature verification in debug builds.
        // Replace with the actual hex fingerprint before release.
        private const val EXPECTED_CERT_SHA256: String? = null

        // Packages that indicate root management tools.
        private val ROOT_PACKAGE_NAMES = listOf(
            "com.topjohnwu.magisk",
            "com.koushikdutta.superuser",
            "com.noshufou.android.su",
            "com.thirdparty.superuser",
            "eu.chainfire.supersu",
            "com.kingroot.kinguser",
            "com.kingo.root",
            "com.smedialink.oneclickroot",
            "com.zhiqupk.root.global",
            "com.alephzain.framaroot",
        )

        // Well-known su binary paths to probe.
        private val SU_PATHS = listOf(
            "/system/xbin/su",
            "/system/bin/su",
            "/sbin/su",
            "/su/bin/su",
            "/data/local/xbin/su",
            "/data/local/bin/su",
            "/data/local/su",
            "/system/sd/xbin/su",
            "/system/bin/failsafe/su",
            "/dev/com.koushikdutta.superuser.daemon/",
        )

        // Magisk-related paths (hide-aware checks).
        private val MAGISK_PATHS = listOf(
            "/data/adb/magisk",
            "/sbin/.magisk",
            "/cache/.disable_magisk",
            "/dev/.magisk.unblock",
            "/cache/magisk.log",
            "/data/adb/magisk.img",
        )

        // Superuser APK paths.
        private val ROOT_APK_PATHS = listOf(
            "/system/app/Superuser.apk",
            "/system/app/SuperUser.apk",
            "/system/app/Supersu.apk",
        )

        // Minimum sensor count for a physical device.
        private const val MIN_SENSOR_COUNT = 3

        // Emulator-related Build property fragments.
        private val EMULATOR_BUILD_FINGERPRINTS = listOf(
            "generic",
            "unknown",
            "google_sdk",
            "emulator",
            "android sdk built for",
        )
        private val EMULATOR_MODELS = listOf(
            "Emulator",
            "Android SDK built for",
            "sdk_gphone",
            "sdk_gphone64",
        )
        private val EMULATOR_MANUFACTURERS = listOf(
            "Genymotion",
            "unknown",
        )
        private val EMULATOR_HARDWARE = listOf(
            "goldfish",
            "ranchu",
        )
    }

    // ── Root detection ─────────────────────────────────────────────────────────

    /**
     * Returns true if any root indicator is detected.
     */
    fun isRooted(): Boolean {
        val rooted = checkSuBinary() || checkMagisk() || checkRootApks() || checkRootManagementApps()
        if (rooted) Log.w(TAG, "Root detected")
        return rooted
    }

    /**
     * Probes standard su binary paths on the filesystem.
     */
    private fun checkSuBinary(): Boolean {
        return SU_PATHS.any { path -> File(path).exists() }
    }

    /**
     * Probes known Magisk artifact paths.
     */
    private fun checkMagisk(): Boolean {
        return MAGISK_PATHS.any { path -> File(path).exists() }
    }

    /**
     * Checks for Superuser APK files embedded in the system image.
     */
    private fun checkRootApks(): Boolean {
        return ROOT_APK_PATHS.any { path -> File(path).exists() }
    }

    /**
     * Checks the installed package list for known root management apps.
     */
    private fun checkRootManagementApps(): Boolean {
        val pm = context.packageManager
        return ROOT_PACKAGE_NAMES.any { pkg ->
            try {
                pm.getPackageInfo(pkg, 0)
                true
            } catch (_: PackageManager.NameNotFoundException) {
                false
            }
        }
    }

    // ── Emulator detection ─────────────────────────────────────────────────────

    /**
     * Returns true if the device is likely an emulator.
     */
    fun isEmulator(): Boolean {
        val isEmulator = checkBuildProps() || checkSensorCount() || checkTelephony()
        if (isEmulator) Log.w(TAG, "Emulator detected")
        return isEmulator
    }

    /**
     * Checks Build constants for emulator fingerprints.
     */
    private fun checkBuildProps(): Boolean {
        val fingerprint = Build.FINGERPRINT.lowercase()
        val model = Build.MODEL.lowercase()
        val manufacturer = Build.MANUFACTURER.lowercase()
        val hardware = Build.HARDWARE.lowercase()
        val product = Build.PRODUCT.lowercase()

        return EMULATOR_BUILD_FINGERPRINTS.any { fingerprint.contains(it) }
            || EMULATOR_MODELS.any { model.contains(it.lowercase()) }
            || EMULATOR_MANUFACTURERS.any { manufacturer.contains(it.lowercase()) }
            || EMULATOR_HARDWARE.any { hardware.contains(it) }
            || product == "sdk" || product == "sdk_x86" || product == "vbox86p"
    }

    /**
     * Physical devices have many sensors; emulators often have very few.
     */
    private fun checkSensorCount(): Boolean {
        val sensorManager = context.getSystemService(Context.SENSOR_SERVICE) as SensorManager
        val sensorCount = sensorManager.getSensorList(Sensor.TYPE_ALL).size
        return sensorCount < MIN_SENSOR_COUNT
    }

    /**
     * Emulators usually return null or "000000000000000" for the device ID.
     */
    private fun checkTelephony(): Boolean {
        return try {
            val tm = context.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
            val networkOp = tm.networkOperatorName
            networkOp.equals("android", ignoreCase = true)
        } catch (_: Exception) {
            false
        }
    }

    // ── APK tamper detection ───────────────────────────────────────────────────

    /**
     * Returns true if the APK signing certificate does not match the expected fingerprint.
     * Returns false (no tampering) if [EXPECTED_CERT_SHA256] is null (debug/CI builds).
     */
    fun isApkTampered(): Boolean {
        val expectedFingerprint = EXPECTED_CERT_SHA256 ?: return false
        return try {
            val actualFingerprint = getSigningCertificateSha256()
            val tampered = !actualFingerprint.equals(expectedFingerprint, ignoreCase = true)
            if (tampered) Log.e(TAG, "APK signature mismatch: $actualFingerprint != $expectedFingerprint")
            tampered
        } catch (e: Exception) {
            Log.e(TAG, "APK tamper check failed: ${e.message}")
            // Fail open — don't block on check failure, but log for analysis.
            false
        }
    }

    /**
     * Returns the SHA-256 hex fingerprint of the first signing certificate.
     */
    @Suppress("DEPRECATION")
    private fun getSigningCertificateSha256(): String {
        val pm = context.packageManager
        val packageName = context.packageName
        val signatures = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val info = pm.getPackageInfo(packageName, PackageManager.GET_SIGNING_CERTIFICATES)
            info.signingInfo?.apkContentsSigners ?: emptyArray()
        } else {
            val info = pm.getPackageInfo(packageName, PackageManager.GET_SIGNATURES)
            @Suppress("DEPRECATION")
            info.signatures ?: emptyArray()
        }

        if (signatures.isEmpty()) return ""

        val certBytes = signatures[0].toByteArray()
        val digest = MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(certBytes)
        return hash.joinToString("") { "%02x".format(it) }
    }

    // ── Composite check ────────────────────────────────────────────────────────

    /**
     * Runs all security checks and returns a [SecurityResult].
     * This is the primary entry point; call from a background coroutine.
     */
    fun performSecurityCheck(): SecurityResult {
        return SecurityResult(
            isRooted = isRooted(),
            isEmulator = isEmulator(),
            isApkTampered = isApkTampered(),
        )
    }
}

data class SecurityResult(
    val isRooted: Boolean,
    val isEmulator: Boolean,
    val isApkTampered: Boolean,
) {
    /**
     * Returns true only if no security threats are detected.
     */
    val isSecure: Boolean get() = !isRooted && !isEmulator && !isApkTampered

    /**
     * Human-readable summary of detected issues, or null if secure.
     */
    val threatSummary: String?
        get() {
            val threats = buildList {
                if (isRooted) add("device is rooted")
                if (isEmulator) add("device is an emulator")
                if (isApkTampered) add("APK signature is invalid")
            }
            return if (threats.isEmpty()) null else threats.joinToString(", ")
        }
}
