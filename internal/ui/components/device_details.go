package components

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
)

// newTitleWithLastReported renders a status icon and the given title on the left and the
// device's last reported time on the right of the same line.
func newTitleWithLastReported(title fyne.CanvasObject, device *client.InventoryListItem) fyne.CanvasObject {
	lastReported := widget.NewLabel("Last connect: " + formatLastReported(device.LastReportedTime))
	// Indent the icon so it lines up with the text of the labels below it, which have
	// their own inner padding.
	statusIcon := container.New(layout.NewCustomPaddedLayout(0, 0, theme.Padding(), 0), newStatusIcon(device.Status))
	return container.NewBorder(nil, nil, statusIcon, lastReported, title)
}

// newDeviceDetails renders the dialog heading: status icon, device name and last reported time.
func newDeviceDetails(deviceTitle string, device *client.InventoryListItem) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(deviceTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return newTitleWithLastReported(title, device)
}

// virtualInterfacePrefixes lists name prefixes of virtual interfaces (loopback, containers,
// bridges) that are listed after the physical ones in the device info tab.
var virtualInterfacePrefixes = []string{"docker", "br-", "veth", "virbr", "cni", "flannel"}

// isVirtualInterface reports whether the interface name looks like a virtual interface
func isVirtualInterface(name string) bool {
	if name == "lo" {
		return true
	}
	return slices.ContainsFunc(virtualInterfacePrefixes, func(prefix string) bool {
		return strings.HasPrefix(name, prefix)
	})
}

// formatLastReported formats a unix timestamp as local time plus a relative age, e.g.
// "2026-11-10 14:03:12 (5m ago)".
func formatLastReported(ts int64) string {
	if ts <= 0 {
		return "never"
	}

	// Be tolerant of millisecond timestamps.
	if ts > 1e12 {
		ts /= 1000
	}

	t := time.Unix(ts, 0)
	return fmt.Sprintf("%s (%s)", t.Local().Format("2006-01-02 15:04:05"), formatAge(time.Since(t)))
}

// formatAge renders a duration as a short, human readable age.
func formatAge(d time.Duration) string {
	switch {
	case d < 0:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
