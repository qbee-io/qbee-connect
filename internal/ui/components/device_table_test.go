package components

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/testutil"
	"go.qbee.io/connect/internal/ui/widgets"
)

const (
	colTitle = iota
	colStatus
	colConnection
	colGroup
	colTags
	colAction
)

func tableDelegateWithItems(t *testing.T, items ...client.InventoryListItem) *fakeDelegate {
	t.Helper()
	d := newFakeDelegate(t)
	d.model.FilteredData.Items = items
	return d
}

// renderCell runs updateCell for one cell and returns the populated cell.
func renderCell(t *testing.T, d *fakeDelegate, row, col int) *fyne.Container {
	t.Helper()
	cell := container.NewStack()
	updateCell(d, widget.TableCellID{Row: row, Col: col}, cell)
	test.NewTempWindow(t, cell)
	return cell
}

func onlyObject(t *testing.T, cell *fyne.Container) fyne.CanvasObject {
	t.Helper()
	if len(cell.Objects) != 1 {
		t.Fatalf("expected 1 object in cell, got %d", len(cell.Objects))
	}
	return cell.Objects[0]
}

func TestNewDeviceTable_Dimensions(t *testing.T) {
	d := tableDelegateWithItems(t,
		client.InventoryListItem{NodeID: "a", Title: "alpha"},
		client.InventoryListItem{NodeID: "b", Title: "bravo"},
	)
	table := NewDeviceTable(d)

	rows, cols := table.Length()
	if rows != 2 || cols != len(d.model.DeviceColumns) {
		t.Fatalf("Length() = %d,%d, want 2,%d", rows, cols, len(d.model.DeviceColumns))
	}
	if !table.ShowHeaderRow {
		t.Fatal("expected header row to be shown")
	}
}

func TestNewDeviceTable_RendersRows(t *testing.T) {
	d := tableDelegateWithItems(t,
		client.InventoryListItem{NodeID: "a", Title: "alpha"},
		client.InventoryListItem{NodeID: "b", Title: "bravo"},
	)
	table := NewDeviceTable(d)
	w := test.NewTempWindow(t, table)
	w.Resize(fyne.NewSize(1200, 400))

	// test.LaidOutObjects does not descend into the table's internal cell
	// widget, so assert on the rendered markup instead.
	markup := test.RenderToMarkup(w.Canvas())
	for _, want := range []string{">title:alpha<", ">title:bravo<", ">Device<", ">Connection Info<"} {
		if !strings.Contains(markup, want) {
			t.Errorf("%q not rendered", want)
		}
	}
}

func TestUpdateCell_RowOutOfRangeClearsCell(t *testing.T) {
	d := tableDelegateWithItems(t)
	cell := container.NewStack(widget.NewLabel("stale"))

	updateCell(d, widget.TableCellID{Row: 3, Col: colTitle}, cell)

	if len(cell.Objects) != 0 {
		t.Fatalf("expected cell to be cleared, got %d objects", len(cell.Objects))
	}
}

func TestTitleCell_TapOpensConnectOrInfoDialog(t *testing.T) {
	d := tableDelegateWithItems(t,
		client.InventoryListItem{NodeID: "idle", Title: "idle"},
		client.InventoryListItem{NodeID: "busy", Title: "busy"},
	)
	d.store.SetActive("busy", &service.DeviceConnections{})

	idle := onlyObject(t, renderCell(t, d, 0, colTitle)).(*widgets.ClickableLabel)
	busy := onlyObject(t, renderCell(t, d, 1, colTitle)).(*widgets.ClickableLabel)

	if idle.Text != "title:idle" {
		t.Errorf("title = %q", idle.Text)
	}
	if idle.Truncation != fyne.TextTruncateEllipsis {
		t.Error("title should truncate with ellipsis")
	}

	test.Tap(idle)
	test.Tap(busy)

	if len(d.connectShown) != 1 || d.connectShown[0] != "idle" {
		t.Errorf("connect dialog shown for %v, want [idle]", d.connectShown)
	}
	if len(d.infoShown) != 1 || d.infoShown[0] != "busy" {
		t.Errorf("info dialog shown for %v, want [busy]", d.infoShown)
	}
}

func TestStatusCell(t *testing.T) {
	tests := []struct {
		status string
		want   fyne.ThemeColorName
	}{
		{StatusOnline, theme.ColorNameSuccess},
		{StatusDelayed, theme.ColorNameWarning},
		{StatusDisconnected, theme.ColorNameError},
		{"unknown", theme.ColorNameDisabled},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			d := tableDelegateWithItems(t, client.InventoryListItem{Status: tt.status})
			icon, ok := onlyObject(t, renderCell(t, d, 0, colStatus)).(*widget.Icon)
			if !ok {
				t.Fatal("expected an icon")
			}
			res, ok := icon.Resource.(*theme.ThemedResource)
			if !ok {
				t.Fatalf("expected a themed resource, got %T", icon.Resource)
			}
			if res.ColorName != tt.want {
				t.Fatalf("color = %q, want %q", res.ColorName, tt.want)
			}
		})
	}
}

func TestConnectionCell(t *testing.T) {
	d := tableDelegateWithItems(t,
		client.InventoryListItem{NodeID: "idle"},
		client.InventoryListItem{NodeID: "busy"},
	)
	d.store.SetActive("busy", &service.DeviceConnections{Targets: []client.RemoteAccessTarget{{}, {}}})

	idle := onlyObject(t, renderCell(t, d, 0, colConnection))
	if l, ok := idle.(*widget.Label); !ok || l.Text != "-" {
		t.Fatalf("idle device: expected '-' label, got %#v", idle)
	}

	busy := renderCell(t, d, 1, colConnection)
	if _, ok := testutil.Find(busy, func(l *widget.Label) bool { return l.Text == "2 active" }); !ok {
		t.Fatal("busy device: expected '2 active' label")
	}
	btn, ok := testutil.Find[*widgets.ButtonPointer](busy, nil)
	if !ok {
		t.Fatal("busy device: expected info button")
	}
	test.Tap(btn)
	if len(d.infoShown) != 1 || d.infoShown[0] != "busy" {
		t.Fatalf("info dialog shown for %v, want [busy]", d.infoShown)
	}
}

func TestGroupAndTagCells(t *testing.T) {
	tests := []struct {
		name      string
		item      client.InventoryListItem
		wantGroup string
		wantTags  string
	}{
		{
			name:      "nested group and tags",
			item:      client.InventoryListItem{AncestorsTitles: []string{"root", "eu", "oslo", "device"}, Tags: []string{"prod", "edge"}},
			wantGroup: "root > eu > oslo",
			wantTags:  "prod, edge",
		},
		{
			name:      "no ancestors or tags",
			item:      client.InventoryListItem{},
			wantGroup: "",
			wantTags:  "-",
		},
		{
			name:      "only self in ancestors",
			item:      client.InventoryListItem{AncestorsTitles: []string{"device"}},
			wantGroup: "",
			wantTags:  "-",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tableDelegateWithItems(t, tt.item)

			group := onlyObject(t, renderCell(t, d, 0, colGroup)).(*widget.Label)
			if group.Text != tt.wantGroup {
				t.Errorf("group = %q, want %q", group.Text, tt.wantGroup)
			}
			tags := onlyObject(t, renderCell(t, d, 0, colTags)).(*widget.Label)
			if tags.Text != tt.wantTags {
				t.Errorf("tags = %q, want %q", tags.Text, tt.wantTags)
			}
			if tags.Truncation != fyne.TextTruncateEllipsis {
				t.Error("label cells should truncate with ellipsis")
			}
		})
	}
}

func TestActionCell_IdleOpensConnectDialog(t *testing.T) {
	d := tableDelegateWithItems(t, client.InventoryListItem{NodeID: "idle"})

	btn := onlyObject(t, renderCell(t, d, 0, colAction)).(*widgets.ButtonPointer)
	if btn.Icon != theme.SettingsIcon() {
		t.Error("idle device should show the settings icon")
	}
	test.Tap(btn)

	if len(d.connectShown) != 1 || d.connectShown[0] != "idle" {
		t.Fatalf("connect dialog shown for %v, want [idle]", d.connectShown)
	}
}

func TestActionCell_ActiveDisconnects(t *testing.T) {
	d := tableDelegateWithItems(t, client.InventoryListItem{NodeID: "busy"})
	cancelled := false
	d.store.SetActive("busy", &service.DeviceConnections{Cancel: func() { cancelled = true }})

	btn := onlyObject(t, renderCell(t, d, 0, colAction)).(*widgets.ButtonPointer)
	if btn.Icon != theme.CancelIcon() {
		t.Error("active device should show the cancel icon")
	}
	test.Tap(btn)

	if !cancelled {
		t.Error("expected connection to be cancelled")
	}
	if _, ok := d.store.GetActive("busy"); ok {
		t.Error("expected active connection to be removed")
	}
	if load, _ := d.refreshes(); load != 1 {
		t.Errorf("RefreshUI called %d times, want 1", load)
	}
}

// renderHeader runs updateHeader for a column and returns the header button.
func renderHeader(t *testing.T, d *fakeDelegate, col int) *widgets.ButtonPointer {
	t.Helper()
	btn := widgets.NewButtonPointer("", nil)
	test.NewTempWindow(t, btn)
	updateHeader(d, widget.TableCellID{Row: -1, Col: col}, btn)
	return btn
}

func TestUpdateHeader_SortableColumnToggles(t *testing.T) {
	d := tableDelegateWithItems(t)
	q := d.model.Query
	q.SortField = d.model.DeviceColumns[colTitle].SortKey
	q.SortDirection = client.SortDirectionAsc

	btn := renderHeader(t, d, colTitle)
	if btn.Text != "Device" || btn.Icon != theme.MoveUpIcon() {
		t.Fatalf("header = %q with icon %v, want Device with up arrow", btn.Text, btn.Icon)
	}

	test.Tap(btn)
	if q.SortDirection != client.SortDirectionDesc {
		t.Fatalf("direction after tap = %q, want desc", q.SortDirection)
	}

	btn = renderHeader(t, d, colTitle)
	if btn.Icon != theme.MoveDownIcon() {
		t.Error("expected down arrow for descending sort")
	}
	test.Tap(btn)
	if q.SortDirection != client.SortDirectionAsc {
		t.Fatalf("direction after second tap = %q, want asc", q.SortDirection)
	}

	if load, _ := d.refreshes(); load != 2 {
		t.Fatalf("RefreshUI called %d times, want 2", load)
	}
}

func TestUpdateHeader_TapOtherColumnSortsAscending(t *testing.T) {
	d := tableDelegateWithItems(t)
	q := d.model.Query
	q.SortField = d.model.DeviceColumns[colTitle].SortKey
	q.SortDirection = client.SortDirectionDesc

	btn := renderHeader(t, d, colStatus)
	if btn.Icon != nil {
		t.Error("unsorted column should not show a sort icon")
	}
	test.Tap(btn)

	if q.SortField != d.model.DeviceColumns[colStatus].SortKey || q.SortDirection != client.SortDirectionAsc {
		t.Fatalf("sort = %s %s, want %s asc", q.SortField, q.SortDirection, d.model.DeviceColumns[colStatus].SortKey)
	}
}

func TestUpdateHeader_NonSortableColumn(t *testing.T) {
	d := tableDelegateWithItems(t)
	btn := renderHeader(t, d, colGroup)

	if btn.Text != "Group" {
		t.Errorf("text = %q", btn.Text)
	}
	if btn.Cursor() != desktop.DefaultCursor {
		t.Error("non-sortable header should use the default cursor")
	}
	if btn.OnTapped != nil {
		t.Error("non-sortable header should not be tappable")
	}
}

func TestUpdateHeader_EmptyTitleHidden(t *testing.T) {
	d := tableDelegateWithItems(t)
	btn := renderHeader(t, d, colAction)

	if btn.Visible() {
		t.Fatal("header for the untitled action column should be hidden")
	}
}

func TestUpdateHeader_OutOfRangeIsIgnored(t *testing.T) {
	d := tableDelegateWithItems(t)
	for _, col := range []int{-1, len(d.model.DeviceColumns)} {
		btn := renderHeader(t, d, col)
		if btn.Text != "" {
			t.Errorf("col %d: header should be untouched, got %q", col, btn.Text)
		}
	}
}
