package com.lenovo.mesh.ipv6diag.diagnostic

import android.content.Context
import android.net.Network
import com.lenovo.mesh.ipv6diag.data.model.NetworkInfo
import com.lenovo.mesh.ipv6diag.data.model.XlatChainStatus
import com.lenovo.mesh.ipv6diag.data.model.XlatDiagnosticSummary
import com.lenovo.mesh.ipv6diag.data.model.XlatSubTestStatus

suspend fun runXlatDiagnostics(
    context: Context,
    network: Network,
    networkInfo: NetworkInfo,
    sessionId: String,
    serverIPv4: String?,
    serverIPv6: String?,
    serverPort: Int,
): XlatDiagnosticSummary {
    // NAT64/DNS64 are network-level facts: discover them regardless of whether a
    // device-side CLAT interface was detected. The PLAT/NAT64 infrastructure can be
    // present even when CLAT detection is unreliable (common on API 30+).
    val nat64 = discoverNat64Prefix(context, network)
    val dns64 = validateDns64(context, network, nat64)

    // Server-assisted detection: ask the server what source it observed on the IPv4 path.
    // This proves 464XLAT end-to-end without relying on device-side CLAT introspection.
    val serverObs = observeServerPath(network, serverIPv4, serverPort, nat64)

    val clatPresent = networkInfo.clatPresent

    // CLAT-specific sub-tests only make sense when a device CLAT interface/address exists.
    val clatQuality = if (clatPresent) {
        assessClatQuality(context, network, networkInfo, serverIPv4, serverIPv6)
    } else {
        com.lenovo.mesh.ipv6diag.data.model.ClatQualityResult(
            interfaceName = "", clatIPv4Address = null, interfaceMtu = null,
            effectiveIPv4Mtu = null, clatLatencyMs = null, nativeIPv6LatencyMs = null,
            latencyDeltaMs = null, status = XlatSubTestStatus.SKIPPED,
            failureReason = "no device CLAT interface detected"
        )
    }
    val platVerif = if (clatPresent) {
        verifyPlatPath(network, serverIPv4, serverPort, nat64, clatQuality)
    } else {
        com.lenovo.mesh.ipv6diag.data.model.PlatVerificationResult(
            serverObservedIPv6Source = null, decodedEmbeddedIPv4 = null,
            matchesClatIPv4 = false, platIPv6Prefix = null, prefixMatchesDiscovered = false,
            status = XlatSubTestStatus.SKIPPED,
            failureReason = "no device CLAT interface — PLAT path not exercised"
        )
    }

    val overall = computeOverallStatus(clatPresent, serverObs, nat64, dns64, clatQuality, platVerif)

    return XlatDiagnosticSummary(
        sessionId = sessionId,
        nat64Prefix = nat64,
        dns64Validation = dns64,
        clatQuality = clatQuality,
        platVerification = platVerif,
        overallStatus = overall,
        serverObservation = serverObs,
    )
}

private fun computeOverallStatus(
    clatPresent: Boolean,
    serverObs: com.lenovo.mesh.ipv6diag.data.model.ServerObservationResult,
    nat64: com.lenovo.mesh.ipv6diag.data.model.NAT64PrefixResult,
    dns64: com.lenovo.mesh.ipv6diag.data.model.DNS64ValidationResult,
    clat: com.lenovo.mesh.ipv6diag.data.model.ClatQualityResult,
    plat: com.lenovo.mesh.ipv6diag.data.model.PlatVerificationResult,
): XlatChainStatus {
    val nat64Found = nat64.status == XlatSubTestStatus.PASS

    // Strongest evidence: the server saw our forced-IPv4 traffic arrive as a NAT64-embedded
    // IPv6 source. That is end-to-end proof the 464XLAT/PLAT path works, even when the device
    // could not introspect its own CLAT interface.
    if (serverObs.status == XlatSubTestStatus.PASS && serverObs.translationDetected) {
        return XlatChainStatus.WORKING
    }

    if (clatPresent) {
        // Device CLAT is active — evaluate the full 464XLAT chain.
        if (nat64.status == XlatSubTestStatus.FAIL ||
            clat.status == XlatSubTestStatus.FAIL ||
            plat.status == XlatSubTestStatus.FAIL
        ) {
            return XlatChainStatus.BROKEN
        }
        val allPass = nat64Found &&
            dns64.status == XlatSubTestStatus.PASS &&
            clat.status == XlatSubTestStatus.PASS &&
            plat.status == XlatSubTestStatus.PASS
        return if (allPass) XlatChainStatus.WORKING else XlatChainStatus.PARTIAL
    }

    // No device-side CLAT. NAT64/DNS64 may still exist (IPv6-only network where apps
    // use IPv6 sockets directly): report PARTIAL so the discovered facts are visible.
    return if (nat64Found || dns64.status == XlatSubTestStatus.PASS) {
        XlatChainStatus.PARTIAL
    } else {
        XlatChainStatus.ABSENT
    }
}
