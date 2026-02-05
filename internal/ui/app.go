package ui

import (
	"context"
	_ "embed" // for tray icon embedding
	"flag"
	"fmt"
	"image/color"
	"log"
	"sort"
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

//go:embed qbee-connect.png
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

	// mainContent holds the main content of the application
	mainContent fyne.CanvasObject

	// tabs & active connections
	activeTabs *container.AppTabs
	activeView fyne.CanvasObject
	activeTab  *container.TabItem

	// deviceModel represents the device data model
	deviceModel *model.DeviceModel

	// deviceList is the table widget displaying devices
	deviceList *widget.Table

	// pageInfoLabel displays pagination information
	pageInfoLabel *widget.Label

	// loadingOverlay is the overlay shown during loading operations
	loadingOverlay *fyne.Container

	// loadingProgressBar indicates loading progress
	loadingProgressBar *widget.ProgressBarInfinite

	// mainWindowVisible indicates if the main window is currently visible
	mainWindowVisible atomic.Bool

	// accountSwitcher allows switching between tenant accounts the user has access to
	accountSwitcher *fyne.Container

	// userData
	user *model.User
}

// NewApp initializes the main application structure
func NewApp() *App {

	// read flags for base URL override
	var baseURLFlag = flag.String("base-url", "", "Override the default base URL")
	flag.Parse()

	baseURL := *baseURLFlag

	ctx := context.Background()

	a := app.New()
	w := a.NewWindow("qbee-connect - qbee.io")

	if len(trayIcon) > 0 {
		w.SetIcon(fyne.NewStaticResource("icon", trayIcon))
	}

	store, err := service.NewConnectionStore(a.Storage())
	if err != nil {
		log.Fatalln("failed to initialize connection store:", err)
	}

	cli, err := client.LoginGetAuthenticatedClient(ctx)
	if err != nil {
		cli = client.New()
	}

	if baseURL != "" && baseURL != cli.GetBaseURL() {
		cli = client.New().WithBaseURL(baseURL)
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

// ShowInfoDialog displays the device information dialog
func (app *App) ShowInfoDialog(item *client.InventoryListItem) {
	components.NewDeviceInfoDialog(app, item).Show()
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

		app.deviceModel.CurrentPage = 0
		app.deviceModel.Query.ItemsPerPage = pp

		app.RefreshUI()

	})
	setItemsPerPage.SetSelected("10")

	app.accountSwitcher = container.NewHBox()

	pagination := container.NewHBox(
		app.accountSwitcher,
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

	if err := app.SetLoggedIn(); err != nil {
		// Not logged in yet, show login dialog
		components.NewLoginDialog(app)
	}

	searchBar := components.NewSearchBar(app)

	content := container.NewBorder(
		searchBar,
		pagination, nil, nil,
		container.New(&layouts.RatioLayout{Table: app.deviceList}, app.deviceList),
	)

	// Loading Overlay
	app.loadingProgressBar = widget.NewProgressBarInfinite()
	overlay := canvas.NewRectangle(color.NRGBA{0, 0, 0, 180})

	inputBlocker := layouts.NewBlocker()
	overlayContainer := container.NewStack(overlay, container.NewCenter(container.NewVBox(widget.NewLabel("Loading..."), app.loadingProgressBar)))

	app.loadingOverlay = container.NewStack(inputBlocker, overlayContainer)

	// Initial hide of loading overlay, stop the progress bar to avoid CPU usage
	app.loadingProgressBar.Stop()
	app.loadingOverlay.Hide()

	// Build tabs: Devices + Active (count)
	app.activeView = components.NewActiveConnections(app)
	app.activeTab = container.NewTabItem("Active (0)", container.NewVScroll(app.activeView))
	devicesTab := container.NewTabItem("Devices", content)
	app.activeTabs = container.NewAppTabs(devicesTab, app.activeTab)
	app.activeTabs.SetTabLocation(container.TabLocationTop)
	// Initial title update
	count := len(app.store.SnapshotActive())
	app.activeTab.Text = fmt.Sprintf("Active (%d)", count)

	app.mainContent = container.NewStack(app.activeTabs, app.loadingOverlay)
	app.mainWin.SetContent(app.mainContent)

	app.mainWin.SetCloseIntercept(func() { app.mainWindowVisible.Store(false); app.mainWin.Hide() })
	app.mainWin.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))
	app.mainWin.CenterOnScreen()

	// Initial data load
	app.fyneApp.Lifecycle().SetOnStarted(func() {
		app.mainWindowVisible.Store(true)
		app.RefreshUI()
	})

	app.mainWin.ShowAndRun()
}

// RefreshUI() refreshes the UI with device data
func (app *App) RefreshUI() {
	app.LoadDevicesAndRefreshUI(true)
}

// RefreshUINoLoad refreshes the UI without loading device data
func (app *App) RefreshUINoLoad() {
	app.LoadDevicesAndRefreshUI(false)
}

// RefreshUI fetches device data and refreshes the UI
func (app *App) LoadDevicesAndRefreshUI(loadDevices bool) {
	// If the main window is not visible, skip the refresh
	if !app.mainWindowVisible.Load() {
		return
	}

	if app.user == nil {
		loginBtn := widget.NewButton("Log In", func() {
			components.NewLoginDialog(app)
		})
		loginContainer := container.NewCenter(loginBtn)
		app.mainWin.SetContent(loginContainer)
		return
	}

	app.mainWin.SetContent(app.mainContent)

	// Start the loading indicator
	app.loadingProgressBar.Start()
	app.loadingOverlay.Show()

	// Load device data in a separate goroutine
	go func() {
		defer fyne.Do(func() {

			components.UpdateActiveConnectionsView(app, app.activeView)

			// Update active connections count in tab title
			count := len(app.store.SnapshotActive())
			app.activeTab.Text = fmt.Sprintf("Active (%d)", count)
			app.activeTabs.Refresh()

			app.RedrawDeviceList()

			// Stop loading indicator to avoid CPU usage
			app.loadingProgressBar.Stop()
			app.loadingOverlay.Hide()

		})

		if !loadDevices {
			return
		}
		app.GetDevices()
	}()
}

// RedrawDeviceList applies current filters and refreshes the device list UI
func (app *App) RedrawDeviceList() {

	//app.deviceModel.FilteredData = app.deviceModel.DeviceData
	var filtered []client.InventoryListItem

	if app.deviceModel.ActiveTunnelsOnly {
		for _, item := range app.deviceModel.FilteredData.Items {
			if _, ok := app.store.GetActive(item.NodeID); ok {
				filtered = append(filtered, item)
			}
		}
	} else {
		filtered = app.deviceModel.DeviceData.Items
	}

	// Determine if we need to scroll to top if the new filtered list is smaller
	// than the previous one
	if len(filtered) < len(app.deviceModel.FilteredData.Items) {
		app.deviceList.ScrollToTop()
	}

	app.deviceModel.FilteredData.Items = filtered
	app.pageInfoLabel.SetText(fmt.Sprintf("Page %d / %d", app.deviceModel.CurrentPage+1, app.deviceModel.TotalPages()))

	app.deviceList.Refresh()
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

// GetFyneApp returns the underlying Fyne application instance
func (app *App) GetFyneApp() fyne.App {
	return app.fyneApp
}

// SetLoggedOut clears the user data
func (app *App) SetLoggedOut() {
	app.user = nil
}

// SetLoggedIn sets the user data after successful login
func (app *App) SetLoggedIn() error {
	user, err := app.GetUser()
	if err != nil {
		return err
	}
	app.user = user

	if len(app.user.Accounts) < 2 {
		// No account switcher needed
		return nil
	}

	accountMap := make(map[string]string)
	accountSelector := make([]string, 0)

	defaultAccount := ""
	for _, acc := range app.user.Accounts {
		accountSelector = append(accountSelector, acc.Name)
		accountMap[acc.Name] = acc.ID
		if user.User.AccountID == acc.ID {
			defaultAccount = acc.Name
		}
	}

	sort.Strings(accountSelector)

	accountSwitchSelect := widget.NewSelect(accountSelector, func(selected string) {
		if selected == "" {
			return
		}

		id, ok := accountMap[selected]
		if !ok {
			app.DisplayError("Account Switch Error", "Selected account not found.")
			return
		}

		if id == app.user.User.AccountID {
			// No change
			return
		}

		if err := app.SwitchAccount(id); err != nil {
			app.DisplayError("Account Switch Error", err.Error())
			return
		}

		user, err := app.GetUser()
		if err != nil {
			app.DisplayError("Account Switch Error", "Failed to load user data: "+err.Error())
			return
		}
		app.user = user

		app.RefreshUI()
	})

	accountSwitchSelect.SetSelected(defaultAccount)

	app.accountSwitcher.Objects = []fyne.CanvasObject{
		widget.NewLabel("Account:"),
		accountSwitchSelect,
	}
	fyne.Do(func() {
		app.accountSwitcher.Refresh()
	})

	return nil
}
