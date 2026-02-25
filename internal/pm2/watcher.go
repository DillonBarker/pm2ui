package pm2

import (
	"sync"
	"time"
)

// WatcherListener is called when the process list is updated.
type WatcherListener func([]Process)

// ErrorListener is called when polling encounters an error.
type ErrorListener func(error)

// Watcher polls pm2 jlist periodically and notifies listeners.
type Watcher struct {
	client         *Client
	interval       time.Duration
	listeners      []WatcherListener
	errorListeners []ErrorListener
	mu             sync.Mutex
	stopCh         chan struct{}
	lastErr        error
}

// NewWatcher creates a new watcher that polls at the given interval.
func NewWatcher(client *Client, interval time.Duration) *Watcher {
	return &Watcher{
		client:   client,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// OnUpdate registers a listener for process list updates.
func (w *Watcher) OnUpdate(fn WatcherListener) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.listeners = append(w.listeners, fn)
}

// OnError registers a listener for polling errors.
func (w *Watcher) OnError(fn ErrorListener) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.errorListeners = append(w.errorListeners, fn)
}

// Start begins polling in a goroutine.
func (w *Watcher) Start() {
	go w.poll()
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

// Refresh triggers an immediate poll.
func (w *Watcher) Refresh() {
	go w.fetch()
}

// LastError returns the last error from polling.
func (w *Watcher) LastError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastErr
}

func (w *Watcher) poll() {
	w.fetch()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.fetch()
		}
	}
}

func (w *Watcher) fetch() {
	procs, err := w.client.List()
	w.mu.Lock()
	w.lastErr = err
	listeners := make([]WatcherListener, len(w.listeners))
	copy(listeners, w.listeners)
	errListeners := make([]ErrorListener, len(w.errorListeners))
	copy(errListeners, w.errorListeners)
	w.mu.Unlock()

	if err != nil {
		for _, fn := range errListeners {
			fn(err)
		}
		return
	}

	for _, fn := range listeners {
		fn(procs)
	}
}
