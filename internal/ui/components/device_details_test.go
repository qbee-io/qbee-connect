package components

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/testutil"
)

func TestIsVirtualInterface(t *testing.T) {
	for name, want := range map[string]bool{
		"lo": true, "docker0": true, "br-1a2b3c": true, "veth9f8e7d": true, "virbr0": true,
		"eth0": false, "wlan0": false, "qbee0": false, "enp3s0": false, "lowpan0": false,
	} {
		if got := isVirtualInterface(name); got != want {
			t.Errorf("isVirtualInterface(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestDeviceDetailsLongTitleDoesNotWidenHeader(t *testing.T) {
	test.NewTempApp(t)
	device := &client.InventoryListItem{}

	short := newDeviceDetails("dev", device).MinSize().Width
	long := newDeviceDetails(strings.Repeat("very-long-device-name-", 20), device).MinSize().Width

	if long > short {
		t.Errorf("header min width grew with title length: %v > %v", long, short)
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{-time.Second, "just now"},
		{0, "0s ago"},
		{59 * time.Second, "59s ago"},
		{time.Minute, "1m ago"},
		{59 * time.Minute, "59m ago"},
		{time.Hour, "1h ago"},
		{23 * time.Hour, "23h ago"},
		{24 * time.Hour, "1d ago"},
		{10 * 24 * time.Hour, "10d ago"},
	}
	for _, tt := range tests {
		if got := formatAge(tt.d); got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatLastReported(t *testing.T) {
	for _, ts := range []int64{0, -1} {
		if got := formatLastReported(ts); got != "never" {
			t.Errorf("formatLastReported(%d) = %q, want never", ts, got)
		}
	}

	ts := time.Now().Add(-90 * time.Second).Unix()
	want := time.Unix(ts, 0).Local().Format("2006-01-02 15:04:05") + " (1m ago)"

	if got := formatLastReported(ts); got != want {
		t.Errorf("formatLastReported(seconds) = %q, want %q", got, want)
	}
	if got := formatLastReported(ts * 1000); got != want {
		t.Errorf("formatLastReported(milliseconds) = %q, want %q", got, want)
	}
}

func TestNewDeviceDetails_Content(t *testing.T) {
	test.NewTempApp(t)
	device := &client.InventoryListItem{Status: StatusOnline}

	header := newDeviceDetails("gateway-1", device)
	test.NewTempWindow(t, header)

	title, ok := testutil.Find(header, func(l *widget.Label) bool { return l.Text == "gateway-1" })
	if !ok {
		t.Fatal("title label not found")
	}
	if !title.TextStyle.Bold || title.Truncation != fyne.TextTruncateEllipsis {
		t.Error("title should be bold and truncated with an ellipsis")
	}
	if _, ok := testutil.Find(header, func(l *widget.Label) bool { return l.Text == "Last connect: never" }); !ok {
		t.Error("expected 'Last connect: never' for a device that never reported")
	}

	icon, ok := testutil.Find[*widget.Icon](header, nil)
	if !ok {
		t.Fatal("status icon not found")
	}
	if res, ok := icon.Resource.(*theme.ThemedResource); !ok || res.ColorName != theme.ColorNameSuccess {
		t.Errorf("status icon = %#v, want the online (success) color", icon.Resource)
	}
}
