package service

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"go.qbee.io/client"
	"go.qbee.io/connect/internal/testutil"
)

func newTestStore(t *testing.T) (*ConnectionStore, *testutil.DirStorage) {
	t.Helper()
	s := testutil.NewDirStorage(t)
	store, err := NewConnectionStore(s)
	if err != nil {
		t.Fatalf("NewConnectionStore: %v", err)
	}
	return store, s
}

func tcpTarget(localPort, remotePort string) client.RemoteAccessTarget {
	return client.RemoteAccessTarget{
		Protocol:   "tcp",
		LocalHost:  "127.0.0.1",
		LocalPort:  localPort,
		RemoteHost: "127.0.0.1",
		RemotePort: remotePort,
	}
}

func TestNewConnectionStore_CreatesEmptyFile(t *testing.T) {
	_, s := newTestStore(t)

	got := strings.TrimSpace(s.ReadFile(t, connectionsFileName))
	if got != "{}" {
		t.Fatalf("expected empty JSON object in new connections file, got %q", got)
	}
}

func TestNewConnectionStore_LoadsExistingFile(t *testing.T) {
	s := testutil.NewDirStorage(t)
	s.WriteFile(t, connectionsFileName, `{
		"dev-1": {"title": "Gateway", "targets": [{"Protocol":"tcp","LocalHost":"127.0.0.1","LocalPort":"2222","RemoteHost":"127.0.0.1","RemotePort":"22"}]}
	}`)

	store, err := NewConnectionStore(s)
	if err != nil {
		t.Fatalf("NewConnectionStore: %v", err)
	}

	saved, ok := store.GetSaved("dev-1")
	if !ok {
		t.Fatal("expected dev-1 to be loaded from disk")
	}
	if saved.Title != "Gateway" || len(saved.Targets) != 1 || saved.Targets[0].LocalPort != "2222" {
		t.Fatalf("unexpected saved connection: %+v", saved)
	}
}

func TestNewConnectionStore_CorruptFileReturnsError(t *testing.T) {
	s := testutil.NewDirStorage(t)
	s.WriteFile(t, connectionsFileName, `not json`)

	if _, err := NewConnectionStore(s); err == nil {
		t.Fatal("expected error for corrupt connections file")
	}
}

func TestDeviceConnections_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTitle string
		wantPorts []string
		wantErr   bool
	}{
		{
			name:      "new format",
			input:     `{"title":"Gateway","targets":[{"LocalPort":"2222"},{"LocalPort":"8080"}]}`,
			wantTitle: "Gateway",
			wantPorts: []string{"2222", "8080"},
		},
		{
			name:      "legacy format is a bare target list",
			input:     `[{"LocalPort":"2222"}]`,
			wantTitle: "",
			wantPorts: []string{"2222"},
		},
		{
			name:      "empty object",
			input:     `{}`,
			wantPorts: []string{},
		},
		{
			name:    "invalid",
			input:   `"nope"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dc DeviceConnections
			err := json.Unmarshal([]byte(tt.input), &dc)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dc.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", dc.Title, tt.wantTitle)
			}
			if len(dc.Targets) != len(tt.wantPorts) {
				t.Fatalf("got %d targets, want %d", len(dc.Targets), len(tt.wantPorts))
			}
			for i, p := range tt.wantPorts {
				if dc.Targets[i].LocalPort != p {
					t.Errorf("target[%d].LocalPort = %q, want %q", i, dc.Targets[i].LocalPort, p)
				}
			}
		})
	}
}

func TestDeviceConnections_CancelNotSerialized(t *testing.T) {
	dc := DeviceConnections{Title: "x", Cancel: func() {}}
	b, err := json.Marshal(dc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(b)), "cancel") {
		t.Fatalf("Cancel must not be serialized: %s", b)
	}
}

func TestConnectionStore_ActiveLifecycle(t *testing.T) {
	store, _ := newTestStore(t)

	if _, ok := store.GetActive("dev-1"); ok {
		t.Fatal("expected no active connection initially")
	}

	conn := &DeviceConnections{Title: "Gateway"}
	store.SetActive("dev-1", conn)

	got, ok := store.GetActive("dev-1")
	if !ok || got != conn {
		t.Fatalf("GetActive returned %v, %v", got, ok)
	}

	store.DeleteActive("dev-1")
	if _, ok := store.GetActive("dev-1"); ok {
		t.Fatal("expected active connection to be deleted")
	}
}

func TestConnectionStore_SaveToDiskRoundTrip(t *testing.T) {
	store, s := newTestStore(t)

	conn := &DeviceConnections{
		Title:   "Gateway",
		Targets: []client.RemoteAccessTarget{tcpTarget("2222", "22")},
	}
	if err := store.SaveToDisk("dev-1", conn); err != nil {
		t.Fatalf("SaveToDisk: %v", err)
	}

	// A fresh store over the same storage must see the saved data.
	reloaded, err := NewConnectionStore(s)
	if err != nil {
		t.Fatalf("NewConnectionStore: %v", err)
	}
	got, ok := reloaded.GetSaved("dev-1")
	if !ok {
		t.Fatal("expected dev-1 after reload")
	}
	if got.Title != "Gateway" || len(got.Targets) != 1 || got.Targets[0] != conn.Targets[0] {
		t.Fatalf("unexpected reloaded connection: %+v", got)
	}
}

func TestConnectionStore_SaveToDiskRecreatesMissingFile(t *testing.T) {
	store, s := newTestStore(t)

	if err := s.Remove(connectionsFileName); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveToDisk("dev-1", &DeviceConnections{Title: "Gateway"}); err != nil {
		t.Fatalf("SaveToDisk after file removal: %v", err)
	}
	if !strings.Contains(s.ReadFile(t, connectionsFileName), "Gateway") {
		t.Fatal("expected connections file to be recreated with saved data")
	}
}

func TestConnectionStore_SaveToDiskOverwritesShorterContent(t *testing.T) {
	store, s := newTestStore(t)

	long := &DeviceConnections{Title: strings.Repeat("x", 500)}
	if err := store.SaveToDisk("dev-1", long); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveToDisk("dev-1", &DeviceConnections{Title: "short"}); err != nil {
		t.Fatal(err)
	}

	var decoded map[string]*DeviceConnections
	if err := json.Unmarshal([]byte(s.ReadFile(t, connectionsFileName)), &decoded); err != nil {
		t.Fatalf("file is not valid JSON after overwrite: %v", err)
	}
	if decoded["dev-1"].Title != "short" {
		t.Fatalf("got title %q", decoded["dev-1"].Title)
	}
}

func TestConnectionStore_SnapshotsAreCopies(t *testing.T) {
	store, _ := newTestStore(t)
	store.SetActive("dev-1", &DeviceConnections{})
	if err := store.SaveToDisk("dev-1", &DeviceConnections{}); err != nil {
		t.Fatal(err)
	}

	active := store.SnapshotActive()
	saved := store.SnapshotSaved()
	delete(active, "dev-1")
	delete(saved, "dev-1")

	if _, ok := store.GetActive("dev-1"); !ok {
		t.Fatal("mutating active snapshot affected the store")
	}
	if _, ok := store.GetSaved("dev-1"); !ok {
		t.Fatal("mutating saved snapshot affected the store")
	}
}

func TestConnectionStore_IsPortFree(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.SaveToDisk("dev-1", &DeviceConnections{
		Targets: []client.RemoteAccessTarget{tcpTarget("2222", "22")},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		host     string
		port     int
		protocol string
		want     bool
	}{
		{"same host, port and protocol is taken", "127.0.0.1", 2222, "tcp", false},
		{"different port is free", "127.0.0.1", 2223, "tcp", true},
		{"different protocol is free", "127.0.0.1", 2222, "udp", true},
		{"different host is free", "0.0.0.0", 2222, "tcp", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := store.IsPortFree(tt.host, tt.port, tt.protocol); got != tt.want {
				t.Fatalf("IsPortFree(%s, %d, %s) = %v, want %v", tt.host, tt.port, tt.protocol, got, tt.want)
			}
		})
	}
}

func TestConnectionStore_ConcurrentAccess(t *testing.T) {
	store, _ := newTestStore(t)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i))
			store.SetActive(id, &DeviceConnections{})
			_, _ = store.GetActive(id)
			_ = store.SnapshotActive()
			_ = store.IsPortFree("127.0.0.1", 1000+i, "tcp")
			store.DeleteActive(id)
		}(i)
	}
	wg.Wait()

	if n := len(store.SnapshotActive()); n != 0 {
		t.Fatalf("expected no active connections, got %d", n)
	}
}
