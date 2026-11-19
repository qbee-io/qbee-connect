package widgets

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

// Compile-time checks that the custom widgets implement the interfaces the
// desktop driver relies on for tapping and cursor changes.
var (
	_ fyne.Tappable      = (*ButtonPointer)(nil)
	_ desktop.Cursorable = (*ButtonPointer)(nil)
	_ fyne.Tappable      = (*ClickableLabel)(nil)
	_ desktop.Cursorable = (*ClickableLabel)(nil)
)

func TestButtonPointer_Tap(t *testing.T) {
	test.NewTempApp(t)

	tapped := 0
	b := NewButtonPointer("Go", func() { tapped++ })
	test.NewTempWindow(t, b)

	test.Tap(b)
	test.Tap(b)

	if tapped != 2 {
		t.Fatalf("OnTapped called %d times, want 2", tapped)
	}
	if b.Text != "Go" {
		t.Fatalf("Text = %q, want %q", b.Text, "Go")
	}
}

func TestButtonPointer_DisabledDoesNotTap(t *testing.T) {
	test.NewTempApp(t)

	tapped := false
	b := NewButtonPointer("Go", func() { tapped = true })
	test.NewTempWindow(t, b)
	b.Disable()

	test.Tap(b)

	if tapped {
		t.Fatal("disabled button must not fire OnTapped")
	}
}

func TestButtonPointer_Cursor(t *testing.T) {
	test.NewTempApp(t)

	b := NewButtonPointer("", nil)
	if b.Cursor() != desktop.PointerCursor {
		t.Fatalf("default cursor = %v, want PointerCursor", b.Cursor())
	}

	b.SetCursor(desktop.DefaultCursor)
	if b.Cursor() != desktop.DefaultCursor {
		t.Fatalf("cursor after SetCursor = %v, want DefaultCursor", b.Cursor())
	}
}

func TestButtonPointer_NilHandlerTapIsSafe(t *testing.T) {
	test.NewTempApp(t)

	b := NewButtonPointer("x", nil)
	test.NewTempWindow(t, b)
	test.Tap(b) // must not panic
}

func TestClickableLabel_Tap(t *testing.T) {
	test.NewTempApp(t)

	tapped := 0
	l := NewClickableLabel("device-1", func() { tapped++ })
	test.NewTempWindow(t, l)

	test.Tap(l)

	if tapped != 1 {
		t.Fatalf("OnTap called %d times, want 1", tapped)
	}
	if l.Text != "device-1" {
		t.Fatalf("Text = %q", l.Text)
	}
	if l.Cursor() != desktop.PointerCursor {
		t.Fatal("ClickableLabel should show a pointer cursor")
	}
}

func TestClickableLabel_NilHandlerTapIsSafe(t *testing.T) {
	test.NewTempApp(t)

	l := NewClickableLabel("x", nil)
	test.NewTempWindow(t, l)
	test.Tap(l) // must not panic
}

func TestClickableLabel_RendersText(t *testing.T) {
	test.NewTempApp(t)

	l := NewClickableLabel("hello", nil)
	w := test.NewTempWindow(t, l)

	if markup := test.RenderToMarkup(w.Canvas()); !strings.Contains(markup, ">hello<") {
		t.Fatalf("rendered markup does not contain label text:\n%s", markup)
	}
}
