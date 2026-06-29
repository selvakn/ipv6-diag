package selvakn.ipv6diag.ui.network

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Card
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import selvakn.ipv6diag.IPv6DiagApplication
import selvakn.ipv6diag.data.model.NetworkInfo
import selvakn.ipv6diag.diagnostic.CellularNetworkBinder
import selvakn.ipv6diag.diagnostic.NetworkInfoCollector

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NetworkInfoScreen(navController: NavController) {
    val context = LocalContext.current
    var networkInfo by remember { mutableStateOf<NetworkInfo?>(null) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        try {
            val binder = CellularNetworkBinder(context)
            binder.withCellularNetwork { network ->
                networkInfo = NetworkInfoCollector(context).collect(network)
            }
        } catch (e: Exception) {
            error = e.message ?: "Failed to collect network info"
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Network Info") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(16.dp)
                .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            error?.let { Text("Error: $it", color = Color(0xFFD32F2F)) }

            val ni = networkInfo
            if (ni == null && error == null) {
                Text("Acquiring cellular network…")
                return@Column
            }
            if (ni == null) return@Column

            InfoSection("Cellular Interface") {
                InfoRow("Interface", ni.cellularInterfaceName ?: "unknown")
                InfoRow("Service Provider", ni.serviceProviderName ?: "unknown")
                InfoRow("IPv4 Address", ni.cellularIPv4Address ?: "none")
                InfoRow("IPv6 Addresses", ni.cellularIPv6Addresses.joinToString("\n").ifEmpty { "none" })
                InfoRow("Native IPv6", if (ni.hasNativeIPv6) "Yes ✓" else "No")
                InfoRow("Mobile Data", if (ni.mobileDataEnabled) "Enabled" else "DISABLED")
                InfoRow("Android API", ni.apiLevel.toString())
            }

            InfoSection("464XLAT / CLAT") {
                InfoRow("CLAT Present", if (ni.clatPresent) "Yes ✓ (${ni.clatInterfaceName})" else "Not detected")
                ni.clatSyntheticIPv4?.let { InfoRow("Synthetic IPv4", it) }
                ni.clatIPv6Prefix?.let { InfoRow("NAT64 Prefix", it) }
            }

            InfoSection("DNS Resolvers") {
                if (ni.dnsServers.isEmpty()) {
                    Text("No DNS servers detected", style = MaterialTheme.typography.bodyMedium)
                } else {
                    ni.dnsServers.forEachIndexed { i, ip ->
                        val name = ni.dnsServerNames.getOrNull(i)?.takeIf { it.isNotEmpty() }
                        InfoRow("Resolver ${i + 1}", if (name != null) "$ip ($name)" else ip)
                    }
                }
            }
        }
    }
}

@Composable
private fun InfoSection(title: String, content: @Composable () -> Unit) {
    Card(modifier = Modifier.padding(vertical = 2.dp)) {
        Column(Modifier.padding(12.dp)) {
            Text(title, style = MaterialTheme.typography.titleSmall)
            HorizontalDivider(Modifier.padding(vertical = 4.dp))
            content()
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String) {
    Column(Modifier.padding(vertical = 2.dp)) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = Color(0xFF757575))
        Text(value, style = MaterialTheme.typography.bodyMedium)
    }
    Spacer(Modifier.height(2.dp))
}
