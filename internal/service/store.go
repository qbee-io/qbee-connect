package service

import (
	"encoding/json"
	"slices"
	"sync"

	"fyne.io/fyne/v2"
	"go.qbee.io/client"
)

const connectionsFileName = "connections.json"

// DeviceConnections represents active tunnels
type DeviceConnections struct {
	// Targets holds the list of active remote access targets
	Targets []client.RemoteAccessTarget

	// Title is an optional title for the device
	Title string

	// Cancel is the function to cancel active connections
	Cancel func()
}

// ConnectionStore handles active memory state and persistent disk storage
type ConnectionStore struct {
	activeItems map[string]*DeviceConnections
	savedItems  map[string][]client.RemoteAccessTarget
	mutex       sync.Mutex
	storage     fyne.Storage
	listeners   []func()
}

// NewConnectionStore initializes a new ConnectionStore
func NewConnectionStore(s fyne.Storage) (*ConnectionStore, error) {
	store := &ConnectionStore{
		activeItems: make(map[string]*DeviceConnections),
		savedItems:  make(map[string][]client.RemoteAccessTarget),
		storage:     s,
	}

	if err := store.LoadFromDisk(); err != nil {
		return nil, err
	}
	return store, nil
}

// GetActive retrieves active connections from memory
func (cs *ConnectionStore) GetActive(deviceID string) (*DeviceConnections, bool) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	conn, exists := cs.activeItems[deviceID]
	return conn, exists
}

// SetActive sets active connections in memory
func (cs *ConnectionStore) SetActive(deviceID string, conn *DeviceConnections) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.activeItems[deviceID] = conn
	cs.notify()
}

// DeleteActive removes active connections from memory
func (cs *ConnectionStore) DeleteActive(deviceID string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	delete(cs.activeItems, deviceID)
	cs.notify()
}

// GetSaved retrieves saved connections from disk
func (cs *ConnectionStore) GetSaved(deviceID string) ([]client.RemoteAccessTarget, bool) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	t, ok := cs.savedItems[deviceID]
	return t, ok
}

// SaveToDisk saves connections to disk
func (cs *ConnectionStore) SaveToDisk(nodeID string, targets []client.RemoteAccessTarget) error {
	cs.mutex.Lock()
	cs.savedItems[nodeID] = targets
	cs.mutex.Unlock() // Unlock before IO

	var w fyne.URIWriteCloser
	var err error

	w, err = cs.storage.Save(connectionsFileName)
	if err != nil {
		return err
	}
	defer func() { err = w.Close() }()

	err = json.NewEncoder(w).Encode(cs.savedItems)
	return err
}

// LoadFromDisk loads connections from disk
func (cs *ConnectionStore) LoadFromDisk() error {
	var err error
	var r fyne.URIReadCloser

	list := cs.storage.List()

	// if no saved connections, skip loading
	if !slices.Contains(list, connectionsFileName) {
		return nil
	}

	r, err = cs.storage.Open(connectionsFileName)
	if err != nil {
		return err
	}
	defer func() { err = r.Close() }()

	err = json.NewDecoder(r).Decode(&cs.savedItems)
	return err
}

// SnapshotActive returns a copy of the active items map.
func (cs *ConnectionStore) SnapshotActive() map[string]*DeviceConnections {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	out := make(map[string]*DeviceConnections, len(cs.activeItems))
	for k, v := range cs.activeItems {
		out[k] = v
	}
	return out
}

// Subscribe to changes in activeItems. Returns an unsubscribe function.
func (cs *ConnectionStore) Subscribe(l func()) func() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.listeners = append(cs.listeners, l)
	idx := len(cs.listeners) - 1
	return func() {
		cs.mutex.Lock()
		defer cs.mutex.Unlock()
		if idx >= 0 && idx < len(cs.listeners) {
			cs.listeners[idx] = nil
		}
	}
}

// notify all listeners (best-effort, non-blocking)
func (cs *ConnectionStore) notify() {
	for _, l := range cs.listeners {
		if l != nil {
			go l()
		}
	}
}

// Disconnect cancels and removes all active connections for a device id.
func (cs *ConnectionStore) Disconnect(deviceID string) {
	// cancel outside of lock to avoid deadlocks if callbacks involve UI
	if conn, ok := cs.GetActive(deviceID); ok && conn != nil && conn.Cancel != nil {
		conn.Cancel()
	}
	cs.DeleteActive(deviceID)
}
