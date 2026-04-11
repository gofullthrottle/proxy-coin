package com.proxycoin.app.di

import android.content.Context
import androidx.room.Room
import com.proxycoin.app.data.local.dao.EarningsDao
import com.proxycoin.app.data.local.dao.MeteringDao
import com.proxycoin.app.data.local.dao.TransactionDao
import com.proxycoin.app.data.local.db.AppDatabase
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object DatabaseModule {

    @Provides
    @Singleton
    fun provideDatabase(@ApplicationContext context: Context): AppDatabase {
        return Room.databaseBuilder(context, AppDatabase::class.java, "proxycoin.db")
            .fallbackToDestructiveMigration()
            .build()
    }

    @Provides
    fun provideEarningsDao(db: AppDatabase): EarningsDao = db.earningsDao()

    @Provides
    fun provideMeteringDao(db: AppDatabase): MeteringDao = db.meteringDao()

    @Provides
    fun provideTransactionDao(db: AppDatabase): TransactionDao = db.transactionDao()
}
