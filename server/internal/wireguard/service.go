package wireguard

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// Service manages the server-side WireGuard device and UDP echo service.
// It uses wireguard-go's in-process netstack TUN — no root, no kernel modules.
type Service struct {
	cfg            Config
	sessions       *SessionManager
	serverPrivKey  [32]byte
	serverPubKey   [32]byte

	mu      sync.Mutex
	dev     *device.Device
	tnet    *netstack.Net
	cancelFn context.CancelFunc
}

// NewService creates a Service.  Call Start() to bring it up.
func NewService(cfg Config) (*Service, error) {
	priv, pub, err := generateKeypair()
	if err != nil {
		return nil, fmt.Errorf("wireguard service: generate server keypair: %w", err)
	}
	sessions, err := NewSessionManager(cfg)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:           cfg,
		sessions:      sessions,
		serverPrivKey: priv,
		serverPubKey:  pub,
	}, nil
}

// Sessions returns the SessionManager for use by the credential handler.
func (s *Service) Sessions() *SessionManager { return s.sessions }

// ServerPublicKey returns the server's WireGuard public key (base64).
func (s *Service) ServerPublicKey() string { return KeyToBase64(s.serverPubKey) }

// ServerEndpoint returns the WireGuard endpoint address to advertise to clients.
// host is extracted from the HTTP request's Host header.
func (s *Service) ServerEndpoint(host string) string {
	if s.cfg.PublicEndpoint != "" {
		return s.cfg.PublicEndpoint
	}
	// Strip port from host and append WireGuard port.
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	if h == "" || h == "localhost" || isLoopback(h) {
		if detected := firstNonLoopbackIPv4(); detected != "" {
			h = detected
		}
	}
	return fmt.Sprintf("%s:%d", h, s.cfg.Port)
}

// Start brings up the WireGuard device and echo service.
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	serverTunnelIP, err := s.cfg.ServerTunnelIP()
	if err != nil {
		return err
	}
	tunnelAddr := netip.MustParseAddr(serverTunnelIP.String())

	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{tunnelAddr},
		nil,  // no DNS needed for echo service
		1420, // standard WireGuard MTU
	)
	if err != nil {
		return fmt.Errorf("wireguard service: create netstack TUN: %w", err)
	}

	logger := device.NewLogger(device.LogLevelError, "[wg-server] ")
	dev := device.NewDevice(tun, conn.NewDefaultBind(), logger)

	// Apply initial UAPI config (just the server private key and listen port).
	uapi := fmt.Sprintf("private_key=%s\nlisten_port=%d\n",
		KeyToHex(s.serverPrivKey), s.cfg.Port)
	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return fmt.Errorf("wireguard service: IpcSet server config: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return fmt.Errorf("wireguard service: device Up: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.dev = dev
	s.tnet = tnet
	s.cancelFn = cancel

	go s.runEchoService(ctx, serverTunnelIP)
	go s.runPruner(ctx)

	log.Printf("[wireguard] service started on UDP port %d, tunnel IP %s", s.cfg.Port, serverTunnelIP)
	return nil
}

// Stop shuts down the WireGuard device and background goroutines.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
	}
	if s.dev != nil {
		s.dev.Close()
		s.dev = nil
	}
}

// AddPeer registers a client's public key and allowed IP in the WireGuard device.
// Called by the credential handler immediately after issuing a lease.
func (s *Service) AddPeer(pubKey [32]byte, clientIP net.IP) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dev == nil {
		return fmt.Errorf("wireguard service: not running")
	}
	uapi := fmt.Sprintf("public_key=%s\nallowed_ip=%s/32\n",
		KeyToHex(pubKey), clientIP.String())
	return s.dev.IpcSet(uapi)
}

// RemovePeer removes a client peer from the WireGuard device.
func (s *Service) RemovePeer(pubKey [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dev == nil {
		return nil
	}
	uapi := fmt.Sprintf("public_key=%s\nremove=true\n", KeyToHex(pubKey))
	return s.dev.IpcSet(uapi)
}

// runEchoService listens for UDP datagrams on the tunnel IP and echoes them back.
func (s *Service) runEchoService(ctx context.Context, serverTunnelIP net.IP) {
	udpAddr := &net.UDPAddr{IP: serverTunnelIP, Port: s.cfg.EchoPort}
	conn, err := s.tnet.ListenUDP(udpAddr)
	if err != nil {
		log.Printf("[wireguard] echo service: listen %s:%d: %v", serverTunnelIP, s.cfg.EchoPort, err)
		return
	}
	defer conn.Close()
	log.Printf("[wireguard] echo service listening on %s:%d (tunnel)", serverTunnelIP, s.cfg.EchoPort)

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buf := make([]byte, 65535)
	for {
		n, remote, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[wireguard] echo service: read error: %v", err)
			continue
		}
		if _, err := conn.WriteTo(buf[:n], remote); err != nil {
			log.Printf("[wireguard] echo service: write error: %v", err)
		}
	}
}

// runPruner periodically removes expired sessions from the WireGuard device.
func (s *Service) runPruner(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruned := s.sessions.PrunedPeers()
			for _, p := range pruned {
				if err := s.RemovePeer(p.PublicKey); err != nil {
					log.Printf("[wireguard] pruner: remove peer %s: %v", p.IP, err)
				} else {
					log.Printf("[wireguard] pruner: removed expired peer %s", p.IP)
				}
			}
		}
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func firstNonLoopbackIPv4() string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
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
			if ip.To4() != nil {
				return ip.String()
			}
		}
	}
	return ""
}

// isCapacityError checks if an error from Issue() is a capacity error.
func isCapacityError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "capacity exceeded")
}
