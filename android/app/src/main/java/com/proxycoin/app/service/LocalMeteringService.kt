package com.proxycoin.app.service

import android.util.Log
import com.proxycoin.app.data.local.dao.MeteringDao
import com.proxycoin.app.data.local.entity.MeteringEntity
import com.proxycoin.app.data.remote.ApiService
import com.proxycoin.app.data.remote.dto.MeteringEventDto
import com.proxycoin.app.data.remote.dto.MeteringReportRequest
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import javax.inject.Inject
import javax.inject.Singleton

private const val UPLOAD_BATCH_SIZE = 250
private const val TAG = "LocalMeteringService"

/**
 * Records proxy request metering data locally in Room and periodically
 * uploads batches to the coordinator backend.
 *
 * Strategy: events are appended to [MeteringDao] as they arrive, then
 * [uploadPendingEvents] reads the oldest [UPLOAD_BATCH_SIZE] rows, sends them
 * to the API, and deletes the successfully uploaded rows from the local DB.
 * This guarantees at-least-once delivery without a separate "uploaded" flag.
 */
@Singleton
class LocalMeteringService @Inject constructor(
    private val meteringDao: MeteringDao,
    private val apiService: ApiService,
) {

    /**
     * Persists a single proxy request result to the local database.
     *
     * @param requestId Unique identifier assigned by the coordinator.
     * @param bytesIn   Bytes received from the origin server.
     * @param bytesOut  Bytes sent by the client (request body).
     * @param latencyMs Round-trip latency measured by [ProxyEngine].
     * @param success   Whether the request completed with a 2xx status.
     */
    suspend fun recordEvent(
        requestId: String,
        bytesIn: Long,
        bytesOut: Long,
        latencyMs: Int,
        success: Boolean,
    ) = withContext(Dispatchers.IO) {
        val entity = MeteringEntity(
            requestId = requestId,
            bytesIn = bytesIn,
            bytesOut = bytesOut,
            latencyMs = latencyMs,
            success = success,
            timestamp = System.currentTimeMillis(),
        )
        try {
            meteringDao.insert(entity)
            Log.v(TAG, "Recorded event: requestId=$requestId bytes=${bytesIn + bytesOut}")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to persist metering event $requestId: ${e.message}")
        }
    }

    /**
     * Reads up to [UPLOAD_BATCH_SIZE] pending metering events from the local DB,
     * uploads them to the backend, and deletes the rows that were successfully
     * acknowledged.
     *
     * If the API call fails the rows are left in the DB and will be retried on
     * the next invocation.
     *
     * @param nodeId The node identifier returned by the registration endpoint.
     */
    suspend fun uploadPendingEvents(nodeId: String) = withContext(Dispatchers.IO) {
        try {
            val pending = meteringDao.getOldestEvents(UPLOAD_BATCH_SIZE)
            if (pending.isEmpty()) {
                Log.v(TAG, "No pending metering events to upload")
                return@withContext
            }

            Log.d(TAG, "Uploading ${pending.size} metering events for node $nodeId")

            val dtos = pending.map { entity ->
                MeteringEventDto(
                    requestId = entity.requestId,
                    bytesIn = entity.bytesIn,
                    bytesOut = entity.bytesOut,
                    latencyMs = entity.latencyMs,
                    success = entity.success,
                    timestamp = entity.timestamp,
                )
            }

            apiService.reportMetering(
                MeteringReportRequest(nodeId = nodeId, events = dtos)
            )

            // Upload succeeded — delete the rows we just sent.
            val uploadedIds = pending.map { it.id }
            meteringDao.deleteByIds(uploadedIds)
            Log.d(TAG, "Deleted ${uploadedIds.size} uploaded metering events")
        } catch (e: Exception) {
            Log.w(TAG, "Failed to upload metering events (will retry later): ${e.message}")
        }
    }

    /**
     * Returns the total bytes proxied (bytesIn + bytesOut) since [timestamp].
     *
     * This is a one-shot suspend call rather than a Flow, suitable for periodic
     * snapshot reporting.
     *
     * @param timestamp Epoch milliseconds lower bound (exclusive).
     */
    suspend fun getTotalBytesSince(timestamp: Long): Long = withContext(Dispatchers.IO) {
        try {
            meteringDao.getTotalBytesSinceSnapshot(timestamp)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to query total bytes since $timestamp: ${e.message}")
            0L
        }
    }

    /**
     * Returns up to [limit] of the most recent metering events, ordered newest-first.
     *
     * @param limit Maximum number of rows to return.
     */
    suspend fun getRecentEvents(limit: Int): List<MeteringEntity> = withContext(Dispatchers.IO) {
        try {
            // getOldestEvents returns ascending order; reverse for recency.
            meteringDao.getOldestEvents(limit).asReversed()
        } catch (e: Exception) {
            Log.e(TAG, "Failed to query recent events: ${e.message}")
            emptyList()
        }
    }
}
