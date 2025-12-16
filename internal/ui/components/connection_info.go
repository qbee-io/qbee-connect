package components

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/service"
)

type infoDelegate interface {
	GetFyneApp() fyne.App
	GetStore() *service.ConnectionStore
	GetWindow() fyne.Window
	DisplayError(title, msg string)
}

func NewDeviceInfoDialog(d infoDelegate, device *client.InventoryListItem) *widget.PopUp {
	// Implementation of device info dialog goes here

	targetInfo, ok := d.GetStore().GetActive(device.NodeID)
	infoContainer := container.NewVBox(
		widget.NewLabel("Target information: " + device.Title),
		// Add more detailed information about the device here
	)

	if ok {
		/*
			infoContainer.Add(container.NewAdaptiveGrid(
				3,
				widget.NewLabel("Target"),
				widget.NewLabel("Mapped port"),
				widget.NewLabel(""),
			))
		*/
		for _, t := range targetInfo.Targets {
			targetString := fmt.Sprintf("- **%s**: %s:%s → %s:%s\n", t.Protocol, t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort)

			var mappedPort string
			actions := container.NewHBox()

			switch t.RemotePort {
			case "22":
				mappedPort = "ssh -p " + t.LocalPort + " " + t.LocalHost
			case "3389":
				mappedPort = "mstsc /v:" + t.LocalHost + ":" + t.LocalPort
			case "80", "443":
				t.Protocol = "https"
				if t.RemotePort == "80" {
					t.Protocol = "http"
				}
				mappedPort = t.Protocol + "://" + t.LocalHost + ":" + t.LocalPort
				actionButton := widget.NewButton("Open", func() {
					urlObj, err := url.Parse(mappedPort)
					fmt.Printf("Parsed URL: %+v\n", urlObj)
					if err != nil {
						d.DisplayError("Invalid URL", "Failed to parse URL: "+err.Error())
						return
					}
					fyne.CurrentApp().OpenURL(urlObj)
				})
				actions.Add(actionButton)
			default:
				mappedPort = t.LocalPort
			}

			copyToClipboardBtn := widget.NewButton("Copy", func() {
				d.GetFyneApp().Clipboard().SetContent(mappedPort)

			})
			actions.Add(copyToClipboardBtn)

			infoContainer.Add(
				container.NewAdaptiveGrid(
					3,
					widget.NewRichTextFromMarkdown(targetString),
					widget.NewLabel(mappedPort),
					actions,
				),
			)
		}
	}

	var dialog *widget.PopUp

	footer := container.NewHBox(
		layout.NewSpacer(),
		widget.NewButton("Close", func() { dialog.Hide() }),
	)

	dialogContent := container.NewBorder(
		infoContainer,
		footer,
		nil, nil,
		widget.NewCard("", "", container.NewWithoutLayout()),
	)

	dialog = widget.NewModalPopUp(dialogContent, d.GetWindow().Canvas())
	dialog.Resize(fyne.NewSize(800, 500))
	return dialog
}
