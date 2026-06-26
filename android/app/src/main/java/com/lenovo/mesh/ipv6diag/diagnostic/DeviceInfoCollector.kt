package com.lenovo.mesh.ipv6diag.diagnostic

import android.content.Context
import android.os.Build
import android.provider.Settings
import com.lenovo.mesh.ipv6diag.data.model.DeviceInfo

object DeviceInfoCollector {
    fun collect(context: Context): DeviceInfo {
        val deviceId = Settings.Secure.getString(
            context.contentResolver,
            Settings.Secure.ANDROID_ID,
        ) ?: "unknown"
        val name = Build.MODEL.ifBlank { Build.DEVICE ?: "Unknown Device" }
        return DeviceInfo(
            name = name,
            model = Build.MODEL,
            manufacturer = Build.MANUFACTURER,
            androidVersion = Build.VERSION.RELEASE,
            deviceId = deviceId,
        )
    }
}
