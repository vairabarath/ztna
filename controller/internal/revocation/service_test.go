package revocation

import (
	"context"
	"testing"
)

type memoryRepo struct {
	entries map[string]Entry
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{entries: map[string]Entry{}}
}

func (r *memoryRepo) Insert(_ context.Context, in Entry) error {
	r.entries[key(in.WorkspaceID, in.CertFingerprint)] = in
	return nil
}

func (r *memoryRepo) Exists(_ context.Context, workspaceID, fingerprint string) (bool, error) {
	_, ok := r.entries[key(workspaceID, fingerprint)]
	return ok, nil
}

func TestRevokeAndSubscribe(t *testing.T) {
	repo := newMemoryRepo()
	svc := NewService(repo, NewCache(), NewBroker())

	ch, cancel := svc.Subscribe("ws-1")
	defer cancel()

	entry := Entry{
		WorkspaceID:     "ws-1",
		DeviceID:        "dev-1",
		CertFingerprint: "abc",
		Reason:          "test",
	}
	if err := svc.Revoke(context.Background(), entry); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	got := <-ch
	if got.CertFingerprint != "abc" {
		t.Fatalf("unexpected fingerprint: %s", got.CertFingerprint)
	}

	revoked, err := svc.IsRevoked(context.Background(), "ws-1", "abc")
	if err != nil {
		t.Fatalf("IsRevoked: %v", err)
	}
	if !revoked {
		t.Fatalf("expected revoked=true")
	}
}
