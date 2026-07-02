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
					Address: relayAddress(s.cfg.UDP4Addr, "0.0.0.0"),
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
					Address: relayAddress(s.cfg.UDP6Addr, "::"),
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
					Address: relayAddress(s.cfg.TCP4Addr, "0.0.0.0"),
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
					Address: relayAddress(s.cfg.TCP6Addr, "::"),
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

func (s *Service) ActiveURIs(host string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	hasUDP6 := s.statuses["udp6"].State == "active"
	hasUDP4 := s.statuses["udp4"].State == "active"
	hasTCP6 := s.statuses["tcp6"].State == "active"
	hasTCP4 := s.statuses["tcp4"].State == "active"

	var uris []string
	// IPv6-first preference with IPv4 fallback.
	if hasUDP6 {
		uris = append(uris, fmt.Sprintf("turn:%s?transport=udp", hostForURI(host, true)))
	}
	if hasTCP6 {
		uris = append(uris, fmt.Sprintf("turn:%s?transport=tcp", hostForURI(host, true)))
	}
	if hasUDP4 {
		uris = append(uris, fmt.Sprintf("turn:%s?transport=udp", hostForURI(host, false)))
	}
	if hasTCP4 {
		uris = append(uris, fmt.Sprintf("turn:%s?transport=tcp", hostForURI(host, false)))
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
	return host
}

func relayAddress(addr, fallback string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return fallback
	}
	return host
}
