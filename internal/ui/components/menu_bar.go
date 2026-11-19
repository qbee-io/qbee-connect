package components

import (
	"strings"
	"text/template"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/cmd/fyne_settings/settings"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/connect/internal/config"
)

// MakeMenu constructs the main application menu
func MakeMenu(d preferencesDelegate) *fyne.MainMenu {

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
	preferencesItem := fyne.NewMenuItem("Preferences", func() { NewPreferencesDialog(d).Show() })
	mainMenu := fyne.NewMenu("File", aboutItem, appearanceItem, preferencesItem)
	return fyne.NewMainMenu(mainMenu)
}

const docsURL = "https://docs.qbee.io/qbee-connect.html"
const supportEmail = "support@qbee.io"

var aboutText = `**qbee-connect**

Version: {{.Version}}

Build: {{.Build}}

Docs: [{{.DocURL}}]({{.DocURL}})

Support: [{{.Email}}](mailto:{{.Email}})

© {{.Year}} qbee.io
`

func newAbout() fyne.CanvasObject {

	templateData := struct {
		Version string
		Build   int
		Year    int
		DocURL  string
		Email   string
	}{
		Version: fyne.CurrentApp().Metadata().Version,
		Build:   fyne.CurrentApp().Metadata().Build,
		Year:    time.Now().Year(),
		DocURL:  docsURL,
		Email:   supportEmail,
	}

	if templateData.Version == "" {
		templateData.Version = config.DefaultVersion
	}

	if templateData.Build == 0 {
		templateData.Build = -1
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
