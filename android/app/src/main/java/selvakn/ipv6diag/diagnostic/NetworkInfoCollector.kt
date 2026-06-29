package selvakn.ipv6diag.diagnostic

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.os.Build
import android.telephony.TelephonyManager
import selvakn.ipv6diag.data.model.NetworkInfo
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.Inet4Address
import java.net.Inet6Address
import java.net.InetAddress

class NetworkInfoCollector(private val context: Context) {

    private val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager

    suspend fun collect(network: Network): NetworkInfo = withContext(Dispatchers.IO) {
        val linkProps = cm.getLinkProperties(network)
        val netCaps = cm.getNetworkCapabilities(network)

        val allAddresses = linkProps?.linkAddresses?.map { it.address } ?: emptyList()
        val baseInterface = linkProps?.interfaceName

        // Separate IPv4 and IPv6 addresses; exclude link-local IPv6 (fe80::/10)
        val ipv4Addr = allAddresses.firstOrNull { it is Inet4Address }?.hostAddress
        val globalIPv6 = allAddresses
            .filter { it is Inet6Address && !it.isLinkLocalAddress }
            .map { it.hostAddress ?: "" }
            .filter { it.isNotEmpty() }

        // --- CLAT / 464XLAT detection (multi-signal) ---
        // Signal A: a stacked CLAT interface named "clat*" or "v4-*". Reliable on older
        // Android, but on API 30+ the stacked interface is often not enumerable by apps.
        val allIfaces = try {
            java.net.NetworkInterface.getNetworkInterfaces()?.toList() ?: emptyList()
        } catch (_: Exception) {
            emptyList()
        }
        val clatIface = allIfaces.firstOrNull { iface ->
            iface.name.startsWith("clat", ignoreCase = true) ||
                iface.name.startsWith("v4-", ignoreCase = true)
        }

        // Signal B: a synthetic IPv4 in the RFC 7335 464XLAT range 192.0.0.0/29.
        // Modern Android surfaces the CLAT address in LinkProperties (and/or on the
        // stacked interface) even when the v4- interface itself isn't enumerable.
        val clatRangeFromLink = allAddresses
            .filterIsInstance<Inet4Address>()
            .firstOrNull { isClatRange(it) }
        val clatRangeFromIface = clatIface?.inetAddresses?.toList()
            ?.filterIsInstance<Inet4Address>()
            ?.firstOrNull()

        val clatPresent = clatIface != null || clatRangeFromLink != null
        val clatInterfaceName = clatIface?.name
            ?: clatRangeFromLink?.let { baseInterface }
        val clatSyntheticIPv4 = (clatRangeFromLink ?: clatRangeFromIface)?.hostAddress
        val allInterfaces = baseInterface

        // DNS servers from link properties
        val dnsServers = linkProps?.dnsServers?.map { it.hostAddress ?: "" }
            ?.filter { it.isNotEmpty() } ?: emptyList()

        // Best-effort reverse DNS for server names (2s timeout)
        val dnsNames = dnsServers.map { ip ->
            try {
                withContext(Dispatchers.IO) {
                    InetAddress.getByName(ip).canonicalHostName.takeIf { it != ip } ?: ""
                }
            } catch (_: Exception) { "" }
        }

        val tm = context.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
        val serviceProviderName = when {
            tm.networkOperatorName.isNotBlank() -> tm.networkOperatorName
            tm.simOperatorName.isNotBlank() -> tm.simOperatorName
            else -> null
        }

        NetworkInfo(
            cellularInterfaceName = allInterfaces,
            serviceProviderName = serviceProviderName,
            cellularIPv4Address = ipv4Addr,
            cellularIPv6Addresses = globalIPv6,
            hasNativeIPv6 = globalIPv6.isNotEmpty(),
            clatPresent = clatPresent,
            clatInterfaceName = clatInterfaceName,
            clatSyntheticIPv4 = clatSyntheticIPv4,
            clatIPv6Prefix = null, // Populated separately if detectable via DNS64 prefix
            dnsServers = dnsServers,
            dnsServerNames = dnsNames,
            mobileDataEnabled = tm.isDataEnabled,
            apiLevel = Build.VERSION.SDK_INT,
        )
    }

    /** True if [addr] falls in 192.0.0.0/29, the RFC 7335 range reserved for 464XLAT CLAT. */
    private fun isClatRange(addr: Inet4Address): Boolean {
        val b = addr.address
        return b.size == 4 &&
            b[0] == 192.toByte() &&
            b[1] == 0.toByte() &&
            b[2] == 0.toByte() &&
            (b[3].toInt() and 0xF8) == 0
    }
}
