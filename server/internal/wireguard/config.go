package wireguard

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	defaultPort        = 51820
	defaultSubnet      = "10.0.0.0/24"
	defaultMaxSessions = 50
	defaultSessionTTL  = 2 * time.Minute
	defaultEchoPort    = 7000
)

// Config holds runtime configuration for the server WireGuard service.
type Config struct {
	Enabled        bool
	Port           int
	Subnet         string
	MaxSessions    int
	SessionTTL     time.Duration
	EchoPort       int
	PublicEndpoint string // host:port advertised in credentials; auto-detected if empty
}

// LoadConfigFromEnv reads WireGuard configuration from environment variables.
func LoadConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("WG_ENABLED"); v == "true" || v == "1" {
		cfg.Enabled = true
	}
	if v := os.Getenv("WG_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("WG_SUBNET"); v != "" {
		cfg.Subnet = v
	}
	if v := os.Getenv("WG_MAX_SESSIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSessions = n
		}
	}
	if v := os.Getenv("WG_SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SessionTTL = d
		}
	}
	if v := os.Getenv("WG_PUBLIC_ENDPOINT"); v != "" {
		cfg.PublicEndpoint = v
	}
	return cfg
}

// DefaultConfig returns a Config populated with safe defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		Port:        defaultPort,
		Subnet:      defaultSubnet,
		MaxSessions: defaultMaxSessions,
		SessionTTL:  defaultSessionTTL,
		EchoPort:    defaultEchoPort,
	}
}

// Validate checks that the config is self-consistent when Enabled is true.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("wireguard: invalid port %d", c.Port)
	}
	if c.MaxSessions <= 0 {
		return fmt.Errorf("wireguard: max_sessions must be > 0")
	}
	if c.SessionTTL <= 0 {
		return fmt.Errorf("wireguard: session_ttl must be > 0")
	}
	_, ipnet, err := net.ParseCIDR(c.Subnet)
	if err != nil {
		return fmt.Errorf("wireguard: invalid subnet %q: %w", c.Subnet, err)
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 32 || ones > 30 {
		return fmt.Errorf("wireguard: subnet %q too small (need at least /30)", c.Subnet)
	}
	return nil
}

// ServerTunnelIP returns the first usable IP in the subnet (used as the server's
// WireGuard tunnel interface address and the echo service destination).
func (c Config) ServerTunnelIP() (net.IP, error) {
	_, ipnet, err := net.ParseCIDR(c.Subnet)
	if err != nil {
		return nil, err
	}
	ip := ipnet.IP.To4()
	if ip == nil {
		return nil, fmt.Errorf("wireguard: only IPv4 subnets supported for tunnel IPs")
	}
	// First usable IP = network address + 1.
	first := make(net.IP, 4)
	copy(first, ip)
	first[3]++
	return first, nil
}
