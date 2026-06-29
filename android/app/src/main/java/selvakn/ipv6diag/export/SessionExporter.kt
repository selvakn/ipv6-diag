package selvakn.ipv6diag.export

import selvakn.ipv6diag.data.model.DiagnosticSession
import selvakn.ipv6diag.data.model.TestStatus
import selvakn.ipv6diag.data.model.XlatDiagnosticSummary
import selvakn.ipv6diag.data.model.XlatSubTestStatus
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.encodeToJsonElement
import kotlinx.serialization.json.jsonObject
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

private val dateFormat = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US)
private val prettyJson = Json { prettyPrint = true }

object SessionExporter {

    fun exportAsText(session: DiagnosticSession, xlatSummary: XlatDiagnosticSummary? = null): String = buildString {
        appendLine("=== IPv6 Diagnostic Report ===")
        appendLine("Date     : ${dateFormat.format(Date(session.timestamp))}")
        appendLine("Status   : ${session.status}")
        appendLine("Server   : ${session.serverEndpoint.hostname}")
        session.abortReason?.let { appendLine("Aborted  : $it") }
        appendLine()

        appendLine("--- Network Info ---")
        val ni = session.networkInfo
        appendLine("Mobile data   : ${if (ni.mobileDataEnabled) "enabled" else "DISABLED"}")
        appendLine("Interface     : ${ni.cellularInterfaceName ?: "unknown"}")
        appendLine("Provider      : ${ni.serviceProviderName ?: "unknown"}")
        appendLine("IPv4 address  : ${ni.cellularIPv4Address ?: "none"}")
        appendLine("IPv6 addresses: ${ni.cellularIPv6Addresses.joinToString(", ").ifEmpty { "none" }}")
        appendLine("Native IPv6   : ${ni.hasNativeIPv6}")
        appendLine("CLAT / 464XLAT: ${if (ni.clatPresent) "present (${ni.clatInterfaceName})" else "not detected"}")
        ni.clatSyntheticIPv4?.let { appendLine("CLAT IPv4     : $it") }
        appendLine("DNS resolvers : ${ni.dnsServers.joinToString(", ").ifEmpty { "unknown" }}")
        if (ni.dnsServerNames.any { it.isNotEmpty() }) {
            appendLine("DNS names     : ${ni.dnsServerNames.filter { it.isNotEmpty() }.joinToString(", ")}")
        }
        appendLine("Android API   : ${ni.apiLevel}")
        appendLine()

        appendLine("--- Test Results ---")
        session.testResults.forEach { r ->
            val mark = when (r.status) {
                TestStatus.PASS -> "PASS"
                TestStatus.FAIL -> "FAIL"
                TestStatus.SKIPPED -> "SKIP"
                TestStatus.ABORTED -> "ABRT"
            }
            val latency = r.latencyMs?.let { "${it}ms" } ?: "-"
            val extra = buildList {
                r.resolvedAddress?.let { add("addr=$it") }
                r.serverConfirmedFamily?.let { add("server=$it") }
                r.packetLoss?.let { add("loss=${(it * 100).toInt()}%") }
                r.failureReason?.let { add("reason=$it") }
            }.joinToString(" ")
            appendLine("  [${r.testType}][${r.addressFamily}] $mark  latency=$latency  $extra")
        }

        val passed = session.testResults.count { it.status == TestStatus.PASS }
        val total = session.testResults.size
        appendLine()
        appendLine("Summary: $passed/$total tests passed")

        // 464XLAT section
        appendLine()
        appendLine("--- 464XLAT Diagnostics ---")
        if (xlatSummary == null || xlatSummary.nat64Prefix.status == XlatSubTestStatus.SKIPPED) {
            appendLine("464XLAT not detected on this network (no CLAT interface)")
        } else {
            appendLine("Overall chain  : ${xlatSummary.overallStatus}")
            val nat64 = xlatSummary.nat64Prefix
            if (nat64.preferredPrefix != null) {
                val methods = nat64.entries.joinToString(", ") { it.discoveryMethod.name }
                val wk = if (nat64.entries.any { it.isWellKnown }) "well-known" else "carrier-specific"
                appendLine("NAT64 prefix   : ${nat64.preferredPrefix}  [$wk, $methods]")
                if (nat64.entries.size > 1) {
                    appendLine("All prefixes   : ${nat64.entries.joinToString(", ") { it.prefix }}")
                }
            } else {
                appendLine("NAT64 prefix   : not found")
            }
            val dns64 = xlatSummary.dns64Validation
            appendLine("DNS64 synthesis: ${dns64.status}  decoded=${dns64.decodedEmbeddedIPv4 ?: "n/a"}  prefix-match=${dns64.prefixMatches}")
            val clat = xlatSummary.clatQuality
            appendLine("CLAT interface : ${clat.interfaceName}  mtu=${clat.interfaceMtu ?: "n/a"}  effective-ipv4-mtu=${clat.effectiveIPv4Mtu ?: "n/a"}")
            appendLine("CLAT latency   : ${clat.clatLatencyMs?.let { "${it}ms" } ?: "n/a"}  native-ipv6=${clat.nativeIPv6LatencyMs?.let { "${it}ms" } ?: "n/a"}  delta=${clat.latencyDeltaMs?.let { "${if (it >= 0) "+" else ""}${it}ms" } ?: "n/a"}")
            val plat = xlatSummary.platVerification
            appendLine("PLAT verified  : server-saw=${plat.serverObservedIPv6Source ?: "n/a"}  decoded-ipv4=${plat.decodedEmbeddedIPv4 ?: "n/a"}  clat-match=${plat.matchesClatIPv4}")
        }
        appendLine("----------------------------")
        appendLine("==============================")
    }

    fun exportAsJson(session: DiagnosticSession, xlatSummary: XlatDiagnosticSummary? = null): String {
        val sessionJson = prettyJson.encodeToJsonElement(DiagnosticSession.serializer(), session).jsonObject
        val map = sessionJson.toMutableMap()
        if (xlatSummary != null) {
            map["xlatSummary"] = prettyJson.encodeToJsonElement(XlatDiagnosticSummary.serializer(), xlatSummary)
        }
        return prettyJson.encodeToString(JsonObject.serializer(), JsonObject(map))
    }
}
