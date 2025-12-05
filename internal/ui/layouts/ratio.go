package layouts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/connect/internal/model"
)

// WindowPadding defines the padding to subtract from total width for layout calculations
const WindowPadding = 25

// RatioLayout is a custom layout that sets column widths based on predefined ratios
type RatioLayout struct {
	Table *widget.Table
}

// Layout arranges the objects within the given size
func (r *RatioLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	r.Table.Resize(size)
	r.Table.Move(fyne.NewPos(0, 0))

	for i := range model.DeviceColumns {
		newColWidth := (size.Width - WindowPadding) * model.DeviceColumns[i].WidthQuotient
		r.Table.SetColumnWidth(i, newColWidth)
	}
}

// MinSize returns the minimum size for the layout
func (r *RatioLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(100, 100)
}
