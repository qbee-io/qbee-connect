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
	"go.qbee.io/connect/internal/ui/widgets"
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

// newDeviceDetails renders the title line (status icon, device name and last reported time)
// followed by a line with the network interfaces and their IP addresses (each with a copy button).
// onResize is called when the interfaces line is expanded or collapsed, so the caller can
// re-layout the surrounding container.
func newDeviceDetails(app fyne.App, deviceTitle string, device *client.InventoryListItem, onResize func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(deviceTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewVBox(
		newTitleWithLastReported(title, device),
		newInterfacesRow(app, device, onResize),
	)
}

const (
	// maxCollapsedInterfaces is how many interfaces are shown before the "+N more" toggle
	maxCollapsedInterfaces = 3
	// interfaceGridColumns is the number of columns used when all interfaces are shown
	interfaceGridColumns = 3
	// maxExpandedInterfaceRows is how many grid rows are visible before the expanded list scrolls
	maxExpandedInterfaceRows = 4
)

// virtualInterfacePrefixes lists name prefixes of virtual interfaces (loopback, containers,
// bridges) that are hidden until the user expands the interfaces list.
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

type networkInterface struct {
	name string
	ip   string
}

// sortedInterfaces returns the device interfaces with an IP address, physical ones first,
// each group sorted by name.
func sortedInterfaces(device *client.InventoryListItem) []networkInterface {
	var physical, virtual []networkInterface
	for name, ip := range device.System.IPv4 {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		if isVirtualInterface(name) {
			virtual = append(virtual, networkInterface{name, ip})
		} else {
			physical = append(physical, networkInterface{name, ip})
		}
	}

	byName := func(a, b networkInterface) int { return strings.Compare(a.name, b.name) }
	slices.SortFunc(physical, byName)
	slices.SortFunc(virtual, byName)
	return append(physical, virtual...)
}

// newInterfacesRow renders "Interfaces:" followed by up to maxCollapsedInterfaces physical
// interfaces on a single line. Remaining (and virtual) interfaces are available through a
// "+N more" toggle that expands into a scrollable grid.
func newInterfacesRow(app fyne.App, device *client.InventoryListItem, onResize func()) fyne.CanvasObject {
	header := widget.NewLabel("Interfaces:")

	all := sortedInterfaces(device)
	if len(all) == 0 {
		return container.NewHBox(header, widget.NewLabel("none reported"))
	}

	var collapsed []networkInterface
	for _, iface := range all {
		if len(collapsed) == maxCollapsedInterfaces || isVirtualInterface(iface.name) {
			break
		}
		collapsed = append(collapsed, iface)
	}
	hidden := len(all) - len(collapsed)

	items := make([]fyne.CanvasObject, len(all))
	for i, iface := range all {
		items[i] = newInterfaceItem(app, iface)
	}

	collapsedView := container.NewGridWithColumns(maxCollapsedInterfaces, items[:len(collapsed)]...)
	if len(collapsed) == 0 {
		collapsedView = container.NewGridWithColumns(1, widget.NewLabel("only virtual interfaces"))
	}

	if hidden == 0 {
		return container.NewBorder(nil, nil, header, nil, collapsedView)
	}

	expandedGrid := container.NewGridWithColumns(interfaceGridColumns, items...)
	expandedView := container.NewVScroll(expandedGrid)
	rows := min((len(all)+interfaceGridColumns-1)/interfaceGridColumns, maxExpandedInterfaceRows)
	rowHeight := items[0].MinSize().Height
	expandedView.SetMinSize(fyne.NewSize(0, float32(rows)*rowHeight+float32(rows-1)*theme.Padding()))

	center := container.NewStack(collapsedView)
	var toggle *widgets.ButtonPointer
	toggle = widgets.NewButtonPointer(fmt.Sprintf("+%d more", hidden), func() {
		if center.Objects[0] == collapsedView {
			center.Objects = []fyne.CanvasObject{expandedView}
			toggle.SetText("Show less")
		} else {
			center.Objects = []fyne.CanvasObject{collapsedView}
			toggle.SetText(fmt.Sprintf("+%d more", hidden))
		}
		if onResize != nil {
			onResize()
		} else {
			center.Refresh()
		}
	})
	toggle.Importance = widget.LowImportance

	// Give the toggle a fixed width that fits both labels, so the interface columns don't
	// shift sideways when the text changes between "+N more" and "Show less".
	toggleSize := toggle.MinSize()
	toggle.SetText("Show less")
	toggleSize = toggleSize.Max(toggle.MinSize())
	toggle.SetText(fmt.Sprintf("+%d more", hidden))
	fixedToggle := container.New(layout.NewGridWrapLayout(toggleSize), toggle)

	// Keep the header and toggle at the top when the expanded grid is taller than one line.
	return container.NewBorder(nil, nil, container.NewVBox(header), container.NewVBox(fixedToggle), center)
}

// newInterfaceItem renders "name: ip" with a copy button that copies the IP address.
// The text truncates with an ellipsis if the column is too narrow.
func newInterfaceItem(app fyne.App, iface networkInterface) fyne.CanvasObject {
	label := widget.NewLabel(iface.name + ": " + iface.ip)
	label.Truncation = fyne.TextTruncateEllipsis

	copyBtn := widgets.NewButtonPointer("", func() {
		app.Clipboard().SetContent(iface.ip)
	})
	copyBtn.SetIcon(theme.ContentCopyIcon())

	return container.NewBorder(nil, nil, nil, copyBtn, label)
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
