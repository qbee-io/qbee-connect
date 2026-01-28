package service

import (
	"encoding/json"
	"maps"
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

	// SSHUserName is the SSH username for the device
	SSHUserName string `json:"ssh_user_name"`

	// Cancel is the function to cancel active connections
	Cancel func() `json:"-"`
}

// default unmarshalling to handle old formats
func (dc *DeviceConnections) UnmarshalJSON(data []byte) error {
	// attempt to unmarshal new format
	var aux struct {
		Targets     []client.RemoteAccessTarget `json:"targets"`
		Title       string                      `json:"title"`
		SSHUserName string                      `json:"ssh_user_name"`
	}
	if err := json.Unmarshal(data, &aux); err == nil {
		dc.Targets = aux.Targets
		dc.Title = aux.Title
		dc.SSHUserName = aux.SSHUserName
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
	maps.Copy(out, cs.activeItems)
	return out
}
