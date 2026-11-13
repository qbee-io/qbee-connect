package components

import (
	"context"
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/ui/widgets"
)

type infoDelegate interface {
	GetFyneApp() fyne.App
	GetClient() *client.Client
	GetContext() context.Context
	GetStore() *service.ConnectionStore
	GetPreferences() *service.PreferencesStore
	GetWindow() fyne.Window
	DisplayError(title, msg string)
	GetDeviceTitle(device *client.InventoryListItem) string
}

// NewDeviceInfoDialog creates a dialog showing connection info for a device
func NewDeviceInfoDialog(d infoDelegate, device *client.InventoryListItem) *widget.PopUp {

	targetInfo, ok := d.GetStore().GetActive(device.NodeID)
	sshUser := d.GetPreferences().Get().DefaultSSHUserName
	header := newDeviceDetails(d.GetDeviceTitle(device), device)
	infoContainer := container.NewVBox()

	if !ok {
		infoContainer.Add(widget.NewLabel("No active connections"))
	} else {

		infoContainer.Add(container.NewAdaptiveGrid(
			3,
			widget.NewLabel("Target"),
			widget.NewLabel("Mapped port"),
			widget.NewLabel(""),
		))

		for _, t := range targetInfo.Targets {
			targetString := fmt.Sprintf("**%s**: %s:%s => %s:%s\n", t.Protocol, t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort)

			mappedPort := renderMappedPort(t, sshUser)
			openAction := getOpenURLButton(d, t)

			copyToClipboardBtn := widgets.NewButtonPointer("", func() {
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
		widgets.NewButtonPointer("Close", func() { dialog.Hide() }),
	)

	connectionsTab := container.NewTabItemWithIcon("Connections", theme.LoginIcon(),
		container.NewVScroll(infoContainer))
	tabs := newDeviceTabs(d, device, connectionsTab)

	dialogContent := container.NewBorder(
		header,
		footer,
		nil, nil,
		tabs,
	)

	dialog = widget.NewModalPopUp(dialogContent, d.GetWindow().Canvas())
	dialog.Resize(deviceDialogSize(d.GetWindow()))
	return dialog
}

// deviceDialogSize returns the size used for the device dialogs: large enough for the
// device info tab, but never larger than the main window.
func deviceDialogSize(w fyne.Window) fyne.Size {
	const minWidth, minHeight = 800, 500
	win := w.Canvas().Size()
	width := min(max(minWidth, win.Width*0.9), win.Width)
	height := min(max(minHeight, win.Height*0.9), win.Height)
	return fyne.NewSize(width, height)
}

// renderSSHCommand returns the ssh command for a target, including the user name if set
func renderSSHCommand(t client.RemoteAccessTarget, sshUser string) string {
	host := t.LocalHost
	if sshUser != "" {
		host = sshUser + "@" + host
	}
	return "ssh -p " + t.LocalPort + " " + host
}

func renderMappedPort(t client.RemoteAccessTarget, sshUser string) string {
	if t.Protocol == "udp" {
		return t.LocalPort
	}

	switch t.RemotePort {
	case "22":
		return renderSSHCommand(t, sshUser)
	case "3389":
		return "mstsc /v:" + t.LocalHost + ":" + t.LocalPort
	case "80", "443":
		scheme := "https"
		if t.RemotePort == "80" {
			scheme = "http"
		}
		return scheme + "://" + t.LocalHost + ":" + t.LocalPort

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
		openURL := widgets.NewButtonPointer("", func() {
			urlObj, err := url.Parse(renderMappedPort(t, ""))
			if err != nil {
				i.DisplayError("Invalid URL", "Failed to parse URL: "+err.Error())
				return
			}
			if err := i.GetFyneApp().OpenURL(urlObj); err != nil {
				i.DisplayError("Open URL failed", "Failed to open URL: "+err.Error())
			}
		})
		openURL.SetIcon(theme.MailSendIcon())
		return openURL
	default:
		return widget.NewLabel("")
	}
}
