package components

import (
	"strings"
	"text/template"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/cmd/fyne_settings/settings"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MakeMenu constructs the main application menu
func MakeMenu() *fyne.MainMenu {

	openAppearance := func() {
		w := fyne.CurrentApp().NewWindow("Settings")
		w.SetContent(settings.NewSettings().LoadAppearanceScreen(w))
		w.Resize(fyne.NewSize(440, 520))
		w.CenterOnScreen()
		w.Show()
	}
	showAbout := func() {
		w := fyne.CurrentApp().NewWindow("About")
		w.SetContent(
			container.NewCenter(newAbout()),
		)
		w.Resize(fyne.NewSize(400, 400))
		w.CenterOnScreen()
		w.Show()
	}
	aboutItem := fyne.NewMenuItem("About", showAbout)
	appearanceItem := fyne.NewMenuItem("Appearance", openAppearance)
	mainMenu := fyne.NewMenu("File", aboutItem, appearanceItem)
	return fyne.NewMainMenu(mainMenu)
}

var aboutText = `**qbee-connect**

Version: {{.Version}}

© {{.Year}} qbee.io
`

func newAbout() fyne.CanvasObject {

	templateData := struct {
		Version string
		Year    int
	}{
		Version: fyne.CurrentApp().Metadata().Version,
		Year:    time.Now().Year(),
	}

	tmpl, err := template.New("about").Parse(aboutText)
	if err != nil {
		return widget.NewLabel("Error loading about information")
	}

	var renderedText strings.Builder
	err = tmpl.Execute(&renderedText, templateData)
	if err != nil {
		return widget.NewLabel("Error loading about information")
	}

	newRichText := widget.NewRichTextFromMarkdown(renderedText.String())

	return newRichText
}
