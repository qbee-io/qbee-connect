package components

import (
	"testing"
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
