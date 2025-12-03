package layouts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/connect/internal/model"
)

const WindowPadding = 25

type RatioLayout struct {
	Table *widget.Table
}

func (r *RatioLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	r.Table.Resize(size)
	r.Table.Move(fyne.NewPos(0, 0))

	for i := range model.DeviceColumns {
		newColWidth := (size.Width - WindowPadding) * model.DeviceColumns[i].WidthQuotient
		r.Table.SetColumnWidth(i, newColWidth)
	}
}

func (r *RatioLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(100, 100)
}
