package com.lenovo.mesh.ipv6diag.data.model

import kotlinx.serialization.Serializable

enum class Nat64DiscoveryMethod { WELL_KNOWN_PROBE, RFC7050_DNS }

enum class XlatSubTestStatus { PASS, FAIL, SKIPPED }

enum class XlatChainStatus { WORKING, PARTIAL, ABSENT, BROKEN }

@Serializable
data class Nat64PrefixEntry(
    val prefix: String,
    val prefixLengthBits: Int,
    val discoveryMethod: Nat64DiscoveryMethod,
    val isWellKnown: Boolean,
)

@Serializable
data class NAT64PrefixResult(
    val entries: List<Nat64PrefixEntry>,
    val preferredPrefix: String?,
    val status: XlatSubTestStatus,
    val failureReason: String? = null,
)

@Serializable
data class DNS64ValidationResult(
    val queriedHostname: String,
    val rawAAAARecords: List<String>,
    val decodedEmbeddedIPv4: String?,
    val synthesisTested: Boolean,
    val prefixMatches: Boolean,
    val status: XlatSubTestStatus,
    val failureReason: String? = null,
)

@Serializable
data class ClatQualityResult(
    val interfaceName: String,
    val clatIPv4Address: String?,
    val interfaceMtu: Int?,
    val effectiveIPv4Mtu: Int?,
    val clatLatencyMs: Long?,
    val nativeIPv6LatencyMs: Long?,
    val latencyDeltaMs: Long?,
    val status: XlatSubTestStatus,
    val failureReason: String? = null,
)

@Serializable
data class PlatVerificationResult(
    val serverObservedIPv6Source: String?,
    val decodedEmbeddedIPv4: String?,
    val matchesClatIPv4: Boolean,
    val platIPv6Prefix: String?,
    val prefixMatchesDiscovered: Boolean,
    val status: XlatSubTestStatus,
    val failureReason: String? = null,
)

@Serializable
data class ServerObservationResult(
    val probedVia: String?,            // e.g. "IPv4 (CLAT path)"
    val serverObservedSource: String?, // the client_address the server reported seeing
    val observedFamily: String?,       // "IPv4" or "IPv6"
    val translationDetected: Boolean,  // sent over IPv4 but server saw a NAT64 IPv6 source
    val decodedEmbeddedIPv4: String?,  // IPv4 decoded from the observed NAT64 address
    val status: XlatSubTestStatus,
    val failureReason: String? = null,
)

@Serializable
data class XlatDiagnosticSummary(
    val sessionId: String,
    val nat64Prefix: NAT64PrefixResult,
    val dns64Validation: DNS64ValidationResult,
    val clatQuality: ClatQualityResult,
    val platVerification: PlatVerificationResult,
    val overallStatus: XlatChainStatus,
    val serverObservation: ServerObservationResult? = null,
    val timestamp: Long = System.currentTimeMillis(),
)
