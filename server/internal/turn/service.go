package turn

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/pion/logging"
	pionturn "github.com/pion/turn/v4"
)

type ListenerStatus struct {
	Key         string
	BindAddress string
	State       string // active | degraded
	Error       string
}

type Service struct {
	cfg         Config
	credentials *CredentialManager

	mu       sync.Mutex
	server   *pionturn.Server
	statuses map[string]ListenerStatus
}

func NewService(cfg Config, credentials *CredentialManager) *Service {
	return &Service{
		cfg:         cfg,
		credentials: credentials,
		statuses:    map[string]ListenerStatus{},
	}
}

func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.Enabled {
		return nil
	}
	if !s.cfg.HasAnyListener() {
		return fmt.Errorf("turn enabled but no listeners configured")
	}

	packetConfigs := make([]pionturn.PacketConnConfig, 0, 2)
	listenerConfigs := make([]pionturn.ListenerConfig, 0, 2)
	closers := make([]func(), 0, 4)

	addStatus := func(key, addr, state, errMsg string) {
		s.statuses[key] = ListenerStatus{
			Key:         key,
			BindAddress: addr,
			State:       state,
			Error:       errMsg,
		}
	}

	if s.cfg.UDP4Addr != "" {
		pc, err := net.ListenPacket("udp4", s.cfg.UDP4Addr)
		if err != nil {
			addStatus("udp4", s.cfg.UDP4Addr, "degraded", err.Error())
		} else {
			packetConfigs = append(packetConfigs, pionturn.PacketConnConfig{
				PacketConn: pc,
				RelayAddressGenerator: &pionturn.RelayAddressGeneratorNone{
					Address: relayAddress(s.cfg.UDP4Addr, "127.0.0.1", s.cfg.PublicIPv4),
				},
			})
			closers = append(closers, func() { _ = pc.Close() })
			addStatus("udp4", s.cfg.UDP4Addr, "active", "")
		}
	}
	if s.cfg.UDP6Addr != "" {
		pc, err := net.ListenPacket("udp6", s.cfg.UDP6Addr)
		if err != nil {
			addStatus("udp6", s.cfg.UDP6Addr, "degraded", err.Error())
		} else {
			packetConfigs = append(packetConfigs, pionturn.PacketConnConfig{
				PacketConn: pc,
				RelayAddressGenerator: &pionturn.RelayAddressGeneratorNone{
					Address: relayAddress(s.cfg.UDP6Addr, "::1", s.cfg.PublicIPv6),
				},
			})
			closers = append(closers, func() { _ = pc.Close() })
			addStatus("udp6", s.cfg.UDP6Addr, "active", "")
		}
	}
	if s.cfg.TCP4Addr != "" {
		ln, err := net.Listen("tcp4", s.cfg.TCP4Addr)
		if err != nil {
			addStatus("tcp4", s.cfg.TCP4Addr, "degraded", err.Error())
		} else {
			listenerConfigs = append(listenerConfigs, pionturn.ListenerConfig{
				Listener: ln,
				RelayAddressGenerator: &pionturn.RelayAddressGeneratorNone{
					Address: relayAddress(s.cfg.TCP4Addr, "127.0.0.1", s.cfg.PublicIPv4),
				},
			})
			closers = append(closers, func() { _ = ln.Close() })
			addStatus("tcp4", s.cfg.TCP4Addr, "active", "")
		}
	}
	if s.cfg.TCP6Addr != "" {
		ln, err := net.Listen("tcp6", s.cfg.TCP6Addr)
		if err != nil {
			addStatus("tcp6", s.cfg.TCP6Addr, "degraded", err.Error())
		} else {
			listenerConfigs = append(listenerConfigs, pionturn.ListenerConfig{
				Listener: ln,
				RelayAddressGenerator: &pionturn.RelayAddressGeneratorNone{
					Address: relayAddress(s.cfg.TCP6Addr, "::1", s.cfg.PublicIPv6),
				},
			})
			closers = append(closers, func() { _ = ln.Close() })
			addStatus("tcp6", s.cfg.TCP6Addr, "active", "")
		}
	}

	if len(packetConfigs) == 0 && len(listenerConfigs) == 0 {
		return fmt.Errorf("no turn listeners active")
	}

	srv, err := pionturn.NewServer(pionturn.ServerConfig{
		Realm:             s.cfg.Realm,
		PacketConnConfigs: packetConfigs,
		ListenerConfigs:   listenerConfigs,
		AuthHandler:       s.credentials.AuthHandler,
		LoggerFactory:     logging.NewDefaultLoggerFactory(),
	})
	if err != nil {
		for _, closeFn := range closers {
			closeFn()
		}
		return err
	}
	s.server = srv

	for _, status := range s.statuses {
		if status.State == "degraded" {
			log.Printf("TURN listener %s degraded (%s): %s", status.Key, status.BindAddress, status.Error)
		} else {
			log.Printf("TURN listener %s active on %s", status.Key, status.BindAddress)
		}
	}

	return nil
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return nil
	}
	err := s.server.Close()
	s.server = nil
	return err
}

func (s *Service) Statuses() []ListenerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ListenerStatus, 0, len(s.statuses))
	for _, st := range s.statuses {
		out = append(out, st)
	}
	return out
}

// isLoopbackHost reports whether host (already stripped of port) is a loopback
// address. Browsers (Firefox, Chrome) refuse TURN allocations to loopback
// hosts as a security measure, so we substitute the real LAN IP instead.
func isLoopbackHost(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Service) ActiveURIs(host string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	hasUDP6 := s.statuses["udp6"].State == "active"
	hasUDP4 := s.statuses["udp4"].State == "active"
	hasTCP6 := s.statuses["tcp6"].State == "active"
	hasTCP4 := s.statuses["tcp4"].State == "active"

	// Browsers block TURN allocation requests to loopback/localhost addresses.
	// When the HTTP Host header resolves to loopback, substitute the machine's
	// real LAN IP. The TURN server binds to 0.0.0.0 so it accepts connections
	// on all interfaces including the LAN IP.
	turnHost4 := host
	turnHost6 := host
	if isLoopbackHost(host) {
		if v4 := firstNonLoopbackIP(false); v4 != "" {
			log.Printf("TURN credentials: loopback host %q → using LAN IP %s for TURN URIs", host, v4)
			turnHost4 = v4
		}
		if v6 := firstNonLoopbackIP(true); v6 != "" {
			turnHost6 = v6
		}
	}

	var uris []string
	// IPv6-first preference with IPv4 fallback.
	if hasUDP6 {
		uris = append(uris, fmt.Sprintf("turn:%s?transport=udp", hostForURI(turnHost6, true)))
	}
	if hasTCP6 {
		uris = append(uris, fmt.Sprintf("turn:%s?transport=tcp", hostForURI(turnHost6, true)))
	}
	if hasUDP4 {
		uris = append(uris, fmt.Sprintf("turn:%s?transport=udp", hostForURI(turnHost4, false)))
	}
	if hasTCP4 {
		uris = append(uris, fmt.Sprintf("turn:%s?transport=tcp", hostForURI(turnHost4, false)))
	}
	return uris
}

func hostForURI(host string, v6 bool) string {
	if host == "" {
		if v6 {
			return "[::1]:3478"
		}
		return "127.0.0.1:3478"
	}
	parsedHost, parsedPort, err := net.SplitHostPort(host)
	if err == nil {
		if v6 && parsedHost == "" {
			parsedHost = "::1"
		}
		if !v6 && parsedHost == "" {
			parsedHost = "127.0.0.1"
		}
		return net.JoinHostPort(parsedHost, parsedPort)
	}
	// Bare IPv6 address (no port, no brackets) — add brackets for a valid TURN URI.
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return "[" + host + "]"
	}
	return host
}

// isPrivateLANIP reports whether ip is an RFC1918 private address.
// We prefer these over CGNAT (100.64.0.0/10, used by Tailscale) and
// other shared/special ranges because RFC1918 IPs work reliably for
// same-machine TURN hairpin relay without going through a WireGuard tunnel.
func isPrivateLANIP(ip net.IP) bool {
	private4 := []struct{ net *net.IPNet }{
		{mustCIDR("10.0.0.0/8")},
		{mustCIDR("172.16.0.0/12")},
		{mustCIDR("192.168.0.0/16")},
	}
	private6 := []struct{ net *net.IPNet }{
		{mustCIDR("fc00::/7")}, // ULA — but exclude Tailscale fd7a:115c::/32
	}
	if ip4 := ip.To4(); ip4 != nil {
		for _, r := range private4 {
			if r.net.Contains(ip4) {
				return true
			}
		}
		return false
	}
	// For IPv6: accept ULA (fc00::/7) but skip the well-known Tailscale ULA prefix.
	tailscale6 := mustCIDR("fd7a:115c:a1e0::/48")
	if tailscale6.Contains(ip) {
		return false
	}
	for _, r := range private6 {
		if r.net.Contains(ip) {
			return true
		}
	}
	return false
}

func mustCIDR(s string) *net.IPNet {
	_, n, _ := net.ParseCIDR(s)
	return n
}

// firstNonLoopbackIP returns the best non-loopback, non-link-local IP from
// active interfaces. v6=true returns an IPv6 address; v6=false returns IPv4.
// RFC1918 private LAN addresses are preferred over CGNAT/Tailscale ranges
// because browsers use these addresses to connect to the TURN server and
// TURN relay hairpin works reliably on normal LAN interfaces.
func firstNonLoopbackIP(v6 bool) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	var lanIP, anyIP string

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			isTarget := (!v6 && ip.To4() != nil) || (v6 && ip.To4() == nil && ip.To16() != nil)
			if !isTarget {
				continue
			}
			s := ip.String()
			if isPrivateLANIP(ip) && lanIP == "" {
				lanIP = s
			}
			if anyIP == "" {
				anyIP = s
			}
		}
	}
	if lanIP != "" {
		return lanIP
	}
	return anyIP
}

func relayAddress(addr, fallback, public string) string {
	if public != "" {
		return public
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return fallback
	}
	if host == "0.0.0.0" || host == "::" || host == "[::]" {
		v6 := fallback == "::1"
		if detected := firstNonLoopbackIP(v6); detected != "" {
			log.Printf("TURN relay: wildcard bind %s, using detected LAN IP %s", addr, detected)
			return detected
		}
		log.Printf("TURN relay: wildcard bind %s, no LAN IP found, falling back to %s", addr, fallback)
		return fallback
	}
	return host
}
