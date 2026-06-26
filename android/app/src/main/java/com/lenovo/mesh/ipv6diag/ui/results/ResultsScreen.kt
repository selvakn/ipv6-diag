package com.lenovo.mesh.ipv6diag.ui.results

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import androidx.compose.runtime.collectAsState
import com.lenovo.mesh.ipv6diag.IPv6DiagApplication
import com.lenovo.mesh.ipv6diag.data.model.DiagnosticSession
import com.lenovo.mesh.ipv6diag.data.model.TestResult
import com.lenovo.mesh.ipv6diag.data.model.TestStatus
import com.lenovo.mesh.ipv6diag.data.model.XlatChainStatus
import com.lenovo.mesh.ipv6diag.data.model.XlatDiagnosticSummary
import com.lenovo.mesh.ipv6diag.data.model.XlatSubTestStatus
import com.lenovo.mesh.ipv6diag.export.SessionExporter
import com.lenovo.mesh.ipv6diag.upload.UploadStatus
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ResultsScreen(sessionId: String, navController: NavController) {
    val context = LocalContext.current
    val app = context.applicationContext as IPv6DiagApplication
    val scope = rememberCoroutineScope()
    var session by remember { mutableStateOf<DiagnosticSession?>(null) }
    var xlatSummary by remember { mutableStateOf<XlatDiagnosticSummary?>(null) }
    val uploadStatusMap by app.uploadStatus.collectAsState()
    val uploadStatus = uploadStatusMap[sessionId] ?: UploadStatus.Idle

    LaunchedEffect(sessionId) {
        session = app.sessionRepository.getSessionById(sessionId)
        xlatSummary = app.sessionRepository.getXlatSummary(sessionId)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Test Results") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        }
    ) { padding ->
        val s = session
        if (s == null) {
            Column(Modifier.padding(padding).padding(16.dp)) { Text("Loading…") }
            return@Scaffold
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            item {
                Text("Server: ${s.serverEndpoint.hostname}", style = MaterialTheme.typography.labelMedium)
                Text("Status: ${s.status}", style = MaterialTheme.typography.labelMedium)
                s.abortReason?.let {
                    Text("⚠ $it", color = Color(0xFFF57C00), style = MaterialTheme.typography.bodySmall)
                }
                val passed = s.testResults.count { it.status == TestStatus.PASS }
                Text("${passed}/${s.testResults.size} tests passed", style = MaterialTheme.typography.titleMedium)
                Spacer(Modifier.height(4.dp))
                val (uploadLabel, uploadColor) = when (uploadStatus) {
                    is UploadStatus.Idle -> "" to Color.Transparent
                    is UploadStatus.Uploading -> "Uploading…" to Color(0xFF1565C0)
                    is UploadStatus.Success -> "Uploaded ✓" to Color(0xFF388E3C)
                    is UploadStatus.Failed -> "Upload failed: ${uploadStatus.reason}" to Color(0xFFD32F2F)
                }
                if (uploadLabel.isNotEmpty()) {
                    Text(uploadLabel, color = uploadColor, style = MaterialTheme.typography.bodySmall)
                }
                Spacer(Modifier.height(8.dp))
            }

            items(s.testResults) { result ->
                TestResultCard(result)
            }

            // 464XLAT section
            item {
                Spacer(Modifier.height(8.dp))
                XlatSummarySection(xlatSummary)
            }

            item {
                Spacer(Modifier.height(16.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(
                        onClick = { shareResults(context, s, xlatSummary, "text/plain") },
                        Modifier.weight(1f),
                    ) { Text("Share Text") }
                    Button(
                        onClick = { shareResults(context, s, xlatSummary, "application/json") },
                        Modifier.weight(1f),
                    ) { Text("Share JSON") }
                }
                OutlinedButton(
                    onClick = {
                        val clip = ClipData.newPlainText(
                            "IPv6 Diagnostic",
                            SessionExporter.exportAsText(s, xlatSummary),
                        )
                        (context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager).setPrimaryClip(clip)
                    },
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Text("Copy to Clipboard")
                }
            }
        }
    }
}

@Composable
private fun XlatSummarySection(summary: XlatDiagnosticSummary?) {
    val chainColor = when (summary?.overallStatus) {
        XlatChainStatus.WORKING -> Color(0xFF388E3C)
        XlatChainStatus.PARTIAL -> Color(0xFFF57C00)
        XlatChainStatus.BROKEN -> Color(0xFFD32F2F)
        XlatChainStatus.ABSENT, null -> Color(0xFF757575)
    }

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
    ) {
        Column(Modifier.padding(12.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text("464XLAT Diagnostics", style = MaterialTheme.typography.titleSmall)
                Text(
                    summary?.overallStatus?.name ?: "NOT RUN",
                    color = chainColor,
                    style = MaterialTheme.typography.labelMedium,
                )
            }

            if (summary == null || summary.nat64Prefix.status == XlatSubTestStatus.SKIPPED) {
                Spacer(Modifier.height(4.dp))
                Text(
                    "464XLAT not detected on this network",
                    style = MaterialTheme.typography.bodySmall,
                    color = Color(0xFF757575),
                )
                return@Card
            }

            HorizontalDivider(Modifier.padding(vertical = 6.dp))

            XlatRow("NAT64 Prefix", summary.nat64Prefix.status,
                summary.nat64Prefix.preferredPrefix ?: "not found")
            if (summary.nat64Prefix.entries.size > 1) {
                Text(
                    "All: ${summary.nat64Prefix.entries.joinToString(", ") { it.prefix }}",
                    style = MaterialTheme.typography.bodySmall,
                )
            }

            XlatRow("DNS64 Synthesis", summary.dns64Validation.status,
                summary.dns64Validation.decodedEmbeddedIPv4?.let { "decoded $it" }
                    ?: summary.dns64Validation.failureReason ?: "")

            val clatDetail = buildString {
                summary.clatQuality.interfaceMtu?.let { append("mtu=$it") }
                summary.clatQuality.clatLatencyMs?.let { append(" clat=${it}ms") }
                summary.clatQuality.nativeIPv6LatencyMs?.let { append(" ipv6=${it}ms") }
                summary.clatQuality.latencyDeltaMs?.let { append(" Δ${if (it >= 0) "+$it" else "$it"}ms") }
            }.trim()
            XlatRow("CLAT Quality", summary.clatQuality.status,
                clatDetail.ifEmpty { summary.clatQuality.failureReason ?: "" })

            val platDetail = when {
                summary.platVerification.matchesClatIPv4 ->
                    "IPv4 match ✓ (${summary.platVerification.decodedEmbeddedIPv4})"
                summary.platVerification.decodedEmbeddedIPv4 != null ->
                    "IPv4 mismatch: ${summary.platVerification.decodedEmbeddedIPv4}"
                else -> summary.platVerification.failureReason ?: ""
            }
            XlatRow("PLAT Verified", summary.platVerification.status, platDetail)
        }
    }
}

@Composable
private fun XlatRow(label: String, status: XlatSubTestStatus, detail: String) {
    val color = when (status) {
        XlatSubTestStatus.PASS -> Color(0xFF388E3C)
        XlatSubTestStatus.FAIL -> Color(0xFFD32F2F)
        XlatSubTestStatus.SKIPPED -> Color(0xFF757575)
    }
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(label, style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
        Text(status.name, color = color, style = MaterialTheme.typography.labelSmall)
    }
    if (detail.isNotEmpty()) {
        Text(detail, style = MaterialTheme.typography.bodySmall, color = Color(0xFF616161))
    }
}

@Composable
private fun TestResultCard(result: TestResult) {
    val statusColor = when (result.status) {
        TestStatus.PASS -> Color(0xFF388E3C)
        TestStatus.FAIL -> Color(0xFFD32F2F)
        TestStatus.SKIPPED -> Color(0xFF757575)
        TestStatus.ABORTED -> Color(0xFFF57C00)
    }
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(12.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text("[${result.testType}] ${result.addressFamily}", style = MaterialTheme.typography.titleSmall)
                Text(result.status.name, color = statusColor, style = MaterialTheme.typography.labelMedium)
            }
            result.latencyMs?.let { Text("Latency: ${it}ms", style = MaterialTheme.typography.bodySmall) }
            result.packetLoss?.let { Text("Packet loss: ${(it * 100).toInt()}%", style = MaterialTheme.typography.bodySmall) }
            result.resolvedAddress?.let { Text("Address: $it", style = MaterialTheme.typography.bodySmall) }
            result.serverConfirmedFamily?.let { Text("Server confirmed: $it", style = MaterialTheme.typography.bodySmall) }
            result.failureReason?.let { Text("Reason: $it", color = Color(0xFFD32F2F), style = MaterialTheme.typography.bodySmall) }
        }
    }
}

private fun shareResults(
    context: Context,
    session: DiagnosticSession,
    xlatSummary: XlatDiagnosticSummary?,
    mimeType: String,
) {
    val text = if (mimeType == "application/json") SessionExporter.exportAsJson(session, xlatSummary)
               else SessionExporter.exportAsText(session, xlatSummary)
    val intent = Intent(Intent.ACTION_SEND).apply {
        type = mimeType
        putExtra(Intent.EXTRA_TEXT, text)
    }
    context.startActivity(Intent.createChooser(intent, "Share Diagnostic Report"))
}
