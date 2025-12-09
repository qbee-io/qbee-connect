package components

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/service"
)

// connectDelegate defines the methods required by the connect dialog
type connectDelegate interface {
	GetClient() *client.Client
	GetContext() context.Context
	GetStore() *service.ConnectionStore
	GetWindow() fyne.Window
	RefreshUI()
	DisplayError(title, msg string)
}

// NewConnectDialog creates a new connection configuration dialog
func NewConnectDialog(d connectDelegate, device *client.InventoryListItem) *widget.PopUp {
	targetsContainer := container.NewVBox()
	formContent := container.NewVBox()

	saved, exists := d.GetStore().GetDeviceSaved(device.NodeID)
	if exists {
		for _, t := range saved {
			addConnectRow(targetsContainer, formContent, &t)
		}
	} else {
		addConnectRow(targetsContainer, formContent, nil)
	}

	addBtn := widget.NewButtonWithIcon("Add Target", theme.ContentAddIcon(), func() {
		addConnectRow(targetsContainer, formContent, nil)
	})

	content := container.NewVBox(
		widget.NewLabel("Configure port forwarding: "+device.Title),
		widget.NewSeparator(),
		targetsContainer,
		addBtn,
	)

	// Declare dialog variable first so we can close it inside the callback
	var dialog *widget.PopUp

	connectBtn := widget.NewButton("Save & Connect", func() {
		var targets []client.RemoteAccessTarget
		for _, obj := range targetsContainer.Objects {
			if row, ok := obj.(*fyne.Container); ok {
				targets = append(targets, client.RemoteAccessTarget{
					LocalPort:  row.Objects[0].(*widget.Entry).Text,
					LocalHost:  row.Objects[1].(*widget.Entry).Text,
					RemotePort: row.Objects[2].(*widget.Entry).Text,
					RemoteHost: row.Objects[3].(*widget.Entry).Text,
					Protocol:   row.Objects[4].(*widget.Select).Selected,
				})
			}
		}

		ctx, cancel := context.WithCancel(d.GetContext())
		d.GetStore().SetActive(device.NodeID, &service.DeviceConnections{Targets: targets, Cancel: cancel})

		go func() {
			defer func() {
				cancel()
				d.GetStore().DeleteActive(device.NodeID)
				fyne.DoAndWait(func() { d.RefreshUI() })
			}()
			if err := d.GetClient().Connect(ctx, device.NodeID, targets); err != nil {
				d.DisplayError("Connection Error", err.Error())
			}
		}()

		err := d.GetStore().SaveOnConnect(device.NodeID, targets)
		if err != nil {
			d.DisplayError("Save Error", err.Error())
		}
		d.RefreshUI()
		dialog.Hide()
	})

	footer := container.NewHBox(
		layout.NewSpacer(),
		connectBtn,
		widget.NewButton("Cancel", func() { dialog.Hide() }),
	)

	dialogContent := container.NewBorder(
		content,
		footer,
		nil, nil,
		widget.NewCard("", "", container.NewWithoutLayout()),
	)

	dialog = widget.NewModalPopUp(dialogContent, d.GetWindow().Canvas())
	dialog.Resize(fyne.NewSize(800, 500))
	return dialog
}

func addConnectRow(c *fyne.Container, form *fyne.Container, prefill *client.RemoteAccessTarget) {
	lp := widget.NewEntry()
	lp.SetPlaceHolder("Local Port")
	la := widget.NewEntry()
	la.SetPlaceHolder("Local Addr")
	la.SetText("127.0.0.1")
	rp := widget.NewEntry()
	rp.SetPlaceHolder("Remote Port")
	ra := widget.NewEntry()
	ra.SetPlaceHolder("Remote Addr")
	ra.SetText("127.0.0.1")
	proto := widget.NewSelect([]string{"tcp", "udp"}, nil)
	proto.SetSelected("tcp")

	if prefill != nil {
		lp.SetText(prefill.LocalPort)
		la.SetText(prefill.LocalHost)
		rp.SetText(prefill.RemotePort)
		ra.SetText(prefill.RemoteHost)
		proto.SetSelected(prefill.Protocol)
	}

	rmBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
	row := container.NewGridWithColumns(6, lp, la, rp, ra, proto, rmBtn)

	rmBtn.OnTapped = func() {
		c.Remove(row)
		form.Refresh()
	}
	c.Add(row)
}
