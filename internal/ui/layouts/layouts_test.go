package layouts

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

var _ fyne.Tappable = (*blocker)(nil)

func TestBlocker_ConsumesTaps(t *testing.T) {
	test.NewTempApp(t)

	tapped := false
	underneath := widget.NewButton("below", func() { tapped = true })
	b := NewBlocker()

	// Stack the blocker on top of the button, as used for loading overlays.
	w := test.NewTempWindow(t, container.NewStack(underneath, b))
	w.Resize(fyne.NewSize(200, 100))

	test.TapCanvas(w.Canvas(), fyne.NewPos(50, 30))

	if tapped {
		t.Fatal("tap passed through the blocker to the widget underneath")
	}
}

func TestBlocker_RendersTransparentRectangle(t *testing.T) {
	test.NewTempApp(t)

	b := NewBlocker()
	r := test.TempWidgetRenderer(t, b)

	objs := r.Objects()
	if len(objs) != 1 {
		t.Fatalf("got %d renderer objects, want 1", len(objs))
	}
	rect, ok := objs[0].(*canvas.Rectangle)
	if !ok {
		t.Fatalf("renderer object is %T, want *canvas.Rectangle", objs[0])
	}
	if _, _, _, a := rect.FillColor.RGBA(); a != 0 {
		t.Fatalf("blocker fill alpha = %d, want transparent", a)
	}
}

func TestRatioLayout_ResizesTableToContainer(t *testing.T) {
	test.NewTempApp(t)

	table := widget.NewTable(
		func() (int, int) { return 1, 6 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(widget.TableCellID, fyne.CanvasObject) {},
	)
	c := container.New(&RatioLayout{Table: table}, table)
	w := test.NewTempWindow(t, c)
	w.Resize(fyne.NewSize(800, 400))

	size := c.Size()
	if table.Size() != size {
		t.Fatalf("table size = %v, want %v", table.Size(), size)
	}
	if table.Position() != fyne.NewPos(0, 0) {
		t.Fatalf("table position = %v, want origin", table.Position())
	}
}

func TestRatioLayout_MinSize(t *testing.T) {
	l := &RatioLayout{}
	if got := l.MinSize(nil); got != fyne.NewSize(100, 100) {
		t.Fatalf("MinSize = %v, want 100x100", got)
	}
}
