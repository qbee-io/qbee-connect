package components

import (
	"testing"

	"go.qbee.io/client"
)

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
