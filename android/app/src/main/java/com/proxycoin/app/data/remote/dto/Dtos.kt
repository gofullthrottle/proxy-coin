package com.proxycoin.app.data.remote.dto

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

// ─── Node ────────────────────────────────────────────────────────────────────

@JsonClass(generateAdapter = true)
data class RegisterNodeRequest(
    @Json(name = "device_id") val deviceId: String,
    @Json(name = "wallet_address") val walletAddress: String,
    @Json(name = "app_version") val appVersion: String,
    @Json(name = "os_version") val osVersion: String,
    @Json(name = "device_model") val deviceModel: String,
    @Json(name = "network_type") val networkType: String,
)

@JsonClass(generateAdapter = true)
data class RegisterNodeResponse(
    @Json(name = "node_id") val nodeId: String,
    val config: NodeConfigDto,
)

@JsonClass(generateAdapter = true)
data class NodeConfigResponse(
    @Json(name = "prxy_per_mb") val prxyPerMb: Long,
    @Json(name = "heartbeat_interval_ms") val heartbeatIntervalMs: Long,
    @Json(name = "max_concurrent") val maxConcurrent: Int,
)

@JsonClass(generateAdapter = true)
data class NodeConfigDto(
    @Json(name = "prxy_per_mb") val prxyPerMb: Long,
    @Json(name = "heartbeat_interval_ms") val heartbeatIntervalMs: Long,
    @Json(name = "max_concurrent") val maxConcurrent: Int,
)

@JsonClass(generateAdapter = true)
data class AttestationRequest(
    val token: String,
)

// ─── Earnings ─────────────────────────────────────────────────────────────────

@JsonClass(generateAdapter = true)
data class EarningsResponse(
    val earnings: List<EarningDto>,
)

@JsonClass(generateAdapter = true)
data class EarningDto(
    val date: String,
    @Json(name = "bandwidth_earnings") val bandwidthEarnings: Long,
    @Json(name = "uptime_bonus") val uptimeBonus: Long,
    @Json(name = "quality_bonus") val qualityBonus: Long,
    @Json(name = "referral_earnings") val referralEarnings: Long,
    @Json(name = "total_bytes") val totalBytes: Long,
    @Json(name = "requests_served") val requestsServed: Int,
)

@JsonClass(generateAdapter = true)
data class EarningsSummaryResponse(
    @Json(name = "today_earnings") val todayEarnings: Long,
    @Json(name = "all_time_earnings") val allTimeEarnings: Long,
    @Json(name = "pending_claim") val pendingClaim: Long,
    @Json(name = "trust_score") val trustScore: Float,
)

@JsonClass(generateAdapter = true)
data class PendingEarningsResponse(
    @Json(name = "pending_amount") val pendingAmount: Long,
    @Json(name = "last_calculated") val lastCalculated: Long,
)

// ─── Rewards ──────────────────────────────────────────────────────────────────

@JsonClass(generateAdapter = true)
data class ClaimProofResponse(
    @Json(name = "cumulative_amount") val cumulativeAmount: String,
    val proof: List<String>,
    val epoch: Long,
)

@JsonClass(generateAdapter = true)
data class RewardHistoryResponse(
    val claims: List<ClaimDto>,
)

@JsonClass(generateAdapter = true)
data class ClaimDto(
    @Json(name = "tx_hash") val txHash: String,
    val amount: String,
    val timestamp: Long,
    val status: String,
)

// ─── Metering ─────────────────────────────────────────────────────────────────

@JsonClass(generateAdapter = true)
data class MeteringReportRequest(
    @Json(name = "node_id") val nodeId: String,
    val events: List<MeteringEventDto>,
)

@JsonClass(generateAdapter = true)
data class MeteringEventDto(
    @Json(name = "request_id") val requestId: String,
    @Json(name = "bytes_in") val bytesIn: Long,
    @Json(name = "bytes_out") val bytesOut: Long,
    @Json(name = "latency_ms") val latencyMs: Int,
    val success: Boolean,
    val timestamp: Long,
)

// ─── Referral ─────────────────────────────────────────────────────────────────

@JsonClass(generateAdapter = true)
data class ReferralCodeResponse(
    val code: String,
)

@JsonClass(generateAdapter = true)
data class ReferralStatsResponse(
    @Json(name = "total_referrals") val totalReferrals: Int,
    @Json(name = "total_earnings") val totalEarnings: Long,
    val referrals: List<ReferralDto>,
)

@JsonClass(generateAdapter = true)
data class ReferralDto(
    @Json(name = "node_id") val nodeId: String,
    val earnings: Long,
    @Json(name = "joined_at") val joinedAt: Long,
)

@JsonClass(generateAdapter = true)
data class ApplyReferralRequest(
    val code: String,
)
