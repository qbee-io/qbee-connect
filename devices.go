package main

import (
	"context"
	"fmt"

	"go.qbee.io/client"
)

const defaultPageSize = 10
const defaultOffset = 0
const defaultSortField = "title"
const defaultSortDirection = client.SortDirectionAsc
const defaultReportType = "short"
const defaultSearchTerm = ""

func newDeviceModel() *deviceModel {
	return &deviceModel{
		deviceData:  &client.InventoryListResponse{},
		currentPage: 0,
		query: &client.InventoryListQuery{
			SortField:     defaultSortField,
			SortDirection: defaultSortDirection,
			ReportType:    defaultReportType,
			Offset:        defaultOffset,
			ItemsPerPage:  defaultPageSize,
			Search: client.InventoryListSearch{
				Title: defaultSearchTerm,
			},
		},
	}
}

func (m *deviceModel) totalPages() int {
	if m.deviceData.Total == 0 {
		return 1
	}
	pages := m.deviceData.Total / m.query.ItemsPerPage
	if m.deviceData.Total%m.query.ItemsPerPage != 0 {
		pages++
	}

	return pages
}

// loadDevices calls qbee-cli and unmarshals the JSON.
// Adjust the command and JSON schema to match your environment.
func (app *App) loadDevices(ctx context.Context) (*client.InventoryListResponse, error) {

	devices, err := app.cli.ListDeviceInventory(ctx, *app.deviceModel.query)
	if err != nil {
		return nil, fmt.Errorf("error fetching device list: %w", err)
	}
	return devices, nil
}
