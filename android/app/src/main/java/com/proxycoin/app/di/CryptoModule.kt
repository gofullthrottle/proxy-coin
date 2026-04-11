package com.proxycoin.app.di

import com.proxycoin.app.crypto.KeystoreHelper
import com.proxycoin.app.crypto.TransactionBuilder
import com.proxycoin.app.crypto.WalletManager
import com.proxycoin.app.data.local.dao.TransactionDao
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object CryptoModule {

    /**
     * [KeystoreHelper] and [WalletManager] are @Singleton classes annotated with @Inject,
     * so Hilt generates their bindings automatically. No manual @Provides is needed for them.
     *
     * [TransactionBuilder] requires [TransactionDao] which lives in the database module,
     * so we wire it here explicitly.
     */

    @Provides
    @Singleton
    fun provideTransactionBuilder(transactionDao: TransactionDao): TransactionBuilder {
        return TransactionBuilder(transactionDao)
    }
}
