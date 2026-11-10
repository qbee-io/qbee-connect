package components

import (
	"slices"
	"testing"

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

func TestSortedInterfaces(t *testing.T) {
	device := &client.InventoryListItem{}
	device.System.IPv4 = map[string]string{
		"veth1":   "172.17.0.3",
		"wlan0":   "10.0.0.5",
		"docker0": "172.17.0.1",
		"eth0":    "192.168.1.10",
		"empty":   " ",
		"lo":      "127.0.0.1",
	}

	var names []string
	for _, iface := range sortedInterfaces(device) {
		names = append(names, iface.name)
	}

	want := []string{"eth0", "wlan0", "docker0", "lo", "veth1"}
	if !slices.Equal(names, want) {
		t.Errorf("sortedInterfaces() = %v, want %v", names, want)
	}
}
