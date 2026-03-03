package workspace

import (
	"context"
	"sync"
)

type MemoryRepository struct {
	mu   sync.RWMutex
	data map[string]Workspace
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{data: map[string]Workspace{}}
}

func (r *MemoryRepository) Insert(_ context.Context, ws Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.data[ws.ID]; ok {
		return ErrAlreadyExist
	}
	r.data[ws.ID] = ws
	return nil
}

func (r *MemoryRepository) GetByID(_ context.Context, id string) (Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ws, ok := r.data[id]
	if !ok {
		return Workspace{}, ErrNotFound
	}
	return ws, nil
}
