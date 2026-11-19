package components

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/testutil"
	"go.qbee.io/connect/internal/ui/widgets"
)

func target(proto, localPort, remotePort string) client.RemoteAccessTarget {
	return client.RemoteAccessTarget{
		Protocol:   proto,
		LocalHost:  "127.0.0.1",
		LocalPort:  localPort,
		RemoteHost: "10.0.0.5",
		RemotePort: remotePort,
	}
}

func TestRenderMappedPort(t *testing.T) {
	tests := []struct {
		name    string
		target  client.RemoteAccessTarget
		sshUser string
		want    string
	}{
		{"ssh", target("tcp", "2222", "22"), "", "ssh -p 2222 127.0.0.1"},
		{"ssh with user", target("tcp", "2222", "22"), "root", "ssh -p 2222 root@127.0.0.1"},
		{"rdp", target("tcp", "13389", "3389"), "root", "mstsc /v:127.0.0.1:13389"},
		{"http", target("tcp", "8080", "80"), "root", "http://127.0.0.1:8080"},
		{"https", target("tcp", "8443", "443"), "", "https://127.0.0.1:8443"},
		{"vnc falls back to port", target("tcp", "15900", "5900"), "", "15900"},
		{"custom falls back to port", target("tcp", "9000", "9000"), "", "9000"},
		{"udp always shows port", target("udp", "5353", "22"), "root", "5353"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderMappedPort(tt.target, tt.sshUser); got != tt.want {
				t.Fatalf("renderMappedPort = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsCustomTarget(t *testing.T) {
	type tc struct {
		name   string
		target client.RemoteAccessTarget
		want   bool
	}
	tests := []tc{
		{"unknown tcp port", target("tcp", "9000", "9000"), true},
		{"udp is never custom", target("udp", "9000", "9000"), false},
	}
	for name, port := range servicePortMap {
		tests = append(tests, tc{"predefined " + name, target("tcp", "9000", port), false})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCustomTarget(tt.target); got != tt.want {
				t.Fatalf("isCustomTarget = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrowserURL(t *testing.T) {
	for _, tc := range []struct {
		name       string
		remotePort string
		protocol   string
		wantURL    string
		wantOK     bool
	}{
		{"http", "80", "tcp", "http://127.0.0.1:8080", true},
		{"https", "443", "tcp", "https://127.0.0.1:8080", true},
		{"custom uses http", "8443", "tcp", "http://127.0.0.1:8080", true},
		{"custom udp", "8443", "udp", "", false},
		{"ssh", "22", "tcp", "", false},
		{"rdp", "3389", "tcp", "", false},
		{"vnc", "5900", "tcp", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := client.RemoteAccessTarget{
				LocalHost:  "127.0.0.1",
				LocalPort:  "8080",
				RemotePort: tc.remotePort,
				Protocol:   tc.protocol,
			}
			gotURL, gotOK := browserURL(target)
			if gotURL != tc.wantURL || gotOK != tc.wantOK {
				t.Errorf("browserURL() = (%q, %v), want (%q, %v)", gotURL, gotOK, tc.wantURL, tc.wantOK)
			}
		})
	}
}

func TestBrowserURL_IPv6LocalHost(t *testing.T) {
	tg := target("tcp", "9000", "9000")
	tg.LocalHost = "::1"

	got, ok := browserURL(tg)
	if !ok || got != "http://[::1]:9000" {
		t.Fatalf("browserURL = (%q, %v), want bracketed IPv6 host", got, ok)
	}
}

func TestGetOpenURLButton(t *testing.T) {
	tests := []struct {
		name       string
		target     client.RemoteAccessTarget
		wantButton bool
	}{
		{"http", target("tcp", "8080", "80"), true},
		{"https", target("tcp", "8443", "443"), true},
		{"custom tcp", target("tcp", "9000", "9000"), true},
		{"ssh", target("tcp", "2222", "22"), false},
		{"rdp", target("tcp", "13389", "3389"), false},
		{"udp on web port", target("udp", "8080", "80"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newFakeDelegate(t)
			obj := getOpenURLButton(d, tt.target)

			btn, isButton := obj.(*widgets.ButtonPointer)
			if isButton != tt.wantButton {
				t.Fatalf("got %T, want button=%v", obj, tt.wantButton)
			}
			if !isButton {
				return
			}
			test.NewTempWindow(t, btn)
			test.Tap(btn) // test app's OpenURL is a no-op that succeeds
			if errs := d.errorTitles(); len(errs) != 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestDeviceDialogSize(t *testing.T) {
	tests := []struct {
		name string
		win  fyne.Size
		want fyne.Size
	}{
		{"large window uses 90%", fyne.NewSize(2000, 1000), fyne.NewSize(1800, 900)},
		{"medium window uses the minimum", fyne.NewSize(850, 520), fyne.NewSize(800, 500)},
		{"small window is never exceeded", fyne.NewSize(600, 400), fyne.NewSize(600, 400)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test.NewTempApp(t)
			w := test.NewTempWindow(t, nil)
			w.Resize(tt.win)

			if got := deviceDialogSize(w); got != tt.want {
				t.Fatalf("deviceDialogSize(%v) = %v, want %v", w.Canvas().Size(), got, tt.want)
			}
		})
	}
}

func TestNewDeviceInfoDialog_NoActiveConnection(t *testing.T) {
	d := newFakeDelegate(t)
	dialog := NewDeviceInfoDialog(d, &client.InventoryListItem{NodeID: "dev-1", Title: "gw"})
	dialog.Show()

	if _, ok := testutil.Find(dialog, func(l *widget.Label) bool { return l.Text == "title:gw" }); !ok {
		t.Error("header with device title not found")
	}
	if _, ok := testutil.Find(dialog, func(l *widget.Label) bool { return l.Text == "No active connections" }); !ok {
		t.Error("expected a 'No active connections' message")
	}
	if _, ok := testutil.Find(dialog, func(l *widget.Label) bool { return l.Text == "Mapped port" }); ok {
		t.Error("target table should not be shown without an active connection")
	}

	test.Tap(findButton(t, dialog, "Close"))
	if dialog.Visible() {
		t.Fatal("dialog should be hidden after Close")
	}
}

func TestNewDeviceInfoDialog_ListsActiveTargets(t *testing.T) {
	d := newFakeDelegate(t)
	d.store.SetActive("dev-1", &service.DeviceConnections{Targets: []client.RemoteAccessTarget{
		target("tcp", "2222", "22"),
		target("tcp", "8443", "443"),
		target("udp", "5353", "53"),
		target("tcp", "9000", "9000"),
	}})

	dialog := NewDeviceInfoDialog(d, &client.InventoryListItem{NodeID: "dev-1"})
	dialog.Show()

	for _, want := range []string{"Mapped port", "ssh -p 2222 127.0.0.1", "https://127.0.0.1:8443", "5353", "9000"} {
		if _, ok := testutil.Find(dialog, func(l *widget.Label) bool { return l.Text == want }); !ok {
			t.Errorf("label %q not found", want)
		}
	}

	copyButtons := testutil.FindAll(dialog, func(b *widgets.ButtonPointer) bool {
		return b.Icon == theme.ContentCopyIcon()
	})
	if len(copyButtons) != 4 {
		t.Fatalf("expected one copy button per target, got %d", len(copyButtons))
	}
	openButtons := testutil.FindAll(dialog, func(b *widgets.ButtonPointer) bool {
		return b.Icon == theme.MailSendIcon()
	})
	if len(openButtons) != 2 {
		t.Fatalf("expected open-URL buttons for the https and custom targets, got %d", len(openButtons))
	}

	// Copying puts the mapped command on the clipboard.
	test.Tap(copyButtons[0])
	if got := d.app.Clipboard().Content(); got != "ssh -p 2222 127.0.0.1" {
		t.Fatalf("clipboard = %q", got)
	}
	test.Tap(copyButtons[1])
	if got := d.app.Clipboard().Content(); got != "https://127.0.0.1:8443" {
		t.Fatalf("clipboard = %q", got)
	}
}

func TestNewDeviceInfoDialog_UsesDefaultSSHUser(t *testing.T) {
	d := newFakeDelegate(t)
	if err := d.prefs.Save(service.Preferences{DefaultSSHUserName: "admin"}); err != nil {
		t.Fatal(err)
	}
	d.store.SetActive("dev-1", &service.DeviceConnections{Targets: []client.RemoteAccessTarget{
		target("tcp", "2222", "22"),
	}})

	dialog := NewDeviceInfoDialog(d, &client.InventoryListItem{NodeID: "dev-1"})
	dialog.Show()

	if _, ok := testutil.Find(dialog, func(l *widget.Label) bool { return l.Text == "ssh -p 2222 admin@127.0.0.1" }); !ok {
		t.Fatal("ssh command should include the default SSH user")
	}
	copyBtn, _ := testutil.Find(dialog, func(b *widgets.ButtonPointer) bool { return b.Icon == theme.ContentCopyIcon() })
	test.Tap(copyBtn)
	if got := d.app.Clipboard().Content(); got != "ssh -p 2222 admin@127.0.0.1" {
		t.Fatalf("clipboard = %q", got)
	}
}
