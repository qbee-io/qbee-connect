package widgets

import (
	"time"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type LabelHover struct {
	widget.Label
	isHovered bool
	popup     *widget.PopUp
}

func NewLabelHover(text string) *LabelHover {
	w := &LabelHover{}
	w.ExtendBaseWidget(w)
	w.SetText(text)
	return w
}

func (w *LabelHover) MouseIn(*desktop.MouseEvent) {
	w.isHovered = true
	if w.Text != "" {
		tooltip := widget.NewCard("", w.Text, nil)
		popup := widget.NewPopUp(tooltip, fyne.CurrentApp().Driver().CanvasForObject(w))
		position := fyne.CurrentApp().Driver().AbsolutePositionForObject(w)
		position.Y -= tooltip.MinSize().Height + 5

		w.popup = popup
		time.AfterFunc(200*time.Millisecond, func() {
			if w.isHovered {
				popup.ShowAtPosition(position)
			}
		})
	}
	w.Refresh()
}

func (w *LabelHover) MouseOut() {
	w.isHovered = false
	if w.popup != nil {
		w.popup.Hide()
		w.popup = nil
	}
	w.Refresh()
}

func (w *LabelHover) MouseMoved(*desktop.MouseEvent) {}
