package config

import "sync"

// ConfigNotifier broadcasts config-change signals to subscribers.
// Components that need to react to config updates call Subscribe to
// obtain a channel, which receives a signal each time Notify is called.
type ConfigNotifier struct {
	mu        sync.RWMutex
	listeners []chan struct{}
}

// NewConfigNotifier creates a ready-to-use ConfigNotifier.
func NewConfigNotifier() *ConfigNotifier {
	return &ConfigNotifier{}
}

// Subscribe returns a buffered channel that receives a signal every time
// the config is updated via Notify. The caller should select on this
// channel (or range over it) to react to changes.
func (n *ConfigNotifier) Subscribe() <-chan struct{} {
	ch := make(chan struct{}, 1)
	n.mu.Lock()
	n.listeners = append(n.listeners, ch)
	n.mu.Unlock()
	return ch
}

// Notify sends a non-blocking signal to every subscriber.
func (n *ConfigNotifier) Notify() {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, ch := range n.listeners {
		select {
		case ch <- struct{}{}:
		default:
			// listener already has a pending signal; skip to avoid blocking
		}
	}
}
