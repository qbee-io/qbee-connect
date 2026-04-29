package components

import (
	"context"
	"fmt"
	"net"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/ui/widgets"
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

	addBtn := widgets.NewButtonPointer("Add Target", func() {
		addConnectRow(targetsContainer, formContent, nil)
	})
	addBtn.SetIcon(theme.ContentAddIcon())

	content := container.NewVBox(
		widget.NewLabel("Configure port forwarding: "+device.Title),
		widget.NewSeparator(),
		targetsContainer,
		addBtn,
	)

	// Declare dialog variable first so we can close it inside the callback
	var dialog *widget.PopUp
	connectBtn := widgets.NewButtonPointer("Save & Connect", func() {
		saveAndConnect(d, device, targetsContainer, dialog)
	})

	footer := container.NewHBox(
		layout.NewSpacer(),
		connectBtn,
		widgets.NewButtonPointer("Cancel", func() { dialog.Hide() }),
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
		var row *fyne.Container
		var ok bool
		if row, ok = obj.(*fyne.Container); !ok {
			continue
		}

		var target *client.RemoteAccessTarget
		target, err := setTargetEntries(d, row, targets)
		if err != nil {
			d.DisplayError("Invalid Target", fmt.Sprintf("Error in row %d: %v", rowIndex, err))
			return
		}
		targets = append(targets, *target)
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

// maxFreePortRetries defines how many times to attempt finding a free local port when "0" or empty is specified
const maxFreePortRetries = 10

// setTargetEntries reads the entries from the UI row and constructs a RemoteAccessTarget.
func setTargetEntries(d connectDelegate, row *fyne.Container, targets []client.RemoteAccessTarget) (*client.RemoteAccessTarget, error) {
	target := &client.RemoteAccessTarget{
		LocalPort:  row.Objects[0].(*widget.Entry).Text,
		LocalHost:  row.Objects[1].(*widget.Entry).Text,
		RemotePort: row.Objects[3].(*widget.Entry).Text,
		RemoteHost: row.Objects[4].(*widget.Entry).Text,
		Protocol:   row.Objects[5].(*widget.Select).Selected,
	}

	if target.LocalPort != "" && target.LocalPort != "0" {
		return target, nil
	}

	for range maxFreePortRetries {
		var err error
		if target.LocalPort, err = generateRandomPort(d, target.LocalHost, target.Protocol); err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(targets, func(t client.RemoteAccessTarget) bool {
			return t.LocalHost == target.LocalHost && t.LocalPort == target.LocalPort && t.Protocol == target.Protocol
		}) {
			return target, nil
		}
	}
	return nil, fmt.Errorf("failed to find a free local port after %d attempts", maxFreePortRetries)
}

// generateRandomPort attempts to find a free local port on the specified localhost and protocol. It checks with the store to ensure the port is not already used in another target.
func generateRandomPort(d connectDelegate, localhost, protocol string) (string, error) {

	port, err := getFreePort(localhost, protocol)
	if err != nil {
		return "", err
	}

	store := d.GetStore()
	for range maxFreePortRetries {
		if store.IsPortFree(localhost, port, protocol) {
			return fmt.Sprintf("%d", port), nil
		}
		port, err = getFreePort(localhost, protocol)
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("failed to find a free local port after %d attempts", maxFreePortRetries)
}

// getFreePort finds a free local port for the given address and protocol by asking the OS to assign one. It delegates to protocol-specific functions for TCP and UDP.
func getFreePort(address, protocol string) (int, error) {
	if protocol == "udp" {
		return getFreeUDPPort(address)
	}
	return getFreeTCPPort(address)
}

// getFreeUDPPort finds a free local UDP port for the given address by asking the OS to assign one.
func getFreeUDPPort(address string) (int, error) {
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(address, "0"))
	if err != nil {
		return 0, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()

	return conn.LocalAddr().(*net.UDPAddr).Port, nil
}

// getFreeTCPPort finds a free local TCP port for the given address by asking the OS to assign one.
func getFreeTCPPort(address string) (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(address, "0"))
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()

	return l.Addr().(*net.TCPAddr).Port, nil
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
	rmBtn := widgets.NewButtonPointer("", nil)
	rmBtn.SetIcon(theme.DeleteIcon())

	row := container.NewGridWithColumns(7, lp, la, portSelector, rp, ra, proto, rmBtn)

	rmBtn.OnTapped = func() {
		c.Remove(row)
		form.Refresh()
	}
	c.Add(row)
}
