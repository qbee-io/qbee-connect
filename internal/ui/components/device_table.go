package components

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/model"
	"go.qbee.io/connect/internal/service"
)

// tableDelegate defines the methods required by the device table
type tableDelegate interface {
	RefreshUI()
	GetDeviceModel() *model.DeviceModel
	GetStore() *service.ConnectionStore
	ShowConnectDialog(item *client.InventoryListItem)
}

// NewDeviceTable creates a new device table widget
func NewDeviceTable(d tableDelegate) *widget.Table {
	table := widget.NewTable(
		func() (int, int) {
			return len(d.GetDeviceModel().FilteredData.Items), len(model.DeviceColumns)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			return container.NewStack(widget.NewButton("", nil), label)
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			updateCell(d, id, obj)
		},
	)

	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject { return widget.NewButton("", nil) }
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
		return
	}
	item := items[id.Row]

	// Data Columns
	if id.Col < len(model.DeviceColumns) {
		switch id.Col {
		case 0:
			updateTitleCell(cell, item)
		case 1:
			updateStatusCell(cell, item)
		case 2:
			updateGroupCell(cell, item)
		case 3:
			updateTagsCell(cell, item)
		case 4: // Connection Info Column
			updateConnectionStatusCell(d, cell, item)
		case 5:
			updateActionCell(d, cell, item)
		}
	}
}

func updateTitleCell(cell *fyne.Container, item client.InventoryListItem) {
	cell.RemoveAll()
	title := widget.NewLabel(item.Title)
	title.Truncation = fyne.TextTruncateEllipsis
	cell.Add(title)
}

// updateStatusCell updates the status cell with the device's online/offline status
func updateStatusCell(cell *fyne.Container, item client.InventoryListItem) {
	cell.RemoveAll()
	var statusIcon fyne.Resource
	var colorName fyne.ThemeColorName

	if item.Status == "online" {
		statusIcon = theme.ConfirmIcon()
		colorName = theme.ColorNameSuccess
	} else {
		statusIcon = theme.CancelIcon()
		colorName = theme.ColorNameError
	}
	themeResource := theme.NewThemedResource(statusIcon)
	themeResource.ColorName = colorName
	status := widget.NewIcon(themeResource)

	cell.Add(status)
}

// updateTagsCell updates the tags cell with the device's tags
func updateTagsCell(cell *fyne.Container, item client.InventoryListItem) {
	cell.RemoveAll()
	tags := "-"

	if len(item.Tags) > 0 {
		tags = strings.Join(item.Tags, ", ")
	}
	tagLabel := widget.NewLabel(tags)
	tagLabel.Truncation = fyne.TextTruncateEllipsis
	cell.Add(tagLabel)
}

// updateGroupCell updates the group cell with the device's group hierarchy
func updateGroupCell(cell *fyne.Container, item client.InventoryListItem) {
	cell.RemoveAll()
	text := ""
	if len(item.AncestorsTitles) > 0 {
		text = strings.Join(item.AncestorsTitles[:len(item.AncestorsTitles)-1], " > ")
	}
	group := widget.NewLabel(text)
	group.Truncation = fyne.TextTruncateEllipsis
	cell.Add(group)
}

// updateConnectionStatusCell updates the connection status cell with current connection info
func updateConnectionStatusCell(d tableDelegate, cell *fyne.Container, item client.InventoryListItem) {
	cell.RemoveAll()
	text := updateConnectionStatus(d, item)
	status := widget.NewLabel(text)
	status.Truncation = fyne.TextTruncateEllipsis
	cell.Add(status)
}

// updateActionCell updates the action cell with the appropriate button based on connection status
func updateActionCell(d tableDelegate, cell *fyne.Container, item client.InventoryListItem) {
	cell.RemoveAll()
	btn := widget.NewButton("", nil)
	conn, ok := d.GetStore().GetActive(item.NodeID)
	if ok {
		btn.SetIcon(theme.CancelIcon())
		btn.OnTapped = func() {
			conn.Cancel()
			d.GetStore().DeleteActive(item.NodeID)
			d.RefreshUI()
		}
	} else {
		btn.SetIcon(theme.SettingsIcon())
		btn.OnTapped = func() { d.ShowConnectDialog(&item) }
	}

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
	btn := obj.(*widget.Button)
	col := model.DeviceColumns[id.Col]

	if col.Title == "" {
		btn.Hide()
		return
	}
	btn.Show()
	btn.SetText(col.Title)

	if col.Sortable {
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
}

// updateConnectionStatus returns the formatted connection information for a device
func updateConnectionStatus(d tableDelegate, item client.InventoryListItem) string {
	var activeConn *service.DeviceConnections
	var ok bool

	if activeConn, ok = d.GetStore().GetActive(item.NodeID); !ok {
		return "-"
	}
	targetStrings := []string{}

	for _, target := range activeConn.Targets {
		targetStrings = append(
			targetStrings,
			fmt.Sprintf("%s: %s:%s > %s:%s", target.Protocol, target.LocalHost, target.LocalPort, target.RemoteHost, target.RemotePort),
		)
	}

	return fmt.Sprintf("%d [%s]", len(activeConn.Targets), strings.Join(targetStrings, ", "))
}
