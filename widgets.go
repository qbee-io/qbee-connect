package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// LabelHover is a custom widget that implements desktop.Hoverable
type LabelHover struct {
	widget.Label
	isHovered bool
	popup     *widget.PopUp
}

// NewLabelHover creates a new instance of LabelHover
func NewLabelHover(text string) *LabelHover {
	w := &LabelHover{}
	w.ExtendBaseWidget(w)
	w.SetText(text)
	return w
}

// MouseIn is called when the mouse enters the widget
func (w *LabelHover) MouseIn(*desktop.MouseEvent) {

	w.isHovered = true

	// wait a moment to avoid flickering

	// show a tooltip
	// Create and show tooltip
	if w.Text != "" {
		tooltip := widget.NewCard("", w.Text, nil)
		popup := widget.NewPopUp(tooltip, fyne.CurrentApp().Driver().CanvasForObject(w))
		position := fyne.CurrentApp().Driver().AbsolutePositionForObject(w)
		// show the popup just above the widget
		position.Y -= tooltip.MinSize().Height + 5

		// Store popup reference to hide it on MouseOut
		w.popup = popup

		time.AfterFunc(200*time.Millisecond, func() {
			if w.isHovered {
				popup.ShowAtPosition(position)
			}
		})
	}

	w.Refresh()
}

// MouseOut is called when the mouse exits the widget
func (w *LabelHover) MouseOut() {
	w.isHovered = false
	// hide tooltip
	if w.popup != nil {
		w.popup.Hide()
		w.popup = nil
	}
	w.Refresh()
}

// MouseMoved is called when the mouse moves within the widget
func (w *LabelHover) MouseMoved(*desktop.MouseEvent) {
	// You can implement custom logic here if needed
}
