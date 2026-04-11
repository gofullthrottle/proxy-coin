package com.proxycoin.app.data.remote

import com.proxycoin.app.data.remote.dto.ApplyReferralRequest
import com.proxycoin.app.data.remote.dto.AttestationRequest
import com.proxycoin.app.data.remote.dto.ClaimProofResponse
import com.proxycoin.app.data.remote.dto.EarningsResponse
import com.proxycoin.app.data.remote.dto.EarningsSummaryResponse
import com.proxycoin.app.data.remote.dto.MeteringReportRequest
import com.proxycoin.app.data.remote.dto.NodeConfigResponse
import com.proxycoin.app.data.remote.dto.PendingEarningsResponse
import com.proxycoin.app.data.remote.dto.ReferralCodeResponse
import com.proxycoin.app.data.remote.dto.ReferralStatsResponse
import com.proxycoin.app.data.remote.dto.RegisterNodeRequest
import com.proxycoin.app.data.remote.dto.RegisterNodeResponse
import com.proxycoin.app.data.remote.dto.RewardHistoryResponse
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Query

interface ApiService {

    // Node endpoints
    @POST("v1/node/register")
    suspend fun registerNode(@Body request: RegisterNodeRequest): RegisterNodeResponse

    @GET("v1/node/config")
    suspend fun getNodeConfig(): NodeConfigResponse

    @POST("v1/node/attest")
    suspend fun submitAttestation(@Body attestation: AttestationRequest)

    // Earnings endpoints
    @GET("v1/earnings")
    suspend fun getEarnings(
        @Query("from") from: String,
        @Query("to") to: String,
    ): EarningsResponse

    @GET("v1/earnings/summary")
    suspend fun getEarningsSummary(): EarningsSummaryResponse

    @GET("v1/earnings/pending")
    suspend fun getPendingEarnings(): PendingEarningsResponse

    // Rewards endpoints
    @GET("v1/rewards/proof")
    suspend fun getClaimProof(): ClaimProofResponse

    @GET("v1/rewards/history")
    suspend fun getRewardHistory(): RewardHistoryResponse

    // Metering
    @POST("v1/metering/report")
    suspend fun reportMetering(@Body report: MeteringReportRequest)

    // Referral
    @GET("v1/referral/code")
    suspend fun getReferralCode(): ReferralCodeResponse

    @GET("v1/referral/stats")
    suspend fun getReferralStats(): ReferralStatsResponse

    @POST("v1/referral/apply")
    suspend fun applyReferralCode(@Body request: ApplyReferralRequest)
}
