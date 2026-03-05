package service

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sync"

	"fyne.io/fyne/v2"
	"go.qbee.io/client"
)

const connectionsFileName = "connections.json"

// DeviceConnections represents active tunnels
type DeviceConnections struct {
	// Targets holds the list of active remote access targets
	Targets []client.RemoteAccessTarget `json:"targets"`
	// Title is an optional title for the device
	Title string `json:"title"`

	// Cancel is the function to cancel active connections
	Cancel func() `json:"-"`
}

// default unmarshalling to handle old formats
func (dc *DeviceConnections) UnmarshalJSON(data []byte) error {
	// attempt to unmarshal new format
	var aux struct {
		Targets []client.RemoteAccessTarget `json:"targets"`
		Title   string                      `json:"title"`
	}
	if err := json.Unmarshal(data, &aux); err == nil {
		dc.Targets = aux.Targets
		dc.Title = aux.Title
		return nil
	}

	// attempt to unmarshal old format
	var oldTargets []client.RemoteAccessTarget
	if err := json.Unmarshal(data, &oldTargets); err != nil {
		return err
	}
	dc.Targets = oldTargets
	return nil
}

// ConnectionStore handles active memory state and persistent disk storage
type ConnectionStore struct {
	activeItems map[string]*DeviceConnections
	savedItems  map[string]*DeviceConnections
	mutex       sync.Mutex
	storage     fyne.Storage
}

// NewConnectionStore initializes a new ConnectionStore
func NewConnectionStore(s fyne.Storage) (*ConnectionStore, error) {
	store := &ConnectionStore{
		activeItems: make(map[string]*DeviceConnections),
		savedItems:  make(map[string]*DeviceConnections),
		storage:     s,
	}

	// Ensure the connections file exists for future saves
	if store.FileExists(connectionsFileName) {
		if err := store.LoadFromDisk(); err != nil {
			return nil, fmt.Errorf("error loading existing connections: %w", err)
		}
		return store, nil
	}

	// Create and save an empty connections file if it doesn't exist. This ensures that we have
	// a valid file to write to later, and can also help catch any storage issues early.
	writer, err := s.Create(connectionsFileName)
	if err != nil {
		return nil, fmt.Errorf("error creating %s: %w", filepath.Join(s.RootURI().Path(), connectionsFileName), err)
	}
	defer func() { _ = writer.Close() }()

	// Save the empty connections map to disk
	if err := json.NewEncoder(writer).Encode(store.savedItems); err != nil {
		return nil, fmt.Errorf("error initializing %s: %w", filepath.Join(s.RootURI().Path(), connectionsFileName), err)
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
func (cs *ConnectionStore) GetSaved(deviceID string) (*DeviceConnections, bool) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	t, ok := cs.savedItems[deviceID]
	return t, ok
}

// SaveToDisk saves connections to disk
func (cs *ConnectionStore) SaveToDisk(nodeID string, conn *DeviceConnections) error {
	cs.mutex.Lock()
	cs.savedItems[nodeID] = conn
	cs.mutex.Unlock() // Unlock before IO

	var w fyne.URIWriteCloser
	var err error

	w, err = cs.storage.Save(connectionsFileName)
	if err != nil {
		return fmt.Errorf("error saving %s: %w", filepath.Join(cs.storage.RootURI().Path(), connectionsFileName), err)
	}
	defer func() { err = w.Close() }()

	err = json.NewEncoder(w).Encode(cs.savedItems)
	return err
}

// LoadFromDisk loads connections from disk
func (cs *ConnectionStore) LoadFromDisk() error {
	var err error
	var r fyne.URIReadCloser

	r, err = cs.storage.Open(connectionsFileName)
	if err != nil {
		return err
	}
	defer func() { err = r.Close() }()

	err = json.NewDecoder(r).Decode(&cs.savedItems)
	return err
}

// FileExists checks if a file exists in storage
func (cs *ConnectionStore) FileExists(name string) bool {
	list := cs.storage.List()
	return slices.Contains(list, name)
}

// SnapshotActive returns a copy of the active items map.
func (cs *ConnectionStore) SnapshotActive() map[string]*DeviceConnections {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	out := make(map[string]*DeviceConnections, len(cs.activeItems))
	maps.Copy(out, cs.activeItems)
	return out
}
