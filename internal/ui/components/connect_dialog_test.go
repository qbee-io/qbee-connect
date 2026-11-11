package components

import (
	"net"
	"net/http"
	"strconv"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/testutil"
	"go.qbee.io/connect/internal/ui/widgets"
)

// connectRow gives named access to the widgets in a row created by addConnectRow.
type connectRow struct {
	row          *fyne.Container
	localPort    *widget.Entry
	localAddr    *widget.Entry
	portSelector *widget.Select
	remotePort   *widget.Entry
	remoteAddr   *widget.Entry
	protocol     *widget.Select
	remove       *widgets.ButtonPointer
}

func asConnectRow(t *testing.T, obj fyne.CanvasObject) connectRow {
	t.Helper()
	row, ok := obj.(*fyne.Container)
	if !ok || len(row.Objects) != 7 {
		t.Fatalf("expected a 7-column row container, got %T", obj)
	}
	return connectRow{
		row:          row,
		localPort:    row.Objects[0].(*widget.Entry),
		localAddr:    row.Objects[1].(*widget.Entry),
		portSelector: row.Objects[2].(*widget.Select),
		remotePort:   row.Objects[3].(*widget.Entry),
		remoteAddr:   row.Objects[4].(*widget.Entry),
		protocol:     row.Objects[5].(*widget.Select),
		remove:       row.Objects[6].(*widgets.ButtonPointer),
	}
}

// newRow builds a single connect row inside a window and returns it.
func newRow(t *testing.T, prefill *client.RemoteAccessTarget) (connectRow, *fyne.Container) {
	t.Helper()
	test.NewTempApp(t)
	targets := container.NewVBox()
	form := container.NewVBox(targets)
	test.NewTempWindow(t, form)

	addConnectRow(targets, form, prefill)
	if len(targets.Objects) != 1 {
		t.Fatalf("expected 1 row, got %d", len(targets.Objects))
	}
	return asConnectRow(t, targets.Objects[0]), targets
}

func TestAddConnectRow_Defaults(t *testing.T) {
	r, _ := newRow(t, nil)

	if r.localPort.Text != "" {
		t.Errorf("local port = %q, want empty (auto-assign)", r.localPort.Text)
	}
	if r.localAddr.Text != "127.0.0.1" || r.remoteAddr.Text != "127.0.0.1" {
		t.Errorf("addresses = %q / %q, want 127.0.0.1", r.localAddr.Text, r.remoteAddr.Text)
	}
	if r.portSelector.Selected != serviceSSHName {
		t.Errorf("service = %q, want %q", r.portSelector.Selected, serviceSSHName)
	}
	if r.remotePort.Text != serviceSSHPort || !r.remotePort.Disabled() {
		t.Errorf("remote port = %q (disabled=%v), want 22 and disabled", r.remotePort.Text, r.remotePort.Disabled())
	}
	if r.protocol.Selected != "tcp" || !r.protocol.Disabled() {
		t.Errorf("protocol = %q (disabled=%v), want tcp and disabled", r.protocol.Selected, r.protocol.Disabled())
	}
}

func TestAddConnectRow_ServiceSelection(t *testing.T) {
	r, _ := newRow(t, nil)

	for name, port := range servicePortMap {
		r.portSelector.SetSelected(name)
		if r.remotePort.Text != port {
			t.Errorf("%s: remote port = %q, want %q", name, r.remotePort.Text, port)
		}
		if !r.remotePort.Disabled() || !r.protocol.Disabled() {
			t.Errorf("%s: remote port and protocol should be locked", name)
		}
	}

	r.portSelector.SetSelected(serviceCustomName)
	if r.remotePort.Text != "" {
		t.Errorf("custom: remote port = %q, want cleared", r.remotePort.Text)
	}
	if r.remotePort.Disabled() || r.protocol.Disabled() {
		t.Error("custom: remote port and protocol should be editable")
	}

	// The user can now type a custom port.
	test.Type(r.remotePort, "8443")
	if r.remotePort.Text != "8443" {
		t.Errorf("typed remote port = %q", r.remotePort.Text)
	}
}

func TestAddConnectRow_SwitchingBackFromCustomResetsProtocol(t *testing.T) {
	r, _ := newRow(t, nil)

	r.portSelector.SetSelected(serviceCustomName)
	r.protocol.SetSelected("udp")
	r.portSelector.SetSelected(serviceHTTPName)

	if r.protocol.Selected != "tcp" {
		t.Fatalf("protocol = %q, want tcp after selecting a well-known service", r.protocol.Selected)
	}
}

func TestAddConnectRow_Prefill(t *testing.T) {
	tests := []struct {
		name        string
		prefill     client.RemoteAccessTarget
		wantService string
		wantLocked  bool
	}{
		{
			name:        "well-known tcp port selects the service",
			prefill:     client.RemoteAccessTarget{Protocol: "tcp", LocalHost: "0.0.0.0", LocalPort: "2222", RemoteHost: "10.0.0.1", RemotePort: "22"},
			wantService: serviceSSHName,
			wantLocked:  true,
		},
		{
			name:        "well-known port over udp is custom",
			prefill:     client.RemoteAccessTarget{Protocol: "udp", LocalHost: "127.0.0.1", LocalPort: "5353", RemoteHost: "127.0.0.1", RemotePort: "22"},
			wantService: serviceCustomName,
		},
		{
			name:        "unknown port is custom",
			prefill:     client.RemoteAccessTarget{Protocol: "tcp", LocalHost: "127.0.0.1", LocalPort: "9000", RemoteHost: "127.0.0.1", RemotePort: "8080"},
			wantService: serviceCustomName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.prefill
			r, _ := newRow(t, &p)

			if r.portSelector.Selected != tt.wantService {
				t.Errorf("service = %q, want %q", r.portSelector.Selected, tt.wantService)
			}
			if r.localPort.Text != p.LocalPort || r.localAddr.Text != p.LocalHost || r.remoteAddr.Text != p.RemoteHost {
				t.Errorf("prefilled entries = %q %q %q", r.localPort.Text, r.localAddr.Text, r.remoteAddr.Text)
			}
			if r.remotePort.Text != p.RemotePort {
				t.Errorf("remote port = %q, want %q", r.remotePort.Text, p.RemotePort)
			}
			if r.protocol.Selected != p.Protocol {
				t.Errorf("protocol = %q, want %q", r.protocol.Selected, p.Protocol)
			}
			if r.remotePort.Disabled() != tt.wantLocked {
				t.Errorf("remote port disabled = %v, want %v", r.remotePort.Disabled(), tt.wantLocked)
			}
		})
	}
}

func TestAddConnectRow_RemoveButton(t *testing.T) {
	r, targets := newRow(t, nil)
	addConnectRow(targets, targets, nil)
	if len(targets.Objects) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(targets.Objects))
	}

	test.Tap(r.remove)

	if len(targets.Objects) != 1 {
		t.Fatalf("expected 1 row after removal, got %d", len(targets.Objects))
	}
	if targets.Objects[0] == r.row {
		t.Fatal("the tapped row should have been removed")
	}
}

func TestSetTargetEntries_ExplicitPort(t *testing.T) {
	d := newFakeDelegate(t)
	r, _ := newRow(t, nil)
	test.Type(r.localPort, "2222")

	target, err := setTargetEntries(d, r.row, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := client.RemoteAccessTarget{Protocol: "tcp", LocalHost: "127.0.0.1", LocalPort: "2222", RemoteHost: "127.0.0.1", RemotePort: "22"}
	if *target != want {
		t.Fatalf("target = %+v, want %+v", *target, want)
	}
}

func TestSetTargetEntries_AutoAssignsPort(t *testing.T) {
	for _, input := range []string{"", "0"} {
		for _, proto := range []string{"tcp", "udp"} {
			t.Run(proto+"/"+strconv.Quote(input), func(t *testing.T) {
				d := newFakeDelegate(t)
				r, _ := newRow(t, nil)
				r.portSelector.SetSelected(serviceCustomName)
				r.protocol.SetSelected(proto)
				r.remotePort.SetText("53")
				r.localPort.SetText(input)

				target, err := setTargetEntries(d, r.row, nil)
				if err != nil {
					t.Fatal(err)
				}
				port, err := strconv.Atoi(target.LocalPort)
				if err != nil || port <= 0 {
					t.Fatalf("expected an assigned port, got %q", target.LocalPort)
				}
				if target.Protocol != proto {
					t.Fatalf("protocol = %q, want %q", target.Protocol, proto)
				}
			})
		}
	}
}

func TestSetTargetEntries_InvalidAddress(t *testing.T) {
	d := newFakeDelegate(t)
	r, _ := newRow(t, nil)
	r.localAddr.SetText("not a valid host")

	if _, err := setTargetEntries(d, r.row, nil); err == nil {
		t.Fatal("expected an error when no port can be bound")
	}
}

func TestGetFreePort(t *testing.T) {
	for _, proto := range []string{"tcp", "udp"} {
		t.Run(proto, func(t *testing.T) {
			port, err := getFreePort("127.0.0.1", proto)
			if err != nil {
				t.Fatal(err)
			}
			if port <= 0 || port > 65535 {
				t.Fatalf("port %d out of range", port)
			}

			// The port must actually be free again once returned.
			addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			if proto == "udp" {
				c, err := net.ListenPacket("udp", addr)
				if err != nil {
					t.Fatalf("returned UDP port is not bindable: %v", err)
				}
				_ = c.Close()
			} else {
				l, err := net.Listen("tcp", addr)
				if err != nil {
					t.Fatalf("returned TCP port is not bindable: %v", err)
				}
				_ = l.Close()
			}
		})
	}
}

// connectDialogRows returns the target rows in a shown connect dialog.
func connectDialogRows(t *testing.T, dialog *widget.PopUp) []connectRow {
	t.Helper()
	var rows []connectRow
	for _, c := range testutil.FindAll(dialog, func(c *fyne.Container) bool {
		if len(c.Objects) != 7 {
			return false
		}
		_, ok := c.Objects[0].(*widget.Entry)
		return ok
	}) {
		rows = append(rows, asConnectRow(t, c))
	}
	return rows
}

func findButton(t *testing.T, root fyne.CanvasObject, text string) *widgets.ButtonPointer {
	t.Helper()
	b, ok := testutil.Find(root, func(b *widgets.ButtonPointer) bool { return b.Text == text })
	if !ok {
		t.Fatalf("button %q not found", text)
	}
	return b
}

func TestNewConnectDialog_NewDevice(t *testing.T) {
	d := newFakeDelegate(t)
	device := &client.InventoryListItem{NodeID: "dev-1", Title: "gw"}

	dialog := NewConnectDialog(d, device)
	dialog.Show()

	if !dialog.Visible() {
		t.Fatal("dialog should be visible after Show")
	}
	if _, ok := testutil.Find(dialog, func(l *widget.Label) bool {
		return l.Text == "Configure port forwarding: title:gw"
	}); !ok {
		t.Error("dialog header with device title not found")
	}
	if rows := connectDialogRows(t, dialog); len(rows) != 1 {
		t.Fatalf("expected one empty row for a new device, got %d", len(rows))
	}

	test.Tap(findButton(t, dialog, "Add Target"))
	if rows := connectDialogRows(t, dialog); len(rows) != 2 {
		t.Fatalf("expected 2 rows after Add Target, got %d", len(rows))
	}

	test.Tap(findButton(t, dialog, "Cancel"))
	if dialog.Visible() {
		t.Fatal("dialog should be hidden after Cancel")
	}
}

func TestNewConnectDialog_PrefillsSavedTargets(t *testing.T) {
	d := newFakeDelegate(t)
	saved := &service.DeviceConnections{Targets: []client.RemoteAccessTarget{
		{Protocol: "tcp", LocalHost: "127.0.0.1", LocalPort: "2222", RemoteHost: "127.0.0.1", RemotePort: "22"},
		{Protocol: "tcp", LocalHost: "127.0.0.1", LocalPort: "8443", RemoteHost: "127.0.0.1", RemotePort: "443"},
	}}
	if err := d.store.SaveToDisk("dev-1", saved); err != nil {
		t.Fatal(err)
	}

	dialog := NewConnectDialog(d, &client.InventoryListItem{NodeID: "dev-1"})
	dialog.Show()

	rows := connectDialogRows(t, dialog)
	if len(rows) != 2 {
		t.Fatalf("expected 2 prefilled rows, got %d", len(rows))
	}
	if rows[0].localPort.Text != "2222" || rows[0].portSelector.Selected != serviceSSHName {
		t.Errorf("row 0 = %q/%q", rows[0].localPort.Text, rows[0].portSelector.Selected)
	}
	if rows[1].localPort.Text != "8443" || rows[1].portSelector.Selected != serviceHTTPSName {
		t.Errorf("row 1 = %q/%q", rows[1].localPort.Text, rows[1].portSelector.Selected)
	}
}

func TestNewConnectDialog_SaveAndConnect(t *testing.T) {
	d := newFakeDelegate(t)
	// Every API call fails, so the background Connect returns an error.
	d.withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})
	device := &client.InventoryListItem{NodeID: "dev-1", Title: "gw"}

	dialog := NewConnectDialog(d, device)
	dialog.Show()
	rows := connectDialogRows(t, dialog)
	test.Type(rows[0].localPort, "2222")

	test.Tap(findButton(t, dialog, "Save & Connect"))

	if dialog.Visible() {
		t.Error("dialog should close after Save & Connect")
	}

	saved, ok := d.store.GetSaved("dev-1")
	if !ok {
		t.Fatal("targets should be saved to disk")
	}
	if saved.Title != "title:gw" || len(saved.Targets) != 1 || saved.Targets[0].LocalPort != "2222" {
		t.Fatalf("unexpected saved connection: %+v", saved)
	}

	// The failing connection must be reported and cleaned up.
	eventually(t, func() bool { return contains(d.errorTitles(), "Connection Error") }, "connection error displayed")
	eventually(t, func() bool { _, active := d.store.GetActive("dev-1"); return !active }, "active connection removed")
	eventually(t, func() bool { _, noLoad := d.refreshes(); return noLoad >= 2 }, "UI refreshed after save and after disconnect")
}
