package turn

import (
	"testing"
	"time"
)

func TestCredentialIssueAndAuth(t *testing.T) {
	mgr := NewCredentialManager("test-realm", 5*time.Minute)
	lease, err := mgr.Issue("test-client")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	key, ok := mgr.AuthHandler(lease.Username, lease.Realm, nil)
	if !ok {
		t.Fatalf("AuthHandler() expected ok")
	}
	if len(key) == 0 {
		t.Fatalf("AuthHandler() returned empty key")
	}
}

func TestCredentialExpires(t *testing.T) {
	mgr := NewCredentialManager("test-realm", 50*time.Millisecond)
	lease, err := mgr.Issue("test-client")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	time.Sleep(70 * time.Millisecond)
	_, ok := mgr.AuthHandler(lease.Username, lease.Realm, nil)
	if ok {
		t.Fatalf("AuthHandler() expected expired lease to fail")
	}
}
