package com.proxycoin.app.ui.onboarding

import android.content.Context
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.ImportExport
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedCard
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.proxycoin.app.crypto.WalletManager
import com.proxycoin.app.ui.settings.SettingsKeys
import com.proxycoin.app.ui.theme.ProxyCoinAccent
import com.proxycoin.app.ui.theme.ProxyCoinPrimary
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

// ── ViewModel ─────────────────────────────────────────────────────────────────

private val Context.walletSetupDataStore: DataStore<Preferences> by preferencesDataStore(name = "wallet_setup")
private val WALLET_CONFIGURED_KEY = stringPreferencesKey("wallet_address")

enum class WalletSetupMode { CREATE, IMPORT }
enum class SetupStep { MODE_SELECT, CREATE_MNEMONIC, CONFIRM_BACKUP, IMPORT_KEY, DONE }

@HiltViewModel
class WalletSetupViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
    private val walletManager: WalletManager,
) : ViewModel() {

    data class UiState(
        val mode: WalletSetupMode = WalletSetupMode.CREATE,
        val step: SetupStep = SetupStep.MODE_SELECT,
        val generatedMnemonic: String = "",
        val mnemonicWords: List<String> = emptyList(),
        val confirmationWords: List<String> = emptyList(),
        val selectedConfirmIndices: List<Int> = emptyList(),
        val importInput: String = "",
        val walletAddress: String = "",
        val isLoading: Boolean = false,
        val error: String? = null,
        val setupComplete: Boolean = false,
    )

    private val _uiState = MutableStateFlow(UiState())
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    private val dataStore = context.walletSetupDataStore

    // ── Mode selection ────────────────────────────────────────────────────────

    fun selectMode(mode: WalletSetupMode) {
        _uiState.update { it.copy(mode = mode) }
        when (mode) {
            WalletSetupMode.CREATE -> generateMnemonic()
            WalletSetupMode.IMPORT -> _uiState.update { it.copy(step = SetupStep.IMPORT_KEY) }
        }
    }

    // ── Create flow ───────────────────────────────────────────────────────────

    private fun generateMnemonic() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            val mnemonic = walletManager.generateMnemonic()
            val words = mnemonic.trim().split(" ")
            // Pick 3 random word indices for confirmation challenge.
            val challengeIndices = words.indices.shuffled().take(3).sorted()
            val confirmationWords = words.shuffled()
            _uiState.update {
                it.copy(
                    isLoading = false,
                    generatedMnemonic = mnemonic,
                    mnemonicWords = words,
                    confirmationWords = confirmationWords,
                    selectedConfirmIndices = challengeIndices,
                    step = SetupStep.CREATE_MNEMONIC,
                )
            }
        }
    }

    fun proceedToConfirm() {
        _uiState.update { it.copy(step = SetupStep.CONFIRM_BACKUP) }
    }

    fun confirmAndCreateWallet() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val address = walletManager.importFromMnemonic(_uiState.value.generatedMnemonic)
                persistWalletAddress(address)
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        walletAddress = address,
                        step = SetupStep.DONE,
                        setupComplete = true,
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        error = "Wallet creation failed: ${e.message}",
                    )
                }
            }
        }
    }

    // ── Import flow ───────────────────────────────────────────────────────────

    fun setImportInput(value: String) {
        _uiState.update { it.copy(importInput = value, error = null) }
    }

    fun importWallet() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val input = _uiState.value.importInput.trim()
                val address = if (input.startsWith("0x") || (!input.contains(" ") && input.length == 64)) {
                    walletManager.importFromPrivateKey(input)
                } else {
                    walletManager.importFromMnemonic(input)
                }
                persistWalletAddress(address)
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        walletAddress = address,
                        step = SetupStep.DONE,
                        setupComplete = true,
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        error = "Import failed: ${e.message}",
                    )
                }
            }
        }
    }

    // ── Persistence ───────────────────────────────────────────────────────────

    private suspend fun persistWalletAddress(address: String) {
        dataStore.edit { prefs -> prefs[WALLET_CONFIGURED_KEY] = address }
        // Also write to settings DataStore so SettingsScreen shows it.
        context.getSharedPreferences("app_settings", Context.MODE_PRIVATE)
            .edit().putString("wallet_address", address).apply()
    }

    fun clearError() {
        _uiState.update { it.copy(error = null) }
    }
}

// ── Screen ────────────────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun WalletSetupScreen(
    onSetupComplete: () -> Unit,
    viewModel: WalletSetupViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }
    val scope = rememberCoroutineScope()

    LaunchedEffect(uiState.error) {
        uiState.error?.let {
            snackbarHostState.showSnackbar(it)
            viewModel.clearError()
        }
    }

    LaunchedEffect(uiState.setupComplete) {
        if (uiState.setupComplete) {
            onSetupComplete()
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "Set Up Wallet",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold,
                    )
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
    ) { innerPadding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding),
        ) {
            when (uiState.step) {
                SetupStep.MODE_SELECT -> ModeSelectContent(onSelectMode = viewModel::selectMode)
                SetupStep.CREATE_MNEMONIC -> MnemonicDisplayContent(
                    words = uiState.mnemonicWords,
                    isLoading = uiState.isLoading,
                    onContinue = viewModel::proceedToConfirm,
                )
                SetupStep.CONFIRM_BACKUP -> ConfirmBackupContent(
                    words = uiState.mnemonicWords,
                    confirmationWords = uiState.confirmationWords,
                    challengeIndices = uiState.selectedConfirmIndices,
                    isLoading = uiState.isLoading,
                    onConfirm = viewModel::confirmAndCreateWallet,
                )
                SetupStep.IMPORT_KEY -> ImportKeyContent(
                    input = uiState.importInput,
                    isLoading = uiState.isLoading,
                    onInputChange = viewModel::setImportInput,
                    onImport = viewModel::importWallet,
                )
                SetupStep.DONE -> DoneContent(address = uiState.walletAddress)
            }
        }
    }
}

// ── Mode select ───────────────────────────────────────────────────────────────

@Composable
private fun ModeSelectContent(onSelectMode: (WalletSetupMode) -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Icon(
            imageVector = Icons.Default.Lock,
            contentDescription = null,
            modifier = Modifier.size(64.dp),
            tint = ProxyCoinPrimary,
        )

        Spacer(modifier = Modifier.height(24.dp))

        Text(
            text = "Choose Wallet Setup",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            textAlign = TextAlign.Center,
        )
        Spacer(modifier = Modifier.height(8.dp))
        Text(
            text = "Create a new wallet or import an existing one using your seed phrase or private key.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            textAlign = TextAlign.Center,
        )

        Spacer(modifier = Modifier.height(40.dp))

        Button(
            onClick = { onSelectMode(WalletSetupMode.CREATE) },
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(containerColor = ProxyCoinPrimary),
        ) {
            Icon(Icons.Default.Lock, contentDescription = null, modifier = Modifier.size(18.dp))
            Spacer(modifier = Modifier.size(8.dp))
            Text("Create New Wallet", fontWeight = FontWeight.SemiBold)
        }

        Spacer(modifier = Modifier.height(12.dp))

        Button(
            onClick = { onSelectMode(WalletSetupMode.IMPORT) },
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.secondaryContainer,
                contentColor = MaterialTheme.colorScheme.onSecondaryContainer,
            ),
        ) {
            Icon(Icons.Default.ImportExport, contentDescription = null, modifier = Modifier.size(18.dp))
            Spacer(modifier = Modifier.size(8.dp))
            Text("Import Existing Wallet", fontWeight = FontWeight.SemiBold)
        }
    }
}

// ── Mnemonic display ──────────────────────────────────────────────────────────

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun MnemonicDisplayContent(
    words: List<String>,
    isLoading: Boolean,
    onContinue: () -> Unit,
) {
    var confirmedBackup by remember { mutableStateOf(false) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(
            text = "Your Recovery Phrase",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
        )

        Card(
            colors = CardDefaults.cardColors(
                containerColor = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.5f),
            ),
            shape = RoundedCornerShape(12.dp),
        ) {
            Row(
                modifier = Modifier.padding(12.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Icon(
                    imageVector = Icons.Default.Warning,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.error,
                    modifier = Modifier.size(20.dp),
                )
                Text(
                    text = "Write these 12 words down and store them somewhere safe. Anyone with these words can access your wallet. Never share them.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onErrorContainer,
                )
            }
        }

        if (isLoading) {
            Box(modifier = Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator()
            }
        } else {
            // Word grid.
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(16.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant)
                    .padding(16.dp),
            ) {
                FlowRow(
                    modifier = Modifier.fillMaxWidth(),
                    maxItemsInEachRow = 3,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    words.forEachIndexed { index, word ->
                        WordChip(index = index + 1, word = word)
                    }
                }
            }
        }

        // Backup confirmation checkbox replacement.
        OutlinedCard(
            onClick = { confirmedBackup = !confirmedBackup },
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            border = androidx.compose.foundation.BorderStroke(
                width = 1.5.dp,
                color = if (confirmedBackup) ProxyCoinAccent else MaterialTheme.colorScheme.outline,
            ),
        ) {
            Row(
                modifier = Modifier.padding(12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Icon(
                    imageVector = if (confirmedBackup) Icons.Default.CheckCircle else Icons.Default.Warning,
                    contentDescription = null,
                    tint = if (confirmedBackup) ProxyCoinAccent else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f),
                    modifier = Modifier.size(24.dp),
                )
                Text(
                    text = "I have written down my recovery phrase and stored it safely",
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
        }

        Button(
            onClick = onContinue,
            modifier = Modifier.fillMaxWidth(),
            enabled = confirmedBackup && !isLoading,
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(containerColor = ProxyCoinPrimary),
        ) {
            Text("Continue to Verify", fontWeight = FontWeight.SemiBold)
        }
    }
}

// ── Confirm backup ────────────────────────────────────────────────────────────

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ConfirmBackupContent(
    words: List<String>,
    confirmationWords: List<String>,
    challengeIndices: List<Int>,
    isLoading: Boolean,
    onConfirm: () -> Unit,
) {
    var selectedWords by remember { mutableStateOf(listOf<String>()) }
    val challengeAnswers = challengeIndices.map { words[it] }
    val isCorrect = selectedWords == challengeAnswers

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(
            text = "Verify Your Phrase",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
        )
        Text(
            text = "Select the words in the correct order for positions ${challengeIndices.map { it + 1 }.joinToString(", ")}.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f),
        )

        // Selected words display.
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(12.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant)
                .padding(12.dp),
            contentAlignment = Alignment.Center,
        ) {
            if (selectedWords.isEmpty()) {
                Text(
                    text = "Tap words below to confirm",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f),
                )
            } else {
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    selectedWords.forEachIndexed { idx, word ->
                        FilterChip(
                            selected = true,
                            onClick = {
                                selectedWords = selectedWords.toMutableList().also { it.removeAt(idx) }
                            },
                            label = {
                                Text(
                                    text = "${challengeIndices.getOrNull(idx)?.plus(1) ?: "?"}. $word",
                                    style = MaterialTheme.typography.labelMedium,
                                )
                            },
                        )
                    }
                }
            }
        }

        // Shuffled word options.
        FlowRow(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            confirmationWords.forEach { word ->
                val alreadySelected = selectedWords.contains(word)
                FilterChip(
                    selected = alreadySelected,
                    onClick = {
                        if (!alreadySelected && selectedWords.size < challengeIndices.size) {
                            selectedWords = selectedWords + word
                        }
                    },
                    enabled = !alreadySelected || selectedWords.size < challengeIndices.size,
                    label = {
                        Text(
                            text = word,
                            style = MaterialTheme.typography.labelMedium,
                        )
                    },
                )
            }
        }

        if (selectedWords.size == challengeIndices.size) {
            Text(
                text = if (isCorrect) "Correct! Your backup is verified." else "Incorrect order. Try again.",
                style = MaterialTheme.typography.bodySmall,
                color = if (isCorrect) ProxyCoinAccent else MaterialTheme.colorScheme.error,
                fontWeight = FontWeight.SemiBold,
            )
        }

        Button(
            onClick = onConfirm,
            modifier = Modifier.fillMaxWidth(),
            enabled = isCorrect && !isLoading,
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(containerColor = ProxyCoinPrimary),
        ) {
            if (isLoading) {
                CircularProgressIndicator(
                    modifier = Modifier.size(18.dp),
                    strokeWidth = 2.dp,
                    color = MaterialTheme.colorScheme.onPrimary,
                )
                Spacer(modifier = Modifier.size(8.dp))
            }
            Text("Create Wallet", fontWeight = FontWeight.SemiBold)
        }
    }
}

// ── Import key ────────────────────────────────────────────────────────────────

@Composable
private fun ImportKeyContent(
    input: String,
    isLoading: Boolean,
    onInputChange: (String) -> Unit,
    onImport: () -> Unit,
) {
    var tabIndex by remember { mutableIntStateOf(0) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(
            text = "Import Wallet",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
        )

        TabRow(selectedTabIndex = tabIndex) {
            Tab(
                selected = tabIndex == 0,
                onClick = { tabIndex = 0 },
                text = { Text("Seed Phrase") },
            )
            Tab(
                selected = tabIndex == 1,
                onClick = { tabIndex = 1 },
                text = { Text("Private Key") },
            )
        }

        Spacer(modifier = Modifier.height(4.dp))

        when (tabIndex) {
            0 -> {
                Text(
                    text = "Enter your 12 or 24-word BIP-39 seed phrase, separated by spaces.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
                OutlinedTextField(
                    value = input,
                    onValueChange = onInputChange,
                    label = { Text("Seed Phrase") },
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(120.dp),
                    maxLines = 5,
                    placeholder = { Text("word1 word2 word3 ...") },
                )
            }
            1 -> {
                Text(
                    text = "Enter your 64-character hex private key (with or without 0x prefix).",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
                OutlinedTextField(
                    value = input,
                    onValueChange = onInputChange,
                    label = { Text("Private Key") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    placeholder = { Text("0x...") },
                )
            }
        }

        Card(
            colors = CardDefaults.cardColors(
                containerColor = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.4f),
            ),
            shape = RoundedCornerShape(12.dp),
        ) {
            Row(
                modifier = Modifier.padding(12.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Icon(
                    imageVector = Icons.Default.Warning,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.error,
                    modifier = Modifier.size(18.dp),
                )
                Text(
                    text = "Never share your seed phrase or private key. ProxyCoin staff will never ask for it.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onErrorContainer,
                )
            }
        }

        Button(
            onClick = onImport,
            modifier = Modifier.fillMaxWidth(),
            enabled = input.isNotBlank() && !isLoading,
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(containerColor = ProxyCoinPrimary),
        ) {
            if (isLoading) {
                CircularProgressIndicator(
                    modifier = Modifier.size(18.dp),
                    strokeWidth = 2.dp,
                    color = MaterialTheme.colorScheme.onPrimary,
                )
                Spacer(modifier = Modifier.size(8.dp))
            }
            Text("Import Wallet", fontWeight = FontWeight.SemiBold)
        }
    }
}

// ── Done ──────────────────────────────────────────────────────────────────────

@Composable
private fun DoneContent(address: String) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Icon(
            imageVector = Icons.Default.CheckCircle,
            contentDescription = null,
            tint = ProxyCoinAccent,
            modifier = Modifier.size(72.dp),
        )
        Spacer(modifier = Modifier.height(16.dp))
        Text(
            text = "Wallet Ready!",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
        )
        Spacer(modifier = Modifier.height(8.dp))
        Text(
            text = "Your wallet has been configured successfully.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            textAlign = TextAlign.Center,
        )
        Spacer(modifier = Modifier.height(16.dp))
        Text(
            text = address,
            style = MaterialTheme.typography.bodySmall,
            fontFamily = FontFamily.Monospace,
            textAlign = TextAlign.Center,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f),
        )
    }
}

// ── Word chip ─────────────────────────────────────────────────────────────────

@Composable
private fun WordChip(index: Int, word: String) {
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(8.dp))
            .background(MaterialTheme.colorScheme.surface)
            .border(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.4f), RoundedCornerShape(8.dp))
            .padding(horizontal = 12.dp, vertical = 8.dp),
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(
                text = "$index",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f),
            )
            Text(
                text = word,
                style = MaterialTheme.typography.bodyMedium,
                fontFamily = FontFamily.Monospace,
                fontWeight = FontWeight.Medium,
            )
        }
    }
}
