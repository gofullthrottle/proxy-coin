package com.proxycoin.app.util

object Constants {
    const val HEARTBEAT_INTERVAL_MS = 30_000L
    const val RECONNECT_INITIAL_DELAY_MS = 1_000L
    const val RECONNECT_MAX_DELAY_MS = 30_000L
    const val NOTIFICATION_CHANNEL_ID = "proxy_service"
    const val NOTIFICATION_ID = 1001
    const val MAX_CONCURRENT_REQUESTS = 5
    const val CHUNK_SIZE_BYTES = 64 * 1024 // 64KB
    const val MAX_RESPONSE_SIZE_BYTES = 10 * 1024 * 1024 // 10MB
    const val DEFAULT_REQUEST_TIMEOUT_MS = 30_000
    const val DEFAULT_BATTERY_THRESHOLD = 20
    const val RESOURCE_MONITOR_INTERVAL_MS = 10_000L
    const val NOTIFICATION_UPDATE_INTERVAL_MS = 30_000L
    const val MIN_CLAIM_AMOUNT = 100L // 100 PRXY minimum claim
}
