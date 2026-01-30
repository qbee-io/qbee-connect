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
	RefreshUINoLoad()
	DisplayError(title, msg string)
}

// NewConnectDialog creates a new connection configuration dialog
func NewConnectDialog(d connectDelegate, device *client.InventoryListItem) *widget.PopUp {
	targetsContainer := container.NewVBox(
		container.NewGridWithColumns(7,
			widget.NewLabel("Local Port"),
			widget.NewLabel("Local Addr"),
			widget.NewLabel("Type"),
			widget.NewLabel("Remote Port"),
			widget.NewLabel("Remote Addr"),
			widget.NewLabel("Protocol"),
			widget.NewLabel(""),
		),
	)
	formContent := container.NewVBox()

	saved, exists := d.GetStore().GetSaved(device.NodeID)
	if exists {
		for _, t := range saved.Targets {
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
		saveAndConnect(d, device, targetsContainer, dialog)
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

func saveAndConnect(d connectDelegate, device *client.InventoryListItem, targetsContainer *fyne.Container, dialog *widget.PopUp) {
	var targets []client.RemoteAccessTarget
	for rowIndex, obj := range targetsContainer.Objects {
		// Skip the header row
		if rowIndex == 0 {
			continue
		}
		if row, ok := obj.(*fyne.Container); ok {
			targets = append(targets, client.RemoteAccessTarget{
				LocalPort:  row.Objects[0].(*widget.Entry).Text,
				LocalHost:  row.Objects[1].(*widget.Entry).Text,
				RemotePort: row.Objects[3].(*widget.Entry).Text,
				RemoteHost: row.Objects[4].(*widget.Entry).Text,
				Protocol:   row.Objects[5].(*widget.Select).Selected,
			})
		}
	}

	ctx, cancel := context.WithCancel(d.GetContext())
	d.GetStore().SetActive(device.NodeID, &service.DeviceConnections{
		Title:   device.Title,
		Targets: targets,
		Cancel:  cancel,
	})

	go func() {
		defer func() {
			cancel()
			d.GetStore().DeleteActive(device.NodeID)
			fyne.Do(func() {
				d.RefreshUINoLoad()
			})
		}()
		if err := d.GetClient().Connect(ctx, device.NodeID, targets); err != nil {
			fyne.Do(func() {
				d.DisplayError("Connection Error", err.Error())
			})
		}
	}()

	err := d.GetStore().SaveToDisk(device.NodeID, &service.DeviceConnections{
		Title:   device.Title,
		Targets: targets,
	})
	if err != nil {
		d.DisplayError("Save Error", err.Error())
	}
	d.RefreshUINoLoad()
	dialog.Hide()
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

	portSelector := widget.NewSelect(servicePortNames,
		func(s string) {
			if s != serviceCustomName {
				proto.SetSelected("tcp")
				rp.SetText(servicePortMap[s])
				rp.Disable()
				proto.Disable()
			} else {
				rp.SetText("")
				rp.Enable()
				proto.Enable()
			}
		})

	if prefill != nil {
		lp.SetText(prefill.LocalPort)
		la.SetText(prefill.LocalHost)
		ra.SetText(prefill.RemoteHost)
		// set port selector based on prefill
		found := false
		for name, port := range servicePortMap {
			if port == prefill.RemotePort {
				// Only select the predefined port if the protocol matches the default ("tcp")
				if prefill.Protocol == "tcp" {
					portSelector.SetSelected(name)
					found = true
					break
				}
				// If protocol does not match, treat as custom
				break
			}
		}
		if !found {
			portSelector.SetSelected(serviceCustomName)
			rp.SetText(prefill.RemotePort)
			proto.SetSelected(prefill.Protocol)
			rp.Enable()
			proto.Enable()
			proto.SetSelected(prefill.Protocol)
		}
	} else {
		// trigger port selector to set initial state
		portSelector.SetSelected(servicePortNames[0])
	}

	rmBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
	row := container.NewGridWithColumns(7, lp, la, portSelector, rp, ra, proto, rmBtn)

	rmBtn.OnTapped = func() {
		c.Remove(row)
		form.Refresh()
	}
	c.Add(row)
}
