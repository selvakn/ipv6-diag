package selvakn.ipv6diag.data.model

import kotlinx.serialization.Serializable

@Serializable
data class NetworkInfo(
    val cellularInterfaceName: String? = null,
    val serviceProviderName: String? = null,
    val cellularIPv4Address: String? = null,
    val cellularIPv6Addresses: List<String> = emptyList(),
    val hasNativeIPv6: Boolean = false,
    val clatPresent: Boolean = false,
    val clatInterfaceName: String? = null,
    val clatSyntheticIPv4: String? = null,
    val clatIPv6Prefix: String? = null,
    val dnsServers: List<String> = emptyList(),
    val dnsServerNames: List<String> = emptyList(),
    val mobileDataEnabled: Boolean = true,
    val apiLevel: Int = android.os.Build.VERSION.SDK_INT,
)
