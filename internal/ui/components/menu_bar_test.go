package components

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"go.qbee.io/connect/internal/config"
)

func TestMakeMenu_Structure(t *testing.T) {
	test.NewTempApp(t)

	menu := MakeMenu()
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
	if strings.Join(labels, ",") != "About,Appearance" {
		t.Fatalf("menu items = %v", labels)
	}
}

func TestMakeMenu_ActionsOpenWindows(t *testing.T) {
	test.NewTempApp(t)

	for _, item := range MakeMenu().Items[0].Items {
		t.Run(item.Label, func(t *testing.T) {
			item.Action() // must not panic under the test driver
		})
	}
}

func TestNewAbout_Content(t *testing.T) {
	test.NewTempApp(t) // test app metadata has no version or build

	markup := test.RenderObjectToMarkup(newAbout())

	for _, want := range []string{
		config.DefaultVersion,
		"-1",
		strconv.Itoa(time.Now().Year()),
		docsUrl,
		supportEmail,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("about content missing %q:\n%s", want, markup)
		}
	}
}
