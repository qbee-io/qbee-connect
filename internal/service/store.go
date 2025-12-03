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
	Targets []client.RemoteAccessTarget
	Cancel  func()
}

// ConnectionStore handles active memory state and persistent disk storage
type ConnectionStore struct {
	activeItems map[string]*DeviceConnections
	savedItems  map[string][]client.RemoteAccessTarget
	mutex       sync.Mutex
	storage     fyne.Storage
}

func NewConnectionStore(s fyne.Storage) *ConnectionStore {
	store := &ConnectionStore{
		activeItems: make(map[string]*DeviceConnections),
		savedItems:  make(map[string][]client.RemoteAccessTarget),
		storage:     s,
	}
	store.LoadFromDisk()
	return store
}

// Active Connection Logic
func (cs *ConnectionStore) GetActive(deviceID string) (*DeviceConnections, bool) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	conn, exists := cs.activeItems[deviceID]
	return conn, exists
}

func (cs *ConnectionStore) SetActive(deviceID string, conn *DeviceConnections) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.activeItems[deviceID] = conn
}

func (cs *ConnectionStore) DeleteActive(deviceID string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	delete(cs.activeItems, deviceID)
}

// Persistence Logic
func (cs *ConnectionStore) GetSaved(deviceID string) ([]client.RemoteAccessTarget, bool) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	t, ok := cs.savedItems[deviceID]
	return t, ok
}

func (cs *ConnectionStore) SaveToDisk(nodeID string, targets []client.RemoteAccessTarget) error {
	cs.mutex.Lock()
	cs.savedItems[nodeID] = targets
	cs.mutex.Unlock() // Unlock before IO

	w, err := cs.storage.Save(connectionsFileName)
	if err != nil {
		return err
	}
	defer w.Close()
	return json.NewEncoder(w).Encode(cs.savedItems)
}

func (cs *ConnectionStore) LoadFromDisk() error {
	list := cs.storage.List()
	if !slices.Contains(list, connectionsFileName) {
		return nil
	}

	r, err := cs.storage.Open(connectionsFileName)
	if err != nil {
		return err
	}
	defer r.Close()

	return json.NewDecoder(r).Decode(&cs.savedItems)
}
