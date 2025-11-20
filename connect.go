package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
)

func (app *App) newConnectTargetRow(targetsContainer *fyne.Container, formContent *fyne.Container) *fyne.Container {
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

func (app *App) newConnectButton(device *client.InventoryListItem, targetsContainer *fyne.Container, dialog *widget.PopUp) *widget.Button {

	return widget.NewButton("Save & Connect", func() {

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

		app.saveConnections(device.NodeID, targets)

		app.redrawDeviceList()
		dialog.Hide()
	})
}

func (app *App) newConnectDialog(device *client.InventoryListItem) *widget.PopUp {

	// Container for multiple targets
	targetsContainer := container.NewVBox()

	formContent := container.NewVBox()

	savedConns, exists := app.savedConns[device.NodeID]
	if exists {
		for _, target := range savedConns {
			row := app.newConnectTargetRow(targetsContainer, formContent)

			// Set saved values
			row.Objects[0].(*widget.Entry).SetText(target.LocalPort)
			row.Objects[1].(*widget.Entry).SetText(target.LocalHost)
			row.Objects[2].(*widget.Entry).SetText(target.RemotePort)
			row.Objects[3].(*widget.Entry).SetText(target.RemoteHost)
			row.Objects[4].(*widget.Select).SetSelected(target.Protocol)

			targetsContainer.Add(row)
		}
	} else {
		targetsContainer.Add(app.newConnectTargetRow(targetsContainer, formContent))
	}

	// Add plus button to add more targets
	addBtn := widget.NewButtonWithIcon("Add Target", theme.ContentAddIcon(), func() {
		targetsContainer.Add(app.newConnectTargetRow(targetsContainer, formContent))
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
	connectBtn := app.newConnectButton(device, targetsContainer, dialog)

	controls.Add(connectBtn)

	cancelBtn := widget.NewButton("Cancel", func() {})
	controls.Add(cancelBtn)

	cancelBtn.OnTapped = func() {
		dialog.Hide()
	}

	return dialog
}

const connectionsFileName = "connections.json"

func (app *App) saveConnections(nodeID string, targets []client.RemoteAccessTarget) {

	app.savedConns[nodeID] = targets

	appStore := app.fyneApp.Storage()
	list := appStore.List()

	exists := slices.Contains(list, connectionsFileName)

	var fileDescriptor fyne.URIWriteCloser
	var err error

	if !exists {
		if fileDescriptor, err = appStore.Create(connectionsFileName); err != nil {
			fmt.Println("Error opening connections file:", err)
			return
		}
	} else {
		if fileDescriptor, err = appStore.Save(connectionsFileName); err != nil {
			fmt.Println("Error opening connections file:", err)
			return
		}
	}
	defer fileDescriptor.Close()

	jsonData, err := json.Marshal(app.savedConns)
	if err != nil {
		fmt.Println("Error marshaling connections:", err)
		return
	}
	if _, err := fileDescriptor.Write(jsonData); err != nil {
		fmt.Println("Error writing connections to file:", err)
		return
	}

}

func (app *App) loadSavedConnections() {

	fileDescriptor, err := app.fyneApp.Storage().Open(connectionsFileName)
	if err != nil {
		return
	}
	defer fileDescriptor.Close()

	var data map[string][]client.RemoteAccessTarget
	decoder := json.NewDecoder(fileDescriptor)
	if err := decoder.Decode(&data); err != nil {
		app.fyneApp.SendNotification(
			&fyne.Notification{
				Title:   "Error",
				Content: "Failed to load saved connections: " + err.Error(),
			},
		)
	}

	app.savedConns = data
}
