package main

import (
	_ "embed"

	"go.qbee.io/connect/internal/ui"
)

//go:embed Icon.png
var icon []byte

func main() {
	ui.NewApp(icon).Run()
}
