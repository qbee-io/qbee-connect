package components

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/model"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/testutil"
)

// fakeDelegate implements every delegate interface used by the components
// package. It records calls so tests can assert on UI-driven behaviour without
// a real App or backend.
type fakeDelegate struct {
	app    fyne.App
	win    fyne.Window
	store  *service.ConnectionStore
	cli    *client.Client
	ctx    context.Context
	model  *model.DeviceModel
	tags   []string
	groups *client.GroupTree

	mu             sync.Mutex
	refreshCount   int
	noLoadCount    int
	errors         []string
	connectShown   []string
	infoShown      []string
	searchQueries  []client.InventoryListSearch
	loggedInCalls  int
	persistedCalls int
}

// newFakeDelegate creates a fake with a temp fyne app, window, store and a
// client whose API calls all fail (no server is listening).
func newFakeDelegate(t *testing.T) *fakeDelegate {
	t.Helper()
	a := test.NewTempApp(t)
	store, err := service.NewConnectionStore(testutil.NewDirStorage(t))
	if err != nil {
		t.Fatal(err)
	}
	w := test.NewTempWindow(t, nil)
	w.Resize(fyne.NewSize(1000, 700))

	return &fakeDelegate{
		app:    a,
		win:    w,
		store:  store,
		cli:    client.New().WithBaseURL("http://127.0.0.1:1"),
		ctx:    context.Background(),
		model:  model.NewDeviceModel(),
		groups: &client.GroupTree{},
	}
}

// withServer points the fake's client at an httptest server using handler.
func (f *fakeDelegate) withServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	f.cli = client.New().WithBaseURL(srv.URL)
}

// connectDelegate, infoDelegate, loginDelegate
func (f *fakeDelegate) GetClient() *client.Client          { return f.cli }
func (f *fakeDelegate) GetContext() context.Context        { return f.ctx }
func (f *fakeDelegate) GetStore() *service.ConnectionStore { return f.store }
func (f *fakeDelegate) GetWindow() fyne.Window             { return f.win }
func (f *fakeDelegate) GetFyneApp() fyne.App               { return f.app }
func (f *fakeDelegate) GetDeviceModel() *model.DeviceModel { return f.model }
func (f *fakeDelegate) GetTags() []string                  { return append([]string(nil), f.tags...) }
func (f *fakeDelegate) GetGroups() *client.GroupTree       { return f.groups }
func (f *fakeDelegate) GetDeviceTitle(item *client.InventoryListItem) string {
	return "title:" + item.Title
}

func (f *fakeDelegate) RefreshUI() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshCount++
}

func (f *fakeDelegate) RefreshUINoLoad() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.noLoadCount++
}

func (f *fakeDelegate) DisplayError(title, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errors = append(f.errors, title)
}

func (f *fakeDelegate) ShowConnectDialog(item *client.InventoryListItem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connectShown = append(f.connectShown, item.NodeID)
}

func (f *fakeDelegate) ShowInfoDialog(item *client.InventoryListItem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.infoShown = append(f.infoShown, item.NodeID)
}

func (f *fakeDelegate) SetSearchQuery(q client.InventoryListSearch) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searchQueries = append(f.searchQueries, q)
}

func (f *fakeDelegate) SetLoggedIn() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loggedInCalls++
	return nil
}

func (f *fakeDelegate) PersistLoginConfig() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.persistedCalls++
	return nil
}

func (f *fakeDelegate) errorTitles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.errors...)
}

func (f *fakeDelegate) refreshes() (load, noLoad int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refreshCount, f.noLoadCount
}

func (f *fakeDelegate) lastQuery(t *testing.T) client.InventoryListSearch {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.searchQueries) == 0 {
		t.Fatal("expected SetSearchQuery to have been called")
	}
	return f.searchQueries[len(f.searchQueries)-1]
}

// eventually polls cond until it returns true or the timeout elapses.
func eventually(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within timeout: %s", msg)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
