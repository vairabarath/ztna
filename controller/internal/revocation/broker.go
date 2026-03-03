package revocation

import "sync"

type Broker struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[string]map[int]chan Entry
}

func NewBroker() *Broker {
	return &Broker{subscribers: map[string]map[int]chan Entry{}}
}

func (b *Broker) Subscribe(workspaceID string) (<-chan Entry, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID
	if b.subscribers[workspaceID] == nil {
		b.subscribers[workspaceID] = map[int]chan Entry{}
	}
	ch := make(chan Entry, 32)
	b.subscribers[workspaceID][id] = ch

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		wsSubs := b.subscribers[workspaceID]
		if wsSubs == nil {
			return
		}
		if c, ok := wsSubs[id]; ok {
			delete(wsSubs, id)
			close(c)
		}
		if len(wsSubs) == 0 {
			delete(b.subscribers, workspaceID)
		}
	}
	return ch, cancel
}

func (b *Broker) Publish(in Entry) {
	b.mu.RLock()
	subs := b.subscribers[in.WorkspaceID]
	channels := make([]chan Entry, 0, len(subs))
	for _, ch := range subs {
		channels = append(channels, ch)
	}
	b.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- in:
		default:
			// Drop when subscriber is slow to keep publisher non-blocking.
		}
	}
}
