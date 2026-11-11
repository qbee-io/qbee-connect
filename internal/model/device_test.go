package model

import (
	"testing"

	"go.qbee.io/client"
)

func TestNewDeviceModel_Defaults(t *testing.T) {
	m := NewDeviceModel()

	if m.Query.SortField != DefaultSortField || m.Query.SortDirection != DefaultSortDirection {
		t.Errorf("unexpected default sort: %s %s", m.Query.SortField, m.Query.SortDirection)
	}
	if m.Query.ItemsPerPage != DefaultPageSize {
		t.Errorf("ItemsPerPage = %d, want %d", m.Query.ItemsPerPage, DefaultPageSize)
	}
	if len(m.DeviceColumns) != len(DefaultDeviceColumns) {
		t.Fatalf("got %d columns, want %d", len(m.DeviceColumns), len(DefaultDeviceColumns))
	}
}

func TestNewDeviceModel_ColumnsAreCopied(t *testing.T) {
	m := NewDeviceModel()
	m.DeviceColumns[0].SortKey = "changed"

	if DefaultDeviceColumns[0].SortKey == "changed" {
		t.Fatal("modifying a model's columns must not modify DefaultDeviceColumns")
	}
}

func TestDefaultDeviceColumns_WidthsSumToOne(t *testing.T) {
	var sum float32
	for _, c := range DefaultDeviceColumns {
		sum += c.WidthQuotient
	}
	if sum < 0.999 || sum > 1.001 {
		t.Fatalf("column width quotients sum to %f, want 1.0", sum)
	}
}

func TestTotalPages(t *testing.T) {
	tests := []struct {
		total, perPage, want int
	}{
		{0, 10, 1},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{100, 25, 4},
		{101, 25, 5},
	}
	for _, tt := range tests {
		m := NewDeviceModel()
		m.DeviceData.Total = tt.total
		m.Query.ItemsPerPage = tt.perPage
		if got := m.TotalPages(); got != tt.want {
			t.Errorf("TotalPages(total=%d, perPage=%d) = %d, want %d", tt.total, tt.perPage, got, tt.want)
		}
	}
}

func TestInitDeviceSortKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"device_name", "device_name"},
		{"fqhost", "fqhost"},
		{"", DefaultSortField},
		{"something_else", DefaultSortField},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := NewDeviceModel()
			m.Query.SortDirection = client.SortDirectionDesc

			m.InitDeviceSortKey(tt.input)

			if m.Query.SortField != tt.want {
				t.Errorf("Query.SortField = %q, want %q", m.Query.SortField, tt.want)
			}
			if m.Query.SortDirection != client.SortDirectionAsc {
				t.Errorf("Query.SortDirection = %q, want asc", m.Query.SortDirection)
			}
			for _, c := range m.DeviceColumns {
				if c.Title == "Device" && c.SortKey != tt.want {
					t.Errorf("Device column SortKey = %q, want %q", c.SortKey, tt.want)
				}
			}
		})
	}
}
