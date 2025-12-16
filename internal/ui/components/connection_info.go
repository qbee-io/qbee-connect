package components

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

	targetInfo, ok := d.GetStore().GetActive(device.NodeID)
	infoContainer := container.NewVBox(
		widget.NewLabel("Target information: " + device.Title),
	)

	if ok {

		infoContainer.Add(container.NewAdaptiveGrid(
			3,
			widget.NewLabel("Target"),
			widget.NewLabel("Mapped port"),
			widget.NewLabel(""),
		))

		for _, t := range targetInfo.Targets {
			targetString := fmt.Sprintf("**%s**: %s:%s → %s:%s\n", t.Protocol, t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort)

			mappedPort := renderMappedPort(t)
			openAction := getOpenURLButton(d, t)

			copyToClipboardBtn := widget.NewButton("", func() {
				d.GetFyneApp().Clipboard().SetContent(mappedPort)
			})
			copyToClipboardBtn.SetIcon(theme.ContentCopyIcon())

			infoContainer.Add(
				container.NewAdaptiveGrid(
					3,
					widget.NewRichTextFromMarkdown(targetString),
					widget.NewLabel(mappedPort),
					container.NewAdaptiveGrid(
						3,
						layout.NewSpacer(),
						copyToClipboardBtn,
						openAction,
					),
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

func renderMappedPort(t client.RemoteAccessTarget) string {
	if t.Protocol == "udp" {
		return t.LocalPort
	}

	switch t.RemotePort {
	case "22":
		return "ssh -p " + t.LocalPort + " " + t.LocalHost
	case "3389":
		return "mstsc /v:" + t.LocalHost + ":" + t.LocalPort
	case "80", "443":
		t.Protocol = "https"
		if t.RemotePort == "80" {
			t.Protocol = "http"
		}
		return t.Protocol + "://" + t.LocalHost + ":" + t.LocalPort

	default:
		return t.LocalPort
	}
}

func getOpenURLButton(i infoDelegate, t client.RemoteAccessTarget) fyne.CanvasObject {

	if t.Protocol == "udp" {
		return widget.NewLabel("")
	}

	switch t.RemotePort {
	case "80", "443":
		openURL := widget.NewButton("", func() {
			urlObj, err := url.Parse(renderMappedPort(t))
			if err != nil {
				i.DisplayError("Invalid URL", "Failed to parse URL: "+err.Error())
				return
			}
			i.GetFyneApp().OpenURL(urlObj)
		})
		openURL.SetIcon(theme.MailSendIcon())
		return openURL
	default:
		return widget.NewLabel("")
	}
}
