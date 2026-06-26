package com.lenovo.mesh.ipv6diag.ui.home

import android.app.Application
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavController
import com.lenovo.mesh.ipv6diag.IPv6DiagApplication
import com.lenovo.mesh.ipv6diag.diagnostic.DeviceInfoCollector
import com.lenovo.mesh.ipv6diag.diagnostic.DiagnosticRunner
import com.lenovo.mesh.ipv6diag.diagnostic.NetworkInfoCollector
import com.lenovo.mesh.ipv6diag.diagnostic.TestFilter
import com.lenovo.mesh.ipv6diag.upload.UploadStatus
import com.lenovo.mesh.ipv6diag.upload.uploadReport
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(navController: NavController) {
    val context = LocalContext.current
    val app = context.applicationContext as IPv6DiagApplication
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }

    var isRunning by remember { mutableStateOf(false) }
    var selectedFilter by remember { mutableStateOf(TestFilter.ALL) }

    val runner = remember {
        DiagnosticRunner(context, app.sessionRepository, NetworkInfoCollector(context))
    }

    Scaffold(
        topBar = { TopAppBar(title = { Text("IPv6 Diagnostic") }) },
        snackbarHost = { SnackbarHost(snackbar) },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text("Protocol Filter", style = MaterialTheme.typography.labelMedium)
            Spacer(Modifier.height(8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TestFilter.entries.forEach { f ->
                    FilterChip(
                        selected = selectedFilter == f,
                        onClick = { selectedFilter = f },
                        label = { Text(f.name.replace("_", "/")) },
                    )
                }
            }

            Spacer(Modifier.height(24.dp))

            if (isRunning) {
                CircularProgressIndicator()
                Spacer(Modifier.height(8.dp))
                Text("Running tests…", style = MaterialTheme.typography.bodyMedium)
            } else {
                Button(
                    onClick = {
                        scope.launch {
                            isRunning = true
                            try {
                                val endpoint = app.sessionRepository.getActiveEndpoint()
                                if (endpoint == null) {
                                    snackbar.showSnackbar("No server configured. Check Settings.")
                                    return@launch
                                }
                                val session = runner.runTests(endpoint, selectedFilter)
                                // Auto-upload: mark uploading, then fire-and-forget
                                app.setUploadStatus(session.id, UploadStatus.Uploading)
                                launch {
                                    val device = DeviceInfoCollector.collect(context)
                                    val xlat = app.sessionRepository.getXlatSummary(session.id)
                                    val serverUrl = "http://${endpoint.hostname}:${endpoint.httpPort}"
                                    val status = uploadReport(session, device, xlat, serverUrl)
                                    app.setUploadStatus(session.id, status)
                                }
                                navController.navigate("results/${session.id}")
                            } catch (e: Exception) {
                                snackbar.showSnackbar(e.message ?: "Test failed")
                            } finally {
                                isRunning = false
                            }
                        }
                    },
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Text("Run ${selectedFilter.name.replace("_", "/")} Tests")
                }
            }

            Spacer(Modifier.height(16.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly,
            ) {
                TextButton(onClick = { navController.navigate("networkInfo") }) { Text("Network Info") }
                TextButton(onClick = { navController.navigate("history") }) { Text("History") }
                TextButton(onClick = { navController.navigate("settings") }) { Text("Settings") }
            }
        }
    }
}
