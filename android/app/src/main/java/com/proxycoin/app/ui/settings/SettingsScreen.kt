package com.proxycoin.app.ui.settings

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.OpenInNew
import androidx.compose.material.icons.filled.BatteryChargingFull
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Wifi
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    viewModel: SettingsViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    val clipboardManager = LocalClipboardManager.current

    fun copyToClipboard(label: String, value: String) {
        clipboardManager.setText(AnnotatedString(value))
        scope.launch { snackbarHostState.showSnackbar("$label copied to clipboard") }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "Settings",
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
                .padding(innerPadding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // ── Bandwidth section ─────────────────────────────────────────────
            SettingsSectionCard(
                icon = Icons.Default.Wifi,
                title = "Bandwidth",
            ) {
                SliderSetting(
                    label = "Max Bandwidth Usage",
                    value = uiState.maxBandwidthPercent,
                    range = 10f..100f,
                    steps = 17,
                    displayValue = "${uiState.maxBandwidthPercent}%",
                    onValueChange = { viewModel.setMaxBandwidthPercent(it) },
                )

                HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

                SwitchSetting(
                    label = "Wi-Fi Only",
                    description = "Only share bandwidth on Wi-Fi connections",
                    checked = uiState.wifiOnly,
                    onCheckedChange = { viewModel.setWifiOnly(it) },
                )

                HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

                SwitchSetting(
                    label = "Allow Cellular",
                    description = "Share bandwidth over mobile data (may incur charges)",
                    checked = uiState.allowCellular,
                    onCheckedChange = { viewModel.setAllowCellular(it) },
                )
            }

            // ── Power section ─────────────────────────────────────────────────
            SettingsSectionCard(
                icon = Icons.Default.BatteryChargingFull,
                title = "Power",
            ) {
                SliderSetting(
                    label = "Battery Threshold",
                    value = uiState.batteryThreshold,
                    range = 5f..50f,
                    steps = 8,
                    displayValue = "${uiState.batteryThreshold}%",
                    onValueChange = { viewModel.setBatteryThreshold(it) },
                )

                HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

                SwitchSetting(
                    label = "Charging Only",
                    description = "Only share bandwidth when plugged in",
                    checked = uiState.chargingOnly,
                    onCheckedChange = { viewModel.setChargingOnly(it) },
                )
            }

            // ── Notifications section ─────────────────────────────────────────
            SettingsSectionCard(
                icon = Icons.Default.Notifications,
                title = "Notifications",
            ) {
                SwitchSetting(
                    label = "Earnings Updates",
                    description = "Get notified when PRXY is credited to your account",
                    checked = uiState.notifyEarnings,
                    onCheckedChange = { viewModel.setNotifyEarnings(it) },
                )

                HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

                SwitchSetting(
                    label = "Connection Status",
                    description = "Notifications when your node connects or disconnects",
                    checked = uiState.notifyConnection,
                    onCheckedChange = { viewModel.setNotifyConnection(it) },
                )

                HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

                SwitchSetting(
                    label = "Weekly Summary",
                    description = "Get a weekly earnings summary every Sunday",
                    checked = uiState.notifyWeeklySummary,
                    onCheckedChange = { viewModel.setNotifyWeeklySummary(it) },
                )
            }

            // ── Account section ───────────────────────────────────────────────
            SettingsSectionCard(
                icon = Icons.Default.Person,
                title = "Account",
            ) {
                if (uiState.deviceId.isNotBlank()) {
                    CopyableInfoRow(
                        label = "Device ID",
                        value = uiState.deviceId,
                        onCopy = { copyToClipboard("Device ID", uiState.deviceId) },
                    )
                    HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))
                }
                if (uiState.nodeId.isNotBlank()) {
                    CopyableInfoRow(
                        label = "Node ID",
                        value = uiState.nodeId,
                        onCopy = { copyToClipboard("Node ID", uiState.nodeId) },
                    )
                    HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))
                }
                if (uiState.walletAddress.isNotBlank()) {
                    CopyableInfoRow(
                        label = "Wallet",
                        value = truncateAddress(uiState.walletAddress),
                        onCopy = { copyToClipboard("Wallet Address", uiState.walletAddress) },
                    )
                    HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))
                }
                InfoRow(
                    label = "Trust Score",
                    value = "%.1f%%".format(uiState.trustScore * 100f),
                )
            }

            // ── About section ─────────────────────────────────────────────────
            SettingsSectionCard(
                icon = Icons.Default.Info,
                title = "About",
            ) {
                InfoRow(label = "Version", value = uiState.appVersion)

                HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

                LinkRow(
                    label = "Privacy Policy",
                    onClick = {
                        val intent = Intent(Intent.ACTION_VIEW, Uri.parse("https://proxycoin.io/privacy"))
                        context.startActivity(intent)
                    },
                )

                HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

                LinkRow(
                    label = "Terms of Service",
                    onClick = {
                        val intent = Intent(Intent.ACTION_VIEW, Uri.parse("https://proxycoin.io/terms"))
                        context.startActivity(intent)
                    },
                )

                HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

                LinkRow(
                    label = "Support",
                    onClick = {
                        val intent = Intent(Intent.ACTION_VIEW, Uri.parse("https://proxycoin.io/support"))
                        context.startActivity(intent)
                    },
                )
            }

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}

@Composable
private fun SettingsSectionCard(
    icon: ImageVector,
    title: String,
    content: @Composable () -> Unit,
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
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.padding(bottom = 8.dp),
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary,
                )
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            content()
        }
    }
}

@Composable
private fun SliderSetting(
    label: String,
    value: Int,
    range: ClosedFloatingPointRange<Float>,
    steps: Int,
    displayValue: String,
    onValueChange: (Int) -> Unit,
) {
    Column(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = label,
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
            )
            Text(
                text = displayValue,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.primary,
                fontWeight = FontWeight.Bold,
            )
        }
        Slider(
            value = value.toFloat(),
            onValueChange = { onValueChange(it.toInt()) },
            valueRange = range,
            steps = steps,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
private fun SwitchSetting(
    label: String,
    description: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f).padding(end = 16.dp)) {
            Text(
                text = label,
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
            )
            Text(
                text = description,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            )
        }
        Switch(
            checked = checked,
            onCheckedChange = onCheckedChange,
        )
    }
}

@Composable
private fun InfoRow(
    label: String,
    value: String,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f),
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Medium,
        )
    }
}

@Composable
private fun CopyableInfoRow(
    label: String,
    value: String,
    onCopy: () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = label,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            )
            Text(
                text = value,
                style = MaterialTheme.typography.bodyMedium,
                fontFamily = FontFamily.Monospace,
                fontWeight = FontWeight.Medium,
            )
        }
        IconButton(onClick = onCopy) {
            Icon(
                imageVector = Icons.Default.ContentCopy,
                contentDescription = "Copy $label",
                tint = MaterialTheme.colorScheme.primary,
            )
        }
    }
}

@Composable
private fun LinkRow(
    label: String,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Medium,
        )
        Icon(
            imageVector = Icons.AutoMirrored.Filled.OpenInNew,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.primary,
        )
    }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

private fun truncateAddress(address: String): String {
    if (address.length <= 12) return address
    return "${address.take(6)}...${address.takeLast(4)}"
}
