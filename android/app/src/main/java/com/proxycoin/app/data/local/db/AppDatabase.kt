package com.proxycoin.app.data.local.db

import androidx.room.Database
import androidx.room.RoomDatabase
import com.proxycoin.app.data.local.dao.EarningsDao
import com.proxycoin.app.data.local.dao.MeteringDao
import com.proxycoin.app.data.local.dao.TransactionDao
import com.proxycoin.app.data.local.entity.EarningsEntity
import com.proxycoin.app.data.local.entity.MeteringEntity
import com.proxycoin.app.data.local.entity.TransactionEntity

@Database(
    entities = [EarningsEntity::class, MeteringEntity::class, TransactionEntity::class],
    version = 1,
    exportSchema = false,
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun earningsDao(): EarningsDao
    abstract fun meteringDao(): MeteringDao
    abstract fun transactionDao(): TransactionDao
}
