package main

import (
	"sync"
)

type connectionsMap struct {
	items map[string]*deviceConnections
	mutex sync.Mutex
}

func newConnectionsMap() *connectionsMap {
	return &connectionsMap{
		items: make(map[string]*deviceConnections),
	}
}

func (cm *connectionsMap) get(deviceID string) (*deviceConnections, bool) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	conn, exists := cm.items[deviceID]
	return conn, exists
}

func (cm *connectionsMap) set(deviceID string, conn *deviceConnections) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.items[deviceID] = conn
}

func (cm *connectionsMap) delete(deviceID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.items, deviceID)
}
