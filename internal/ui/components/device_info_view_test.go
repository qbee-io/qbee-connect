package components

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/testutil"
	"go.qbee.io/connect/internal/ui/widgets"
)

// inventoryServer serves testInventory() for the device inventory endpoint, or a 500 error
// while fail is set. It counts the inventory requests it receives.
type inventoryServer struct {
	requests atomic.Int32
	fail     atomic.Bool
}

func newInventoryServer(t *testing.T, d *fakeDelegate) *inventoryServer {
	t.Helper()
	s := &inventoryServer{}
	d.withServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/inventory/dev-1" {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		s.requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if s.fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"inventory unavailable"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(testInventory())
	})
	return s
}

// loadSynchronously makes the device info loader run inline, so tests can inspect the result
// as soon as the load call returns.
func loadSynchronously(t *testing.T) {
	t.Helper()
	orig := runInBackground
	runInBackground = func(f func()) { f() }
	t.Cleanup(func() { runInBackground = orig })
}

// deviceTabsInWindow builds the device tabs for dev-1 and shows them in the fake's window.
func deviceTabsInWindow(t *testing.T, d *fakeDelegate) (*container.AppTabs, *container.TabItem) {
	t.Helper()
	loadSynchronously(t)
	mainTab := container.NewTabItem("Connect", widget.NewLabel("main content"))
	tabs := newDeviceTabs(d, &client.InventoryListItem{NodeID: "dev-1"}, mainTab)
	d.win.SetContent(tabs)
	if len(tabs.Items) != 2 || tabs.Items[1].Text != "Device info" {
		t.Fatalf("expected the main tab plus a Device info tab, got %d tabs", len(tabs.Items))
	}
	return tabs, tabs.Items[1]
}

// assertShows fails unless the window renders text containing want.
func assertShows(t *testing.T, d *fakeDelegate, want string) {
	t.Helper()
	if markup := test.RenderToMarkup(d.win.Canvas()); !strings.Contains(markup, want) {
		t.Fatalf("window does not show %s:\n%s", want, markup)
	}
}

// assertLoadError fails unless the info tab shows the load error.
func assertLoadError(t *testing.T, infoTab *container.TabItem) {
	t.Helper()
	// The message wraps per word inside a centered box, so look at the label, not the markup.
	if _, ok := testutil.Find(infoTab.Content, func(l *widget.Label) bool {
		return strings.HasPrefix(l.Text, "Could not load device information:") && strings.Contains(l.Text, "inventory unavailable")
	}); !ok {
		t.Fatal("load error not shown")
	}
}

func TestDeviceTabs_DefaultsToMainTabWithoutLoading(t *testing.T) {
	d := newFakeDelegate(t)
	srv := newInventoryServer(t, d)

	tabs, _ := deviceTabsInWindow(t, d)

	if tabs.Selected() != tabs.Items[0] {
		t.Fatalf("selected tab = %q, want the main tab", tabs.Selected().Text)
	}
	if n := srv.requests.Load(); n != 0 {
		t.Fatalf("inventory should not load until the info tab is shown, got %d requests", n)
	}
}

func TestDeviceTabs_SelectingInfoLoadsInventoryOnce(t *testing.T) {
	d := newFakeDelegate(t)
	srv := newInventoryServer(t, d)
	tabs, infoTab := deviceTabsInWindow(t, d)

	tabs.Select(infoTab)
	assertShows(t, d, ">mips<")

	if got := d.app.Preferences().String(deviceTabPreferenceKey); got != deviceTabInfo {
		t.Errorf("tab preference = %q, want %q", got, deviceTabInfo)
	}

	// Switching away and back must not fetch the inventory again.
	tabs.Select(tabs.Items[0])
	if got := d.app.Preferences().String(deviceTabPreferenceKey); got != deviceTabMain {
		t.Errorf("tab preference = %q, want %q", got, deviceTabMain)
	}
	tabs.Select(infoTab)
	if n := srv.requests.Load(); n != 1 {
		t.Fatalf("inventory requests = %d, want 1", n)
	}
}

func TestDeviceTabs_RemembersInfoTab(t *testing.T) {
	d := newFakeDelegate(t)
	srv := newInventoryServer(t, d)
	d.app.Preferences().SetString(deviceTabPreferenceKey, deviceTabInfo)

	tabs, infoTab := deviceTabsInWindow(t, d)

	if tabs.Selected() != infoTab {
		t.Fatalf("selected tab = %q, want Device info", tabs.Selected().Text)
	}
	assertShows(t, d, ">mips<")
	if n := srv.requests.Load(); n != 1 {
		t.Fatalf("inventory requests = %d, want 1", n)
	}
}

func TestDeviceTabs_RendersInventory(t *testing.T) {
	d := newFakeDelegate(t)
	newInventoryServer(t, d)
	tabs, infoTab := deviceTabsInWindow(t, d)
	d.win.Resize(fyne.NewSize(1200, 900))

	tabs.Select(infoTab)
	assertShows(t, d, ">mips<")

	markup := test.RenderToMarkup(d.win.Canvas())
	for _, want := range []string{
		">System<", ">Network<", // section headings
		">Interface<", ">IPv4<", ">MAC<", ">State<", // interface table headers
		">OpenWrt<",       // hostname
		">192.168.1.244<", // primary IPv4
		">wlan0.1<",       // interface with merged MAC
		">down<",          // its state
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("rendered device info missing %s", want)
		}
	}
}

func TestDeviceTabs_CopyAllAndRefresh(t *testing.T) {
	d := newFakeDelegate(t)
	srv := newInventoryServer(t, d)
	tabs, infoTab := deviceTabsInWindow(t, d)

	tabs.Select(infoTab)
	assertShows(t, d, ">Copy all<")

	test.Tap(findButton(t, infoTab.Content, "Copy all"))
	if got, want := d.app.Clipboard().Content(), formatDeviceInfo(testInventory()); got != want {
		t.Fatalf("clipboard =\n%s\nwant\n%s", got, want)
	}

	test.Tap(findButton(t, infoTab.Content, "Refresh"))
	if n := srv.requests.Load(); n != 2 {
		t.Fatalf("Refresh should fetch the inventory again, got %d requests", n)
	}
	assertShows(t, d, ">Copy all<")
}

func TestDeviceTabs_ErrorShowsRetry(t *testing.T) {
	d := newFakeDelegate(t)
	srv := newInventoryServer(t, d)
	srv.fail.Store(true)
	tabs, infoTab := deviceTabsInWindow(t, d)

	tabs.Select(infoTab)
	assertLoadError(t, infoTab)

	// Re-selecting the tab after a failure tries again.
	tabs.Select(tabs.Items[0])
	tabs.Select(infoTab)
	if n := srv.requests.Load(); n != 2 {
		t.Fatalf("reselecting the tab should retry after a failure, got %d requests", n)
	}
	assertLoadError(t, infoTab)

	// The Retry button loads again and shows the inventory once the API recovers.
	srv.fail.Store(false)
	test.Tap(findButton(t, infoTab.Content, "Retry"))
	assertShows(t, d, ">mips<")
	if n := srv.requests.Load(); n != 3 {
		t.Fatalf("inventory requests = %d, want 3", n)
	}
}

func TestNewDeviceInfoContent_NoInterfaces(t *testing.T) {
	test.NewTempApp(t)
	inv := &client.DeviceInventory{}

	content := newDeviceInfoContent(test.NewTempApp(t), inv, func() {})
	test.NewTempWindow(t, content).Resize(fyne.NewSize(800, 600))

	markup := test.RenderObjectToMarkup(content)
	if strings.Count(markup, ">Nothing reported<") != 2 {
		t.Errorf("expected 'Nothing reported' for both empty sections:\n%s", markup)
	}
	if strings.Contains(markup, ">Interface<") {
		t.Error("interface table should be hidden when no interfaces are reported")
	}
}

func TestNewInterfacesTable_EmptyCellsShowDash(t *testing.T) {
	test.NewTempApp(t)
	table := newInterfacesTable([]interfaceRow{{name: "eth0"}})
	test.NewTempWindow(t, table)

	dashes := testutil.FindAll(table, func(l *widget.Label) bool { return l.Text == "-" })
	if len(dashes) != 3 {
		t.Fatalf("expected '-' for the 3 empty cells, got %d", len(dashes))
	}
}

func TestNewDeviceInfoError_RetryButton(t *testing.T) {
	test.NewTempApp(t)
	retried := 0
	view := newDeviceInfoError(errTest("boom"), func() { retried++ })
	test.NewTempWindow(t, view)

	if _, ok := testutil.Find(view, func(l *widget.Label) bool {
		return strings.Contains(l.Text, "boom")
	}); !ok {
		t.Error("error message not shown")
	}
	btn, ok := testutil.Find(view, func(b *widgets.ButtonPointer) bool { return b.Text == "Retry" })
	if !ok {
		t.Fatal("Retry button not found")
	}
	test.Tap(btn)
	if retried != 1 {
		t.Fatalf("retry called %d times, want 1", retried)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

func TestNetworkFields_HostFallback(t *testing.T) {
	inv := &client.DeviceInventory{}
	inv.SystemInfo.Host = "short-host"

	fields := networkFields(inv)
	if len(fields) != 1 || fields[0].label != "Hostname" || fields[0].value != "short-host" {
		t.Fatalf("networkFields = %+v, want Hostname from Host when FQHost is empty", fields)
	}

	inv.SystemInfo.FQHost = "host.example.com"
	if got := networkFields(inv)[0].value; got != "host.example.com" {
		t.Fatalf("Hostname = %q, want FQHost to take precedence", got)
	}
}

func TestFormatUnixStringAndTimestamp(t *testing.T) {
	for _, s := range []string{"", "  ", "abc", "0", "-5"} {
		if got := formatUnixString(s); got != "" {
			t.Errorf("formatUnixString(%q) = %q, want empty", s, got)
		}
	}
	if got := formatUnixString(" 1782821265 "); !strings.HasPrefix(got, "2026-") {
		t.Errorf("formatUnixString with spaces = %q", got)
	}
	if got := formatTimestamp(0); got != "" {
		t.Errorf("formatTimestamp(0) = %q, want empty", got)
	}
}

func TestAppendField_SkipsBlankValues(t *testing.T) {
	var f []infoField
	f = appendField(f, "a", "  ")
	f = appendField(f, "b", " value ")
	if len(f) != 1 || f[0] != (infoField{"b", "value"}) {
		t.Fatalf("appendField result = %+v", f)
	}
}
