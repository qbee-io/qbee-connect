// Package testutil contains helpers shared by tests across packages.
package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

// DirStorage is a fyne.Storage implementation backed by a private temporary
// directory. The storage returned by fyne's test app shares os.TempDir() and
// does not implement file operations, so it cannot be used to exercise code
// that persists data.
type DirStorage struct {
	root string
}

// NewDirStorage returns a DirStorage rooted in a directory that is removed
// when the test finishes.
func NewDirStorage(t testing.TB) *DirStorage {
	t.Helper()
	return &DirStorage{root: t.TempDir()}
}

// Path returns the absolute path of a file in the storage.
func (s *DirStorage) Path(name string) string {
	return filepath.Join(s.root, name)
}

// RootURI implements fyne.Storage.
func (s *DirStorage) RootURI() fyne.URI {
	return storage.NewFileURI(s.root)
}

// Create implements fyne.Storage. It fails if the file already exists.
func (s *DirStorage) Create(name string) (fyne.URIWriteCloser, error) {
	f, err := os.OpenFile(s.Path(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	return &uriFile{File: f, uri: storage.NewFileURI(f.Name())}, nil
}

// Open implements fyne.Storage.
func (s *DirStorage) Open(name string) (fyne.URIReadCloser, error) {
	f, err := os.Open(s.Path(name))
	if err != nil {
		return nil, err
	}
	return &uriFile{File: f, uri: storage.NewFileURI(f.Name())}, nil
}

// Save implements fyne.Storage. It fails if the file does not exist.
func (s *DirStorage) Save(name string) (fyne.URIWriteCloser, error) {
	f, err := os.OpenFile(s.Path(name), os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	return &uriFile{File: f, uri: storage.NewFileURI(f.Name())}, nil
}

// Remove implements fyne.Storage.
func (s *DirStorage) Remove(name string) error {
	return os.Remove(s.Path(name))
}

// List implements fyne.Storage.
func (s *DirStorage) List() []string {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// WriteFile writes raw content into the storage, for seeding test fixtures.
func (s *DirStorage) WriteFile(t testing.TB, name, content string) {
	t.Helper()
	if err := os.WriteFile(s.Path(name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ReadFile reads raw content from the storage.
func (s *DirStorage) ReadFile(t testing.TB, name string) string {
	t.Helper()
	b, err := os.ReadFile(s.Path(name))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return string(b)
}

type uriFile struct {
	*os.File
	uri fyne.URI
}

func (f *uriFile) URI() fyne.URI { return f.uri }
