package main

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
)

func (app *App) newConnectDialog(device *client.InventoryListItem) *widget.PopUp {

	// Container for multiple targets
	targetsContainer := container.NewVBox()

	formContent := container.NewVBox()

	// Function to create a target row
	createTargetRow := func() *fyne.Container {
		localPort := widget.NewEntry()
		localPort.SetPlaceHolder("Local port")
		localAddr := widget.NewEntry()
		localAddr.SetPlaceHolder("Local address")
		localAddr.SetText("127.0.0.1")
		remotePort := widget.NewEntry()
		remotePort.SetPlaceHolder("Remote port")
		remoteAddr := widget.NewEntry()
		remoteAddr.SetPlaceHolder("Remote address")
		remoteAddr.SetText("127.0.0.1")
		protocol := widget.NewSelect([]string{"tcp", "udp"}, func(value string) {})
		protocol.SetSelected("tcp")

		removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)

		row := container.NewGridWithColumns(6,
			localPort, localAddr, remotePort, remoteAddr, protocol, removeBtn,
		)

		// Set remove button action after row is created
		removeBtn.OnTapped = func() {
			for i, obj := range targetsContainer.Objects {
				if obj == row {
					targetsContainer.Objects = append(targetsContainer.Objects[:i], targetsContainer.Objects[i+1:]...)
					formContent.Refresh()
					break
				}
			}
		}

		return row
	}

	targetsContainer.Add(createTargetRow())

	// Add plus button to add more targets
	addBtn := widget.NewButtonWithIcon("Add Target", theme.ContentAddIcon(), func() {
		targetsContainer.Add(createTargetRow())
	})

	header := container.NewGridWithColumns(6,
		widget.NewLabel("Local Port"),
		widget.NewLabel("Local Address"),
		widget.NewLabel("Remote Port"),
		widget.NewLabel("Remote Address"),
		widget.NewLabel("Protocol"),
		widget.NewLabel(""),
	)

	// Form content
	formContent.Add(
		container.NewVBox(
			widget.NewLabel("Configure port forwarding for "+device.Title),
			widget.NewSeparator(),
			header,
			targetsContainer,
			addBtn,
		),
	)

	controls := container.NewHBox(
		layout.NewSpacer(),
	)

	dialog := widget.NewModalPopUp(
		container.NewBorder(
			formContent,
			controls,
			nil,
			nil,
			widget.NewCard("", "", container.NewWithoutLayout()),
		),
		app.mainWin.Canvas(),
	)

	dialog.Resize(fyne.NewSize(800, 500))

	// Dialog buttons
	connectBtn := widget.NewButton("Connect", func() {

		var targets []client.RemoteAccessTarget
		for _, obj := range targetsContainer.Objects {
			if row, ok := obj.(*fyne.Container); ok {

				target := client.RemoteAccessTarget{
					LocalPort:  row.Objects[0].(*widget.Entry).Text,
					LocalHost:  row.Objects[1].(*widget.Entry).Text,
					RemotePort: row.Objects[2].(*widget.Entry).Text,
					RemoteHost: row.Objects[3].(*widget.Entry).Text,
					Protocol:   row.Objects[4].(*widget.Select).Selected,
				}

				targets = append(targets, target)
			}
		}

		connectCtx, cancel := context.WithCancel(app.ctx)
		app.connectionsMap.set(device.NodeID, &deviceConnections{
			targets: targets,
			cancel:  cancel,
		})

		go func() {
			defer func() {
				cancel()

				app.connectionsMap.delete(device.NodeID)

				fyne.DoAndWait(func() {
					app.redrawDeviceList()
				})
			}()

			if err := app.cli.Connect(connectCtx, device.NodeID, targets); err != nil {
				app.fyneApp.SendNotification(&fyne.Notification{
					Title:   "Connection Error",
					Content: fmt.Sprintf("Failed to connect to device %s: %v", device.Title, err),
				})
				return
			}
		}()

		app.redrawDeviceList()
		dialog.Hide()
	})

	controls.Add(connectBtn)

	cancelBtn := widget.NewButton("Cancel", func() {})
	controls.Add(cancelBtn)

	cancelBtn.OnTapped = func() {
		dialog.Hide()
	}

	return dialog
}
