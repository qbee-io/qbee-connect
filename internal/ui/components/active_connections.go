package components

import (
	"fmt"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/connect/internal/service"
	"go.qbee.io/connect/internal/ui/widgets"
)

// activeConnectionsDelegate defines the interface for accessing the ConnectionStore
type activeConnectionsDelegate interface {
	GetStore() *service.ConnectionStore
	GetPreferences() *service.PreferencesStore
}

// NewActiveConnections creates a new ActiveConnections view
func NewActiveConnections(a activeConnectionsDelegate) *fyne.Container {
	root := container.NewVBox()

	UpdateActiveConnectionsView(a, root)
	return root
}

func UpdateActiveConnectionsView(a activeConnectionsDelegate, activeView fyne.CanvasObject) {

	root, ok := activeView.(*fyne.Container)
	if !ok {
		return
	}

	root.RemoveAll()
	active := a.GetStore().SnapshotActive()

	if len(active) == 0 {
		root.Add(widget.NewLabel("No active connections."))
		root.Refresh()
		return
	}

	ids := make([]string, 0, len(active))
	for id := range active {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	sshUser := a.GetPreferences().Get().DefaultSSHUserName

	for _, id := range ids {
		conn := active[id]
		md := "# " + conn.Title + "\n**Device ID:** " + id + "\n\n"
		for _, t := range conn.Targets {
			md += fmt.Sprintf("- **%s**: %s:%s => %s:%s", t.Protocol, t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort)
			if t.Protocol != "udp" && t.RemotePort == serviceSSHPort {
				md += fmt.Sprintf(" (`%s`)", renderSSHCommand(t, sshUser))
			}
			md += "\n"
		}
		mdWidget := widget.NewRichTextFromMarkdown(md)

		devID := id
		btn := widgets.NewButtonPointer("Disconnect", func() {
			activeConn, ok := a.GetStore().GetActive(devID)
			if ok && activeConn.Cancel != nil {
				activeConn.Cancel()
			}
		})

		row := container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), btn), nil, nil, mdWidget)
		root.Add(row)
	}

	fyne.Do(func() {
		root.Refresh()
	})
}
