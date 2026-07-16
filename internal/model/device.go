package model

import (
	"go.qbee.io/client"
)

const (
	// DefaultPageSize defines the default number of items per page
	DefaultPageSize = 10
	// DefaultOffset defines the default offset for pagination
	DefaultOffset = 0
	// DefaultSortField defines the default field to sort by
	DefaultSortField = "fqhost"
	// DefaultSortDirection defines the default sort direction
	DefaultSortDirection = client.SortDirectionAsc
	// DefaultReportType defines the default report type
	DefaultReportType = "short"
	// DefaultSearchTerm defines the default search term
	DefaultSearchTerm = ""
)

// DeviceModel holds the state for pagination and filtering
type DeviceModel struct {
	// DeviceData holds the complete list of devices
	DeviceData client.InventoryListResponse
	// FilteredData holds the filtered list of devices based on current query
	FilteredData client.InventoryListResponse
	// CurrentPage indicates the current page in pagination
	CurrentPage int
	// ActiveTunnelsOnly indicates if only devices with active tunnels should be shown
	ActiveTunnelsOnly bool
	// Query holds the current query parameters for filtering and sorting
	Query *client.InventoryListQuery
	// DeviceColumns defines the columns for the device table
	DeviceColumns []DeviceColumn
}

// NewDeviceModel initializes a new DeviceModel with default values
func NewDeviceModel() *DeviceModel {
	return &DeviceModel{
		DeviceData:   client.InventoryListResponse{},
		FilteredData: client.InventoryListResponse{},
		CurrentPage:  0,
		Query: &client.InventoryListQuery{
			SortField:     DefaultSortField,
			SortDirection: DefaultSortDirection,
			ReportType:    DefaultReportType,
			Offset:        DefaultOffset,
			ItemsPerPage:  DefaultPageSize,
			Search: client.InventoryListSearch{
				Title: DefaultSearchTerm,
			},
		},
		DeviceColumns: DefaultDeviceColumns,
	}
}

// TotalPages calculates the total number of pages based on total items and items per page
func (m *DeviceModel) TotalPages() int {
	if m.DeviceData.Total == 0 {
		return 1
	}
	pages := m.DeviceData.Total / m.Query.ItemsPerPage
	if m.DeviceData.Total%m.Query.ItemsPerPage != 0 {
		pages++
	}
	return pages
}

// DeviceColumn defines the structure for device table columns
type DeviceColumn struct {
	// Title is the display name of the column
	Title string
	// Sortable indicates if the column can be sorted
	Sortable bool
	// SortKey is the key used for sorting
	SortKey string
	// WidthQuotient determines the relative width of the column
	WidthQuotient float32
}

// DeviceColumns defines the columns for the device table
var DefaultDeviceColumns = []DeviceColumn{
	{Title: "Device", Sortable: true, SortKey: DefaultSortField, WidthQuotient: 0.19},
	{Title: "Online", Sortable: true, SortKey: "exp_hard", WidthQuotient: 0.10},
	{Title: "Connection Info", WidthQuotient: 0.15},
	{Title: "Group", WidthQuotient: 0.30},
	{Title: "Tags", WidthQuotient: 0.20},
	{Title: "", WidthQuotient: 0.06}, // Actions column
}

// InitDeviceSortKey initializes the sort key for the devices based on the provided sortKey.
// This is only for the device label which can be set to "device_name" or "fqhost".
func (m *DeviceModel) InitDeviceSortKey(sortKey string) {
	m.Query.SortField = sortKey
	m.Query.SortDirection = client.SortDirectionAsc
	for i, col := range m.DeviceColumns {
		if col.Title == "Device" {
			m.DeviceColumns[i].SortKey = sortKey
			return
		}
	}
}
