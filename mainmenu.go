package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/cmd/fyne_settings/settings"
	"fyne.io/fyne/v2/widget"
)

func (app *App) makeMenu() *fyne.MainMenu {

	openSettings := func() {
		w := app.fyneApp.NewWindow("Fyne Settings")
		w.SetContent(settings.NewSettings().LoadAppearanceScreen(w))
		w.Resize(fyne.NewSize(440, 520))
		w.Show()
	}
	showAbout := func() {
		w := app.fyneApp.NewWindow("About")
		w.SetContent(widget.NewLabel("About Fyne Demo app..."))
		w.Show()
	}
	aboutItem := fyne.NewMenuItem("About", showAbout)
	settingsItem := fyne.NewMenuItem("Settings", openSettings)
	mainMenu := fyne.NewMenu("File", aboutItem, settingsItem)
	return fyne.NewMainMenu(mainMenu)
}
