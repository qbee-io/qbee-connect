package ui

import (
	"context"
	_ "embed"
	"fmt"
	"image/color"
	"log"
	"net/http"

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

type App struct {
	FyneApp           fyne.App
	MainWin           fyne.Window
	Cli               *client.Client
	Ctx               context.Context
	Store             *service.ConnectionStore
	DeviceModel       *model.DeviceModel
	DeviceList        *widget.Table
	PageInfoLabel     *widget.Label
	LoadingOverlay    *fyne.Container
	MainWindowVisible bool
}

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

	return &App{
		FyneApp:       a,
		MainWin:       w,
		Cli:           cli,
		Ctx:           ctx,
		Store:         service.NewConnectionStore(a.Storage()),
		DeviceModel:   model.NewDeviceModel(),
		PageInfoLabel: widget.NewLabel("Page 1 / 1"),
	}
}

// Implement Delegate Interfaces
func (app *App) GetDeviceModel() *model.DeviceModel { return app.DeviceModel }
func (app *App) GetStore() *service.ConnectionStore { return app.Store }
func (app *App) GetClient() *client.Client          { return app.Cli }
func (app *App) GetContext() context.Context        { return app.Ctx }
func (app *App) GetWindow() fyne.Window             { return app.MainWin }
func (app *App) ShowConnectDialog(item *client.InventoryListItem) {
	components.NewConnectDialog(app, item).Show()
}
func (app *App) DisplayError(title, content string) {
	app.FyneApp.SendNotification(&fyne.Notification{Title: title, Content: content})
}

func (app *App) Run() {
	app.MakeTray()
	app.DeviceList = components.NewDeviceTable(app)

	menu := components.MakeMenu()
	app.MainWin.SetMainMenu(menu)

	// Initial Load
	app.LoadDeviceData()
	app.RedrawDeviceList()

	filterActive := widget.NewCheck("Open tunnels on page", func(checked bool) {
		app.DeviceModel.ActiveTunnelsOnly = checked
		app.RedrawDeviceList()
	})

	setItemsPerPage := widget.NewSelect([]string{"10", "25", "50", "100"}, func(selected string) {
		var pp int
		fmt.Sscanf(selected, "%d", &pp)
		app.DeviceModel.Query.ItemsPerPage = pp
		app.DeviceModel.CurrentPage = 0
		app.RefreshUI()
	})
	setItemsPerPage.SetSelected("10")

	pagination := container.NewHBox(
		layout.NewSpacer(),
		widget.NewLabel("Items per page:"),
		setItemsPerPage,
		filterActive,
		widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			if app.DeviceModel.CurrentPage > 0 {
				app.DeviceModel.CurrentPage--
				app.RefreshUI()
			}
		}),
		app.PageInfoLabel,
		widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
			if app.DeviceModel.CurrentPage < app.DeviceModel.TotalPages()-1 {
				app.DeviceModel.CurrentPage++
				app.RefreshUI()
			}
		}),
	)

	searchBar := components.NewSearchBar(app)

	content := container.NewBorder(
		searchBar,
		pagination, nil, nil,
		container.New(&layouts.RatioLayout{Table: app.DeviceList}, app.DeviceList),
	)

	// Loading Overlay
	loader := widget.NewProgressBarInfinite()
	overlay := canvas.NewRectangle(color.NRGBA{0, 0, 0, 180})
	app.LoadingOverlay = container.NewStack(overlay, container.NewCenter(container.NewVBox(widget.NewLabel("Loading..."), loader)))
	app.LoadingOverlay.Hide()

	app.MainWin.SetContent(container.NewStack(content, app.LoadingOverlay))
	app.MainWin.SetCloseIntercept(func() { app.MainWin.Hide() })
	app.MainWin.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))
	app.MainWin.CenterOnScreen()
	app.MainWin.Show()

	app.MainWindowVisible = true
	app.FyneApp.Run()
}

// RefreshUI fetches device data and refreshes the UI
func (app *App) RefreshUI() {
	if !app.MainWindowVisible {
		return
	}
	app.LoadingOverlay.Show()
	go func() {
		defer fyne.DoAndWait(func() { app.LoadingOverlay.Hide() })

		if err := app.LoadDeviceData(); err != nil {
			app.DisplayError("Data Load Error", "Failed to load device data: "+err.Error())
			return
		}
		app.RedrawDeviceList()
	}()
}

// LoadDeviceData loads device data from the backend
func (app *App) LoadDeviceData() error {
	app.DeviceModel.Query.Offset = app.DeviceModel.CurrentPage * app.DeviceModel.Query.ItemsPerPage
	devices, err := app.Cli.ListDeviceInventory(app.Ctx, *app.DeviceModel.Query)
	if err != nil {
		return err
	}
	app.DeviceModel.DeviceData = *devices
	return nil
}

// RedrawDeviceList applies current filters and refreshes the device list UI
func (app *App) RedrawDeviceList() {
	app.DeviceModel.FilteredData = app.DeviceModel.DeviceData
	if app.DeviceModel.ActiveTunnelsOnly {
		var filtered []client.InventoryListItem
		for _, item := range app.DeviceModel.FilteredData.Items {
			if _, ok := app.Store.GetActive(item.NodeID); ok {
				filtered = append(filtered, item)
			}
		}
		app.DeviceModel.FilteredData.Items = filtered
	}

	fyne.Do(func() {
		app.PageInfoLabel.SetText(fmt.Sprintf("Page %d / %d", app.DeviceModel.CurrentPage+1, app.DeviceModel.TotalPages()))
		app.DeviceList.Refresh()
	})
}

// MakeTray creates a system tray icon with menu
func (app *App) MakeTray() {
	if desk, ok := app.FyneApp.(desktop.App); ok && len(trayIcon) > 0 {
		menu := fyne.NewMenu("qbee-connect",
			fyne.NewMenuItem("Show", func() { app.MainWin.Show() }),
			fyne.NewMenuItem("Quit", func() { app.FyneApp.Quit() }),
		)
		desk.SetSystemTrayIcon(fyne.NewStaticResource("icon", trayIcon))
		desk.SetSystemTrayMenu(menu)
	}
}

// SetSearchQuery sets the search query and refreshes the UI
func (app *App) SetSearchQuery(query client.InventoryListSearch) {
	app.DeviceModel.Query.Search = query
	app.DeviceModel.CurrentPage = 0
	app.RefreshUI()
}

// GetAllGroups fetches all groups from the backend
func (app *App) GetAllGroups() *client.GroupTree {
	allGroups, err := app.Cli.GroupTreeGet(app.Ctx, false)

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
	err := app.Cli.Call(app.Ctx, http.MethodGet, tagsListQuery, nil, &allTags)
	if err != nil {
		app.DisplayError("Tag Fetch Error", "Failed to load tags: "+err.Error())
		return []string{}
	}
	return allTags
}

func (app *App) GetFyneApp() fyne.App {
	return app.FyneApp
}
