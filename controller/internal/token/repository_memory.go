package token

import (
	"context"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu       sync.RWMutex
	byID     map[string]EnrollmentToken
	hashToID map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		byID:     map[string]EnrollmentToken{},
		hashToID: map[string]string{},
	}
}

func (r *MemoryRepository) Insert(_ context.Context, in EnrollmentToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[in.ID]; ok {
		return ErrTokenAlreadyExists
	}
	r.byID[in.ID] = in
	r.hashToID[in.WorkspaceID+":"+string(in.TokenHash)] = in.ID
	return nil
}

func (r *MemoryRepository) GetByWorkspaceAndHash(_ context.Context, workspaceID string, tokenHash []byte) (EnrollmentToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.hashToID[workspaceID+":"+string(tokenHash)]
	if !ok {
		return EnrollmentToken{}, ErrTokenNotFound
	}
	record, ok := r.byID[id]
	if !ok {
		return EnrollmentToken{}, ErrTokenNotFound
	}
	return record, nil
}

func (r *MemoryRepository) MarkUsed(_ context.Context, tokenID string, usedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.byID[tokenID]
	if !ok {
		return ErrTokenNotFound
	}
	if record.Used {
		return ErrTokenUsed
	}
	_ = usedAt
	record.Used = true
	r.byID[tokenID] = record
	return nil
}
