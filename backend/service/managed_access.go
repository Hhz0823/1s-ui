package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/util/common"
)

const (
	managedPanelAccessTTL  = time.Minute
	maxManagedAccessGrants = 64
)

type ManagedPanelAccessRequest struct {
	Actor string `json:"actor"`
}

type ManagedPanelAccessGrant struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type managedAccessEntry struct {
	username  string
	expiresAt time.Time
}

type ManagedAccessService struct{}

var managedAccessStore = struct {
	sync.Mutex
	grants map[[sha256.Size]byte]managedAccessEntry
}{grants: make(map[[sha256.Size]byte]managedAccessEntry)}

func (s *ManagedAccessService) Issue(request ManagedPanelAccessRequest) (*ManagedPanelAccessGrant, error) {
	if _, err := normalizeRemoteActor(request.Actor); err != nil {
		return nil, err
	}
	user, err := (&UserService{}).GetFirstUser()
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := time.Now().Add(managedPanelAccessTTL)
	hash := sha256.Sum256([]byte(token))

	managedAccessStore.Lock()
	defer managedAccessStore.Unlock()
	pruneManagedAccessLocked(time.Now())
	if len(managedAccessStore.grants) >= maxManagedAccessGrants {
		return nil, common.NewError("too many active managed panel access requests")
	}
	managedAccessStore.grants[hash] = managedAccessEntry{username: user.Username, expiresAt: expiresAt}
	return &ManagedPanelAccessGrant{Token: token, ExpiresAt: expiresAt.Unix()}, nil
}

func (s *ManagedAccessService) Consume(token string) (string, error) {
	if !validAgentCredential(token) {
		return "", common.NewError("invalid or expired managed panel access token")
	}
	hash := sha256.Sum256([]byte(token))
	now := time.Now()
	managedAccessStore.Lock()
	defer managedAccessStore.Unlock()
	entry, found := managedAccessStore.grants[hash]
	delete(managedAccessStore.grants, hash)
	pruneManagedAccessLocked(now)
	if !found || now.After(entry.expiresAt) {
		return "", common.NewError("invalid or expired managed panel access token")
	}
	return entry.username, nil
}

func pruneManagedAccessLocked(now time.Time) {
	for hash, entry := range managedAccessStore.grants {
		if now.After(entry.expiresAt) {
			delete(managedAccessStore.grants, hash)
		}
	}
}
