package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sync"

	"fyne.io/fyne/v2"
)

const preferencesFileName = "preferences.json"

// Preferences holds user preferences persisted to disk
type Preferences struct {
	// DefaultSSHUserName is the user name used when rendering ssh commands
	DefaultSSHUserName string `json:"default_ssh_user_name"`
}

// PreferencesStore handles loading and saving user preferences
type PreferencesStore struct {
	prefs   Preferences
	mutex   sync.Mutex
	storage fyne.Storage
}

// NewPreferencesStore initializes a PreferencesStore, loading existing preferences if present
func NewPreferencesStore(s fyne.Storage) (*PreferencesStore, error) {
	ps := &PreferencesStore{storage: s}

	if !slices.Contains(s.List(), preferencesFileName) {
		return ps, nil
	}

	r, err := s.Open(preferencesFileName)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", ps.path(), err)
	}
	defer func() { _ = r.Close() }()

	if err := json.NewDecoder(r).Decode(&ps.prefs); err != nil {
		return nil, fmt.Errorf("error loading %s: %w", ps.path(), err)
	}

	return ps, nil
}

// Get returns a copy of the current preferences
func (ps *PreferencesStore) Get() Preferences {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	return ps.prefs
}

// Save updates the preferences and persists them to disk
func (ps *PreferencesStore) Save(p Preferences) error {
	ps.mutex.Lock()
	ps.prefs = p
	ps.mutex.Unlock()

	// Attempt to save to the existing file; fall back to creating it if it doesn't exist.
	w, err := ps.storage.Save(preferencesFileName)
	if err != nil {
		w, err = ps.storage.Create(preferencesFileName)
	}
	if err != nil {
		return fmt.Errorf("error saving %s: %w", ps.path(), err)
	}
	defer func() { _ = w.Close() }()

	return json.NewEncoder(w).Encode(p)
}

func (ps *PreferencesStore) path() string {
	return filepath.Join(ps.storage.RootURI().Path(), preferencesFileName)
}
