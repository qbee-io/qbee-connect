package components

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/model"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/ui/widgets"
)

// tableDelegate defines the methods required by the device table
type tableDelegate interface {
	RefreshUI()
	GetDeviceModel() *model.DeviceModel
	GetStore() *service.ConnectionStore
	ShowConnectDialog(item *client.InventoryListItem)
	ShowInfoDialog(item *client.InventoryListItem)
}

const (
	StatusOnline       = "online"
	StatusDelayed      = "delayed"
	StatusDisconnected = "disconnected"
)

var statusColors = map[string]fyne.ThemeColorName{
	StatusOnline:       theme.ColorNameSuccess,
	StatusDelayed:      theme.ColorNameWarning,
	StatusDisconnected: theme.ColorNameError,
}

var statusIcons = map[string]fyne.ThemeIconName{
	StatusOnline:       theme.IconNameConfirm,
	StatusDelayed:      theme.IconNameWarning,
	StatusDisconnected: theme.IconNameCancel,
}

// NewDeviceTable creates a new device table widget
func NewDeviceTable(d tableDelegate) *widget.Table {
	table := widget.NewTable(
		func() (int, int) {
			return len(d.GetDeviceModel().FilteredData.Items), len(model.DeviceColumns)
		},
		func() fyne.CanvasObject {
			return container.NewStack()
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			updateCell(d, id, obj)
		},
	)

	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject { return widgets.NewButtonPointer("", nil) }
	table.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		updateHeader(d, id, obj)
	}

	table.HideSeparators = true

	return table
}

func updateCell(d tableDelegate, id widget.TableCellID, obj fyne.CanvasObject) {

	cell := obj.(*fyne.Container)
	items := d.GetDeviceModel().FilteredData.Items
	if id.Row >= len(items) {
		cell.RemoveAll()
		return
	}

	item := items[id.Row]

	// Data Columns
	switch id.Col {
	case 0:
		updateTitleCell(d, cell, item)
	case 1:
		updateStatusCell(cell, item)
	case 2: // Connection Info Column
		updateConnectionStatusCell(d, cell, item)
	case 3:
		updateGroupCell(cell, item)
	case 4:
		updateTagsCell(cell, item)
	case 5:
		updateActionCell(d, cell, item)
	}
}

// updateTitleCell updates the title cell with the device's name
func updateTitleCell(d tableDelegate, cell *fyne.Container, item client.InventoryListItem) {
	cell.RemoveAll()
	titleLable := widgets.NewClickableLabel(item.Title, func() {
		// check if item has active connections
		if _, ok := d.GetStore().GetActive(item.NodeID); ok {
			d.ShowInfoDialog(&item)
		} else {
			d.ShowConnectDialog(&item)
		}
	})
	titleLable.Truncation = fyne.TextTruncateEllipsis
	cell.Add(titleLable)
}

// updateStatusCell updates the status cell with the device's online/offline status
func updateStatusCell(cell *fyne.Container, item client.InventoryListItem) {
	iconName, ok := statusIcons[item.Status]
	if !ok {
		iconName = theme.IconNameQuestion
	}

	colorName, ok := statusColors[item.Status]
	if !ok {
		colorName = theme.ColorNameDisabled
	}
	icon := theme.NewThemedResource(theme.Icon(iconName))
	icon.ColorName = colorName

	cell.RemoveAll()
	cell.Add(widget.NewIcon(icon))
}

// updateTagsCell updates the tags cell with the device's tags
func updateTagsCell(cell *fyne.Container, item client.InventoryListItem) {
	tags := "-"
	if len(item.Tags) > 0 {
		tags = strings.Join(item.Tags, ", ")
	}
	updateLabelCell(cell, tags)
}

// updateGroupCell updates the group cell with the device's group hierarchy
func updateGroupCell(cell *fyne.Container, item client.InventoryListItem) {
	text := ""
	if len(item.AncestorsTitles) > 0 {
		text = strings.Join(item.AncestorsTitles[:len(item.AncestorsTitles)-1], " > ")
	}
	updateLabelCell(cell, text)
}

// updateConnectionStatusCell updates the connection status cell with current connection info
func updateConnectionStatusCell(d tableDelegate, cell *fyne.Container, item client.InventoryListItem) {
	connectionCount := updateConnectionStatus(d, item)

	if connectionCount == 0 {
		updateLabelCell(cell, "-")
		return
	}

	button := widgets.NewButtonPointer("", nil)
	text := fmt.Sprintf("%d active", connectionCount)
	label := widget.NewLabel(text)

	button.SetIcon(theme.InfoIcon())
	button.OnTapped = func() {
		d.ShowInfoDialog(&item)
	}

	cell.RemoveAll()
	cell.Add(container.NewBorder(nil, nil, label, button))
}

// updateLabelCell updates a cell with a simple text label
func updateLabelCell(cell *fyne.Container, text string) {
	cell.RemoveAll()
	label := widget.NewLabel(text)
	label.Truncation = fyne.TextTruncateEllipsis
	cell.Add(label)
}

// updateActionCell updates the action cell with the appropriate button based on connection status
func updateActionCell(d tableDelegate, cell *fyne.Container, item client.InventoryListItem) {
	conn, ok := d.GetStore().GetActive(item.NodeID)

	icon := theme.SettingsIcon()
	tapped := func() { d.ShowConnectDialog(&item) }
	if ok {
		icon = theme.CancelIcon()
		tapped = func() {
			conn.Cancel()
			d.GetStore().DeleteActive(item.NodeID)
			d.RefreshUI()
		}
	}

	cell.RemoveAll()

	btn := widgets.NewButtonPointer("", tapped)
	btn.SetIcon(icon)
	cell.Add(btn)
}

// updateHeader updates the header cell with the column title and sorting functionality
func updateHeader(d tableDelegate, id widget.TableCellID, obj fyne.CanvasObject) {
	if id.Col < 0 {
		return
	}
	if id.Col >= len(model.DeviceColumns) {
		return
	}
	btn := obj.(*widgets.ButtonPointer)
	col := model.DeviceColumns[id.Col]

	if col.Title == "" {
		btn.Hide()
		return
	}
	btn.Show()
	btn.SetText(col.Title)

	if !col.Sortable {
		btn.SetCursor(desktop.DefaultCursor)
		return
	}

	q := d.GetDeviceModel().Query
	if q.SortField == col.SortKey {
		if q.SortDirection == client.SortDirectionAsc {
			btn.SetIcon(theme.MoveUpIcon())
		} else {
			btn.SetIcon(theme.MoveDownIcon())
		}
	} else {
		btn.SetIcon(nil)
	}
	btn.OnTapped = func() {
		if q.SortField == col.SortKey {
			if q.SortDirection == client.SortDirectionAsc {
				q.SortDirection = client.SortDirectionDesc
			} else {
				q.SortDirection = client.SortDirectionAsc
			}
		} else {
			q.SortField = col.SortKey
			q.SortDirection = client.SortDirectionAsc
		}
		d.RefreshUI()
	}
}

// updateConnectionStatus returns the formatted connection information for a device
func updateConnectionStatus(d tableDelegate, item client.InventoryListItem) int {
	var activeConn *service.DeviceConnections
	var ok bool

	if activeConn, ok = d.GetStore().GetActive(item.NodeID); !ok {
		return 0
	}

	return len(activeConn.Targets)
}
