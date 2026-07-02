package turn

import (
	"os"
	"strconv"
	"time"
)

const defaultCredentialTTL = 5 * time.Minute

type Config struct {
	Enabled          bool
	Realm            string
	CredentialsToken string

	UDP4Addr string
	UDP6Addr string
	TCP4Addr string
	TCP6Addr string

	PublicIPv4 string
	PublicIPv6 string

	CredentialTTL time.Duration
}

func LoadConfigFromEnv() Config {
	ttl := defaultCredentialTTL
	if raw := os.Getenv("TURN_CREDENTIAL_TTL_SECONDS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			ttl = time.Duration(parsed) * time.Second
		}
	}

	// Spec clarification requires 5-minute leases.
	ttl = defaultCredentialTTL

	cfg := Config{
		Enabled:          envBool("TURN_ENABLED", false),
		Realm:            envString("TURN_REALM", "androidipv6diag"),
		CredentialsToken: os.Getenv("TURN_CREDENTIALS_TOKEN"),
		UDP4Addr:         envString("TURN_UDP4_ADDR", "0.0.0.0:3478"),
		UDP6Addr:         envString("TURN_UDP6_ADDR", "[::]:3478"),
		TCP4Addr:         envString("TURN_TCP4_ADDR", "0.0.0.0:3478"),
		TCP6Addr:         envString("TURN_TCP6_ADDR", "[::]:3478"),
		PublicIPv4:       os.Getenv("TURN_PUBLIC_IPV4"),
		PublicIPv6:       os.Getenv("TURN_PUBLIC_IPV6"),
		CredentialTTL:    ttl,
	}

	if !cfg.Enabled {
		cfg.UDP4Addr = ""
		cfg.UDP6Addr = ""
		cfg.TCP4Addr = ""
		cfg.TCP6Addr = ""
	}

	return cfg
}

func (c Config) HasAnyListener() bool {
	return c.UDP4Addr != "" || c.UDP6Addr != "" || c.TCP4Addr != "" || c.TCP6Addr != ""
}

func envString(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "TRUE" || v == "yes" || v == "YES"
}
