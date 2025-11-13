package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"go.qbee.io/client"
)

// page state
type deviceModel struct {
	allDevices   []client.InventoryListItem
	filtered     []client.InventoryListItem
	pageSize     int
	currentPage  int
	searchFilter string
}

type deviceConnections struct {
}

func newDeviceModel() *deviceModel {
	return &deviceModel{
		allDevices: []client.InventoryListItem{},
		filtered:   []client.InventoryListItem{},
		pageSize:   10,

		currentPage: 0,
	}
}

func (m *deviceModel) applyFilter() {
	if m.searchFilter == "" {
		m.filtered = append([]client.InventoryListItem(nil), m.allDevices...)
	} else {
		filter := strings.ToLower(m.searchFilter)
		m.filtered = m.filtered[:0]
		for _, d := range m.allDevices {
			if strings.Contains(strings.ToLower(d.Title), filter) {
				m.filtered = append(m.filtered, d)
			}
		}
	}

	// sort by name like in the UI screenshot
	sort.Slice(m.filtered, func(i, j int) bool {
		return strings.ToLower(m.filtered[i].Title) < strings.ToLower(m.filtered[j].Title)
	})

	m.currentPage = 0
}

func (m *deviceModel) totalPages() int {
	if len(m.filtered) == 0 {
		return 1
	}
	pages := len(m.filtered) / m.pageSize
	if len(m.filtered)%m.pageSize != 0 {
		pages++
	}
	return pages
}

func (m *deviceModel) pageDevices() []client.InventoryListItem {
	if len(m.filtered) == 0 {
		return nil
	}
	start := m.currentPage * m.pageSize
	if start >= len(m.filtered) {
		start = 0
	}
	end := start + m.pageSize
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	return m.filtered[start:end]
}

// loadDevices calls qbee-cli and unmarshals the JSON.
// Adjust the command and JSON schema to match your environment.
func loadDevices(ctx context.Context, offset, itemsPerPage int) (*client.InventoryListResponse, error) {
	// Example: qbee-cli devices list --json
	cli, err := client.LoginGetAuthenticatedClient(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to create qbee client: %w", err)
	}

	query := client.InventoryListQuery{
		Search: client.InventoryListSearch{
			Title: "",
		},
		SortField:     "title",
		SortDirection: client.SortDirectionAsc,
		ReportType:    "short",
		Offset:        offset,
		ItemsPerPage:  itemsPerPage,
	}

	devices, err := cli.ListDeviceInventory(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error fetching device list: %w", err)
	}
	return devices, nil
}

func main() {
	a := app.New()
	w := a.NewWindow("qbee-connect - qbee.io")

	model := newDeviceModel()

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

	// ---- Device list container (will be replaced on pagination / refresh) ----
	deviceListContainer := container.NewVBox()

	// ---- Pagination controls ----
	pageInfoLabel := widget.NewLabel("Page 1 / 1")

	prevButton := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if model.currentPage > 0 {
			model.currentPage--
			refreshDeviceListUI(w, model, deviceListContainer, pageInfoLabel)
		}
	})
	nextButton := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		if model.currentPage < model.totalPages()-1 {
			model.currentPage++
			refreshDeviceListUI(w, model, deviceListContainer, pageInfoLabel)
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
		deviceListContainer,
	)

	root := container.NewBorder(topBar, nil, nil, nil, content)
	w.SetContent(root)

	// ---- Handlers ----
	ctx := context.Background()
	refresh := func() {
		devices, err := loadDevices(ctx, 0, 100)
		if err != nil {
			log.Println("failed to load devices:", err)
			dialog := widget.NewPopUp(
				widget.NewLabel("Failed to load devices. Please try again."),
				w.Canvas(),
			)
			dialog.Show()
			return
		}

		model.allDevices = devices.Items
		model.applyFilter()
		refreshDeviceListUI(w, model, deviceListContainer, pageInfoLabel)
	}

	refreshButton.OnTapped = refresh

	searchEntry.OnChanged = func(s string) {
		model.searchFilter = s
		model.applyFilter()
		refreshDeviceListUI(w, model, deviceListContainer, pageInfoLabel)
	}

	// run in goroutine to avoid locking UI
	devices, err := loadDevices(ctx, 0, 100)
	if err != nil {
		log.Println("initial load error:", err)
		return
	}
	model.allDevices = devices.Items
	model.applyFilter()
	refreshDeviceListUI(w, model, deviceListContainer, pageInfoLabel)

	// ---- Show window ----
	w.ShowAndRun()
}

// refreshDeviceListUI rebuilds the list for the current page.
func refreshDeviceListUI(w fyne.Window, model *deviceModel, list *fyne.Container, pageInfo *widget.Label) {
	list.Objects = list.Objects[:0]

	devs := model.pageDevices()
	if len(devs) == 0 {
		list.Add(widget.NewLabel("No devices found"))
		list.Refresh()
		pageInfo.SetText("Page 1 / 1")
		return
	}

	for _, d := range devs {
		row := buildDeviceRow(w, d)
		list.Add(row)
	}

	list.Refresh()
	pageInfo.SetText(
		fmt.Sprintf("Page %d / %d", model.currentPage+1, model.totalPages()),
	)
}

// headerLabel is a bold header cell for the grid header row.
func headerLabel(text string) *widget.Label {
	lbl := widget.NewLabel(text)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	return lbl
}

// buildDeviceRow builds a single row similar to the qbee web UI.
func buildDeviceRow(w fyne.Window, d client.InventoryListItem) fyne.CanvasObject {
	// Device name (left aligned)
	deviceLabel := widget.NewLabel(d.Title)

	// Status: colored circle + text
	statusCircle := canvas.NewCircle(statusColor(d.Status))
	statusCircle.Resize(fyne.NewSize(10, 10))
	statusText := widget.NewLabel(strings.Title(d.Status))
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

	// Actions (placeholder "⋮")
	actionsBtn := widget.NewButton("Connect", func() {
		// Create form fields
		localPortEntry := widget.NewEntry()
		localPortEntry.SetPlaceHolder("Local port")

		localAddrEntry := widget.NewEntry()
		localAddrEntry.SetPlaceHolder("Local address")
		localAddrEntry.SetText("127.0.0.1")

		remotePortEntry := widget.NewEntry()
		remotePortEntry.SetPlaceHolder("Remote port")

		remoteAddrEntry := widget.NewEntry()
		remoteAddrEntry.SetPlaceHolder("Remote address")
		remoteAddrEntry.SetText("127.0.0.1")

		protocolSelect := widget.NewSelect([]string{"tcp", "udp"}, func(value string) {})
		protocolSelect.SetSelected("tcp")

		// Container for multiple targets
		targetsContainer := container.NewVBox()

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
						targetsContainer.Refresh()
						break
					}
				}
			}

			return row
		}

		// Add initial target row
		targetsContainer.Add(createTargetRow())

		// Add plus button to add more targets
		addBtn := widget.NewButtonWithIcon("Add Target", theme.ContentAddIcon(), func() {
			targetsContainer.Add(createTargetRow())
		})

		// Headers
		headers := container.NewGridWithColumns(6,
			widget.NewLabel("Local Port"),
			widget.NewLabel("Local Address"),
			widget.NewLabel("Remote Port"),
			widget.NewLabel("Remote Address"),
			widget.NewLabel("Protocol"),
			widget.NewLabel(""),
		)

		// Form content
		formContent := container.NewVBox(
			widget.NewLabel("Configure port forwarding for "+d.Title),
			widget.NewSeparator(),
			headers,
			container.NewScroll(targetsContainer),
			addBtn,
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
			w.Canvas(),
		)

		dialog.Resize(fyne.NewSize(800, 500))

		// Dialog buttons
		connectBtn := widget.NewButton("Connect", func() {

			// extract target configurations
			var targets []string
			for _, obj := range targetsContainer.Objects {
				if row, ok := obj.(*fyne.Container); ok {
					localPort := row.Objects[0].(*widget.Entry).Text
					localAddr := row.Objects[1].(*widget.Entry).Text
					remotePort := row.Objects[2].(*widget.Entry).Text
					remoteAddr := row.Objects[3].(*widget.Entry).Text
					protocol := row.Objects[4].(*widget.Select).Selected

					target := fmt.Sprintf("%s:%s:%s:%s", localAddr, localPort, remoteAddr, remotePort)

					if protocol == "udp" {
						target += "/udp"
					}
					targets = append(targets, target)
				}
			}

			// Here you would initiate the port forwarding using the collected targets
			log.Println("Starting port forwarding for device:", d.NodeID)

			for _, t := range targets {

				log.Println("Target:", t)
			}
			ctx := context.Background()
			cli, err := client.LoginGetAuthenticatedClient(ctx)

			if err != nil {
				return
			}

			go func() {
				err := cli.ParseConnectRetry(ctx, d.NodeID, targets, 1)

				if err != nil {
					log.Println("Port forwarding error:", err)
				}
			}()

			// Close the dialog after starting port forwarding
			dialog.Hide()
		})

		controls.Add(connectBtn)

		cancelBtn := widget.NewButton("Cancel", func() {})
		controls.Add(cancelBtn)

		cancelBtn.OnTapped = func() {
			dialog.Hide()
		}

		dialog.Show()
	})

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
