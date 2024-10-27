// Package fake provides a fake client for testing. It implements the [client.Client] interface but saves all entries in
// memory.
package fake

import (
	"sync"

	"github.com/tslnc04/loki-logger/pkg/client"
)

// Client is a fake client that stores all pushed entries in memory.
type Client struct {
	entries []client.Entry
	lock    *sync.RWMutex
}

// New creates a new Client. It is safe to call concurrently from multiple goroutines.
func New() *Client {
	return &Client{
		lock: &sync.RWMutex{},
	}
}

// Push pushes the given entry to the Client. It is safe to call concurrently from multiple goroutines.
func (client *Client) Push(entry client.Entry) error {
	client.lock.Lock()
	defer client.lock.Unlock()

	client.entries = append(client.entries, entry)

	return nil
}

// Entries returns all entries that have been pushed to the Client. Entries should not be modified by the caller. It
// locks the Client for reading, so new entries cannot be pushed to the Client until after Close is called. It is safe
// to call concurrently from multiple goroutines.
func (client *Client) Entries() []client.Entry {
	client.lock.RLock()

	return client.entries
}

// Close unlocks the Client for reading. Note that if there are multiple open read locks due to Entries being called
// multiple times, Close will only release one of them. Each call to Entries should therefore be followed by a call to
// Close. It is safe to call concurrently from multiple goroutines.
func (client *Client) Close() {
	client.lock.RUnlock()
}
