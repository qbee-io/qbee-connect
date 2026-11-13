package components

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"go.qbee.io/client"
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
