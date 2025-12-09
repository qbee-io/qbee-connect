package ui

import (
	"context"
	_ "embed" // for tray icon embedding
	"fmt"
	"image/color"
	"log"
	"net/http"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"go.qbee.io/client"
	"go.qbee.io/connect/internal/model"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/ui/components"
	"go.qbee.io/connect/internal/ui/layouts"
)

//go:embed qbee-connect-icon.png
var trayIcon []byte

const (
	defaultWindowWidth  = 945
	defaultWindowHeight = 600
)

// App represents the main application structure
type App struct {
	// fyneApp is the main Fyne application instance
	fyneApp fyne.App

	// mainWin is the main application window
	mainWin fyne.Window

	// cli is the authenticated qbee client
	cli *client.Client

	// ctx is the application context
	ctx context.Context

	// store is the connection store
	store *service.ConnectionStore

	// deviceModel represents the device data model
	deviceModel *model.DeviceModel

	// deviceList is the table widget displaying devices
	deviceList *widget.Table

	// pageInfoLabel displays pagination information
	pageInfoLabel *widget.Label

	// loadingOverlay is the overlay shown during loading operations
	loadingOverlay *fyne.Container

	// mainWindowVisible indicates if the main window is currently visible
	mainWindowVisible atomic.Bool
}

// NewApp initializes the main application structure
func NewApp() *App {
	ctx := context.Background()
	cli, err := client.LoginGetAuthenticatedClient(ctx)
	if err != nil {
		cli, err = service.TerminalLogin()
		if err != nil {
			log.Fatalln("failed to login:", err)
		}
	}

	a := app.New()
	w := a.NewWindow("qbee-connect - qbee.io")

	if len(trayIcon) > 0 {
		w.SetIcon(fyne.NewStaticResource("icon", trayIcon))
	}

	store, err := service.NewConnectionStore(a.Storage())
	if err != nil {
		log.Fatalln("failed to initialize connection store:", err)
	}

	return &App{
		fyneApp:       a,
		mainWin:       w,
		cli:           cli,
		ctx:           ctx,
		store:         store,
		deviceModel:   model.NewDeviceModel(),
		pageInfoLabel: widget.NewLabel("Page 1 / 1"),
	}
}

// GetDeviceModel returns the device model
func (app *App) GetDeviceModel() *model.DeviceModel { return app.deviceModel }

// GetStore returns the connection store
func (app *App) GetStore() *service.ConnectionStore { return app.store }

// GetClient returns the authenticated client
func (app *App) GetClient() *client.Client { return app.cli }

// GetContext returns the application context
func (app *App) GetContext() context.Context { return app.ctx }

// GetWindow returns the main application window
func (app *App) GetWindow() fyne.Window { return app.mainWin }

// ShowConnectDialog displays the connection dialog for a device
func (app *App) ShowConnectDialog(item *client.InventoryListItem) {
	components.NewConnectDialog(app, item).Show()
}

// DisplayError shows an error notification
func (app *App) DisplayError(title, content string) {
	app.fyneApp.SendNotification(&fyne.Notification{Title: title, Content: content})
}

// Run starts the application
func (app *App) Run() {
	app.MakeTray()
	app.deviceList = components.NewDeviceTable(app)

	menu := components.MakeMenu()
	app.mainWin.SetMainMenu(menu)

	filterActive := widget.NewCheck("Open tunnels on page", func(checked bool) {
		app.deviceModel.ActiveTunnelsOnly = checked
		app.RedrawDeviceList()
	})

	setItemsPerPage := widget.NewSelect([]string{"10", "25", "50", "100"}, func(selected string) {
		var pp int
		if _, err := fmt.Sscanf(selected, "%d", &pp); err != nil {
			app.DisplayError("Invalid Selection", "Please select a valid number of items per page.")
			return
		}
		app.deviceModel.Query.ItemsPerPage = pp
		app.deviceModel.CurrentPage = 0
		app.RefreshUI()
	})
	setItemsPerPage.SetSelected("10")

	pagination := container.NewHBox(
		layout.NewSpacer(),
		widget.NewLabel("Items per page:"),
		setItemsPerPage,
		filterActive,
		widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			if app.deviceModel.CurrentPage > 0 {
				app.deviceModel.CurrentPage--
				app.RefreshUI()
			}
		}),
		app.pageInfoLabel,
		widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
			if app.deviceModel.CurrentPage < app.deviceModel.TotalPages()-1 {
				app.deviceModel.CurrentPage++
				app.RefreshUI()
			}
		}),
	)

	searchBar := components.NewSearchBar(app)

	content := container.NewBorder(
		searchBar,
		pagination, nil, nil,
		container.New(&layouts.RatioLayout{Table: app.deviceList}, app.deviceList),
	)

	// Loading Overlay
	loader := widget.NewProgressBarInfinite()
	overlay := canvas.NewRectangle(color.NRGBA{0, 0, 0, 180})
	app.loadingOverlay = container.NewStack(overlay, container.NewCenter(container.NewVBox(widget.NewLabel("Loading..."), loader)))
	app.loadingOverlay.Hide()

	app.mainWin.SetContent(container.NewStack(content, app.loadingOverlay))
	app.mainWin.SetCloseIntercept(func() { app.mainWin.Hide() })
	app.mainWin.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))
	app.mainWin.CenterOnScreen()
	app.mainWin.Show()

	// Initial data load
	app.fyneApp.Lifecycle().SetOnStarted(func() {
		app.mainWindowVisible.Store(true)
		app.RefreshUI()
	})

	app.fyneApp.Run()
}

// RefreshUI fetches device data and refreshes the UI
func (app *App) RefreshUI() {
	// If the main window is not visible, skip the refresh
	if !app.mainWindowVisible.Load() {
		return
	}
	app.loadingOverlay.Show()
	go func() {
		defer fyne.DoAndWait(func() { app.loadingOverlay.Hide() })

		if err := app.LoadDeviceData(); err != nil {
			app.DisplayError("Data Load Error", "Failed to load device data: "+err.Error())
			return
		}
		app.RedrawDeviceList()
	}()
}

// LoadDeviceData loads device data from the backend
func (app *App) LoadDeviceData() error {
	app.deviceModel.Query.Offset = app.deviceModel.CurrentPage * app.deviceModel.Query.ItemsPerPage
	devices, err := app.cli.ListDeviceInventory(app.ctx, *app.deviceModel.Query)
	if err != nil {
		return err
	}
	app.deviceModel.DeviceData = *devices
	return nil
}

// RedrawDeviceList applies current filters and refreshes the device list UI
func (app *App) RedrawDeviceList() {
	app.deviceModel.FilteredData = app.deviceModel.DeviceData
	if app.deviceModel.ActiveTunnelsOnly {
		var filtered []client.InventoryListItem
		for _, item := range app.deviceModel.FilteredData.Items {
			if _, ok := app.store.GetActive(item.NodeID); ok {
				filtered = append(filtered, item)
			}
		}
		app.deviceModel.FilteredData.Items = filtered
	}

	fyne.Do(func() {
		app.pageInfoLabel.SetText(fmt.Sprintf("Page %d / %d", app.deviceModel.CurrentPage+1, app.deviceModel.TotalPages()))
		app.deviceList.Refresh()
	})
}

// MakeTray creates a system tray icon with menu
func (app *App) MakeTray() {
	if desk, ok := app.fyneApp.(desktop.App); ok && len(trayIcon) > 0 {
		menu := fyne.NewMenu("qbee-connect",
			fyne.NewMenuItem("Show", func() { app.mainWin.Show() }),
			fyne.NewMenuItem("Quit", func() { app.fyneApp.Quit() }),
		)
		desk.SetSystemTrayIcon(fyne.NewStaticResource("icon", trayIcon))
		desk.SetSystemTrayMenu(menu)
	}
}

// SetSearchQuery sets the search query and refreshes the UI
func (app *App) SetSearchQuery(query client.InventoryListSearch) {
	app.deviceModel.Query.Search = query
	app.deviceModel.CurrentPage = 0
	app.RefreshUI()
}

// GetAllGroups fetches all groups from the backend
func (app *App) GetAllGroups() *client.GroupTree {
	allGroups, err := app.cli.GroupTreeGet(app.ctx, false)

	if err != nil {
		app.DisplayError("Group Fetch Error", "Failed to load groups: "+err.Error())
		return &client.GroupTree{}
	}
	return allGroups
}

const tagsListPath = "/api/v2/tagslist"

// GetAllTags fetches all tags from the backend
func (app *App) GetAllTags() []string {
	urlParams := make(map[string]string)
	urlParams["scope"] = "nodes"
	urlParams["format"] = "simple"

	tagsListQuery := fmt.Sprintf("%s?scope=%s&format=%s", tagsListPath, urlParams["scope"], urlParams["format"])

	allTags := make([]string, 0)
	err := app.cli.Call(app.ctx, http.MethodGet, tagsListQuery, nil, &allTags)
	if err != nil {
		app.DisplayError("Tag Fetch Error", "Failed to load tags: "+err.Error())
		return []string{}
	}
	return allTags
}

// GetFyneApp returns the underlying Fyne application instance
func (app *App) GetFyneApp() fyne.App {
	return app.fyneApp
}
