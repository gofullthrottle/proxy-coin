package com.proxycoin.app.data.remote.interceptor

import okhttp3.Interceptor
import okhttp3.Response
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AuthInterceptor @Inject constructor() : Interceptor {

    @Volatile
    private var token: String? = null

    fun setToken(token: String?) {
        this.token = token
    }

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val builder = request.newBuilder()

        token?.let {
            builder.addHeader("Authorization", "Bearer $it")
        }

        return chain.proceed(builder.build())
    }
}
