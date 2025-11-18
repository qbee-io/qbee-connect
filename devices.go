package main

import (
	"context"
	"fmt"

	"go.qbee.io/client"
)

func newDeviceModel() *deviceModel {
	return &deviceModel{
		deviceData:  &client.InventoryListResponse{},
		pageSize:    10,
		currentPage: 0,
	}
}

func (m *deviceModel) totalPages() int {
	if m.deviceData.Total == 0 {
		return 1
	}
	pages := m.deviceData.Total / m.pageSize
	if m.deviceData.Total%m.pageSize != 0 {
		pages++
	}

	return pages
}

// loadDevices calls qbee-cli and unmarshals the JSON.
// Adjust the command and JSON schema to match your environment.
func (app *App) loadDevices(ctx context.Context, search string, offset, itemsPerPage int) (*client.InventoryListResponse, error) {

	query := client.InventoryListQuery{
		Search: client.InventoryListSearch{
			Title: search,
		},
		SortField:     "title",
		SortDirection: client.SortDirectionAsc,
		ReportType:    "short",
		Offset:        offset,
		ItemsPerPage:  itemsPerPage,
	}

	devices, err := app.cli.ListDeviceInventory(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error fetching device list: %w", err)
	}
	return devices, nil
}
