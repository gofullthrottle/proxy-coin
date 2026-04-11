package com.proxycoin.app.ui.navigation

import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.AccountBalanceWallet
import androidx.compose.material.icons.outlined.TrendingUp
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.proxycoin.app.ui.dashboard.DashboardScreen
import com.proxycoin.app.ui.earnings.EarningsScreen
import com.proxycoin.app.ui.onboarding.PermissionsScreen
import com.proxycoin.app.ui.onboarding.WalletSetupScreen
import com.proxycoin.app.ui.onboarding.WelcomeScreen
import com.proxycoin.app.ui.referral.ReferralScreen
import com.proxycoin.app.ui.settings.SettingsScreen
import com.proxycoin.app.ui.wallet.WalletScreen

// ── Route definitions ─────────────────────────────────────────────────────────

sealed class Screen(val route: String, val title: String, val icon: ImageVector) {
    data object Dashboard : Screen("dashboard", "Home", Icons.Default.Home)
    data object Earnings : Screen("earnings", "Earn", Icons.Outlined.TrendingUp)
    data object Wallet : Screen("wallet", "Wallet", Icons.Outlined.AccountBalanceWallet)
    data object Settings : Screen("settings", "Settings", Icons.Default.Settings)
}

// Non-bottom-nav screens.
sealed class ScreenRoute(val route: String) {
    data object Welcome : ScreenRoute("welcome")
    data object Permissions : ScreenRoute("permissions")
    data object WalletSetup : ScreenRoute("wallet_setup")
    data object Referral : ScreenRoute("referral")
}

val bottomNavScreens = listOf(
    Screen.Dashboard,
    Screen.Earnings,
    Screen.Wallet,
    Screen.Settings,
)

// ── Start destination constants ───────────────────────────────────────────────

const val START_ONBOARDING = "welcome"
const val START_MAIN = "dashboard"

// ── NavHost ───────────────────────────────────────────────────────────────────

/**
 * Root navigation host for ProxyCoin.
 *
 * [startDestination] selects between the onboarding flow and the main bottom-nav shell:
 *   - Pass [START_ONBOARDING] on first launch (no wallet configured).
 *   - Pass [START_MAIN] once onboarding is complete.
 *
 * Onboarding flow:
 *   Welcome → Permissions → WalletSetup → Dashboard
 *
 * Main tabs (bottom nav):
 *   Dashboard | Earnings | Wallet | Settings
 *
 * Overlay screens (no bottom nav):
 *   Referral
 */
@Composable
fun ProxyCoinNavHost(startDestination: String = START_MAIN) {
    val navController = rememberNavController()

    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = navBackStackEntry?.destination?.route

    // Only show the bottom bar on main tab screens.
    val showBottomBar = bottomNavScreens.any { it.route == currentRoute }

    Scaffold(
        bottomBar = {
            if (showBottomBar) {
                NavigationBar {
                    val currentDestination = navBackStackEntry?.destination

                    bottomNavScreens.forEach { screen ->
                        NavigationBarItem(
                            icon = { Icon(screen.icon, contentDescription = screen.title) },
                            label = { Text(screen.title) },
                            selected = currentDestination?.hierarchy?.any { it.route == screen.route } == true,
                            onClick = {
                                navController.navigate(screen.route) {
                                    popUpTo(navController.graph.findStartDestination().id) {
                                        saveState = true
                                    }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            },
                        )
                    }
                }
            }
        },
    ) { innerPadding ->
        NavHost(
            navController = navController,
            startDestination = startDestination,
            modifier = Modifier.padding(innerPadding),
        ) {

            // ── Onboarding flow ───────────────────────────────────────────────

            composable(ScreenRoute.Welcome.route) {
                WelcomeScreen(
                    onGetStarted = {
                        navController.navigate(ScreenRoute.Permissions.route) {
                            launchSingleTop = true
                        }
                    },
                )
            }

            composable(ScreenRoute.Permissions.route) {
                PermissionsScreen(
                    onContinue = {
                        navController.navigate(ScreenRoute.WalletSetup.route) {
                            launchSingleTop = true
                        }
                    },
                )
            }

            composable(ScreenRoute.WalletSetup.route) {
                WalletSetupScreen(
                    onSetupComplete = {
                        // Navigate to Dashboard and clear the entire onboarding back stack.
                        navController.navigate(Screen.Dashboard.route) {
                            popUpTo(ScreenRoute.Welcome.route) { inclusive = true }
                            launchSingleTop = true
                        }
                    },
                )
            }

            // ── Main bottom-nav tabs ──────────────────────────────────────────

            composable(Screen.Dashboard.route) {
                DashboardScreen()
            }

            composable(Screen.Earnings.route) {
                EarningsScreen()
            }

            composable(Screen.Wallet.route) {
                WalletScreen()
            }

            composable(Screen.Settings.route) {
                SettingsScreen()
            }

            // ── Overlay / detail screens ──────────────────────────────────────

            composable(ScreenRoute.Referral.route) {
                ReferralScreen()
            }
        }
    }
}
