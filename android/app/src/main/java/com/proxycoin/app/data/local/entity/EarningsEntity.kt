package com.proxycoin.app.data.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "earnings")
data class EarningsEntity(
    @PrimaryKey val date: String,           // YYYY-MM-DD
    val bandwidthEarnings: Long,            // smallest token unit
    val uptimeBonus: Long,
    val qualityBonus: Long,
    val referralEarnings: Long,
    val totalBytes: Long,
    val requestsServed: Int,
    val avgLatencyMs: Int,
    val updatedAt: Long,                    // epoch millis
)
