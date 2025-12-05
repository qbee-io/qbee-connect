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

	// Cancel is the function to cancel active connections
	Cancel func()
}

// ConnectionStore handles active memory state and persistent disk storage
type ConnectionStore struct {
	activeItems map[string]*DeviceConnections
	savedItems  map[string][]client.RemoteAccessTarget
	mutex       sync.Mutex
	storage     fyne.Storage
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
}

// DeleteActive removes active connections from memory
func (cs *ConnectionStore) DeleteActive(deviceID string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	delete(cs.activeItems, deviceID)
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
