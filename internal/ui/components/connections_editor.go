package components

import (
	"encoding/json"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/connect/internal/service"
)

type connectionsEditorDelegate interface {
	GetStore() *service.ConnectionStore
}

// buildUI constructs the layout and logic
func NewConnectionsEditor(c connectionsEditorDelegate, parentWindow fyne.Window) fyne.CanvasObject {
	// 1. Initialize Widgets

	status := widget.NewLabel("Ready")
	status.TextStyle = fyne.TextStyle{Italic: true}

	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder("Paste JSON here...")

	saved, err := json.MarshalIndent(c.GetStore().GetSaved(), "", "    ")
	if err == nil {
		entry.SetText(string(saved))
	}

	entry.TextStyle = fyne.TextStyle{Monospace: true}

	// 2. Set Validator Logic
	entry.Validator = validation.NewRegexp(`.*`, "ignored")
	entry.Validator = func(text string) error {
		if strings.TrimSpace(text) == "" {
			status.SetText("Waiting for input...")
			return nil
		}

		var js service.SavedConnections
		err := json.Unmarshal([]byte(text), &js)
		if err != nil {
			// Truncate error if too long for status bar
			errMsg := fmt.Sprintf("Error: %v", err)
			if len(errMsg) > 120 {
				errMsg = errMsg[:117] + "..."
			}
			status.SetText(errMsg)
			return err
		}

		status.SetText("Valid JSON")
		return nil
	}

	// 3. Format Button Logic
	formatBtn := widget.NewButtonWithIcon("Format", theme.DocumentSaveIcon(), func() {
		text := entry.Text
		if text == "" {
			return
		}
		var js service.SavedConnections
		if err := json.Unmarshal([]byte(text), &js); err == nil {
			pretty, _ := json.MarshalIndent(js, "", "    ")
			entry.SetText(string(pretty))
			entry.Refresh()
		}
	})

	saveBtn := widget.NewButtonWithIcon("Save", theme.ConfirmIcon(), func() {
		text := entry.Text
		if text == "" {
			status.SetText("Nothing to save")
			return
		}
		var js service.SavedConnections
		if err := json.Unmarshal([]byte(text), &js); err != nil {
			status.SetText("Cannot save: Invalid JSON")
			return
		}

		c.GetStore().SetSaved(js)
		err := c.GetStore().SaveToDisk()
		if err != nil {
			status.SetText("Error saving: " + err.Error())
			return
		}

		status.SetText("Connections saved successfully")
	})

	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		parentWindow.Close()
	})

	// 4. Layout
	bottomBar := container.NewHBox(
		status,
		layout.NewSpacer(),
		formatBtn,
		saveBtn,
		cancelBtn,
	)

	return container.NewBorder(nil, bottomBar, nil, nil, entry)
}
