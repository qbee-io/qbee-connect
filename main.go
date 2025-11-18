package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"strings"

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

*/

// page state
type deviceModel struct {
	deviceData   *client.InventoryListResponse
	pageSize     int
	currentPage  int
	offset       int
	searchFilter string
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
	deviceModel    *deviceModel
	deviceList     *fyne.Container
}

func newApp() *App {
	a := app.NewWithID("io.qbee.qbee-connect")
	w := a.NewWindow("qbee-connect - qbee.io")
	ctx := context.Background()

	cli, err := client.LoginGetAuthenticatedClient(ctx)
	if err != nil {
		log.Fatalln("failed to authenticate client:", err)
	}

	connectionsMap := newConnectionsMap()

	return &App{
		fyneApp:        a,
		mainWin:        w,
		cli:            cli,
		ctx:            ctx,
		connectionsMap: connectionsMap,
		deviceModel:    newDeviceModel(),
		deviceList:     container.NewVBox(),
	}
}

func main() {

	app := newApp()
	w := app.mainWin

	app.makeTray()
	app.mainWin.SetMainMenu(app.makeMenu())

	w.SetIcon(fyne.NewStaticResource("qbee-connect-icon.png", trayIcon))

	// ---- Top toolbar (search + buttons) ----
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search devices")

	refreshButton := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {})

	rightControls := container.NewHBox(
		refreshButton,
	)

	topBar := container.NewBorder(
		nil, nil,
		nil,
		rightControls,
		searchEntry,
	)

	// ---- Table header row ----
	header := container.NewGridWithColumns(5,
		headerLabel("Device"),
		headerLabel("Status"),
		headerLabel("Group"),
		headerLabel("Tags"),
		headerLabel("Actions"),
	)

	// ---- Pagination controls ----
	pageInfoLabel := widget.NewLabel("Page 1 / 1")

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
			pageInfoLabel.SetText(
				fmt.Sprintf("Page %d / %d", app.deviceModel.currentPage+1, app.deviceModel.totalPages()),
			)
		}
	})

	pagination := container.NewHBox(
		layout.NewSpacer(),
		prevButton,
		pageInfoLabel,
		nextButton,
	)

	// ---- Center content: header + list + pagination ----
	content := container.NewBorder(
		header,
		pagination,
		nil,
		nil,
		app.deviceList,
	)

	root := container.NewBorder(topBar, nil, nil, nil, content)
	w.SetContent(root)

	deviceFetchError := widget.NewPopUp(
		widget.NewLabel("Failed to load devices. Please try again."),
		w.Canvas(),
	)
	deviceFetchError.Hide()

	// ---- Handlers ----
	refresh := func() {
		err := app.refreshDeviceListUI()
		if err != nil {
			deviceFetchError.Show()
			log.Println("refresh error:", err)
		} else {
			deviceFetchError.Hide()
		}
		pageInfoLabel.SetText(
			fmt.Sprintf("Page %d / %d", app.deviceModel.currentPage+1, app.deviceModel.totalPages()),
		)
	}

	refreshButton.OnTapped = refresh

	searchEntry.OnChanged = func(s string) {
		app.deviceModel.searchFilter = s
		app.deviceModel.currentPage = 0
		err := app.refreshDeviceListUI()
		if err != nil {
			deviceFetchError.Show()
			log.Println("refresh error:", err)
		} else {
			deviceFetchError.Hide()
		}
		pageInfoLabel.SetText(
			fmt.Sprintf("Page %d / %d", app.deviceModel.currentPage+1, app.deviceModel.totalPages()),
		)

	}

	err := app.refreshDeviceListUI()
	if err != nil {
		deviceFetchError.Show()
		log.Println("refresh error:", err)
	} else {
		deviceFetchError.Hide()
	}

	pageInfoLabel.SetText(
		fmt.Sprintf("Page %d / %d", app.deviceModel.currentPage+1, app.deviceModel.totalPages()),
	)

	w.SetCloseIntercept(func() {
		w.Hide()
	})

	// ---- Show window ----
	w.ShowAndRun()
}

// refreshDeviceListUI rebuilds the list for the current page.
func (app *App) refreshDeviceListUI() error {
	var err error

	app.deviceModel.offset = app.deviceModel.currentPage * app.deviceModel.pageSize
	app.deviceModel.deviceData, err = app.loadDevices(app.ctx, app.deviceModel.searchFilter, app.deviceModel.offset, app.deviceModel.pageSize)
	if err != nil {
		log.Println("failed to load devices for UI refresh:", err)
		return err
	}

	app.redrawDeviceList()

	return nil
}

func (app *App) redrawDeviceList() {
	app.deviceList.Objects = app.deviceList.Objects[:0]

	for _, d := range app.deviceModel.deviceData.Items {
		row := app.buildDeviceRow(d)
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
func (app *App) buildDeviceRow(d client.InventoryListItem) fyne.CanvasObject {
	// Device name (left aligned)
	deviceLabel := widget.NewLabel(d.Title)

	// Status: colored circle + text
	statusCircle := canvas.NewCircle(statusColor(d.Status))
	statusCircle.Resize(fyne.NewSize(10, 10))
	statusText := widget.NewLabel(cases.Title(language.English).String(d.Status))
	statusBox := container.NewHBox(
		statusCircle,
		widget.NewLabel(" "),
		statusText,
	)

	// Group (breadcrumb style). Skop last element (device name)
	groupLabel := widget.NewLabel(strings.Join(d.AncestorsTitles[0:len(d.AncestorsTitles)-1], " > "))
	groupLabel.Truncation = fyne.TextTruncateEllipsis
	groupLabel.Alignment = fyne.TextAlignLeading

	// Tags (comma joined)
	tagsLabel := widget.NewLabel(strings.Join(d.Tags, ", "))
	tagsLabel.Truncation = fyne.TextTruncateEllipsis
	tagsLabel.Alignment = fyne.TextAlignLeading
	var actionsBtn fyne.CanvasObject
	if _, ok := app.connectionsMap.get(d.NodeID); !ok {
		actionsBtn = widget.NewButton("Connect", func() {
			dialog := app.newConnectDialog(&d)
			dialog.Show()
		})
	} else {
		actionsBtn = widget.NewButton("Disconnect", func() {
			if conn, ok := app.connectionsMap.get(d.NodeID); ok {
				conn.cancel()
				app.connectionsMap.delete(d.NodeID)
				app.redrawDeviceList()
			}
		})
	}

	rowGrid := container.NewGridWithColumns(5,
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

// statusColor picks a color based on the status string.
// Tweak these to match your preferred palette.
func statusColor(status string) color.Color {
	s := strings.ToLower(status)
	switch s {
	case "online", "connected":
		return theme.Color(theme.ColorNameForeground)
	case "offline", "disconnected":
		return theme.Color(theme.ColorNameError)
	default:
		return theme.Color(theme.ColorNameDisabled)
	}
}
