package turn

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pion/turn/v4"
)

type CredentialLease struct {
	Username  string
	Password  string
	Realm     string
	IssuedAt  time.Time
	ExpiresAt time.Time
	ClientID  string
}

type CredentialManager struct {
	mu    sync.Mutex
	ttl   time.Duration
	realm string

	leases map[string]CredentialLease
}

func NewCredentialManager(realm string, ttl time.Duration) *CredentialManager {
	if ttl <= 0 {
		ttl = defaultCredentialTTL
	}
	return &CredentialManager{
		ttl:    ttl,
		realm:  realm,
		leases: make(map[string]CredentialLease),
	}
}

func (m *CredentialManager) Issue(clientID string) (CredentialLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneLocked(time.Now())

	password, err := randomSecret()
	if err != nil {
		return CredentialLease{}, err
	}

	now := time.Now().UTC()
	username := fmt.Sprintf("%d:%s", now.Add(m.ttl).Unix(), sanitizeClientID(clientID))
	lease := CredentialLease{
		Username:  username,
		Password:  password,
		Realm:     m.realm,
		IssuedAt:  now,
		ExpiresAt: now.Add(m.ttl),
		ClientID:  clientID,
	}
	m.leases[username] = lease
	return lease, nil
}

func (m *CredentialManager) AuthHandler(username, realm string, _ net.Addr) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.pruneLocked(now)

	lease, ok := m.leases[username]
	if !ok || now.After(lease.ExpiresAt) || realm != lease.Realm {
		return nil, false
	}

	return turn.GenerateAuthKey(username, realm, lease.Password), true
}

func (m *CredentialManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())
	return len(m.leases)
}

func randomSecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func sanitizeClientID(clientID string) string {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "client"
	}
	clientID = strings.ReplaceAll(clientID, " ", "-")
	if len(clientID) > 64 {
		return clientID[:64]
	}
	return clientID
}

func (m *CredentialManager) pruneLocked(now time.Time) {
	for username, lease := range m.leases {
		if now.After(lease.ExpiresAt) {
			delete(m.leases, username)
		}
	}
}
