package components

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/ui/widgets"
)

// preferencesDelegate defines the methods required by the preferences dialog
type preferencesDelegate interface {
	GetPreferences() *service.PreferencesStore
	GetWindow() fyne.Window
	RefreshUINoLoad()
	DisplayError(title, msg string)
}

// NewPreferencesDialog creates the preferences dialog
func NewPreferencesDialog(d preferencesDelegate) *widget.PopUp {
	prefs := d.GetPreferences().Get()

	sshUserEntry := widget.NewEntry()
	sshUserEntry.SetPlaceHolder("e.g. root")
	sshUserEntry.SetText(prefs.DefaultSSHUserName)
	sshUserEntry.Validator = func(s string) error {
		return service.ValidateSSHUserName(strings.TrimSpace(s))
	}

	form := widget.NewForm(
		widget.NewFormItem("Default SSH user name", sshUserEntry),
	)

	var dialog *widget.PopUp

	saveBtn := widgets.NewButtonPointer("Save", func() {
		prefs.DefaultSSHUserName = strings.TrimSpace(sshUserEntry.Text)
		if err := d.GetPreferences().Save(prefs); err != nil {
			d.DisplayError("Preferences", "Failed to save preferences: "+err.Error())
			return
		}
		dialog.Hide()
		d.RefreshUINoLoad()
	})
	saveBtn.Importance = widget.HighImportance

	footer := container.NewHBox(
		layout.NewSpacer(),
		saveBtn,
		widgets.NewButtonPointer("Cancel", func() { dialog.Hide() }),
	)

	content := container.NewPadded(container.NewBorder(
		container.NewVBox(
			widget.NewLabel("Preferences"),
			widget.NewSeparator(),
			form,
		),
		footer,
		nil, nil,
	))

	dialog = widget.NewModalPopUp(content, d.GetWindow().Canvas())
	dialog.Resize(fyne.NewSize(500, content.MinSize().Height+40))
	return dialog
}
