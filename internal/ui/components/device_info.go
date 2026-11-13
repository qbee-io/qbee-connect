package components

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
	"go.qbee.io/client/types"
	"go.qbee.io/connect/internal/ui/widgets"
)

// deviceInfoDelegate defines the methods required to fetch and render device information
type deviceInfoDelegate interface {
	GetFyneApp() fyne.App
	GetClient() *client.Client
	GetContext() context.Context
}

const (
	// deviceTabPreferenceKey stores which tab was last selected in the device dialogs, so
	// users who always look at the device info don't have to switch tabs every time.
	deviceTabPreferenceKey = "device_dialog_tab"
	deviceTabInfo          = "info"
	deviceTabMain          = "main"

	// deviceInfoTimeout bounds how long we wait for the inventory API
	deviceInfoTimeout = 20 * time.Second
)

// newDeviceTabs returns tabs holding the dialog's main content and a "Device info" tab.
// The device inventory is fetched the first time the info tab is shown.
func newDeviceTabs(d deviceInfoDelegate, device *client.InventoryListItem, mainTab *container.TabItem) *container.AppTabs {
	infoContent, load := newDeviceInfoView(d, device.NodeID)
	infoTab := container.NewTabItemWithIcon("Device info", theme.InfoIcon(), infoContent)

	tabs := container.NewAppTabs(mainTab, infoTab)

	prefs := d.GetFyneApp().Preferences()
	if prefs.String(deviceTabPreferenceKey) == deviceTabInfo {
		tabs.Select(infoTab)
		load()
	}

	tabs.OnSelected = func(item *container.TabItem) {
		if item == infoTab {
			prefs.SetString(deviceTabPreferenceKey, deviceTabInfo)
			load()
		} else {
			prefs.SetString(deviceTabPreferenceKey, deviceTabMain)
		}
	}

	return tabs
}

// newDeviceInfoView returns the device info content and a function that loads it. Calling
// the load function more than once is a no-op unless the previous attempt failed.
func newDeviceInfoView(d deviceInfoDelegate, nodeID string) (fyne.CanvasObject, func()) {
	content := container.NewStack()
	loaded, loading := false, false

	var load func()
	load = func() {
		if loaded || loading {
			return
		}
		loading = true
		loadingView, progress := newDeviceInfoLoading()
		content.Objects = []fyne.CanvasObject{loadingView}
		content.Refresh()

		go func() {
			ctx, cancel := context.WithTimeout(d.GetContext(), deviceInfoTimeout)
			defer cancel()
			inv, err := d.GetClient().GetDeviceInventory(ctx, nodeID)

			fyne.Do(func() {
				// Stop the animation before the loading view is replaced, so it doesn't keep
				// running off-screen.
				progress.Stop()
				loading = false
				if err != nil {
					content.Objects = []fyne.CanvasObject{newDeviceInfoError(err, load)}
				} else {
					loaded = true
					reload := func() {
						loaded = false
						load()
					}
					content.Objects = []fyne.CanvasObject{newDeviceInfoContent(d.GetFyneApp(), inv, reload)}
				}
				content.Refresh()
			})
		}()
	}

	return content, load
}

// newDeviceInfoLoading returns the loading view and its progress bar, which the caller
// must stop when the view is replaced.
func newDeviceInfoLoading() (fyne.CanvasObject, *widget.ProgressBarInfinite) {
	progress := widget.NewProgressBarInfinite()
	return container.NewCenter(container.NewVBox(
		progress,
		widget.NewLabelWithStyle("Loading device information…", fyne.TextAlignCenter, fyne.TextStyle{}),
	)), progress
}

func newDeviceInfoError(err error, retry func()) fyne.CanvasObject {
	msg := widget.NewLabelWithStyle("Could not load device information:\n"+err.Error(),
		fyne.TextAlignCenter, fyne.TextStyle{})
	msg.Wrapping = fyne.TextWrapWord
	retryBtn := widgets.NewButtonPointer("Retry", retry)
	retryBtn.SetIcon(theme.ViewRefreshIcon())
	return container.NewCenter(container.NewVBox(msg, container.NewCenter(retryBtn)))
}

// newDeviceInfoContent renders the System and Network sections side by side, followed by a
// table of all network interfaces.
func newDeviceInfoContent(app fyne.App, inv *client.DeviceInventory, reload func()) fyne.CanvasObject {
	copyBtn := widgets.NewButtonPointer("Copy all", func() {
		app.Clipboard().SetContent(formatDeviceInfo(inv))
	})
	copyBtn.SetIcon(theme.ContentCopyIcon())
	copyBtn.Importance = widget.LowImportance

	refreshBtn := widgets.NewButtonPointer("Refresh", reload)
	refreshBtn.SetIcon(theme.ViewRefreshIcon())
	refreshBtn.Importance = widget.LowImportance

	toolbar := container.NewHBox(layout.NewSpacer(), refreshBtn, copyBtn)

	// System and Network side by side, like the web app; both start at the top
	sections := container.NewGridWithColumns(2,
		container.NewVBox(newInfoSection("System", newInfoForm(systemFields(inv)))),
		container.NewVBox(newInfoSection("Network", newInfoForm(networkFields(inv)))),
	)

	body := container.NewVBox(sections)
	if rows := interfaceRows(inv.SystemInfo); len(rows) > 0 {
		body.Add(newInfoSection("Interfaces", newInterfacesTable(rows)))
	}

	return container.NewBorder(toolbar, nil, nil, nil, container.NewVScroll(body))
}

// newInfoSection renders a compact bold heading with a separator above the content
func newInfoSection(title string, content fyne.CanvasObject) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	heading.SizeName = theme.SizeNameSubHeadingText
	return container.NewVBox(heading, widget.NewSeparator(), content)
}

// newInfoForm renders the fields as a two-column form with selectable values
func newInfoForm(fields []infoField) fyne.CanvasObject {
	if len(fields) == 0 {
		return widget.NewLabel("Nothing reported")
	}

	form := widget.NewForm()
	for _, f := range fields {
		value := widget.NewLabel(f.value)
		value.Wrapping = fyne.TextWrapWord
		value.Selectable = true
		form.Append(f.label, value)
	}
	return form
}

// newInterfacesTable renders one row per interface: name, IPv4, MAC and link state
func newInterfacesTable(rows []interfaceRow) fyne.CanvasObject {
	grid := container.NewGridWithColumns(4)
	for _, h := range []string{"Interface", "IPv4", "MAC", "State"} {
		grid.Add(widget.NewLabelWithStyle(h, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	}

	cell := func(text string) *widget.Label {
		if text == "" {
			text = "-"
		}
		l := widget.NewLabel(text)
		l.Selectable = true
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}

	for _, r := range rows {
		grid.Add(cell(r.name))
		grid.Add(cell(r.ipv4))
		grid.Add(cell(r.mac))
		grid.Add(cell(r.state))
	}
	return grid
}

// infoField is a single label/value pair in the device info view
type infoField struct {
	label string
	value string
}

// appendField appends the field unless its value is empty, so the view only shows what the
// device actually reports.
func appendField(fields []infoField, label, value string) []infoField {
	value = strings.TrimSpace(value)
	if value == "" {
		return fields
	}
	return append(fields, infoField{label, value})
}

// systemFields returns the machine related fields of the device inventory
func systemFields(inv *client.DeviceInventory) []infoField {
	s := inv.SystemInfo
	var f []infoField

	f = appendField(f, "Operating system", s.OS)
	f = appendField(f, "OS type", s.OSType)
	f = appendField(f, "Flavor", s.Flavor)
	f = appendField(f, "Architecture", s.Architecture)
	f = appendField(f, "Kernel release", s.Release)
	f = appendField(f, "Kernel version", s.Version)
	f = appendField(f, "Number of CPUs", s.CPUs)
	f = appendField(f, "CPU hardware", s.CPUHardware)
	f = appendField(f, "CPU revision", s.CPURevision)
	f = appendField(f, "CPU serial number", s.CPUSerialNumber)
	f = appendField(f, "Boot time", formatUnixString(s.BootTime))
	f = appendField(f, "Agent version", s.AgentVersion)
	// Only show TPM state when the agent reported, otherwise "disabled" could be misleading
	if s.AgentVersion != "" {
		tpm := "disabled"
		if s.TPMEnabled {
			tpm = "enabled"
		}
		f = appendField(f, "TPM", tpm)
	}
	f = appendField(f, "Last config update", formatTimestamp(s.LastConfigUpdate))
	f = appendField(f, "Description", inv.Attributes.Description)
	f = appendField(f, "Location", formatLocation(inv.Attributes))

	return f
}

// networkFields returns the network related summary fields of the device inventory
func networkFields(inv *client.DeviceInventory) []infoField {
	s := inv.SystemInfo
	var f []infoField

	host := s.FQHost
	if host == "" {
		host = s.Host
	}
	f = appendField(f, "Hostname", host)
	f = appendField(f, "Default interface", s.Interface)
	f = appendField(f, "Primary IPv4", s.IPv4First)
	f = appendField(f, "All IP addresses", strings.Join(strings.Fields(s.IPAddresses), ", "))
	f = appendField(f, "Remote address", s.RemoteAddress)

	return f
}

// interfaceRow holds the information about one network interface
type interfaceRow struct {
	name  string
	ipv4  string
	mac   string
	state string
}

// interfaceRows merges the per-interface maps of the inventory into one row per interface,
// physical interfaces first, each group sorted by name.
func interfaceRows(s client.SystemInfo) []interfaceRow {
	names := map[string]bool{}
	for name := range s.IPv4 {
		names[name] = true
	}
	for name := range s.InterfaceFlags {
		names[name] = true
	}

	// The agent reports MAC addresses with "." in interface names replaced by "_"
	// (e.g. wlan0.1 => wlan0_1), so map them back to the real interface name.
	macs := map[string]string{}
	usedMACKeys := map[string]bool{}
	for name := range names {
		for _, key := range []string{name, strings.ReplaceAll(name, ".", "_")} {
			if mac, ok := s.HardwareMAC[key]; ok {
				macs[name] = mac
				usedMACKeys[key] = true
				break
			}
		}
	}
	// Interfaces that only report a MAC address (e.g. no IP and no flags)
	for key, mac := range s.HardwareMAC {
		if !usedMACKeys[key] {
			names[key] = true
			macs[key] = mac
		}
	}

	rows := make([]interfaceRow, 0, len(names))
	for name := range names {
		rows = append(rows, interfaceRow{
			name:  name,
			ipv4:  strings.TrimSpace(s.IPv4[name]),
			mac:   strings.TrimSpace(macs[name]),
			state: interfaceState(s.InterfaceFlags[name]),
		})
	}

	slices.SortFunc(rows, func(a, b interfaceRow) int {
		av, bv := isVirtualInterface(a.name), isVirtualInterface(b.name)
		if av != bv {
			if av {
				return 1
			}
			return -1
		}
		return strings.Compare(a.name, b.name)
	})
	return rows
}

// interfaceState condenses the interface flags into "up", "up, running" or "down".
func interfaceState(flags string) string {
	fields := strings.Fields(flags)
	if len(fields) == 0 {
		return ""
	}
	if !slices.Contains(fields, "up") {
		return "down"
	}
	if slices.Contains(fields, "running") {
		return "up, running"
	}
	return "up"
}

// formatLocation joins the configured address attributes into a single line
func formatLocation(a client.DeviceAttributes) string {
	cityLine := strings.TrimSpace(a.Zip + " " + a.City)
	var parts []string
	for _, p := range []string{a.Address, cityLine, a.Country} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

// formatUnixString formats a unix timestamp given as a string; empty or invalid input
// results in an empty string.
func formatUnixString(s string) string {
	ts, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || ts <= 0 {
		return ""
	}
	return formatLastReported(ts)
}

// formatTimestamp formats a timestamp, returning an empty string if it isn't set
func formatTimestamp(ts types.Timestamp) string {
	if ts <= 0 {
		return ""
	}
	return formatLastReported(int64(ts))
}

// formatDeviceInfo renders the device info as plain text, e.g. for pasting into a support ticket
func formatDeviceInfo(inv *client.DeviceInventory) string {
	var b strings.Builder

	writeSection := func(title string, fields []infoField) {
		fmt.Fprintf(&b, "%s\n", title)
		for _, f := range fields {
			fmt.Fprintf(&b, "  %s: %s\n", f.label, f.value)
		}
		b.WriteString("\n")
	}

	writeSection("System", systemFields(inv))
	writeSection("Network", networkFields(inv))

	if rows := interfaceRows(inv.SystemInfo); len(rows) > 0 {
		b.WriteString("Interfaces\n")
		for _, r := range rows {
			fmt.Fprintf(&b, "  %s: ipv4=%s mac=%s state=%s\n",
				r.name, orDash(r.ipv4), orDash(r.mac), orDash(r.state))
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
