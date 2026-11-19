package components

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"go.qbee.io/connect/internal/config"
)

func menuItem(t *testing.T, menu *fyne.MainMenu, label string) *fyne.MenuItem {
	t.Helper()
	for _, item := range menu.Items[0].Items {
		if item.Label == label {
			return item
		}
	}
	t.Fatalf("menu item %q not found", label)
	return nil
}

func TestMakeMenu_Structure(t *testing.T) {
	d := newFakeDelegate(t)

	menu := MakeMenu(d)
	if len(menu.Items) != 1 || menu.Items[0].Label != "File" {
		t.Fatalf("expected a single File menu, got %+v", menu.Items)
	}

	var labels []string
	for _, item := range menu.Items[0].Items {
		labels = append(labels, item.Label)
		if item.Action == nil {
			t.Errorf("menu item %q has no action", item.Label)
		}
	}
	if strings.Join(labels, ",") != "About,Appearance,Preferences" {
		t.Fatalf("menu items = %v", labels)
	}
}

func TestMakeMenu_WindowActions(t *testing.T) {
	for _, label := range []string{"About", "Appearance"} {
		t.Run(label, func(t *testing.T) {
			d := newFakeDelegate(t)
			menuItem(t, MakeMenu(d), label).Action() // must not panic under the test driver
		})
	}
}

func TestMakeMenu_PreferencesOpensDialog(t *testing.T) {
	d := newFakeDelegate(t)

	menuItem(t, MakeMenu(d), "Preferences").Action()

	if overlays := d.win.Canvas().Overlays().List(); len(overlays) != 1 {
		t.Fatalf("expected the preferences dialog as an overlay, got %d overlays", len(overlays))
	}
}

func TestNewAbout_Content(t *testing.T) {
	test.NewTempApp(t) // test app metadata has no version or build

	markup := test.RenderObjectToMarkup(newAbout())

	for _, want := range []string{
		config.DefaultVersion,
		"-1",
		strconv.Itoa(time.Now().Year()),
		docsURL,
		supportEmail,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("about content missing %q:\n%s", want, markup)
		}
	}
}
