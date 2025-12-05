package components

import (
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
			return container.NewStack(widget.NewButton("", nil), widget.NewLabel(""))
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
	btn := cell.Objects[0].(*widget.Button)
	lbl := cell.Objects[1].(*widget.Label)

	items := d.GetDeviceModel().FilteredData.Items
	if id.Row >= len(items) {
		return
	}
	item := items[id.Row]

	// Data Columns
	if id.Col < len(model.DeviceColumns)-1 {
		btn.Hide()
		lbl.Show()
		switch id.Col {
		case 0:
			lbl.SetText(item.Title)
		case 1:
			lbl.SetText(item.Status)
		case 2:
			if len(item.AncestorsTitles) > 0 {
				lbl.SetText(strings.Join(item.AncestorsTitles[:len(item.AncestorsTitles)-1], " > "))
			} else {
				lbl.SetText("")
			}
		case 3:
			lbl.SetText(strings.Join(item.Tags, ", "))
		}
		return
	}

	// Action Column
	lbl.Hide()
	conn, ok := d.GetStore().GetActive(item.NodeID)
	if ok {
		btn.SetText("Disconnect")
		btn.OnTapped = func() {
			conn.Cancel()
			d.GetStore().DeleteActive(item.NodeID)
			d.RefreshUI()
		}
	} else {
		btn.SetText("Configure")
		btn.OnTapped = func() { d.ShowConnectDialog(&item) }
	}
	btn.Show()
}

func updateHeader(d tableDelegate, id widget.TableCellID, obj fyne.CanvasObject) {
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
