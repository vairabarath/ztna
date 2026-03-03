package revocation

import (
	"context"
	"fmt"
	"time"
)

type service struct {
	repo   Repository
	cache  *Cache
	broker *Broker
	now    func() time.Time
}

func NewService(repo Repository, cache *Cache, broker *Broker) Service {
	if cache == nil {
		cache = NewCache()
	}
	if broker == nil {
		broker = NewBroker()
	}
	return &service{repo: repo, cache: cache, broker: broker, now: time.Now}
}

func (s *service) IsRevoked(ctx context.Context, workspaceID, fingerprint string) (bool, error) {
	if workspaceID == "" || fingerprint == "" {
		return false, nil
	}
	k := key(workspaceID, fingerprint)
	if s.cache.Contains(k) {
		return true, nil
	}
	if s.repo == nil {
		return false, nil
	}
	exists, err := s.repo.Exists(ctx, workspaceID, fingerprint)
	if err != nil {
		return false, err
	}
	if exists {
		s.cache.Add(k)
	}
	return exists, nil
}

func (s *service) Revoke(ctx context.Context, in Entry) error {
	if in.WorkspaceID == "" || in.CertFingerprint == "" {
		return fmt.Errorf("workspace id and cert fingerprint are required")
	}
	if in.RevokedUnixMilli == 0 {
		in.RevokedUnixMilli = s.now().UTC().UnixMilli()
	}
	if s.repo != nil {
		if err := s.repo.Insert(ctx, in); err != nil {
			return err
		}
	}

	s.cache.Add(key(in.WorkspaceID, in.CertFingerprint))
	s.broker.Publish(in)
	return nil
}

func (s *service) Subscribe(workspaceID string) (<-chan Entry, func()) {
	return s.broker.Subscribe(workspaceID)
}

func key(workspaceID, fingerprint string) string {
	return workspaceID + ":" + fingerprint
}
