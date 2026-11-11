package components

import (
	"testing"

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
		name   string
		target client.RemoteAccessTarget
		want   string
	}{
		{"ssh", target("tcp", "2222", "22"), "ssh -p 2222 127.0.0.1"},
		{"rdp", target("tcp", "13389", "3389"), "mstsc /v:127.0.0.1:13389"},
		{"http", target("tcp", "8080", "80"), "http://127.0.0.1:8080"},
		{"https", target("tcp", "8443", "443"), "https://127.0.0.1:8443"},
		{"vnc falls back to port", target("tcp", "15900", "5900"), "15900"},
		{"custom falls back to port", target("tcp", "9000", "9000"), "9000"},
		{"udp always shows port", target("udp", "5353", "22"), "5353"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderMappedPort(tt.target); got != tt.want {
				t.Fatalf("renderMappedPort = %q, want %q", got, tt.want)
			}
		})
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
		{"ssh", target("tcp", "2222", "22"), false},
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

func TestNewDeviceInfoDialog_NoActiveConnection(t *testing.T) {
	d := newFakeDelegate(t)
	dialog := NewDeviceInfoDialog(d, &client.InventoryListItem{NodeID: "dev-1", Title: "gw"})
	dialog.Show()

	if _, ok := testutil.Find(dialog, func(l *widget.Label) bool {
		return l.Text == "Target information: title:gw"
	}); !ok {
		t.Error("header with device title not found")
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
	}})

	dialog := NewDeviceInfoDialog(d, &client.InventoryListItem{NodeID: "dev-1"})
	dialog.Show()

	for _, want := range []string{"Mapped port", "ssh -p 2222 127.0.0.1", "https://127.0.0.1:8443", "5353"} {
		if _, ok := testutil.Find(dialog, func(l *widget.Label) bool { return l.Text == want }); !ok {
			t.Errorf("label %q not found", want)
		}
	}

	copyButtons := testutil.FindAll(dialog, func(b *widgets.ButtonPointer) bool {
		return b.Icon == theme.ContentCopyIcon()
	})
	if len(copyButtons) != 3 {
		t.Fatalf("expected one copy button per target, got %d", len(copyButtons))
	}
	openButtons := testutil.FindAll(dialog, func(b *widgets.ButtonPointer) bool {
		return b.Icon == theme.MailSendIcon()
	})
	if len(openButtons) != 1 {
		t.Fatalf("expected an open-URL button only for the https target, got %d", len(openButtons))
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
