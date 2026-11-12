package components

import (
	"slices"
	"strings"
	"testing"

	"go.qbee.io/client"
)

func testInventory() *client.DeviceInventory {
	inv := &client.DeviceInventory{}
	inv.SystemInfo = client.SystemInfo{
		OS:           "linux",
		OSType:       "linux_mips",
		Architecture: "mips",
		Release:      "5.15.167",
		CPUs:         "4",
		BootTime:     "1782821265",
		FQHost:       "OpenWrt",
		Interface:    "phy0-sta0",
		IPAddresses:  "192.168.1.244 100.64.0.1",
		IPv4First:    "192.168.1.244",
		IPv4: map[string]string{
			"br-lan":    "100.64.0.1",
			"phy0-sta0": "192.168.1.244",
		},
		HardwareMAC: map[string]string{
			"br-lan":    "08:bf:b8:e6:bf:88",
			"phy0-sta0": "08:bf:b8:e6:bf:88",
			"wlan0_1":   "08:bf:b8:e6:bf:8c",
			"eth1":      "08:bf:b8:e6:bf:90",
		},
		InterfaceFlags: map[string]string{
			"br-lan":    "up broadcast multicast running",
			"phy0-sta0": "up broadcast multicast running",
			"wlan0.1":   "broadcast multicast",
		},
	}
	return inv
}

func TestInterfaceRows(t *testing.T) {
	rows := interfaceRows(testInventory().SystemInfo)

	var names []string
	for _, r := range rows {
		names = append(names, r.name)
	}
	// physical first, then virtual (br-*); wlan0_1 MAC is merged into wlan0.1
	want := []string{"eth1", "phy0-sta0", "wlan0.1", "br-lan"}
	if !slices.Equal(names, want) {
		t.Fatalf("interfaceRows() names = %v, want %v", names, want)
	}

	byName := map[string]interfaceRow{}
	for _, r := range rows {
		byName[r.name] = r
	}
	if got := byName["wlan0.1"]; got.mac != "08:bf:b8:e6:bf:8c" || got.state != "down" || got.ipv4 != "" {
		t.Errorf("wlan0.1 = %+v", got)
	}
	if got := byName["phy0-sta0"]; got.ipv4 != "192.168.1.244" || got.state != "up, running" {
		t.Errorf("phy0-sta0 = %+v", got)
	}
	if got := byName["eth1"]; got.mac != "08:bf:b8:e6:bf:90" || got.state != "" {
		t.Errorf("eth1 = %+v", got)
	}
}

func TestInterfaceState(t *testing.T) {
	for flags, want := range map[string]string{
		"":                               "",
		"broadcast multicast":            "down",
		"up broadcast multicast":         "up",
		"up broadcast running multicast": "up, running",
	} {
		if got := interfaceState(flags); got != want {
			t.Errorf("interfaceState(%q) = %q, want %q", flags, got, want)
		}
	}
}

func TestSystemFieldsSkipsEmpty(t *testing.T) {
	fields := systemFields(testInventory())

	labels := map[string]string{}
	for _, f := range fields {
		labels[f.label] = f.value
	}

	for _, label := range []string{"CPU hardware", "CPU serial number", "TPM", "Description", "Location"} {
		if _, ok := labels[label]; ok {
			t.Errorf("expected empty field %q to be hidden", label)
		}
	}
	if labels["Architecture"] != "mips" {
		t.Errorf("Architecture = %q", labels["Architecture"])
	}
	if !strings.HasPrefix(labels["Boot time"], "2026-") {
		t.Errorf("Boot time = %q", labels["Boot time"])
	}
}

func TestFormatLocation(t *testing.T) {
	a := client.DeviceAttributes{Address: "Main st 1", Zip: "0150", City: "Oslo", Country: "Norway"}
	if got, want := formatLocation(a), "Main st 1, 0150 Oslo, Norway"; got != want {
		t.Errorf("formatLocation() = %q, want %q", got, want)
	}
	if got := formatLocation(client.DeviceAttributes{City: "Oslo"}); got != "Oslo" {
		t.Errorf("formatLocation(city only) = %q", got)
	}
}

func TestFormatDeviceInfo(t *testing.T) {
	text := formatDeviceInfo(testInventory())
	for _, want := range []string{
		"System\n",
		"  Architecture: mips\n",
		"Network\n",
		"  All IP addresses: 192.168.1.244, 100.64.0.1\n",
		"  wlan0.1: ipv4=- mac=08:bf:b8:e6:bf:8c state=down\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("formatDeviceInfo() missing %q in:\n%s", want, text)
		}
	}
}
