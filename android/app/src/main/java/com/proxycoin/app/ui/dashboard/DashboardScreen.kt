package com.proxycoin.app.ui.dashboard

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.NetworkCheck
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material.icons.filled.Speed
import androidx.compose.material.icons.filled.Timer
import androidx.compose.material.icons.filled.Verified
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledIconToggleButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconToggleButtonColors
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.proxycoin.app.ui.theme.ProxyCoinAccent
import com.proxycoin.app.ui.theme.StatusConnected
import com.proxycoin.app.ui.theme.StatusDisconnected

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DashboardScreen(
    viewModel: DashboardViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }

    LaunchedEffect(uiState.errorMessage) {
        uiState.errorMessage?.let { snackbarHostState.showSnackbar(it) }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "ProxyCoin",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold,
                    )
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { innerPadding ->
        if (uiState.isLoading) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator()
            }
            return@Scaffold
        }

        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // Connection toggle card.
            ConnectionCard(
                isConnected = uiState.isConnected,
                onToggle = { viewModel.toggleConnection() },
            )

            // Earnings cards row.
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                EarningsCard(
                    modifier = Modifier.weight(1f),
                    label = "Today",
                    prxy = uiState.todayEarnings,
                    tokenPrice = uiState.tokenPrice,
                )
                EarningsCard(
                    modifier = Modifier.weight(1f),
                    label = "All Time",
                    prxy = uiState.allTimeEarnings,
                    tokenPrice = uiState.tokenPrice,
                )
            }

            // Bandwidth card.
            BandwidthCard(
                uploadBytes = uiState.uploadBytes,
                downloadBytes = uiState.downloadBytes,
                bandwidthSharedBytes = uiState.bandwidthSharedBytes,
            )

            // Live stats card.
            LiveStatsCard(
                uptimeSeconds = uiState.uptimeSeconds,
                requestsServed = uiState.requestsServed,
                avgLatencyMs = uiState.avgLatencyMs,
                trustScore = uiState.trustScore,
            )

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}

@Composable
private fun ConnectionCard(
    isConnected: Boolean,
    onToggle: () -> Unit,
) {
    val statusColor by animateColorAsState(
        targetValue = if (isConnected) StatusConnected else StatusDisconnected,
        animationSpec = tween(durationMillis = 400),
        label = "statusColor",
    )
    val buttonScale by animateFloatAsState(
        targetValue = if (isConnected) 1.08f else 1f,
        animationSpec = tween(durationMillis = 200),
        label = "buttonScale",
    )

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface,
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
        shape = RoundedCornerShape(20.dp),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // Glowing status indicator.
            Box(
                contentAlignment = Alignment.Center,
            ) {
                Box(
                    modifier = Modifier
                        .size(80.dp)
                        .clip(CircleShape)
                        .background(statusColor.copy(alpha = 0.15f)),
                )
                FilledIconToggleButton(
                    checked = isConnected,
                    onCheckedChange = { onToggle() },
                    modifier = Modifier
                        .size(64.dp)
                        .scale(buttonScale),
                    colors = IconToggleButtonColors(
                        containerColor = statusColor.copy(alpha = 0.2f),
                        contentColor = statusColor,
                        checkedContainerColor = statusColor.copy(alpha = 0.25f),
                        checkedContentColor = statusColor,
                        disabledContainerColor = Color.Gray.copy(alpha = 0.12f),
                        disabledContentColor = Color.Gray,
                    ),
                ) {
                    Icon(
                        imageVector = Icons.Default.PowerSettingsNew,
                        contentDescription = if (isConnected) "Disconnect" else "Connect",
                        modifier = Modifier.size(32.dp),
                    )
                }
            }

            Text(
                text = if (isConnected) "SHARING BANDWIDTH" else "DISCONNECTED",
                style = MaterialTheme.typography.labelLarge,
                fontWeight = FontWeight.Bold,
                color = statusColor,
                letterSpacing = androidx.compose.ui.unit.TextUnit.Unspecified,
            )

            Text(
                text = if (isConnected) {
                    "Your node is active and earning PRXY tokens"
                } else {
                    "Tap the button above to start sharing bandwidth"
                },
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                textAlign = TextAlign.Center,
            )
        }
    }
}

@Composable
private fun EarningsCard(
    modifier: Modifier = Modifier,
    label: String,
    prxy: Double,
    tokenPrice: Double?,
) {
    val usdValue = tokenPrice?.let { prxy * it }

    Card(
        modifier = modifier,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface,
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
        shape = RoundedCornerShape(16.dp),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(
                    Brush.verticalGradient(
                        colors = listOf(
                            ProxyCoinAccent.copy(alpha = 0.08f),
                            Color.Transparent,
                        ),
                    ),
                )
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            )
            Text(
                text = "%.4f".format(prxy),
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.Bold,
                color = ProxyCoinAccent,
            )
            Text(
                text = "PRXY",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
            )
            if (usdValue != null) {
                Text(
                    text = "≈ ${"$%.4f".format(usdValue)}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                )
            }
        }
    }
}

@Composable
private fun BandwidthCard(
    uploadBytes: Long,
    downloadBytes: Long,
    bandwidthSharedBytes: Long,
) {
    val totalMb = bandwidthSharedBytes / (1024.0 * 1024.0)
    val uploadMb = uploadBytes / (1024.0 * 1024.0)
    val downloadMb = downloadBytes / (1024.0 * 1024.0)

    // Progress as fraction of 1 GB cap for display purposes.
    val progressFraction = (totalMb / 1024.0).coerceIn(0.0, 1.0).toFloat()

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface,
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
        shape = RoundedCornerShape(16.dp),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                text = "Bandwidth Shared",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = formatBytes(bandwidthSharedBytes),
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.primary,
                )
                Text(
                    text = "/ 1 GB",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                )
            }

            LinearProgressIndicator(
                progress = { progressFraction },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(8.dp)
                    .clip(RoundedCornerShape(4.dp)),
                color = MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surfaceVariant,
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Column {
                    Text(
                        text = "Upload",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                    )
                    Text(
                        text = formatBytes(uploadBytes),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                    )
                }
                Column(horizontalAlignment = Alignment.End) {
                    Text(
                        text = "Download",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                    )
                    Text(
                        text = formatBytes(downloadBytes),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                    )
                }
            }
        }
    }
}

@Composable
private fun LiveStatsCard(
    uptimeSeconds: Long,
    requestsServed: Int,
    avgLatencyMs: Int,
    trustScore: Float,
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface,
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
        shape = RoundedCornerShape(16.dp),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                text = "Live Stats",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                StatItem(
                    modifier = Modifier.weight(1f),
                    icon = Icons.Default.Timer,
                    label = "Uptime",
                    value = formatUptime(uptimeSeconds),
                )
                StatItem(
                    modifier = Modifier.weight(1f),
                    icon = Icons.Default.NetworkCheck,
                    label = "Requests",
                    value = requestsServed.toString(),
                )
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                StatItem(
                    modifier = Modifier.weight(1f),
                    icon = Icons.Default.Speed,
                    label = "Avg Latency",
                    value = "${avgLatencyMs}ms",
                )
                StatItem(
                    modifier = Modifier.weight(1f),
                    icon = Icons.Default.Verified,
                    label = "Trust Score",
                    value = "%.1f%%".format(trustScore * 100f),
                )
            }
        }
    }
}

@Composable
private fun StatItem(
    modifier: Modifier = Modifier,
    icon: ImageVector,
    label: String,
    value: String,
) {
    Card(
        modifier = modifier,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
        ),
        shape = RoundedCornerShape(12.dp),
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(20.dp),
            )
            Column {
                Text(
                    text = label,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
                Text(
                    text = value,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        }
    }
}

// ── Formatting helpers ────────────────────────────────────────────────────────

private fun formatBytes(bytes: Long): String {
    return when {
        bytes < 1024L -> "${bytes} B"
        bytes < 1024L * 1024L -> "%.1f KB".format(bytes / 1024.0)
        bytes < 1024L * 1024L * 1024L -> "%.2f MB".format(bytes / (1024.0 * 1024.0))
        else -> "%.2f GB".format(bytes / (1024.0 * 1024.0 * 1024.0))
    }
}

private fun formatUptime(seconds: Long): String {
    val h = seconds / 3600
    val m = (seconds % 3600) / 60
    val s = seconds % 60
    return if (h > 0) "%d:%02d:%02d".format(h, m, s) else "%d:%02d".format(m, s)
}
