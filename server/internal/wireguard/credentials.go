package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/curve25519"
)

// SessionLease is issued to a client and contains everything needed to set up
// the client-side WireGuard peer.
type SessionLease struct {
	SessionID        string
	ClientPrivateKey [32]byte // returned to client; public key is derived from this
	ClientPublicKey  [32]byte // registered in the WireGuard device
	ClientIP         net.IP
	IssuedAt         time.Time
	ExpiresAt        time.Time
	ClientID         string
}

// PeerConfig is used to sync a session into/out-of the WireGuard device.
type PeerConfig struct {
	PublicKey [32]byte
	IP        net.IP
}

// SessionManager allocates per-session client keypairs and IP addresses.
// It mirrors CredentialManager in internal/turn/credentials.go.
type SessionManager struct {
	mu          sync.Mutex
	cfg         Config
	subnet      *net.IPNet
	sessions    map[string]SessionLease // keyed by SessionID
	usedIPs     map[string]bool         // keyed by IP string
	nextIP      net.IP
}

// NewSessionManager creates a SessionManager for the given Config.
func NewSessionManager(cfg Config) (*SessionManager, error) {
	_, ipnet, err := net.ParseCIDR(cfg.Subnet)
	if err != nil {
		return nil, fmt.Errorf("wireguard: parse subnet: %w", err)
	}
	serverIP, err := cfg.ServerTunnelIP()
	if err != nil {
		return nil, err
	}
	// Start IP allocation after the server tunnel IP.
	startIP := make(net.IP, 4)
	copy(startIP, serverIP.To4())
	startIP[3]++

	return &SessionManager{
		cfg:     cfg,
		subnet:  ipnet,
		sessions: make(map[string]SessionLease),
		usedIPs:  map[string]bool{serverIP.String(): true},
		nextIP:   startIP,
	}, nil
}

// Issue creates a new WireGuard session lease.
func (m *SessionManager) Issue(clientID string) (SessionLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneLocked(time.Now())

	if len(m.sessions) >= m.cfg.MaxSessions {
		return SessionLease{}, fmt.Errorf("wireguard: session capacity exceeded (%d/%d)", len(m.sessions), m.cfg.MaxSessions)
	}

	clientIP, err := m.allocateIPLocked()
	if err != nil {
		return SessionLease{}, err
	}

	privKey, pubKey, err := generateKeypair()
	if err != nil {
		m.usedIPs[clientIP.String()] = false
		return SessionLease{}, fmt.Errorf("wireguard: generate keypair: %w", err)
	}

	now := time.Now().UTC()
	lease := SessionLease{
		SessionID:        uuid.New().String(),
		ClientPrivateKey: privKey,
		ClientPublicKey:  pubKey,
		ClientIP:         clientIP,
		IssuedAt:         now,
		ExpiresAt:        now.Add(m.cfg.SessionTTL),
		ClientID:         clientID,
	}
	m.sessions[lease.SessionID] = lease
	return lease, nil
}

// ActiveCount returns the number of currently valid (non-expired) sessions.
func (m *SessionManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())
	return len(m.sessions)
}

// PrunedPeers prunes expired sessions and returns their PeerConfigs for removal
// from the WireGuard device. Safe to call from background goroutine.
func (m *SessionManager) PrunedPeers() []PeerConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var pruned []PeerConfig
	for id, lease := range m.sessions {
		if now.After(lease.ExpiresAt) {
			pruned = append(pruned, PeerConfig{PublicKey: lease.ClientPublicKey, IP: lease.ClientIP})
			m.usedIPs[lease.ClientIP.String()] = false
			delete(m.sessions, id)
		}
	}
	return pruned
}

// pruneLocked removes expired sessions. Must be called with m.mu held.
func (m *SessionManager) pruneLocked(now time.Time) {
	for id, lease := range m.sessions {
		if now.After(lease.ExpiresAt) {
			m.usedIPs[lease.ClientIP.String()] = false
			delete(m.sessions, id)
		}
	}
}

// allocateIPLocked returns the next free IP from the subnet pool.
// Must be called with m.mu held.
func (m *SessionManager) allocateIPLocked() (net.IP, error) {
	// Scan forward from nextIP; wrap around once if needed.
	_, broadcast := networkBounds(m.subnet)
	scanned := 0
	ip := make(net.IP, 4)
	copy(ip, m.nextIP.To4())

	for {
		if !m.subnet.Contains(ip) || ip.Equal(broadcast) {
			// Wrap to first usable after server tunnel IP.
			serverIP, _ := m.cfg.ServerTunnelIP()
			copy(ip, serverIP.To4())
			ip[3]++
		}
		if !m.usedIPs[ip.String()] {
			result := make(net.IP, 4)
			copy(result, ip)
			m.usedIPs[result.String()] = true
			// Advance nextIP past the allocated one.
			ip[3]++
			copy(m.nextIP, ip)
			return result, nil
		}
		ip[3]++
		scanned++
		if scanned > 256 {
			return nil, fmt.Errorf("wireguard: IP pool exhausted in %s", m.subnet)
		}
	}
}

// networkBounds returns the network address and broadcast address for an IPNet.
func networkBounds(ipnet *net.IPNet) (network, broadcast net.IP) {
	ip := ipnet.IP.To4()
	mask := ipnet.Mask
	network = make(net.IP, 4)
	broadcast = make(net.IP, 4)
	for i := 0; i < 4; i++ {
		network[i] = ip[i] & mask[i]
		broadcast[i] = ip[i] | ^mask[i]
	}
	return
}

// generateKeypair generates a Curve25519 keypair suitable for WireGuard.
func generateKeypair() (private, public [32]byte, err error) {
	if _, err = rand.Read(private[:]); err != nil {
		return
	}
	// Clamp per RFC 7748 / WireGuard spec.
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64
	curve25519.ScalarBaseMult(&public, &private)
	return
}

// KeyToBase64 encodes a WireGuard key to standard base64.
func KeyToBase64(key [32]byte) string {
	return base64.StdEncoding.EncodeToString(key[:])
}

// KeyFromBase64 decodes a standard-base64 WireGuard key.
func KeyFromBase64(s string) ([32]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return [32]byte{}, err
	}
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("wireguard: key must be 32 bytes, got %d", len(b))
	}
	var key [32]byte
	copy(key[:], b)
	return key, nil
}

// KeyToHex encodes a key to lowercase hex (WireGuard UAPI format).
func KeyToHex(key [32]byte) string {
	return fmt.Sprintf("%x", key[:])
}
