package components

import (
	"net/http"
	"testing"
)

// Only the synchronous error paths of NewLoginDialog are tested. The
// successful path starts background polling goroutines that share
// unsynchronised state (pollData.stopped, the package-level authErr), which
// would need refactoring before it can be tested deterministically.

func TestNewLoginDialog_DeviceAuthorizationFails(t *testing.T) {
	d := newFakeDelegate(t)
	d.withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	})

	NewLoginDialog(d)

	if errs := d.errorTitles(); len(errs) != 1 || errs[0] != "Authentication Error" {
		t.Fatalf("errors = %v, want one Authentication Error", errs)
	}
	if overlays := d.win.Canvas().Overlays().List(); len(overlays) != 0 {
		t.Fatalf("no dialog should be shown on failure, got %d overlays", len(overlays))
	}
}

func TestNewLoginDialog_InvalidVerificationURI(t *testing.T) {
	d := newFakeDelegate(t)
	d.withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"ABCD","verification_uri":"://bad uri","expires_in":60,"interval":1}`))
	})

	NewLoginDialog(d)

	if errs := d.errorTitles(); len(errs) != 1 || errs[0] != "Authentication Error" {
		t.Fatalf("errors = %v, want one Authentication Error", errs)
	}
	if overlays := d.win.Canvas().Overlays().List(); len(overlays) != 0 {
		t.Fatalf("no dialog should be shown on failure, got %d overlays", len(overlays))
	}
}
