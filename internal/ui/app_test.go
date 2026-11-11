package ui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/model"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/testutil"
	"go.qbee.io/connect/internal/ui/components"
	"go.qbee.io/connect/internal/ui/widgets"
)

// newTestApp builds an App around a fyne test app and a client that talks to
// an httptest server serving mux. NewApp is not used because it parses
// command-line flags and reads the real login configuration.
func newTestApp(t *testing.T, mux *http.ServeMux) *App {
	t.Helper()

	// Anything that persists login config must not touch the real home dir.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if mux == nil {
		mux = http.NewServeMux()
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	a := test.NewTempApp(t)
	store, err := service.NewConnectionStore(testutil.NewDirStorage(t))
	if err != nil {
		t.Fatal(err)
	}

	app := &App{
		fyneApp:         a,
		mainWin:         test.NewTempWindow(t, nil),
		cli:             client.New().WithBaseURL(srv.URL).WithAuthToken("token"),
		ctx:             context.Background(),
		store:           store,
		deviceModel:     model.NewDeviceModel(),
		pageInfoLabel:   widget.NewLabel(""),
		accountSwitcher: container.NewHBox(),
	}
	app.deviceList = components.NewDeviceTable(app)
	return app
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Error(err)
	}
}

func unauthorized(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, map[string]any{"code": 401, "error": "unauthorized"})
	}
}

func testUser(hostname string, accounts ...model.Account) *model.User {
	return &model.User{
		User:     model.UserInfo{ID: "u1", Email: "user@example.com", AccountID: "acc-1", HostName: hostname},
		Accounts: accounts,
	}
}

func TestGetDeviceTitle(t *testing.T) {
	item := &client.InventoryListItem{Title: "host.example"}
	named := &client.InventoryListItem{Title: "host.example", Attributes: client.DeviceAttributes{DeviceName: "Gateway"}}

	tests := []struct {
		name string
		user *model.User
		item *client.InventoryListItem
		want string
	}{
		{"logged out uses title", nil, named, "host.example"},
		{"fqhost preference uses title", testUser("fqhost"), named, "host.example"},
		{"device_name preference uses device name", testUser("device_name"), named, "Gateway"},
		{"device_name preference falls back to title", testUser("device_name"), item, "host.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{user: tt.user}
			if got := app.GetDeviceTitle(tt.item); got != tt.want {
				t.Fatalf("GetDeviceTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisplayError_SendsNotification(t *testing.T) {
	app := newTestApp(t, nil)

	test.AssertNotificationSent(t, &fyne.Notification{Title: "Oops", Content: "details"}, func() {
		app.DisplayError("Oops", "details")
	})
}

func TestAuthenticatedRequest(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantLoggedIn bool
	}{
		{"success keeps session", nil, true},
		{"other API error keeps session", client.Error{"code": float64(500)}, true},
		{"non-API error keeps session", io.EOF, true},
		{"401 logs out", client.Error{"code": float64(401)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{user: testUser("fqhost")}
			app.AuthenticatedRequest(func() error { return tt.err })
			if loggedIn := app.user != nil; loggedIn != tt.wantLoggedIn {
				t.Fatalf("logged in = %v, want %v", loggedIn, tt.wantLoggedIn)
			}
		})
	}
}

func TestGetTags(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/tagslist", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scope") != "nodes" || r.URL.Query().Get("format") != "simple" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		writeJSON(t, w, http.StatusOK, []string{"prod", "edge"})
	})
	app := newTestApp(t, mux)

	if got := app.GetTags(); !reflect.DeepEqual(got, []string{"prod", "edge"}) {
		t.Fatalf("GetTags = %v", got)
	}
}

func TestGetTags_UnauthorizedLogsOut(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/tagslist", unauthorized(t))
	app := newTestApp(t, mux)
	app.user = testUser("fqhost")

	if got := app.GetTags(); len(got) != 0 {
		t.Fatalf("GetTags = %v, want empty", got)
	}
	if app.user != nil {
		t.Fatal("a 401 response should log the user out")
	}
}

func TestGetGroups(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/grouptree", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, client.GroupTree{TotalDevices: 3, Tree: client.GroupTreeNode{NodeID: "root", Title: "root"}})
	})
	app := newTestApp(t, mux)

	got := app.GetGroups()
	if got.TotalDevices != 3 || got.Tree.NodeID != "root" {
		t.Fatalf("GetGroups = %+v", got)
	}
}

func TestGetGroups_ErrorReturnsEmptyTree(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/grouptree", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusInternalServerError, map[string]any{"code": 500})
	})
	app := newTestApp(t, mux)

	got := app.GetGroups()
	if got == nil || got.Tree.NodeID != "" {
		t.Fatalf("expected an empty, non-nil tree, got %+v", got)
	}
}

func TestGetUser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/user", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("accounts") != "true" {
			t.Errorf("expected accounts=true, got %s", r.URL.RawQuery)
		}
		writeJSON(t, w, http.StatusOK, testUser("device_name", model.Account{ID: "acc-1", Name: "Main"}))
	})
	app := newTestApp(t, mux)

	user, err := app.GetUser()
	if err != nil {
		t.Fatal(err)
	}
	if user.User.HostName != "device_name" || len(user.Accounts) != 1 || user.Accounts[0].Name != "Main" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestGetUser_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/user", unauthorized(t))
	app := newTestApp(t, mux)

	if _, err := app.GetUser(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestGetDevices_UsesPaginationAndStoresResult(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/inventory", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("offset") != "50" || q.Get("items_per_page") != "25" {
			t.Errorf("unexpected pagination: %s", r.URL.RawQuery)
		}
		writeJSON(t, w, http.StatusOK, client.InventoryListResponse{
			Total: 51,
			Items: []client.InventoryListItem{{NodeID: "d1", Title: "gw-1"}},
		})
	})
	app := newTestApp(t, mux)
	app.deviceModel.CurrentPage = 2
	app.deviceModel.Query.ItemsPerPage = 25

	app.GetDevices()

	data := app.deviceModel.DeviceData
	if data.Total != 51 || len(data.Items) != 1 || data.Items[0].NodeID != "d1" {
		t.Fatalf("unexpected device data: %+v", data)
	}
}

func TestSwitchAccount(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v2/switch-account", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AccountID string `json:"account_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccountID != "acc-2" {
			t.Errorf("unexpected body %+v (%v)", body, err)
		}
		writeJSON(t, w, http.StatusOK, map[string]string{"token": "new-token"})
	})
	mux.HandleFunc("GET /api/v2/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer new-token" {
			t.Errorf("user request should use the new token, got %q", r.Header.Get("Authorization"))
		}
		u := testUser("fqhost")
		u.User.AccountID = "acc-2"
		writeJSON(t, w, http.StatusOK, u)
	})
	app := newTestApp(t, mux)

	if err := app.SwitchAccount("acc-2"); err != nil {
		t.Fatal(err)
	}

	if app.cli.GetAuthToken() != "new-token" {
		t.Errorf("auth token = %q", app.cli.GetAuthToken())
	}
	if app.user == nil || app.user.User.AccountID != "acc-2" {
		t.Errorf("user not refreshed: %+v", app.user)
	}
	cfg, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".qbee", "qbee-cli.json"))
	if err != nil || !strings.Contains(string(cfg), "new-token") {
		t.Errorf("login config not persisted with new token: %s (%v)", cfg, err)
	}
}

func TestSwitchAccount_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/switch-account", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, map[string]any{"code": 403})
	})
	app := newTestApp(t, mux)

	if err := app.SwitchAccount("acc-2"); err == nil {
		t.Fatal("expected an error")
	}
	if app.cli.GetAuthToken() != "token" {
		t.Fatal("auth token must not change when switching fails")
	}
}

func userHandler(t *testing.T, u *model.User) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { writeJSON(t, w, http.StatusOK, u) }
}

func TestSetLoggedIn_SingleAccount(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/user", userHandler(t, testUser("device_name", model.Account{ID: "acc-1", Name: "Main"})))
	app := newTestApp(t, mux)

	if err := app.SetLoggedIn(); err != nil {
		t.Fatal(err)
	}
	if app.user == nil {
		t.Fatal("user should be set")
	}
	if app.deviceModel.Query.SortField != "device_name" {
		t.Errorf("sort field = %q, want the user's hostname preference", app.deviceModel.Query.SortField)
	}
	if len(app.accountSwitcher.Objects) != 0 {
		t.Error("account switcher should be empty with a single account")
	}
}

func TestSetLoggedIn_MultipleAccountsShowsSwitcher(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/user", userHandler(t, testUser("fqhost",
		model.Account{ID: "acc-2", Name: "Zeta"},
		model.Account{ID: "acc-1", Name: "Alpha"},
	)))
	app := newTestApp(t, mux)
	test.NewTempWindow(t, app.accountSwitcher)

	if err := app.SetLoggedIn(); err != nil {
		t.Fatal(err)
	}

	sel, ok := testutil.Find[*widget.Select](app.accountSwitcher, nil)
	if !ok {
		t.Fatal("account selector not found")
	}
	if !reflect.DeepEqual(sel.Options, []string{"Alpha", "Zeta"}) {
		t.Errorf("options = %v, want sorted account names", sel.Options)
	}
	if sel.Selected != "Alpha" {
		t.Errorf("selected = %q, want the current account", sel.Selected)
	}
}

func TestSetLoggedIn_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/user", unauthorized(t))
	app := newTestApp(t, mux)

	if err := app.SetLoggedIn(); err == nil {
		t.Fatal("expected an error")
	}
	if app.user != nil {
		t.Fatal("user must stay unset on failure")
	}
}

func TestSetLoggedOut(t *testing.T) {
	app := &App{user: testUser("fqhost")}
	app.SetLoggedOut()
	if app.user != nil {
		t.Fatal("user should be cleared")
	}
}

func TestRedrawDeviceList(t *testing.T) {
	app := newTestApp(t, nil)
	app.deviceModel.DeviceData = client.InventoryListResponse{
		Total: 25,
		Items: []client.InventoryListItem{{NodeID: "a"}, {NodeID: "b"}, {NodeID: "c"}},
	}
	app.deviceModel.CurrentPage = 1

	app.RedrawDeviceList()

	if n := len(app.deviceModel.FilteredData.Items); n != 3 {
		t.Fatalf("expected all 3 devices, got %d", n)
	}
	if app.pageInfoLabel.Text != "Page 2 / 3" {
		t.Errorf("page label = %q, want %q", app.pageInfoLabel.Text, "Page 2 / 3")
	}

	// Restrict to devices with open tunnels.
	app.store.SetActive("b", &service.DeviceConnections{})
	app.deviceModel.ActiveTunnelsOnly = true
	app.RedrawDeviceList()

	items := app.deviceModel.FilteredData.Items
	if len(items) != 1 || items[0].NodeID != "b" {
		t.Fatalf("expected only device b, got %+v", items)
	}

	// Turning the filter off restores the full page.
	app.deviceModel.ActiveTunnelsOnly = false
	app.RedrawDeviceList()
	if n := len(app.deviceModel.FilteredData.Items); n != 3 {
		t.Fatalf("expected all 3 devices after clearing the filter, got %d", n)
	}
}

func TestLoadDevicesAndRefreshUI_HiddenWindowIsNoop(t *testing.T) {
	app := newTestApp(t, nil)
	before := app.mainWin.Content()

	app.LoadDevicesAndRefreshUI(true)

	if app.mainWin.Content() != before {
		t.Fatal("content should not change while the window is hidden")
	}
}

func TestLoadDevicesAndRefreshUI_LoggedOutShowsLoginButton(t *testing.T) {
	app := newTestApp(t, nil)
	app.mainWindowVisible.Store(true)

	app.RefreshUI()

	if _, ok := testutil.Find(app.mainWin.Content(), func(b *widgets.ButtonPointer) bool {
		return b.Text == "Log In"
	}); !ok {
		t.Fatal("expected a Log In button when logged out")
	}
}

func TestSetSearchQuery_ResetsPage(t *testing.T) {
	app := newTestApp(t, nil)
	app.deviceModel.CurrentPage = 4

	app.SetSearchQuery(client.InventoryListSearch{Title: "gw"})

	if app.deviceModel.Query.Search.Title != "gw" {
		t.Errorf("search = %+v", app.deviceModel.Query.Search)
	}
	if app.deviceModel.CurrentPage != 0 {
		t.Errorf("current page = %d, want 0", app.deviceModel.CurrentPage)
	}
}

func TestPersistLoginConfig(t *testing.T) {
	app := newTestApp(t, nil)

	if err := app.PersistLoginConfig(); err != nil {
		t.Fatal(err)
	}

	var cfg client.LoginConfig
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".qbee", "qbee-cli.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.AuthToken != "token" || cfg.BaseURL != app.cli.GetBaseURL() {
		t.Fatalf("unexpected persisted config: %+v", cfg)
	}
}
