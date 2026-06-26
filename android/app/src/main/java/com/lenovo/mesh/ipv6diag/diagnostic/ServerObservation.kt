package com.lenovo.mesh.ipv6diag.diagnostic

import android.net.Network
import com.lenovo.mesh.ipv6diag.data.model.NAT64PrefixResult
import com.lenovo.mesh.ipv6diag.data.model.ServerObservationResult
import com.lenovo.mesh.ipv6diag.data.model.XlatSubTestStatus
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.OkHttpClient
import okhttp3.Request
import java.net.Inet6Address
import java.net.InetAddress
import java.util.concurrent.TimeUnit

/**
 * Server-assisted 464XLAT detection.
 *
 * Connects to the server's IPv4 address over the cellular network and asks the server
 * what source address it observed (`/diag` → `client_address`). If we forced an IPv4
 * connection but the server saw a NAT64-embedded IPv6 source, that is end-to-end proof
 * the CLAT→PLAT path translated our traffic — independent of whether the device could
 * introspect its own CLAT interface. This is the most reliable signal on modern Android,
 * where the stacked `v4-` interface is often not visible to apps.
 */
suspend fun observeServerPath(
    network: Network,
    serverIPv4: String?,
    serverPort: Int,
    nat64Result: NAT64PrefixResult,
): ServerObservationResult = withContext(Dispatchers.IO) {
    if (serverIPv4 == null) {
        return@withContext ServerObservationResult(
            probedVia = null,
            serverObservedSource = null,
            observedFamily = null,
            translationDetected = false,
            decodedEmbeddedIPv4 = null,
            status = XlatSubTestStatus.SKIPPED,
            failureReason = "no server IPv4 address to probe the translated path",
        )
    }

    val observed = runCatching {
        val client = OkHttpClient.Builder()
            .socketFactory(network.socketFactory)
            .connectTimeout(10L, TimeUnit.SECONDS)
            .readTimeout(10L, TimeUnit.SECONDS)
            .dns(object : okhttp3.Dns {
                override fun lookup(hostname: String): List<InetAddress> =
                    listOf(InetAddress.getByName(serverIPv4))
            })
            .build()
        val request = Request.Builder().url("http://$serverIPv4:$serverPort/diag").get().build()
        client.newCall(request).execute().use { resp ->
            if (!resp.isSuccessful) return@runCatching null
            val body = resp.body?.string() ?: return@runCatching null
            Json.parseToJsonElement(body).jsonObject["client_address"]?.jsonPrimitive?.content
        }
    }.getOrNull()

    if (observed == null) {
        return@withContext ServerObservationResult(
            probedVia = "IPv4 (CLAT path)",
            serverObservedSource = null,
            observedFamily = null,
            translationDetected = false,
            decodedEmbeddedIPv4 = null,
            status = XlatSubTestStatus.FAIL,
            failureReason = "could not reach server over the IPv4/CLAT path",
        )
    }

    val addr = runCatching { InetAddress.getByName(observed) }.getOrNull()
    val isIPv6 = addr is Inet6Address
    // We forced an IPv4 connection; the server seeing an IPv6 source means the device's
    // CLAT translated it to IPv6 and the PLAT/NAT64 carried it — i.e. 464XLAT is active.
    val translated = isIPv6
    val decoded = if (isIPv6 && addr != null) {
        decodeEmbeddedIPv4(addr.address, nat64Result.preferredPrefix)
    } else null

    ServerObservationResult(
        probedVia = "IPv4 (CLAT path)",
        serverObservedSource = observed,
        observedFamily = if (isIPv6) "IPv6" else "IPv4",
        translationDetected = translated,
        decodedEmbeddedIPv4 = decoded,
        status = XlatSubTestStatus.PASS,
        failureReason = if (!translated)
            "server observed an IPv4 source — no NAT64/CLAT translation on this path" else null,
    )
}

// RFC 6052 §2.2: extract the embedded IPv4 from a NAT64 address given the prefix length.
private fun decodeEmbeddedIPv4(ipv6Bytes: ByteArray, preferredPrefix: String?): String? {
    if (ipv6Bytes.size != 16) return null
    val prefixLen = preferredPrefix?.substringAfter('/')?.toIntOrNull() ?: 96
    val ipv4Bytes = when (prefixLen) {
        96 -> ipv6Bytes.sliceArray(12..15)
        64 -> ipv6Bytes.sliceArray(9..12)
        else -> ipv6Bytes.sliceArray(12..15)
    }
    if (ipv4Bytes.size != 4) return null
    return runCatching { InetAddress.getByAddress(ipv4Bytes).hostAddress }.getOrNull()
}
