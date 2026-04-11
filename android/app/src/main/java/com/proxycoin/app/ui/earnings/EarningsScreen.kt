package com.proxycoin.app.ui.earnings

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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountBalanceWallet
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.proxycoin.app.data.local.entity.EarningsEntity
import com.proxycoin.app.ui.theme.ProxyCoinAccent
import com.proxycoin.app.util.Constants

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EarningsScreen(
    viewModel: EarningsViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }

    LaunchedEffect(uiState.errorMessage) {
        uiState.errorMessage?.let { snackbarHostState.showSnackbar(it) }
    }

    LaunchedEffect(uiState.claimSuccess) {
        if (uiState.claimSuccess) {
            snackbarHostState.showSnackbar("Claim submitted successfully!")
            viewModel.clearClaimSuccess()
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "Earnings",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold,
                    )
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding),
        ) {
            // Period tab bar.
            val periods = EarningsPeriod.entries
            val selectedIndex = periods.indexOf(uiState.selectedPeriod)

            TabRow(selectedTabIndex = selectedIndex) {
                periods.forEachIndexed { index, period ->
                    Tab(
                        selected = index == selectedIndex,
                        onClick = { viewModel.selectPeriod(period) },
                        text = {
                            Text(
                                text = when (period) {
                                    EarningsPeriod.DAY -> "Day"
                                    EarningsPeriod.WEEK -> "Week"
                                    EarningsPeriod.MONTH -> "Month"
                                    EarningsPeriod.ALL -> "All"
                                },
                            )
                        },
                    )
                }
            }

            if (uiState.isLoading) {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator()
                }
                return@Scaffold
            }

            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                item { Spacer(modifier = Modifier.height(12.dp)) }

                // Pending / Claimed summary cards.
                item {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        EarningsSummaryCard(
                            modifier = Modifier.weight(1f),
                            label = "Pending",
                            amount = uiState.pendingAmount,
                            highlight = true,
                        )
                        EarningsSummaryCard(
                            modifier = Modifier.weight(1f),
                            label = "Total Earned",
                            amount = uiState.totalEarnings,
                            highlight = false,
                        )
                    }
                }

                // Claim button — only shown when pending >= minimum.
                if (uiState.pendingAmount >= Constants.MIN_CLAIM_AMOUNT) {
                    item {
                        Button(
                            onClick = { viewModel.claimEarnings() },
                            modifier = Modifier.fillMaxWidth(),
                            enabled = !uiState.isClaimInProgress,
                            shape = RoundedCornerShape(12.dp),
                            colors = ButtonDefaults.buttonColors(
                                containerColor = ProxyCoinAccent,
                            ),
                        ) {
                            if (uiState.isClaimInProgress) {
                                CircularProgressIndicator(
                                    modifier = Modifier.size(20.dp),
                                    strokeWidth = 2.dp,
                                    color = MaterialTheme.colorScheme.onPrimary,
                                )
                                Spacer(modifier = Modifier.width(8.dp))
                            } else {
                                Icon(
                                    imageVector = Icons.Default.AccountBalanceWallet,
                                    contentDescription = null,
                                    modifier = Modifier.size(20.dp),
                                )
                                Spacer(modifier = Modifier.width(8.dp))
                            }
                            Text(
                                text = if (uiState.isClaimInProgress) "Claiming..." else "Claim ${formatPrxy(uiState.pendingAmount)} PRXY",
                                fontWeight = FontWeight.SemiBold,
                            )
                        }
                    }
                }

                // Section header.
                item {
                    Text(
                        text = "Earnings Breakdown",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                }

                if (uiState.earnings.isEmpty()) {
                    item {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(vertical = 32.dp),
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(
                                text = "No earnings data for this period",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                                textAlign = TextAlign.Center,
                            )
                        }
                    }
                } else {
                    items(uiState.earnings) { earning ->
                        EarningsRowItem(earning = earning)
                    }
                }

                item { Spacer(modifier = Modifier.height(16.dp)) }
            }
        }
    }
}

@Composable
private fun EarningsSummaryCard(
    modifier: Modifier = Modifier,
    label: String,
    amount: Long,
    highlight: Boolean,
) {
    Card(
        modifier = modifier,
        colors = CardDefaults.cardColors(
            containerColor = if (highlight) {
                ProxyCoinAccent.copy(alpha = 0.12f)
            } else {
                MaterialTheme.colorScheme.surface
            },
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
        shape = RoundedCornerShape(16.dp),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            )
            Text(
                text = formatPrxy(amount),
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.Bold,
                color = if (highlight) ProxyCoinAccent else MaterialTheme.colorScheme.onSurface,
            )
            Text(
                text = "PRXY",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
            )
        }
    }
}

@Composable
private fun EarningsRowItem(earning: EarningsEntity) {
    val total = earning.bandwidthEarnings + earning.uptimeBonus + earning.qualityBonus + earning.referralEarnings

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface,
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
        shape = RoundedCornerShape(12.dp),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = earning.date,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    text = "${formatPrxy(total)} PRXY",
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Bold,
                    color = ProxyCoinAccent,
                )
            }

            HorizontalDivider(
                color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f),
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                EarningsBreakdownCell(label = "Bandwidth", amount = earning.bandwidthEarnings)
                EarningsBreakdownCell(label = "Uptime", amount = earning.uptimeBonus)
                EarningsBreakdownCell(label = "Quality", amount = earning.qualityBonus)
                EarningsBreakdownCell(label = "Referral", amount = earning.referralEarnings)
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text(
                    text = "${earning.requestsServed} requests",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                )
                Text(
                    text = formatBytes(earning.totalBytes),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                )
            }
        }
    }
}

@Composable
private fun EarningsBreakdownCell(
    label: String,
    amount: Long,
) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
        )
        Text(
            text = formatPrxy(amount),
            style = MaterialTheme.typography.bodySmall,
            fontWeight = FontWeight.Medium,
        )
    }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

private fun formatPrxy(units: Long): String {
    val prxy = units / 1_000_000.0
    return "%.4f".format(prxy)
}

private fun formatBytes(bytes: Long): String {
    return when {
        bytes < 1024L -> "${bytes} B"
        bytes < 1024L * 1024L -> "%.1f KB".format(bytes / 1024.0)
        bytes < 1024L * 1024L * 1024L -> "%.2f MB".format(bytes / (1024.0 * 1024.0))
        else -> "%.2f GB".format(bytes / (1024.0 * 1024.0 * 1024.0))
    }
}
