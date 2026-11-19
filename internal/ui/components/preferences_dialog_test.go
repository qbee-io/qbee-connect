package components

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/testutil"
)

// openPreferences shows the preferences dialog and returns it with its SSH user entry.
func openPreferences(t *testing.T, d *fakeDelegate) (*widget.PopUp, *widget.Entry) {
	t.Helper()
	dialog := NewPreferencesDialog(d)
	dialog.Show()

	entry, ok := testutil.Find(dialog, func(e *widget.Entry) bool { return e.PlaceHolder == "e.g. root" })
	if !ok {
		t.Fatal("SSH user name entry not found")
	}
	return dialog, entry
}

func TestPreferencesDialog_ShowsCurrentValue(t *testing.T) {
	d := newFakeDelegate(t)
	if err := d.prefs.Save(service.Preferences{DefaultSSHUserName: "admin"}); err != nil {
		t.Fatal(err)
	}

	dialog, entry := openPreferences(t, d)

	if !dialog.Visible() {
		t.Fatal("dialog should be visible")
	}
	if entry.Text != "admin" {
		t.Fatalf("entry = %q, want the saved user name", entry.Text)
	}
	// Form item labels are rendered as rich text, so check the rendered markup.
	if !strings.Contains(test.RenderToMarkup(d.win.Canvas()), ">Default SSH user name<") {
		t.Error("form label not found")
	}
}

func TestPreferencesDialog_SaveTrimsAndPersists(t *testing.T) {
	d := newFakeDelegate(t)
	dialog, entry := openPreferences(t, d)

	test.Type(entry, "  root  ")
	test.Tap(findButton(t, dialog, "Save"))

	if got := d.prefs.Get().DefaultSSHUserName; got != "root" {
		t.Fatalf("saved user name = %q, want trimmed %q", got, "root")
	}
	if dialog.Visible() {
		t.Error("dialog should close after a successful save")
	}
	if _, noLoad := d.refreshes(); noLoad != 1 {
		t.Errorf("RefreshUINoLoad called %d times, want 1", noLoad)
	}
	if errs := d.errorTitles(); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}

	// The value is persisted, not just kept in memory.
	reloaded, err := service.NewPreferencesStore(d.disk)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get().DefaultSSHUserName; got != "root" {
		t.Fatalf("reloaded user name = %q, want root", got)
	}
}

func TestPreferencesDialog_InvalidValue(t *testing.T) {
	d := newFakeDelegate(t)
	if err := d.prefs.Save(service.Preferences{DefaultSSHUserName: "root"}); err != nil {
		t.Fatal(err)
	}
	dialog, entry := openPreferences(t, d)

	entry.SetText("$(id)")
	if err := entry.Validate(); err == nil {
		t.Error("entry validator should reject the user name")
	}

	test.Tap(findButton(t, dialog, "Save"))

	if errs := d.errorTitles(); len(errs) != 1 || errs[0] != "Preferences" {
		t.Fatalf("errors = %v, want one Preferences error", errs)
	}
	if !dialog.Visible() {
		t.Error("dialog should stay open so the user can fix the value")
	}
	if got := d.prefs.Get().DefaultSSHUserName; got != "root" {
		t.Errorf("stored user name = %q, want unchanged root", got)
	}
	if _, noLoad := d.refreshes(); noLoad != 0 {
		t.Errorf("UI should not refresh after a failed save, got %d refreshes", noLoad)
	}
}

func TestPreferencesDialog_ValidatorAcceptsSurroundingSpaces(t *testing.T) {
	d := newFakeDelegate(t)
	_, entry := openPreferences(t, d)

	for _, v := range []string{"", "root", " root ", "first.last"} {
		entry.SetText(v)
		if err := entry.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", v, err)
		}
	}
}

func TestPreferencesDialog_CancelDiscardsChanges(t *testing.T) {
	d := newFakeDelegate(t)
	dialog, entry := openPreferences(t, d)

	test.Type(entry, "root")
	test.Tap(findButton(t, dialog, "Cancel"))

	if dialog.Visible() {
		t.Error("dialog should close on Cancel")
	}
	if got := d.prefs.Get().DefaultSSHUserName; got != "" {
		t.Errorf("user name = %q, want nothing saved", got)
	}
}
