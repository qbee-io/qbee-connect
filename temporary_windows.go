//go:build windows

package main

import (
	"fmt"
	"runtime"

	"go.qbee.io/client"
)

// Temporary terminal login function for testing purposes until login with backends is implemented.
// Not support on Windows.
func terminalLogin() (*client.Client, error) {
	return nil, fmt.Errorf("not implemented on %s", runtime.GOOS)
}
