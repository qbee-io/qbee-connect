package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

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
		savedConns:     make(map[string][]client.RemoteAccessTarget),
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

	app.loadSavedConnections()

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
				var search client.InventoryListSearch
				if searchType == "Tag" {
					// Need to search by exact match or with * suffix
					search = client.InventoryListSearch{
						Tags: []string{searchTerm},
					}
				} else {
					search = client.InventoryListSearch{
						Title: searchTerm,
					}
				}
				app.deviceModel.currentPage = 0
				app.deviceModel.query.Search = search
				err := app.refreshDeviceListUI(pageInfoLabel)
				if err != nil {
					app.fyneApp.SendNotification(&fyne.Notification{
						Title:   "Device Fetch Error",
						Content: "Failed to load devices for search: " + err.Error(),
					})
				}
			}
			app.deviceModel.currentPage = 0
			err := app.refreshDeviceListUI(pageInfoLabel)
			if err != nil {
				app.fyneApp.SendNotification(&fyne.Notification{
					Title:   "Device Fetch Error",
					Content: "Failed to load devices for selected group: " + err.Error(),
				})
			}

			searchEntry.Refresh()
			return

		}
		if searchType == "Group" {

			allGroups, err := app.cli.GroupTreeGet(app.ctx, false)

			if err != nil {
				app.fyneApp.SendNotification(&fyne.Notification{
					Title:   "Group Fetch Error",
					Content: "Failed to load groups for search dropdown: " + err.Error(),
				})
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
				app.deviceModel.query.Search = client.InventoryListSearch{
					Ancestors: []string{groupMap[selected]},
				}
				app.deviceModel.currentPage = 0
				err := app.refreshDeviceListUI(pageInfoLabel)
				if err != nil {
					app.fyneApp.SendNotification(&fyne.Notification{
						Title:   "Device Fetch Error",
						Content: "Failed to load devices for selected group: " + err.Error(),
					})
				}
			})
			groupDropdown.SetSelectedIndex(0)
			searchEntry.RemoveAll()
			searchEntry.Add(groupDropdown)
			searchEntry.Refresh()
			err = app.refreshDeviceListUI(pageInfoLabel)
			if err != nil {
				app.fyneApp.SendNotification(&fyne.Notification{
					Title:   "Device Fetch Error",
					Content: "Failed to load devices for selected group: " + err.Error(),
				})
			}
			searchEntry.Refresh()
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

	app.deviceModel.query.Offset = app.deviceModel.currentPage * app.deviceModel.query.ItemsPerPage
	app.deviceModel.deviceData, err = app.loadDevices(app.ctx)
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
	//app.deviceList.Objects = app.deviceList.Objects[:0]
	app.deviceList.RemoveAll()

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
	groupLabel := NewLabelHover(strings.Join(d.AncestorsTitles[0:len(d.AncestorsTitles)-1], " > "))
	groupLabel.Truncation = fyne.TextTruncateEllipsis
	groupLabel.Alignment = fyne.TextAlignLeading

	// Tags (comma joined)
	tagsLabel := NewLabelHover(strings.Join(d.Tags, ", "))
	tagsLabel.Truncation = fyne.TextTruncateEllipsis
	tagsLabel.Alignment = fyne.TextAlignLeading

	var actionsBtn fyne.CanvasObject
	if _, ok := app.connectionsMap.get(d.NodeID); !ok {
		actionsBtn = widget.NewButton("Configure", func() {
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
