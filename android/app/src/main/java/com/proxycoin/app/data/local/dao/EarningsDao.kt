package com.proxycoin.app.data.local.dao

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import com.proxycoin.app.data.local.entity.EarningsEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface EarningsDao {

    @Query("SELECT * FROM earnings ORDER BY date DESC")
    fun getAllEarnings(): Flow<List<EarningsEntity>>

    @Query("SELECT * FROM earnings WHERE date = :date")
    suspend fun getByDate(date: String): EarningsEntity?

    @Query(
        "SELECT * FROM earnings WHERE date >= :startDate AND date <= :endDate ORDER BY date DESC",
    )
    fun getEarningsRange(startDate: String, endDate: String): Flow<List<EarningsEntity>>

    @Query(
        "SELECT SUM(bandwidthEarnings + uptimeBonus + qualityBonus + referralEarnings) FROM earnings",
    )
    fun getTotalEarnings(): Flow<Long?>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(earnings: EarningsEntity)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertAll(earnings: List<EarningsEntity>)
}
