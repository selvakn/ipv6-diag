package turn

import (
	"errors"
	"net"
	"strconv"
)

// relayAddressGeneratorIPv6 is a drop-in replacement for pion/turn's
// RelayAddressGeneratorNone, used for all IPv6 TURN listeners (UDP6, TCP6,
// TLS6, DTLS6).
//
// pion/turn v4's AllocationManager.CreateAllocation hardcodes "udp4" when it
// calls RelayAddressGenerator.AllocatePacketConn, regardless of which listener
// received the request or the client's actual address family:
//
//	conn, relayAddr, err := m.allocatePacketConn("udp4", requestedPort)
//
// For an IPv6 relay address (e.g. "2400:6180::1"), net.ListenPacket("udp4",
// "[2400:6180::1]:0") fails immediately with an address-family mismatch
// ("no suitable address found"), which pion surfaces to the client as a 508
// (Insufficient Capacity) Allocate error. This means no IPv6 TURN allocation
// can ever succeed with the stock RelayAddressGeneratorNone — regardless of
// whether the client reaches the server over UDP or TCP, since both transports
// route through the same AllocatePacketConn call for the actual peer-facing
// relay socket.
//
// This generator fixes the problem by ignoring the (always-wrong) network
// argument pion passes in and always opening a "udp6" socket instead. If
// pion/turn ever fixes the hardcoding (tracked upstream; already resolved in
// pion/turn v5, which is not yet a drop-in replacement for this codebase),
// this type can be retired in favor of the stock RelayAddressGeneratorNone.
type relayAddressGeneratorIPv6 struct {
	// Address is the local IPv6 address to bind the relay socket to (bare,
	// no brackets — e.g. "2400:6180::1" or "::").
	Address string
}

func (g *relayAddressGeneratorIPv6) Validate() error {
	if g.Address == "" {
		return errors.New("relayAddressGeneratorIPv6: Address must not be empty")
	}
	return nil
}

// AllocatePacketConn ignores the network argument pion/turn passes (always
// "udp4", even for IPv6 listeners/clients — see type doc) and always opens a
// udp6 socket bound to g.Address instead.
func (g *relayAddressGeneratorIPv6) AllocatePacketConn(_ string, requestedPort int) (net.PacketConn, net.Addr, error) {
	conn, err := net.ListenPacket("udp6", net.JoinHostPort(g.Address, strconv.Itoa(requestedPort)))
	if err != nil {
		return nil, nil, err
	}
	return conn, conn.LocalAddr(), nil
}

// AllocateConn is used only for RFC 6062 TCP relay allocations (a TURN client
// explicitly requesting a TCP relay connection to a peer), which this server
// does not support — matching pion's own RelayAddressGeneratorNone.AllocateConn.
func (g *relayAddressGeneratorIPv6) AllocateConn(string, int) (net.Conn, net.Addr, error) {
	return nil, nil, errors.New("relayAddressGeneratorIPv6: TCP relay (RFC 6062) not supported")
}
