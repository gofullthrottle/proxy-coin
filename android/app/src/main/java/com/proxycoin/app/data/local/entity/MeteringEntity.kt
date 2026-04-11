package com.proxycoin.app.data.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "metering")
data class MeteringEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val requestId: String,
    val bytesIn: Long,
    val bytesOut: Long,
    val latencyMs: Int,
    val success: Boolean,
    val timestamp: Long,
)
