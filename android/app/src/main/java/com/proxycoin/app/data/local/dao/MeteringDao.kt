package com.proxycoin.app.data.local.dao

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.Query
import com.proxycoin.app.data.local.entity.MeteringEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface MeteringDao {

    @Insert
    suspend fun insert(event: MeteringEntity)

    @Insert
    suspend fun insertAll(events: List<MeteringEntity>)

    @Query("SELECT * FROM metering WHERE timestamp > :since ORDER BY timestamp DESC")
    fun getEventsSince(since: Long): Flow<List<MeteringEntity>>

    @Query("SELECT SUM(bytesIn + bytesOut) FROM metering WHERE timestamp > :since")
    fun getTotalBytesSince(since: Long): Flow<Long?>

    @Query("DELETE FROM metering WHERE timestamp < :before")
    suspend fun deleteOlderThan(before: Long)

    /** One-shot fetch of all events, oldest first, for batch upload. */
    @Query("SELECT * FROM metering ORDER BY timestamp ASC LIMIT :limit")
    suspend fun getOldestEvents(limit: Int): List<MeteringEntity>

    /** One-shot sum of (bytesIn + bytesOut) since [since]; returns 0 when there are no rows. */
    @Query("SELECT COALESCE(SUM(bytesIn + bytesOut), 0) FROM metering WHERE timestamp > :since")
    suspend fun getTotalBytesSinceSnapshot(since: Long): Long

    /** Delete specific rows by primary key after successful upload. */
    @Query("DELETE FROM metering WHERE id IN (:ids)")
    suspend fun deleteByIds(ids: List<Long>)
}
