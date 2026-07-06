package turn

import (
	"net"
	"testing"
)

func ipv6Available(t *testing.T) bool {
	t.Helper()
	probe, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 not available on this machine: %v", err)
		return false
	}
	probe.Close()
	return true
}

func TestRelayAddressGeneratorIPv6_Validate(t *testing.T) {
	g := &relayAddressGeneratorIPv6{}
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for empty Address")
	}
	g.Address = "::"
	if err := g.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRelayAddressGeneratorIPv6_IgnoresBrokenNetworkParam proves the fix for
// pion/turn v4's hardcoded "udp4" bug: even when pion passes "udp4" (which it
// always does, regardless of listener or client address family), the
// generator must still produce a genuine IPv6 (udp6) socket.
func TestRelayAddressGeneratorIPv6_IgnoresBrokenNetworkParam(t *testing.T) {
	if !ipv6Available(t) {
		return
	}
	g := &relayAddressGeneratorIPv6{Address: "::1"}
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// pion/turn v4 always passes "udp4" here — the generator must ignore it.
	conn, addr, err := g.AllocatePacketConn("udp4", 0)
	if err != nil {
		t.Fatalf("AllocatePacketConn: %v", err)
	}
	defer conn.Close()

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		t.Fatalf("relay addr is not *net.UDPAddr: %T", addr)
	}
	if udpAddr.IP.To4() != nil {
		t.Fatalf("relay socket must be IPv6, got IPv4 address %s", udpAddr.IP)
	}
	if !udpAddr.IP.Equal(net.ParseIP("::1")) {
		t.Fatalf("relay socket bound to unexpected address %s, want ::1", udpAddr.IP)
	}
}

// TestRelayAddressGeneratorIPv6_EndToEnd proves the allocated socket is
// genuinely usable as an IPv6 UDP socket (not just labeled as one): a real
// IPv6 datagram sent to it must be received.
func TestRelayAddressGeneratorIPv6_EndToEnd(t *testing.T) {
	if !ipv6Available(t) {
		return
	}
	g := &relayAddressGeneratorIPv6{Address: "::1"}
	conn, addr, err := g.AllocatePacketConn("udp4", 0)
	if err != nil {
		t.Fatalf("AllocatePacketConn: %v", err)
	}
	defer conn.Close()

	sender, err := net.Dial("udp6", addr.String())
	if err != nil {
		t.Fatalf("dial relay socket: %v", err)
	}
	defer sender.Close()

	payload := []byte("hello-ipv6-relay")
	if _, err := sender.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, 256)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if string(buf[:n]) != string(payload) {
		t.Fatalf("got %q, want %q", buf[:n], payload)
	}
}

func TestRelayAddressGeneratorIPv6_AllocateConnNotSupported(t *testing.T) {
	g := &relayAddressGeneratorIPv6{Address: "::"}
	if _, _, err := g.AllocateConn("tcp6", 0); err == nil {
		t.Fatal("expected AllocateConn to return an error")
	}
}
