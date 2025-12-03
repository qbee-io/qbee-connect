package model

import (
	"go.qbee.io/client"
)

const (
	DefaultPageSize      = 10
	DefaultOffset        = 0
	DefaultSortField     = "title"
	DefaultSortDirection = client.SortDirectionAsc
	DefaultReportType    = "short"
	DefaultSearchTerm    = ""
)

// DeviceModel holds the state for pagination and filtering
type DeviceModel struct {
	DeviceData        client.InventoryListResponse
	FilteredData      client.InventoryListResponse
	CurrentPage       int
	ActiveTunnelsOnly bool
	Query             *client.InventoryListQuery
}

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
	}
}

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

type DeviceColumn struct {
	Title         string
	Sortable      bool
	SortKey       string
	WidthQuotient float32
}

var DeviceColumns = []DeviceColumn{
	{Title: "Device", Sortable: true, SortKey: "title", WidthQuotient: 0.22},
	{Title: "Status", Sortable: true, SortKey: "exp_hard", WidthQuotient: 0.13},
	{Title: "Group", WidthQuotient: 0.32},
	{Title: "Tags", WidthQuotient: 0.22},
	{Title: "", WidthQuotient: 0.11}, // Actions column
}
