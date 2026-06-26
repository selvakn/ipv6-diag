package com.lenovo.mesh.ipv6diag.data.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class DeviceInfo(
    val name: String,
    val model: String,
    val manufacturer: String,
    @SerialName("android_version") val androidVersion: String,
    @SerialName("device_id") val deviceId: String,
)
