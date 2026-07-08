package selvakn.ipv6diag.ui.home

import android.app.Application
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
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
import selvakn.ipv6diag.IPv6DiagApplication
import selvakn.ipv6diag.diagnostic.DeviceInfoCollector
import selvakn.ipv6diag.diagnostic.DiagnosticRunner
import selvakn.ipv6diag.diagnostic.IpMode
import selvakn.ipv6diag.diagnostic.NetworkInfoCollector
import selvakn.ipv6diag.diagnostic.TestCategory
import selvakn.ipv6diag.upload.UploadStatus
import selvakn.ipv6diag.upload.uploadReport
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(navController: NavController) {
    val context = LocalContext.current
    val app = context.applicationContext as IPv6DiagApplication
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }

    var isRunning by remember { mutableStateOf(false) }
    var selectedCategories by remember { mutableStateOf(TestCategory.entries.toSet()) }
    var selectedIpMode by remember { mutableStateOf(IpMode.AUTO) }

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
            FlowRow(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                FilterChip(
                    selected = selectedCategories.size == TestCategory.entries.size,
                    onClick = { selectedCategories = TestCategory.entries.toSet() },
                    label = { Text("ALL") },
                )
                TestCategory.entries.forEach { category ->
                    FilterChip(
                        selected = category in selectedCategories,
                        onClick = {
                            selectedCategories = if (category in selectedCategories) {
                                selectedCategories - category
                            } else {
                                selectedCategories + category
                            }
                        },
                        label = { Text(category.name.replace("_", "/")) },
                    )
                }
            }

            Spacer(Modifier.height(16.dp))

            Text("IP Stack", style = MaterialTheme.typography.labelMedium)
            Spacer(Modifier.height(8.dp))
            FlowRow(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                IpMode.entries.forEach { mode ->
                    FilterChip(
                        selected = selectedIpMode == mode,
                        onClick = { selectedIpMode = mode },
                        label = { Text(ipModeLabel(mode)) },
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
                                val session = runner.runTests(endpoint, selectedCategories, selectedIpMode)
                                // Auto-upload: mark uploading, then fire-and-forget
                                app.setUploadStatus(session.id, UploadStatus.Uploading)
                                launch {
                                    val device = DeviceInfoCollector.collect(context)
                                    val xlat = app.sessionRepository.getXlatSummary(session.id)
                                    val status = uploadReport(session, device, xlat, app.reportingBaseUrl)
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
                    enabled = selectedCategories.isNotEmpty(),
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Text(
                        if (selectedCategories.isEmpty()) "Select a test to run"
                        else "Run Tests (${selectedCategories.size})"
                    )
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

private fun ipModeLabel(mode: IpMode): String = when (mode) {
    IpMode.AUTO -> "Auto"
    IpMode.IPV4_ONLY -> "IPv4 only"
    IpMode.IPV6_ONLY -> "IPv6 only"
}
