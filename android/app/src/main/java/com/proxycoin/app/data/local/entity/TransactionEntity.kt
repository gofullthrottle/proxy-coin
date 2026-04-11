package com.proxycoin.app.data.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "transactions")
data class TransactionEntity(
    @PrimaryKey val txHash: String,
    val type: String,                       // "claim", "send", "receive"
    val amount: String,                     // BigDecimal as string
    val toAddress: String?,
    val fromAddress: String?,
    val status: String,                     // "pending", "confirmed", "failed"
    val blockNumber: Long?,
    val timestamp: Long,
)
