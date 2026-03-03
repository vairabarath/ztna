package device

import (
	"context"
	"sort"
	"sync"
)

type MemoryRepository struct {
	mu   sync.RWMutex
	data map[string]Device
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{data: map[string]Device{}}
}

func (r *MemoryRepository) Upsert(_ context.Context, d Device) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key(d.WorkspaceID, d.DeviceID)] = d
	return nil
}

func (r *MemoryRepository) GetByID(_ context.Context, workspaceID, deviceID string) (Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.data[key(workspaceID, deviceID)]
	if !ok {
		return Device{}, ErrNotFound
	}
	return d, nil
}

func (r *MemoryRepository) ListByWorkspace(_ context.Context, workspaceID string) ([]Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Device, 0)
	for _, d := range r.data {
		if d.WorkspaceID == workspaceID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastSeenUnixTime == out[j].LastSeenUnixTime {
			return out[i].DeviceID < out[j].DeviceID
		}
		return out[i].LastSeenUnixTime > out[j].LastSeenUnixTime
	})
	return out, nil
}

func (r *MemoryRepository) UpdateStatus(_ context.Context, workspaceID, deviceID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	d, ok := r.data[key(workspaceID, deviceID)]
	if !ok {
		return ErrNotFound
	}
	d.Status = status
	r.data[key(workspaceID, deviceID)] = d
	return nil
}

func key(workspaceID, deviceID string) string {
	return workspaceID + ":" + deviceID
}
