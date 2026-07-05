package selvakn.ipv6diag.ui.settings

import android.content.Context
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExposedDropdownMenuBox
import androidx.compose.material3.ExposedDropdownMenuDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import selvakn.ipv6diag.IPv6DiagApplication
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import okhttp3.Request
import java.util.concurrent.TimeUnit
import java.util.regex.Pattern

private val turnTransportOptions = listOf(
    "udp"  to "UDP / plain (port 3478)",
    "tcp"  to "TCP / plain (port 3478)",
    "tls"  to "TLS/TCP — TURNS encrypted (port 5349)",
    "dtls" to "DTLS/UDP — TURNS encrypted (port 5349)",
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(navController: NavController) {
    val context = LocalContext.current
    val app = context.applicationContext as IPv6DiagApplication
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }

    val prefs = context.getSharedPreferences("ipv6diag_prefs", Context.MODE_PRIVATE)

    var customHostname by remember { mutableStateOf("") }
    var currentEndpoint by remember { mutableStateOf("") }
    var isVerifying by remember { mutableStateOf(false) }
    var turnTransport by remember { mutableStateOf(prefs.getString("turn_transport", "udp") ?: "udp") }
    var transportMenuExpanded by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        val endpoint = app.sessionRepository.getActiveEndpoint()
        currentEndpoint = endpoint?.hostname ?: "none"
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Settings") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
        snackbarHost = { SnackbarHost(snackbar) },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text("Current Server: $currentEndpoint", style = MaterialTheme.typography.bodyMedium)

            Spacer(Modifier.height(8.dp))
            Text("TURN Transport", style = MaterialTheme.typography.labelMedium)
            val selectedLabel = turnTransportOptions.firstOrNull { it.first == turnTransport }?.second
                ?: turnTransport
            ExposedDropdownMenuBox(
                expanded = transportMenuExpanded,
                onExpandedChange = { transportMenuExpanded = it },
            ) {
                OutlinedTextField(
                    value = selectedLabel,
                    onValueChange = {},
                    readOnly = true,
                    label = { Text("Protocol") },
                    trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = transportMenuExpanded) },
                    modifier = Modifier.fillMaxWidth().menuAnchor(),
                )
                ExposedDropdownMenu(
                    expanded = transportMenuExpanded,
                    onDismissRequest = { transportMenuExpanded = false },
                ) {
                    turnTransportOptions.forEach { (value, label) ->
                        DropdownMenuItem(
                            text = { Text(label) },
                            onClick = {
                                turnTransport = value
                                prefs.edit().putString("turn_transport", value).apply()
                                transportMenuExpanded = false
                            },
                        )
                    }
                }
            }

            Spacer(Modifier.height(8.dp))
            Text("Custom Server Hostname", style = MaterialTheme.typography.labelMedium)

            OutlinedTextField(
                value = customHostname,
                onValueChange = { customHostname = it },
                label = { Text("e.g. myserver.example.com") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri),
                modifier = Modifier.fillMaxWidth(),
            )

            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(
                    onClick = {
                        val input = customHostname.trim()
                        if (input.isBlank()) {
                            scope.launch { snackbar.showSnackbar("Enter host or host:port") }
                            return@Button
                        }
                        if (!isValidTestEndpoint(input)) {
                            scope.launch { snackbar.showSnackbar("Use host or host:port only") }
                            return@Button
                        }
                        scope.launch {
                            isVerifying = true
                            val reachable = verifyReachability(input)
                            if (reachable) {
                                app.sessionRepository.saveCustomEndpoint(input)
                                currentEndpoint = input
                                snackbar.showSnackbar("Custom server saved")
                            } else {
                                snackbar.showSnackbar("Server unreachable — saved anyway")
                                app.sessionRepository.saveCustomEndpoint(input)
                                currentEndpoint = input
                            }
                            isVerifying = false
                        }
                    },
                    enabled = !isVerifying,
                    modifier = Modifier.weight(1f),
                ) {
                    Text(if (isVerifying) "Verifying…" else "Save")
                }

                OutlinedButton(
                    onClick = {
                        scope.launch {
                            app.sessionRepository.clearCustomEndpoint()
                            val default = app.sessionRepository.getActiveEndpoint()
                            currentEndpoint = default?.hostname ?: "default"
                            customHostname = ""
                            snackbar.showSnackbar("Reverted to default server")
                        }
                    },
                    modifier = Modifier.weight(1f),
                ) {
                    Text("Reset to Default")
                }
            }
        }
    }
}

private suspend fun verifyReachability(hostname: String): Boolean =
    kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
        runCatching {
            OkHttpClient.Builder()
                .connectTimeout(5, TimeUnit.SECONDS)
                .readTimeout(5, TimeUnit.SECONDS)
                .build()
                .newCall(Request.Builder().url("http://$hostname/health").get().build())
                .execute()
                .use { it.isSuccessful }
        }.getOrDefault(false)
    }

private val endpointPattern = Pattern.compile("^[A-Za-z0-9.-]+(?::\\d{1,5})?$")

private fun isValidTestEndpoint(input: String): Boolean {
    if (input.contains("://") || input.contains("/") || input.contains("?") || input.contains("#")) {
        return false
    }
    if (!endpointPattern.matcher(input).matches()) {
        return false
    }
    val portText = input.substringAfter(':', "")
    if (portText.isNotEmpty()) {
        val port = portText.toIntOrNull() ?: return false
        if (port !in 1..65535) return false
    }
    return true
}
