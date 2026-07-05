package wgmodule

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// Peer is an in-process userspace WireGuard peer backed by gVisor netstack.
// No root, no kernel TUN, no VPN Service required.
type Peer struct {
	cred    *WireGuardCredential
	dev     *device.Device
	tnet    *netstack.Net
	echoUDP *net.UDPAddr // server echo address (server tunnel IP + echo port)
}

// NewPeer creates and starts a WireGuard peer for the given credential.
// The WireGuard handshake is initiated but NewPeer does not wait for it —
// call WaitHandshake(ctx) to block until the tunnel is established.
func NewPeer(cred *WireGuardCredential, serverEchoPort int) (*Peer, error) {
	tunnelAddr, ok := netip.AddrFromSlice(cred.ClientIP.IP)
	if !ok {
		return nil, fmt.Errorf("wgmodule: invalid client IP %s", cred.ClientIP.IP)
	}
	tunnelAddr = tunnelAddr.Unmap()

	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{tunnelAddr},
		nil,
		1420,
	)
	if err != nil {
		return nil, fmt.Errorf("wgmodule: create netstack TUN: %w", err)
	}

	logger := device.NewLogger(device.LogLevelSilent, "")
	dev := device.NewDevice(tun, conn.NewDefaultBind(), logger)

	// Build UAPI config string.
	// Interface section: set private key.
	uapi := fmt.Sprintf("private_key=%s\n", keyToHex(cred.ClientPrivateKey))
	// Peer section: server public key, endpoint, allowed IPs.
	uapi += fmt.Sprintf("public_key=%s\n", keyToHex(cred.ServerPublicKey))
	uapi += fmt.Sprintf("endpoint=%s\n", cred.ServerEndpoint.String())
	uapi += "allowed_ip=0.0.0.0/0\n"
	uapi += "allowed_ip=::/0\n"
	uapi += "persistent_keepalive_interval=25\n"

	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return nil, fmt.Errorf("wgmodule: IpcSet peer config: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("wgmodule: device Up: %w", err)
	}

	// Derive echo address: server tunnel IP is first usable IP in the /24 from the
	// client's CIDR. Since the server issues IPs as 10.0.0.X/24 and the server's
	// tunnel IP is 10.0.0.1, we derive it from the network.
	serverEchoIP := serverTunnelIP(cred.ClientIP)
	echoUDP := &net.UDPAddr{IP: serverEchoIP, Port: serverEchoPort}

	return &Peer{
		cred:    cred,
		dev:     dev,
		tnet:    tnet,
		echoUDP: echoUDP,
	}, nil
}

// WaitHandshake blocks until the WireGuard handshake completes or ctx is cancelled.
// Returns an error if the handshake does not complete before ctx expires.
func (p *Peer) WaitHandshake(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	_ = deadline
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wgmodule: handshake timeout waiting for peer %s", p.cred.ServerEndpoint)
		case <-ticker.C:
			ipc, err := p.dev.IpcGet()
			if err != nil {
				continue
			}
			if strings.Contains(ipc, "last_handshake_time_sec=") {
				// Parse to check if it's non-zero.
				for _, line := range strings.Split(ipc, "\n") {
					if strings.HasPrefix(line, "last_handshake_time_sec=") {
						val := strings.TrimPrefix(line, "last_handshake_time_sec=")
						if val != "0" && val != "" {
							return nil
						}
					}
				}
			}
		}
	}
}

// DialUDP opens a UDP connection from the WireGuard tunnel's netstack to the given address.
func (p *Peer) DialUDP(raddr *net.UDPAddr) (net.Conn, error) {
	return p.tnet.DialUDP(nil, raddr)
}

// EchoServerAddr returns the server echo service UDP address.
func (p *Peer) EchoServerAddr() *net.UDPAddr {
	return p.echoUDP
}

// Close shuts down the WireGuard device.
func (p *Peer) Close() {
	if p.dev != nil {
		p.dev.Close()
	}
}

// serverTunnelIP returns the server's tunnel IP (first usable IP in the /24
// subnet from the client's allocated CIDR).
func serverTunnelIP(clientIPNet *net.IPNet) net.IP {
	ip := clientIPNet.IP.To4()
	if ip == nil {
		return nil
	}
	// Derive network address and add 1.
	ones, _ := clientIPNet.Mask.Size()
	mask := net.CIDRMask(ones, 32)
	network := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		network[i] = ip[i] & mask[i]
	}
	network[3]++
	return network
}
