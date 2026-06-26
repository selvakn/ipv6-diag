package com.lenovo.mesh.ipv6diag.data.model

import kotlinx.serialization.Serializable

@Serializable
data class ServerEndpoint(
    val id: String,
    val hostname: String,
    val ipv4Address: String? = null,
    val ipv6Address: String? = null,
    val httpPort: Int = 80,
    val httpsPort: Int = 443,
    val isDefault: Boolean = false,
    val lastVerified: Long? = null,
    val useHttps: Boolean = false,
) {
    val baseUrl: String get() = if (useHttps) "https://$hostname" else "http://$hostname:$httpPort"
}
