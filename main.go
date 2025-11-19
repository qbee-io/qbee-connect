package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
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
	"golang.org/x/term"
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
	deviceData        *client.InventoryListResponse
	pageSize          int
	currentPage       int
	offset            int
	search            *client.InventoryListSearch
	activeTunnelsOnly bool
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

func promptUsernamePassword() (string, string, error) {
	var email string
	fmt.Print("Email: ")
	_, err := fmt.Scanln(&email)
	if err != nil {
		return "", "", err
	}

	fmt.Print("Password: ")
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", "", err
	}
	password := string(bytePassword)
	fmt.Println() // Move to next line after password input

	return email, password, nil
}

func newApp() *App {
	ctx := context.Background()
	var cli *client.Client
	var err error
	cli, err = client.LoginGetAuthenticatedClient(ctx)

	if err != nil {

		baseURL := os.Getenv("QBEE_BASEURL")
		if baseURL == "" {
			baseURL = "https://www.app.qbee.io"
		}

		username, password, err := promptUsernamePassword()
		if err != nil {
			log.Fatalln("Failed to read username/password:", err)
		}

		// read username from stdin and do interactive login

		cli = client.New().WithBaseURL(baseURL)
		err = cli.Authenticate(ctx, username, password)
		if err != nil {
			log.Fatalln("Failed to authenticate qbee client:", err)
		}

		config := &client.LoginConfig{
			BaseURL:      baseURL,
			AuthToken:    cli.GetAuthToken(),
			RefreshToken: cli.GetRefreshToken(),
		}
		err = client.LoginWriteConfig(*config)
		if err != nil {
			log.Fatalln("Failed to write login config:", err)
		}
	}

	container.NewHBox()
	a := app.NewWithID("io.qbee.qbee-connect")
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
	}
}

func (app *App) getGroupsBreadcrumb(groups client.GroupTree) map[string]string {

	result := make(map[string]string)

	var traverse func(node *client.GroupTreeNode, path []string)
	traverse = func(node *client.GroupTreeNode, path []string) {
		if node == nil {
			return
		}

		// Append current node title to path
		newPath := append(path, node.Title)

		// Recurse for each child
		for _, child := range node.Nodes {
			if child.Type == client.NodeTypeGroup {
				traverse(&child, newPath)
			}
		}
		if node.Type == client.NodeTypeGroup {
			result[strings.Join(newPath, " > ")] = node.NodeID
			return
		}
	}

	traverse(&groups.Tree, []string{})

	return result
}

func main() {

	app := newApp()
	w := app.mainWin

	app.makeTray()
	app.mainWin.SetMainMenu(app.makeMenu())

	w.SetIcon(fyne.NewStaticResource("qbee-connect-icon.png", trayIcon))

	// ---- Top toolbar (search + buttons) ----

	searchEntry := container.NewVBox()

	// drop down of search types could be added here
	// ---- Pagination controls ----
	pageInfoLabel := widget.NewLabel("Page 1 / 1")

	deviceFetchError := widget.NewPopUp(
		widget.NewLabel("Failed to load devices. Please try again."),
		w.Canvas(),
	)
	deviceFetchError.Hide()

	searchBy := widget.NewSelect([]string{"Device name", "Group", "Tag"}, func(searchType string) {
		if searchType == "Device name" || searchType == "Tag" {
			textSearch := widget.NewEntry()
			textSearch.SetPlaceHolder("Search by " + searchType)
			searchEntry.RemoveAll()
			searchEntry.Add(textSearch)

			textSearch.OnSubmitted = func(searchTerm string) {
				if searchType == "Tag" {
					// Need to search by exact match or with * suffix
					app.deviceModel.search = &client.InventoryListSearch{
						Tags: []string{searchTerm},
					}
				} else {
					app.deviceModel.search = &client.InventoryListSearch{
						Title: searchTerm,
					}
				}
				app.deviceModel.currentPage = 0
				err := app.refreshDeviceListUI(pageInfoLabel)
				if err != nil {
					deviceFetchError.Show()
					log.Println("refresh error:", err)
				} else {
					deviceFetchError.Hide()
				}
			}
			app.deviceModel.currentPage = 0
			err := app.refreshDeviceListUI(pageInfoLabel)
			if err != nil {
				deviceFetchError.Show()
				log.Println("refresh error:", err)
			} else {
				deviceFetchError.Hide()
			}
			searchEntry.Refresh()
			return

		}
		if searchType == "Group" {

			allGroups, err := app.cli.GroupTreeGet(app.ctx, false)

			if err != nil {
				log.Println("failed to load groups for search dropdown:", err)
				return
			}

			// get breadcrumb strings
			groupMap := app.getGroupsBreadcrumb(*allGroups)

			groupBreadCrumbs := make([]string, 0, len(groupMap))

			for k := range groupMap {
				groupBreadCrumbs = append(groupBreadCrumbs, k)
			}

			sort.Strings(groupBreadCrumbs)
			// implement search by group
			groupDropdown := widget.NewSelect(groupBreadCrumbs, func(selected string) {
				// set search filter to selected group
				app.deviceModel.search = &client.InventoryListSearch{
					Ancestors: []string{groupMap[selected]},
				}
				app.deviceModel.currentPage = 0
				err := app.refreshDeviceListUI(pageInfoLabel)
				if err != nil {
					log.Println("refresh error:", err)
				}
			})
			groupDropdown.SetSelectedIndex(0)
			searchEntry.RemoveAll()
			searchEntry.Add(groupDropdown)
			searchEntry.Refresh()
			err = app.refreshDeviceListUI(pageInfoLabel)
			if err != nil {
				deviceFetchError.Show()
				log.Println("refresh error:", err)
			} else {
				deviceFetchError.Hide()
			}
			return
		}
		// implement different search types if needed
	})
	searchBy.SetSelected("Device name")

	refreshButton := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {})

	filterActiveTunnels := widget.NewCheck("Active tunnels only", func(checked bool) {
		app.deviceModel.currentPage = 0
		app.deviceModel.activeTunnelsOnly = checked
		err := app.refreshDeviceListUI(pageInfoLabel)
		if err != nil {
			deviceFetchError.Show()
			log.Println("refresh error:", err)
		} else {
			deviceFetchError.Hide()
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
			app.refreshDeviceListUI(pageInfoLabel)

		}
	})
	nextButton := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		if app.deviceModel.currentPage < app.deviceModel.totalPages()-1 {
			app.deviceModel.currentPage++
			app.refreshDeviceListUI(pageInfoLabel)
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

	// ---- Handlers ----
	refresh := func() {
		err := app.refreshDeviceListUI(pageInfoLabel)
		if err != nil {
			deviceFetchError.Show()
			log.Println("refresh error:", err)
		} else {
			deviceFetchError.Hide()
		}
	}

	refreshButton.OnTapped = refresh

	err := app.refreshDeviceListUI(pageInfoLabel)
	if err != nil {
		deviceFetchError.Show()
		log.Println("refresh error:", err)
	} else {
		deviceFetchError.Hide()
	}

	w.SetCloseIntercept(func() {
		w.Hide()
	})

	w.Resize(fyne.NewSize(800, 600))
	// ---- Show window ----
	w.ShowAndRun()
}

// refreshDeviceListUI rebuilds the list for the current page.
func (app *App) refreshDeviceListUI(pageInfoLabel *widget.Label) error {
	var err error

	app.deviceModel.offset = app.deviceModel.currentPage * app.deviceModel.pageSize
	app.deviceModel.deviceData, err = app.loadDevices(app.ctx, app.deviceModel.search, app.deviceModel.offset, app.deviceModel.pageSize)
	if err != nil {
		log.Println("failed to load devices for UI refresh:", err)
		return err
	}

	app.redrawDeviceList()

	pageInfoLabel.SetText(
		fmt.Sprintf("Page %d / %d", app.deviceModel.currentPage+1, app.deviceModel.totalPages()),
	)

	return nil
}

func (app *App) redrawDeviceList() {
	app.deviceList.Objects = app.deviceList.Objects[:0]

	for _, d := range app.deviceModel.deviceData.Items {

		if app.deviceModel.activeTunnelsOnly {
			if _, ok := app.connectionsMap.get(d.NodeID); !ok {
				continue
			}
		}

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

	statusCircle := widget.NewIcon(statusIcon(d.Status))

	statusText := widget.NewLabel(cases.Title(language.English).String(d.Status))
	statusBox := container.NewHBox(
		statusCircle,
		statusText,
	)

	// Group (breadcrumb style). Skop last element (device name)
	groupLabel := NewMyHoverableWidget(strings.Join(d.AncestorsTitles[0:len(d.AncestorsTitles)-1], " > "))
	groupLabel.Truncation = fyne.TextTruncateEllipsis
	groupLabel.Alignment = fyne.TextAlignLeading

	// Tags (comma joined)
	tagsLabel := NewMyHoverableWidget(strings.Join(d.Tags, ", "))
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

// MyHoverableWidget is a custom widget that implements desktop.Hoverable
type MyHoverableWidget struct {
	widget.Label
	isHovered bool
	popup     *widget.PopUp
}

// NewMyHoverableWidget creates a new instance of MyHoverableWidget
func NewMyHoverableWidget(text string) *MyHoverableWidget {
	w := &MyHoverableWidget{}
	w.ExtendBaseWidget(w)
	w.SetText(text)
	return w
}

// MouseIn is called when the mouse enters the widget
func (w *MyHoverableWidget) MouseIn(*desktop.MouseEvent) {

	w.isHovered = true

	// wait a moment to avoid flickering

	// show a tooltip
	// Create and show tooltip
	if w.Text != "" {
		tooltip := widget.NewCard("", w.Text, nil)
		popup := widget.NewPopUp(tooltip, fyne.CurrentApp().Driver().CanvasForObject(w))
		position := fyne.CurrentApp().Driver().AbsolutePositionForObject(w)
		// show the popup just above the widget
		position.Y -= tooltip.MinSize().Height + 5

		// Store popup reference to hide it on MouseOut
		w.popup = popup

		time.AfterFunc(200*time.Millisecond, func() {
			if w.isHovered {
				popup.ShowAtPosition(position)
			}
		})
	}

	w.Refresh()
}

// MouseOut is called when the mouse exits the widget
func (w *MyHoverableWidget) MouseOut() {
	w.isHovered = false
	// hide tooltip
	if w.popup != nil {
		w.popup.Hide()
		w.popup = nil
	}
	w.Refresh()
}

// MouseMoved is called when the mouse moves within the widget
func (w *MyHoverableWidget) MouseMoved(*desktop.MouseEvent) {
	// You can implement custom logic here if needed
}
