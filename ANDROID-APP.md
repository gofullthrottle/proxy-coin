# Proxy Coin - Android App Specification

## Architecture

MVVM + Clean Architecture with 4 layers:

```
┌────────────────────────────────────────┐
│  Presentation Layer (Jetpack Compose)  │
│  Screens, ViewModels, Navigation       │
├────────────────────────────────────────┤
│  Domain Layer (Pure Kotlin)            │
│  Use Cases, Repository Interfaces,     │
│  Domain Models                         │
├────────────────────────────────────────┤
│  Data Layer                            │
│  Repository Impls, API, Room DB,       │
│  DataStore, EncryptedPrefs             │
├────────────────────────────────────────┤
│  Service Layer                         │
│  Foreground Service, Proxy Engine,     │
│  WebSocket Client, Resource Monitor    │
└────────────────────────────────────────┘
```

**Dependency Injection**: Hilt (standard for modern Android)

## Android Dependencies

```toml
# gradle/libs.versions.toml

[versions]
kotlin = "2.0.21"
compose-bom = "2024.12.01"
hilt = "2.51.1"
room = "2.6.1"
retrofit = "2.11.0"
okhttp = "4.12.0"
web3j = "4.12.2"
datastore = "1.1.1"
lifecycle = "2.8.7"
navigation = "2.8.5"
coroutines = "1.9.0"
protobuf = "4.28.3"
work = "2.10.0"
play-integrity = "1.4.0"

[libraries]
# Compose
compose-bom = { group = "androidx.compose", name = "compose-bom", version.ref = "compose-bom" }
compose-material3 = { group = "androidx.compose.material3", name = "material3" }
compose-ui = { group = "androidx.compose.ui", name = "ui" }
compose-navigation = { group = "androidx.navigation", name = "navigation-compose", version.ref = "navigation" }

# Hilt
hilt-android = { group = "com.google.dagger", name = "hilt-android", version.ref = "hilt" }
hilt-compiler = { group = "com.google.dagger", name = "hilt-android-compiler", version.ref = "hilt" }
hilt-navigation = { group = "androidx.hilt", name = "hilt-navigation-compose", version = "1.2.0" }

# Room
room-runtime = { group = "androidx.room", name = "room-runtime", version.ref = "room" }
room-ktx = { group = "androidx.room", name = "room-ktx", version.ref = "room" }
room-compiler = { group = "androidx.room", name = "room-compiler", version.ref = "room" }

# Network
retrofit = { group = "com.squareup.retrofit2", name = "retrofit", version.ref = "retrofit" }
okhttp = { group = "com.squareup.okhttp3", name = "okhttp", version.ref = "okhttp" }
okhttp-logging = { group = "com.squareup.okhttp3", name = "logging-interceptor", version.ref = "okhttp" }

# Crypto
web3j = { group = "org.web3j", name = "core", version.ref = "web3j" }

# Protobuf
protobuf-kotlin = { group = "com.google.protobuf", name = "protobuf-kotlin-lite", version.ref = "protobuf" }

# WorkManager
work-runtime = { group = "androidx.work", name = "work-runtime-ktx", version.ref = "work" }

# Play Integrity
play-integrity = { group = "com.google.android.play", name = "integrity", version.ref = "play-integrity" }

# DataStore
datastore = { group = "androidx.datastore", name = "datastore-preferences", version.ref = "datastore" }
```

## Android Permissions

```xml
<!-- AndroidManifest.xml -->
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
<uses-permission android:name="android.permission.ACCESS_WIFI_STATE" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE_DATA_SYNC" />
<uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
<uses-permission android:name="android.permission.RECEIVE_BOOT_COMPLETED" />
<uses-permission android:name="android.permission.REQUEST_IGNORE_BATTERY_OPTIMIZATIONS" />
<uses-permission android:name="android.permission.WAKE_LOCK" />
```

## Screens

### 1. Onboarding Flow

**WelcomeScreen**
- App logo and tagline
- "Earn PRXY tokens by sharing your unused bandwidth"
- Value proposition cards (earn passively, secure, transparent)
- "Get Started" CTA

**PermissionsScreen**
- Battery optimization exemption request
- Notification permission (Android 13+)
- Explanation of why each permission is needed
- "Allow" buttons for each

**WalletSetupScreen**
- Two options: "Create New Wallet" / "Import Existing"
- Create: generate BIP-39 mnemonic, show seed phrase, confirm backup
- Import: paste existing mnemonic or private key
- Wallet address display with copy button

### 2. Dashboard (Main Screen)

```
┌──────────────────────────────────┐
│  PROXY COIN            ⚙️       │
│                                  │
│  ┌──────────────────────────┐   │
│  │  ● CONNECTED             │   │
│  │  [====== ON/OFF ======]  │   │
│  └──────────────────────────┘   │
│                                  │
│  ┌──────────┐  ┌──────────┐    │
│  │ TODAY     │  │ ALL TIME │    │
│  │ 42.5 PRXY│  │ 1,247    │    │
│  │ ≈ $2.13  │  │ ≈ $62.35 │    │
│  └──────────┘  └──────────┘    │
│                                  │
│  ┌──────────────────────────┐   │
│  │ BANDWIDTH SHARED         │   │
│  │ ████████████░░░░  2.4 GB │   │
│  │ ↑ 1.2 GB  ↓ 1.2 GB     │   │
│  └──────────────────────────┘   │
│                                  │
│  ┌──────────────────────────┐   │
│  │ LIVE STATS               │   │
│  │ Uptime: 4h 23m           │   │
│  │ Requests served: 847     │   │
│  │ Avg latency: 124ms       │   │
│  │ Trust score: 0.87 ★★★★☆  │   │
│  └──────────────────────────┘   │
│                                  │
│  ┌──────────────────────────┐   │
│  │ [📊 chart: 7-day trend]  │   │
│  └──────────────────────────┘   │
│                                  │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━   │
│  🏠 Home  💰 Earn  👛 Wallet ⚙ │
└──────────────────────────────────┘
```

**State**: `DashboardUiState`
- `isConnected: Boolean`
- `todayEarnings: BigDecimal`
- `allTimeEarnings: BigDecimal`
- `bandwidthShared: Long` (bytes)
- `uptimeSeconds: Long`
- `requestsServed: Int`
- `avgLatencyMs: Int`
- `trustScore: Float`
- `weeklyTrend: List<DailyEarning>`
- `tokenPrice: BigDecimal?`

### 3. Earnings Screen

- Tab bar: Day | Week | Month | All
- Earnings chart (line or bar)
- Earnings breakdown table:
  - Bandwidth earnings
  - Uptime bonus
  - Quality bonus
  - Referral earnings
- Pending vs claimed amounts
- Claim button (when threshold met)

### 4. Wallet Screen

- PRXY balance (on-chain confirmed)
- Pending balance (off-chain, not yet claimable)
- Wallet address with QR code and copy
- "Claim Rewards" button (triggers Merkle proof claim)
- "Send PRXY" button
- Transaction history list
- Token price (from DEX)
- Link to view on block explorer

### 5. Settings Screen

- **Bandwidth Settings**
  - Max bandwidth: slider (10% - 100%)
  - WiFi only toggle (default: on)
  - Allow cellular toggle (default: off)
  - Cellular data cap (if enabled): MB/day
- **Power Settings**
  - Battery threshold: slider (10% - 50%, default 20%)
  - Charging only mode toggle
- **Compute Settings** (Phase 2)
  - Enable compute sharing toggle
  - Max CPU usage: slider
- **Notification Settings**
  - Earnings milestones toggle
  - Connection status toggle
  - Weekly summary toggle
- **Account**
  - Device ID
  - Node ID
  - Trust score
  - Referral code
  - Export wallet (show seed phrase with security confirmation)
  - Delete account
- **About**
  - Version
  - Terms of Service link
  - Privacy Policy link
  - Open source licenses

### 6. Referral Screen

- Unique referral code
- Share button (deep link)
- Referral stats: count, total earnings from referrals
- Referral list with earnings per referral

## Core Services

### ProxyForegroundService

The heart of the app. Runs as a persistent foreground service.

```kotlin
@AndroidEntryPoint
class ProxyForegroundService : Service() {

    @Inject lateinit var webSocketClient: WebSocketClient
    @Inject lateinit var proxyEngine: ProxyEngine
    @Inject lateinit var resourceMonitor: ResourceMonitor
    @Inject lateinit var meteringService: MeteringService

    // Lifecycle:
    // 1. Create notification channel
    // 2. Start as foreground with persistent notification
    // 3. Connect WebSocket to backend
    // 4. Listen for proxy requests
    // 5. Monitor resources, pause if thresholds exceeded
    // 6. Update notification with stats

    // START_STICKY: restart if killed by system
    // WakeLock: keep CPU active
    // WifiLock: keep WiFi active
}
```

**Notification**: Shows connection status, today's earnings, bandwidth shared. Updated every 30 seconds.

### WebSocketClient

```kotlin
class WebSocketClient @Inject constructor(
    private val okHttpClient: OkHttpClient,
    private val protobufCodec: ProtobufCodec,
    private val config: AppConfig
) {
    // Connection lifecycle:
    // - Connect with TLS + cert pinning
    // - Send REGISTER message with device info
    // - Receive REGISTERED with node config
    // - Heartbeat every 30s
    // - Reconnect with exponential backoff (1s, 2s, 4s, 8s, 16s, 30s max)
    // - Jitter to prevent thundering herd
    // - Resume session within 30s window

    // Message handling:
    // - Deserialize protobuf messages
    // - Route PROXY_REQUEST to ProxyEngine
    // - Route CONFIG_UPDATE to settings
    // - Route EARNINGS_UPDATE to earnings tracker

    // Multiplexing:
    // - Multiple concurrent requests over single connection
    // - Each request has unique ID
    // - Configurable max concurrent (default: 5)
    // - Back-pressure when overloaded
}
```

### ProxyEngine

```kotlin
class ProxyEngine @Inject constructor(
    private val okHttpClient: OkHttpClient,
    private val domainFilter: DomainFilter,
    private val meteringService: MeteringService
) {
    // For each PROXY_REQUEST:
    // 1. Validate URL against domain blocklist
    // 2. Build OkHttp Request from proxy request
    // 3. Execute request from device's network
    // 4. Stream response back in chunks (64KB)
    // 5. Record bytes in metering service
    // 6. Handle errors (timeout, DNS failure, connection refused)

    // Concurrent request limit via Semaphore
    // Timeout per request: 30s (configurable by backend)
    // Max response size: 10MB (configurable)
}
```

### ResourceMonitor

```kotlin
class ResourceMonitor @Inject constructor(
    private val context: Context,
    private val settingsRepository: SettingsRepository
) {
    // Monitors:
    // - Battery level (BatteryManager)
    // - Charging state
    // - Network type (ConnectivityManager: WiFi, cellular, metered)
    // - Network speed estimation
    // - CPU temperature (thermal API, Android 11+)
    // - Available memory

    // Emits: ResourceState every 10s
    // ProxyForegroundService pauses proxy when:
    //   - Battery below threshold (default 20%)
    //   - Not on WiFi (if WiFi-only enabled)
    //   - On metered network (if cellular disabled)
    //   - Thermal throttling detected
}
```

### WalletManager

```kotlin
class WalletManager @Inject constructor(
    private val keystoreHelper: KeystoreHelper,
    private val web3j: Web3j,
    private val config: AppConfig
) {
    // Key management:
    // - Generate BIP-39 mnemonic (12 or 24 words)
    // - Derive Ethereum keypair from mnemonic (BIP-44 path)
    // - Store private key encrypted in Android Keystore
    // - Never expose private key to any external component

    // Operations:
    // - getAddress(): String — public wallet address
    // - getBalance(): BigDecimal — PRXY token balance (ERC-20 balanceOf)
    // - claimRewards(merkleProof): TxHash — call RewardDistributor.claim()
    // - sendTokens(to, amount): TxHash — ERC-20 transfer
    // - signMessage(message): Signature — for authentication

    // RPC: Alchemy/Infura for Base L2
    // Gas estimation: use web3j gas oracle
    // Transaction confirmation: wait for 1 block on Base
}
```

## Data Layer

### Room Database

```kotlin
@Database(
    entities = [
        EarningsEntity::class,
        MeteringEntity::class,
        TransactionEntity::class
    ],
    version = 1
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun earningsDao(): EarningsDao
    abstract fun meteringDao(): MeteringDao
    abstract fun transactionDao(): TransactionDao
}

@Entity(tableName = "earnings")
data class EarningsEntity(
    @PrimaryKey val date: String,           // YYYY-MM-DD
    val bandwidthEarnings: Long,            // in smallest token unit
    val uptimeBonus: Long,
    val qualityBonus: Long,
    val referralEarnings: Long,
    val totalBytes: Long,
    val requestsServed: Int,
    val avgLatencyMs: Int,
    val updatedAt: Long                     // epoch millis
)

@Entity(tableName = "metering")
data class MeteringEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val requestId: String,
    val bytesIn: Long,
    val bytesOut: Long,
    val latencyMs: Int,
    val success: Boolean,
    val timestamp: Long
)

@Entity(tableName = "transactions")
data class TransactionEntity(
    @PrimaryKey val txHash: String,
    val type: String,                       // "claim", "send", "receive"
    val amount: String,                     // BigDecimal as string
    val toAddress: String?,
    val fromAddress: String?,
    val status: String,                     // "pending", "confirmed", "failed"
    val blockNumber: Long?,
    val timestamp: Long
)
```

### API Service (Retrofit)

```kotlin
interface ApiService {

    @POST("v1/node/register")
    suspend fun registerNode(@Body request: RegisterNodeRequest): RegisterNodeResponse

    @GET("v1/node/config")
    suspend fun getNodeConfig(): NodeConfigResponse

    @GET("v1/earnings")
    suspend fun getEarnings(
        @Query("from") from: String,
        @Query("to") to: String
    ): EarningsResponse

    @GET("v1/earnings/summary")
    suspend fun getEarningsSummary(): EarningsSummaryResponse

    @GET("v1/rewards/proof")
    suspend fun getClaimProof(): ClaimProofResponse

    @POST("v1/metering/report")
    suspend fun reportMetering(@Body report: MeteringReport): Unit

    @GET("v1/referral/stats")
    suspend fun getReferralStats(): ReferralStatsResponse

    @POST("v1/device/attest")
    suspend fun submitAttestation(@Body attestation: AttestationRequest): Unit
}
```

## Security Measures

1. **Certificate Pinning**: Pin backend API certificate in OkHttp
2. **Encrypted Storage**: Android Keystore for wallet keys, EncryptedSharedPreferences for sensitive data
3. **Root Detection**: Check for su binary, Magisk, system modifications
4. **Emulator Detection**: Check build properties, hardware sensors, telephony
5. **Play Integrity**: Periodic attestation to verify genuine device + APK
6. **ProGuard/R8**: Obfuscate release builds
7. **No Logging**: Strip all log statements in release builds
8. **Memory Safety**: Clear sensitive data (keys, mnemonics) from memory after use
