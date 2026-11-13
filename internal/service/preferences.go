package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"sync"

	"fyne.io/fyne/v2"
)

const preferencesFileName = "preferences.json"

// sshUserNamePattern restricts SSH user names to characters that are safe to paste
// unquoted into a shell command. A leading '-' is rejected so the value can't be
// interpreted as an ssh option.
var sshUserNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.][A-Za-z0-9_.-]*$`)

// ValidateSSHUserName returns an error if name is not a supported SSH user name.
// An empty name is valid and means no user name is set.
func ValidateSSHUserName(name string) error {
	if name == "" || sshUserNamePattern.MatchString(name) {
		return nil
	}
	return fmt.Errorf("invalid SSH user name %q: only letters, digits, '.', '_' and '-' are allowed, and it must not start with '-'", name)
}

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

// Save persists the preferences to disk and, only if that succeeds, makes them current
func (ps *PreferencesStore) Save(p Preferences) error {
	if err := ValidateSSHUserName(p.DefaultSSHUserName); err != nil {
		return err
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Attempt to save to the existing file; fall back to creating it if it doesn't exist.
	w, err := ps.storage.Save(preferencesFileName)
	if err != nil {
		w, err = ps.storage.Create(preferencesFileName)
	}
	if err != nil {
		return fmt.Errorf("error saving %s: %w", ps.path(), err)
	}

	if err := json.NewEncoder(w).Encode(p); err != nil {
		_ = w.Close()
		return fmt.Errorf("error writing %s: %w", ps.path(), err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("error writing %s: %w", ps.path(), err)
	}

	ps.prefs = p
	return nil
}

func (ps *PreferencesStore) path() string {
	return filepath.Join(ps.storage.RootURI().Path(), preferencesFileName)
}
