package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"go.qbee.io/client"

	_ "embed"
)

/*
CI inspiration:

https://github.com/smaTc/RemotePlayDetached

https://github.com/massalabs/station/blob/main/Taskfile.yml


Possible to run in Windows VM without hardware acceleration:

https://github.com/mmozeiko/build-mesa/releases/download/25.3.0/mesa-d3d12-x64-25.3.0.7z

Unpack to directory where qbee-connect.exe is located.

Set environment variable:

*/

/*
- pinning of devices
    - persisting of devices and port forwarding locally

- Auto-connect
    - This needs to be account specific

- profile support (switch between profiles, eg, different logins)

- search support:
    - device name
    - group id
    - tags

- Show connected devices (potentially in a separate tab) - or check box
- Show connect devices (

- Change "connect" button text (Open tunnel, stop tunnel)

- "Disconnect all" button
*/

// page state
type deviceModel struct {
	deviceData        *client.InventoryListResponse
	currentPage       int
	activeTunnelsOnly bool
	query             *client.InventoryListQuery
}

type deviceConnections struct {
	// Configured remote access targets for this device
	targets []client.RemoteAccessTarget

	// Context cancellation function to stop the connection
	cancel func()
}

//go:embed qbee-connect-icon.png
var trayIcon []byte

type App struct {
	fyneApp        fyne.App
	mainWin        fyne.Window
	cli            *client.Client
	ctx            context.Context
	connectionsMap *connectionsMap
	savedConns     map[string][]client.RemoteAccessTarget
	deviceModel    *deviceModel
	deviceList     *fyne.Container
	pageInfoLabel  *widget.Label
}

func newApp() *App {
	ctx := context.Background()
	var cli *client.Client
	var err error
	cli, err = client.LoginGetAuthenticatedClient(ctx)

	//
	if err != nil {
		cli, err = terminalLogin()
		if err != nil {
			log.Fatalln("failed to login:", err)
		}
	}

	a := app.New()
	w := a.NewWindow("qbee-connect - qbee.io")

	connectionsMap := newConnectionsMap()

	return &App{
		fyneApp:        a,
		mainWin:        w,
		cli:            cli,
		ctx:            ctx,
		connectionsMap: connectionsMap,
		deviceModel:    newDeviceModel(),
		deviceList:     container.NewVBox(),
		pageInfoLabel:  widget.NewLabel("Page 1 / 1"),
		savedConns:     make(map[string][]client.RemoteAccessTarget),
	}
}

func (app *App) displayError(title, content string) {
	app.fyneApp.SendNotification(&fyne.Notification{
		Title:   title,
		Content: content,
	})
}

func main() {

	app := newApp()

	app.makeTray()
	app.mainWin.SetMainMenu(app.makeMenu())

	err := app.loadSavedConnections()
	if err != nil {
		app.displayError("Load Error", "Failed to load saved connections: "+err.Error())
	}

	app.mainWin.SetIcon(fyne.NewStaticResource("qbee-connect-icon.png", trayIcon))
	searchEntry := container.NewVBox()

	searchBy := app.setupSearchBySelect(searchEntry)
	searchBy.SetSelected("Device name")

	refreshButton := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {})
	refreshButton.OnTapped = func() {
		go func() {
			fyne.DoAndWait(func() {
				refreshButton.Disable()
			})
			time.Sleep(100 * time.Millisecond) // allow UI to update
			fyne.DoAndWait(func() {
				app.refreshDeviceListUI()
			})
			defer fyne.DoAndWait(func() {
				refreshButton.Enable()
			})
		}()
	}

	// Initial device list load

	err = app.refreshDeviceListUI()
	if err != nil {
		app.displayError("Device Fetch Error", "Failed to load devices: "+err.Error())
	}

	filterActiveTunnels := widget.NewCheck("Active tunnels only", func(checked bool) {
		app.deviceModel.currentPage = 0
		app.deviceModel.activeTunnelsOnly = checked
		err := app.refreshDeviceListUI()
		if err != nil {
			app.displayError("Device Fetch Error", "Failed to load devices for active tunnels filter: "+err.Error())
		}
	})

	rightControls := container.NewHBox(
		searchBy,
		filterActiveTunnels,
		refreshButton,
	)

	topBar := container.NewBorder(
		nil, nil,
		nil,
		rightControls,
		searchEntry,
	)

	// ---- Table header row ----
	header := container.NewAdaptiveGrid(5,
		headerLabel("Device"),
		headerLabel("Status"),
		headerLabel("Group"),
		headerLabel("Tags"),
		headerLabel("Actions"),
	)

	prevButton := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if app.deviceModel.currentPage > 0 {
			app.deviceModel.currentPage--
			app.refreshDeviceListUI()

		}
	})
	nextButton := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		if app.deviceModel.currentPage < app.deviceModel.totalPages()-1 {
			app.deviceModel.currentPage++
			app.refreshDeviceListUI()
		}
	})

	pagination := container.NewHBox(
		layout.NewSpacer(), // fill space to push to right
		prevButton,
		app.pageInfoLabel,
		nextButton,
	)

	// ---- Center content: header + list + pagination ----
	content := container.NewBorder(
		header,         // top content
		pagination,     // bottom content
		nil,            // left content
		nil,            // right content
		app.deviceList, // middle content
	)

	root := container.NewBorder(topBar, nil, nil, nil, content)
	app.mainWin.SetContent(root)

	app.mainWin.SetCloseIntercept(func() {
		app.mainWin.Hide()
	})

	// ---- Show window ----
	app.mainWin.Resize(fyne.NewSize(800, 600))
	app.mainWin.ShowAndRun()
}

// refreshDeviceListUI rebuilds the list for the current page.
func (app *App) refreshDeviceListUI() error {
	var err error

	app.deviceModel.query.Offset = app.deviceModel.currentPage * app.deviceModel.query.ItemsPerPage
	app.deviceModel.deviceData, err = app.loadDevices(app.ctx)
	if err != nil {
		log.Println("failed to load devices for UI refresh:", err)
		return err
	}

	app.redrawDeviceList()
	app.pageInfoLabel.SetText(
		fmt.Sprintf("Page %d / %d", app.deviceModel.currentPage+1, app.deviceModel.totalPages()),
	)

	return nil
}

func (app *App) redrawDeviceList() {
	//app.deviceList.Objects = app.deviceList.Objects[:0]
	app.deviceList.RemoveAll()

	for _, d := range app.deviceModel.deviceData.Items {

		existingConnection, ok := app.connectionsMap.get(d.NodeID)
		// Filter active tunnels only if the option is set
		if app.deviceModel.activeTunnelsOnly && !ok {
			continue
		}
		row := app.buildDeviceRow(d, existingConnection)
		app.deviceList.Add(row)
	}

	app.deviceList.Refresh()
}

// headerLabel is a bold header cell for the grid header row.
func headerLabel(text string) *widget.Label {
	lbl := widget.NewLabel(text)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	return lbl
}

func (app *App) makeTray() {
	if desk, ok := app.fyneApp.(desktop.App); ok {
		menu := fyne.NewMenu("qbee-connect",
			fyne.NewMenuItem("Show", func() {
				app.mainWin.Show()
			}),
			fyne.NewMenuItem("Quit", func() {
				app.fyneApp.Quit()
			}),
		)
		desk.SetSystemTrayIcon(fyne.NewStaticResource("qbee-connect-icon.png", trayIcon))
		desk.SetSystemTrayMenu(menu)
	}
}

// buildDeviceRow builds a single row similar to the qbee web UI.
func (app *App) buildDeviceRow(d client.InventoryListItem, existingConnection *deviceConnections) fyne.CanvasObject {
	// Device name (left aligned)
	deviceLabel := widget.NewLabel(d.Title)

	statusCircle := widget.NewIcon(statusIcon(d.Status))

	statusText := widget.NewLabel(cases.Title(language.English).String(d.Status))
	statusBox := container.NewHBox(
		statusCircle,
		statusText,
	)

	// Group (breadcrumb style). Skop last element (device name)
	groupLabel := NewLabelHover(strings.Join(d.AncestorsTitles[0:len(d.AncestorsTitles)-1], " > "))
	groupLabel.Truncation = fyne.TextTruncateEllipsis
	groupLabel.Alignment = fyne.TextAlignLeading

	// Tags (comma joined)
	tagsLabel := NewLabelHover(strings.Join(d.Tags, ", "))
	tagsLabel.Truncation = fyne.TextTruncateEllipsis
	tagsLabel.Alignment = fyne.TextAlignLeading

	var actionsBtn fyne.CanvasObject
	if existingConnection == nil {
		actionsBtn = widget.NewButton("Configure", func() {
			dialog := app.newConnectDialog(&d)
			dialog.Show()
		})
	} else {
		actionsBtn = widget.NewButton("Disconnect", func() {
			existingConnection.cancel()
			app.connectionsMap.delete(d.NodeID)
			app.redrawDeviceList()
		})
	}

	rowGrid := container.NewAdaptiveGrid(5,
		deviceLabel,
		statusBox,
		groupLabel,
		tagsLabel,
		actionsBtn,
	)

	// Add subtle separator line under each row for table look
	sep := canvas.NewRectangle(theme.Color(theme.ColorNameShadow))
	sep.SetMinSize(fyne.NewSize(0, 1))

	return container.NewVBox(
		rowGrid,
		sep,
	)
}

// statusIcon picks an icon based on the status string.
func statusIcon(status string) fyne.Resource {
	s := strings.ToLower(status)
	switch s {
	case "online", "connected":
		return theme.ConfirmIcon()
	case "offline", "disconnected":
		return theme.CancelIcon()
	default:
		return theme.QuestionIcon()
	}
}
