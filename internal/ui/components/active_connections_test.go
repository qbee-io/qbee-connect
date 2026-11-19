package components

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/testutil"
	"go.qbee.io/connect/internal/ui/widgets"
)

func TestNewActiveConnections_Empty(t *testing.T) {
	d := newFakeDelegate(t)
	view := NewActiveConnections(d)
	test.NewTempWindow(t, view)

	if len(view.Objects) != 1 {
		t.Fatalf("expected a single placeholder, got %d objects", len(view.Objects))
	}
	label, ok := view.Objects[0].(*widget.Label)
	if !ok || label.Text != "No active connections." {
		t.Fatalf("expected placeholder label, got %#v", view.Objects[0])
	}
}

func TestNewActiveConnections_ListsSortedDevices(t *testing.T) {
	d := newFakeDelegate(t)
	d.store.SetActive("dev-b", &service.DeviceConnections{
		Title:   "Bravo",
		Targets: []client.RemoteAccessTarget{target("tcp", "2222", "22")},
	})
	d.store.SetActive("dev-a", &service.DeviceConnections{Title: "Alpha"})

	view := NewActiveConnections(d)
	test.NewTempWindow(t, view)

	if len(view.Objects) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(view.Objects))
	}

	// RichText.String() omits list items, so assert on the rendered markup.
	markup := test.RenderObjectToMarkup(view)
	alpha, bravo := strings.Index(markup, ">Alpha<"), strings.Index(markup, ">Bravo<")
	if alpha < 0 || bravo < 0 || alpha > bravo {
		t.Fatalf("expected Alpha before Bravo, got:\n%s", markup)
	}
	for _, want := range []string{"dev-b", ">tcp<", "127.0.0.1:2222 => 10.0.0.5:22"} {
		if !strings.Contains(markup, want) {
			t.Errorf("expected %q in rendered view:\n%s", want, markup)
		}
	}
}

func TestNewActiveConnections_DisconnectCancelsDevice(t *testing.T) {
	d := newFakeDelegate(t)
	cancelled := map[string]int{}
	for _, id := range []string{"dev-a", "dev-b"} {
		d.store.SetActive(id, &service.DeviceConnections{Title: id, Cancel: func() { cancelled[id]++ }})
	}

	view := NewActiveConnections(d)
	test.NewTempWindow(t, view)

	buttons := testutil.FindAll(view, func(b *widgets.ButtonPointer) bool { return b.Text == "Disconnect" })
	if len(buttons) != 2 {
		t.Fatalf("expected 2 Disconnect buttons, got %d", len(buttons))
	}

	test.Tap(buttons[1]) // rows are sorted, so this is dev-b

	if cancelled["dev-b"] != 1 || cancelled["dev-a"] != 0 {
		t.Fatalf("unexpected cancellations: %v", cancelled)
	}
}

func TestNewActiveConnections_DisconnectWithoutCancelIsSafe(t *testing.T) {
	d := newFakeDelegate(t)
	d.store.SetActive("dev-a", &service.DeviceConnections{Title: "Alpha"})

	view := NewActiveConnections(d)
	test.NewTempWindow(t, view)

	d.store.DeleteActive("dev-a") // connection went away before the tap
	test.Tap(findButton(t, view, "Disconnect"))
}

func TestUpdateActiveConnectionsView_Rebuilds(t *testing.T) {
	d := newFakeDelegate(t)
	d.store.SetActive("dev-a", &service.DeviceConnections{Title: "Alpha"})
	view := NewActiveConnections(d)
	test.NewTempWindow(t, view)

	d.store.DeleteActive("dev-a")
	UpdateActiveConnectionsView(d, view)

	if len(view.Objects) != 1 {
		t.Fatalf("expected placeholder only after rebuild, got %d objects", len(view.Objects))
	}
	if l, ok := view.Objects[0].(*widget.Label); !ok || l.Text != "No active connections." {
		t.Fatalf("expected placeholder label, got %#v", view.Objects[0])
	}
}

func TestUpdateActiveConnectionsView_IgnoresNonContainer(t *testing.T) {
	d := newFakeDelegate(t)
	UpdateActiveConnectionsView(d, widget.NewLabel("not a container")) // must not panic
}
