//go:build darwin

package desktop

import "golang.design/x/hotkey"

// modCmd is the Command key on macOS.
var modCmd = hotkey.ModCmd

// modAlt is the Option key on macOS.
var modAlt = hotkey.ModOption
