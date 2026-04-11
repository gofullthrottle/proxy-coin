package com.proxycoin.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.proxycoin.app.ui.navigation.ProxyCoinNavHost
import com.proxycoin.app.ui.theme.ProxyCoinTheme
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            ProxyCoinTheme {
                ProxyCoinNavHost()
            }
        }
    }
}
